package nfq

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/daniellavrushin/b4/capture"
	"github.com/daniellavrushin/b4/config"
	"github.com/daniellavrushin/b4/engine"
	"github.com/daniellavrushin/b4/log"
	"github.com/daniellavrushin/b4/metrics"
	"github.com/daniellavrushin/b4/quic"
	"github.com/daniellavrushin/b4/sni"
	"github.com/daniellavrushin/b4/sock"
	"github.com/daniellavrushin/b4/stun"
	"github.com/daniellavrushin/b4/utils"
)

// ProcessPacket is the engine-agnostic packet processing logic.
// It takes a raw IP packet and returns a verdict indicating whether the
// original packet should be accepted (forwarded unchanged) or dropped
// (modified copies already sent via raw socket).
func (w *Worker) ProcessPacket(raw []byte) engine.PacketVerdict {
	if len(raw) == 0 {
		return engine.VerdictAccept
	}

	cfg := w.getConfig()
	var set *config.SetConfig

	matcher := w.getMatcher()

	atomic.AddUint64(&w.packetsProcessed, 1)

	v := raw[0] >> 4
	if v != IPv4 && v != IPv6 {
		return engine.VerdictAccept
	}
	var proto uint8
	var src, dst net.IP
	var ihl int
	if v == IPv4 {
		if len(raw) < 20 {
			return engine.VerdictAccept
		}
		ihl = int(raw[0]&0x0f) * 4
		if len(raw) < ihl {
			return engine.VerdictAccept
		}

		fragOffset := binary.BigEndian.Uint16(raw[6:8]) & 0x1FFF
		moreFragments := (binary.BigEndian.Uint16(raw[6:8]) & 0x2000) != 0

		if fragOffset != 0 || moreFragments {
			return engine.VerdictAccept
		}

		proto = raw[9]
		src = net.IP(raw[12:16])
		dst = net.IP(raw[16:20])

	} else {
		if len(raw) < IPv6HeaderLen {
			return engine.VerdictAccept
		}
		ihl = IPv6HeaderLen
		nextHeader := raw[6]
		offset := 40

		for {
			switch nextHeader {
			case 0, 43, 60:
				if len(raw) < offset+2 {
					return engine.VerdictAccept
				}
				nextHeader = raw[offset]
				hdrLen := int(raw[offset+1])*8 + 8
				offset += hdrLen
			case 44:
				return engine.VerdictAccept
			default:
				goto done
			}
		}
	done:
		proto = nextHeader
		ihl = offset
		src = net.IP(raw[8:24])
		dst = net.IP(raw[24:40])
	}

	if src.IsLoopback() || dst.IsLoopback() {
		return engine.VerdictAccept
	}
	srcStr := src.String()
	dstStr := dst.String()

	srcMac := w.getMacByIp(srcStr)

	matched, st := matcher.MatchIPWithSource(dst, srcMac)
	if matched {
		set = st
	}

	if proto == 6 && len(raw) >= ihl+TCPHeaderMinLen {
		tcp := raw[ihl:]
		if len(tcp) < TCPHeaderMinLen {
			return engine.VerdictAccept
		}
		datOff := int((tcp[12]>>4)&0x0f) * 4
		if len(tcp) < datOff {
			return engine.VerdictAccept
		}
		payload := tcp[datOff:]
		sport := binary.BigEndian.Uint16(tcp[0:2])
		dport := binary.BigEndian.Uint16(tcp[2:4])

		if cfg.IsTCPPort(sport) {
			return w.HandleIncoming(v, raw, ihl, src, dstStr, dport, srcStr, sport, payload)
		}

		// If IP matched but set has a port filter, verify port matches (AND logic)
		if matched && !set.MatchesTCPDPort(dport) {
			matched = false
			set = nil
		}

		// If IP matching didn't find a set, try TCP port-based set matching
		if !matched && cfg.IsTCPPort(dport) {
			if portMatched, portSet := matcher.MatchTCPPort(dport); portMatched {
				matched = true
				set = portSet
			}
		}

		// Packet duplication path: duplicate ALL outgoing TCP packets on configured ports
		// without TLS/SNI parsing. Bypasses DPI evasion entirely.
		if matched && cfg.IsTCPPort(dport) && set.TCP.Duplicate.Enabled && set.TCP.Duplicate.Count > 0 {
			log.Tracef("TCP duplicate to %s:%d (%d copies, set: %s)", dstStr, dport, set.TCP.Duplicate.Count, set.Name)

			m := metrics.GetMetricsCollector()
			m.RecordConnection("TCP-DUP", "", srcStr, dstStr, true, srcMac, set.Name)
			m.RecordPacket(uint64(len(raw)))

			if !log.IsDiscoveryActive() {
				log.Infof(",TCP-DUP,,,%s:%d,%s,%s:%d,%s", srcStr, sport, set.Name, dstStr, dport, srcMac)
			}

			for i := 0; i < set.TCP.Duplicate.Count; i++ {
				if v == IPv4 {
					_ = w.sock.SendIPv4(raw, dst)
				} else {
					_ = w.sock.SendIPv6(raw, dst)
				}
			}
			return engine.VerdictDrop
		}

		tcpFlags := tcp[13]
		isSyn := (tcpFlags & 0x02) != 0
		isAck := (tcpFlags & 0x10) != 0
		isRst := (tcpFlags & 0x04) != 0
		if isRst && cfg.IsTCPPort(dport) {
			log.Tracef("RST received from %s:%d", dstStr, dport)
		}

		if isSyn && !isAck && cfg.IsTCPPort(dport) && matched && !set.TCP.Duplicate.Enabled {
			log.Tracef("TCP SYN to %s:%d (set: %s)", dstStr, dport, set.Name)

			m := metrics.GetMetricsCollector()
			m.RecordConnection("TCP-SYN", "", srcStr, dstStr, true, srcMac, set.Name)

			if v == IPv4 {
				modsyn := raw

				if set.TCP.SynFake {
					w.sendFakeSyn(set, raw, ihl, datOff)
				}

				if set.Fragmentation.Strategy != config.ConfigNone && set.Faking.TCPMD5 {
					w.sendFakeSynWithMD5(set, raw, ihl, dst)
				}

				_ = w.sock.SendIPv4(modsyn, dst)
			} else {
				if set.TCP.SynFake {
					w.sendFakeSynV6(set, raw, ihl, datOff)
				}

				if set.Fragmentation.Strategy != config.ConfigNone && set.Faking.TCPMD5 {
					w.sendFakeSynWithMD5V6(set, raw, dst)
				}

				_ = w.sock.SendIPv6(raw, dst)
			}

			return engine.VerdictDrop
		}

		host := ""
		matchedIP := st != nil
		matchedSNI := false
		ipTarget := ""
		sniTarget := ""

		// Show port-matched set name in log
		if !matchedIP && matched && set != nil {
			ipTarget = set.Name
		}

		if cfg.IsTCPPort(dport) && len(payload) > 0 {
			log.Tracef("TCP payload to %s: len=%d, first5=%x", dstStr, len(payload), payload[:min(5, len(payload))])
			if len(payload) >= 5 && payload[0] == 0x16 {
				log.Tracef("TLS record: type=%x ver=%x%x len=%d", payload[0], payload[1], payload[2],
					int(payload[3])<<8|int(payload[4]))
			}
			connKey := fmt.Sprintf(connKeyFormat, srcStr, sport, dstStr, dport)

			host, _ = sni.ParseTLSClientHelloSNI(payload)

			if captureManager := capture.GetManager(cfg); captureManager != nil {
				captureManager.CapturePayload(connKey, host, "tls", payload)
			}

			if host != "" {
				if mSNI, stSNI := matcher.MatchSNIWithSource(host, srcMac); mSNI {
					// If SNI-matched set has a port filter, verify port matches (AND logic)
					if stSNI.MatchesTCPDPort(dport) {
						matchedSNI = true
						matched = true
						set = stSNI
						matcher.LearnIPToDomain(dst, host, stSNI)
					}
				}
			}
		}

		if matchedIP {
			ipTarget = st.Name
		}
		if matchedSNI {
			sniTarget = set.Name
		}

		if !log.IsDiscoveryActive() {
			log.Infof(",TCP,%s,%s,%s:%d,%s,%s:%d,%s", sniTarget, host, srcStr, sport, ipTarget, dstStr, dport, srcMac)
		}

		{
			m := metrics.GetMetricsCollector()
			setName := ""
			if matched {
				setName = set.Name
			}
			m.RecordConnection("TCP", host, srcStr, dstStr, matched, srcMac, setName)
			m.RecordPacket(uint64(len(raw)))
		}

		if matched {
			if set.TCP.Incoming.Mode != config.ConfigOff {
				connKey := fmt.Sprintf(connKeyFormat, srcStr, sport, dstStr, dport)
				connState.RegisterOutgoing(connKey, set)
			}

			packetCopy := make([]byte, len(raw))
			copy(packetCopy, raw)

			if set.TCP.DropSACK {
				if v == 4 {
					packetCopy = sock.StripSACKFromTCP(packetCopy)
				} else {
					packetCopy = sock.StripSACKFromTCPv6(packetCopy)
				}
			}

			dstCopy := make(net.IP, len(dst))
			copy(dstCopy, dst)
			setCopy := set

			w.wg.Add(1)
			go func(s *config.SetConfig, pkt []byte, d net.IP) {
				defer w.wg.Done()
				if v == 4 {
					w.dropAndInjectTCP(s, pkt, d)
				} else {
					w.dropAndInjectTCPv6(s, pkt, d)
				}
			}(setCopy, packetCopy, dstCopy)
			return engine.VerdictDrop
		}

		return engine.VerdictAccept
	}

	if proto == 17 && len(raw) >= ihl+8 {
		udp := raw[ihl:]
		if len(udp) < 8 {
			return engine.VerdictAccept
		}

		payload := udp[8:]
		sport := binary.BigEndian.Uint16(udp[0:2])
		dport := binary.BigEndian.Uint16(udp[2:4])
		connKey := fmt.Sprintf(connKeyFormat, srcStr, sport, dstStr, dport)

		if sport == 53 || dport == 53 {
			return w.processDnsPacket(v, sport, dport, payload, raw, ihl, srcMac)
		}

		if utils.IsPrivateIP(dst) {
			return engine.VerdictAccept
		}

		matchedIP := st != nil
		matchedQUIC := false
		isSTUN := false
		host := ""
		ipTarget := ""
		sniTarget := ""

		// If IP matched but set has a port filter, verify port matches (AND logic)
		if matchedIP && !st.MatchesUDPDPort(dport) {
			matchedIP = false
			matched = false
			set = nil
		}

		if matchedIP {
			ipTarget = st.Name
		}

		if !matchedIP {
			if mLearned, learnedSet, learnedDomain := matcher.MatchLearnedIPWithSource(dst, srcMac); mLearned {
				// If learned IP set has a port filter, verify port matches (AND logic)
				if learnedSet.MatchesUDPDPort(dport) {
					matchedIP = true
					matched = true
					set = learnedSet
					host = learnedDomain
					sniTarget = learnedSet.Name
					ipTarget = learnedSet.Name
				}
			}
		}

		// If IP matching didn't find a set, try UDP port-based set matching
		matchedPort := false
		if !matched {
			if portMatched, portSet := matcher.MatchUDPPort(dport); portMatched {
				matchedPort = true
				matched = true
				set = portSet
				ipTarget = portSet.Name
			}
		}

		isSTUN = stun.IsSTUNMessage(payload)

		if host == "" {
			if h, ok := sni.ParseQUICClientHelloSNI(payload); ok {
				host = h
			}
		}

		if host != "" {
			if mSNI, sniSet := matcher.MatchSNIWithSource(host, srcMac); mSNI {
				// If SNI-matched set has a port filter, verify port matches (AND logic)
				if sniSet.MatchesUDPDPort(dport) {
					matchedQUIC = true
					set = sniSet
					sniTarget = sniSet.Name
					matcher.LearnIPToDomain(dst, host, sniSet)
				}
			}
		}

		if !matchedQUIC && (matchedIP || matchedPort) && set.UDP.FilterQUIC == "all" {
			if quic.IsInitial(payload) {
				matchedQUIC = true
			}
		}

		if captureManager := capture.GetManager(cfg); captureManager != nil {
			captureManager.CapturePayload(connKey, host, "quic", payload)
		}

		shouldHandle := (matchedIP || matchedQUIC || matchedPort) && !(isSTUN && set.UDP.FilterSTUN)

		matched = shouldHandle

		if !log.IsDiscoveryActive() {
			log.Infof(",UDP,%s,%s,%s:%d,%s,%s:%d,%s", sniTarget, host, srcStr, sport, ipTarget, dstStr, dport, srcMac)
		}

		if isSTUN && set != nil && set.UDP.FilterSTUN {
			return engine.VerdictAccept
		}

		if !shouldHandle {
			m := metrics.GetMetricsCollector()
			m.RecordConnection("UDP", host, srcStr, dstStr, false, srcMac, "")
			m.RecordPacket(uint64(len(raw)))
			return engine.VerdictAccept
		}

		m := metrics.GetMetricsCollector()
		setName := ""
		if matched {
			setName = set.Name
		}
		m.RecordConnection("UDP", host, srcStr, dstStr, matched, srcMac, setName)
		m.RecordPacket(uint64(len(raw)))

		switch set.UDP.Mode {
		case "drop":
			return engine.VerdictDrop

		case "fake":
			packetCopy := make([]byte, len(raw))
			copy(packetCopy, raw)
			dstCopy := make(net.IP, len(dst))
			copy(dstCopy, dst)
			setCopy := set

			w.wg.Add(1)
			go func(s *config.SetConfig, pkt []byte, d net.IP) {
				defer w.wg.Done()
				if v == IPv4 {
					w.dropAndInjectQUIC(s, pkt, d)
				} else {
					w.dropAndInjectQUICV6(s, pkt, d)
				}
			}(setCopy, packetCopy, dstCopy)
			return engine.VerdictDrop

		default:
			return engine.VerdictAccept
		}
	}

	return engine.VerdictAccept
}
