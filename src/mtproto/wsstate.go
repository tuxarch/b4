package mtproto

import (
	"strconv"
	"sync"
	"time"

	"github.com/daniellavrushin/b4/log"
)

const (
	wsDCFailCooldown      = 30 * time.Second
	wsDialTimeoutCooldown = 2 * time.Second
	tcpFailCooldown       = 30 * time.Second
)

type wsKey struct {
	dc      int
	isMedia bool
}

func (k wsKey) String() string {
	s := strconv.Itoa(k.dc)
	if k.isMedia {
		s += "m"
	}
	return s
}

var (
	wsStateMu    sync.Mutex
	wsBlacklist  = map[wsKey]bool{}
	wsCooldownTo = map[wsKey]time.Time{}

	tcpStateMu    sync.Mutex
	tcpCooldownTo = map[string]time.Time{} // keyed by host:port

	dialLogMu sync.Mutex
	dialLogAt = map[int]time.Time{} // per-DC last full ERROR emit; throttles spam from known-broken DCs
)

const dialLogInterval = 60 * time.Second

// shouldLogDialError returns true if this is the first error for `dc` in the
// last dialLogInterval. Subsequent identical failures are silenced (caller can
// log at Debug instead) so a permanently-broken DC doesn't spam errors.log.
func shouldLogDialError(dc int) bool {
	dialLogMu.Lock()
	defer dialLogMu.Unlock()
	now := time.Now()
	if last, ok := dialLogAt[dc]; ok && now.Sub(last) < dialLogInterval {
		return false
	}
	dialLogAt[dc] = now
	return true
}

// per-addr TCP cooldown: skip an upstream IP/port that just timed out so
// every retrying client doesn't burn another tcpDialTimeout against it.
func tcpAddrInCooldown(addr string) bool {
	tcpStateMu.Lock()
	defer tcpStateMu.Unlock()
	t, ok := tcpCooldownTo[addr]
	if !ok {
		return false
	}
	if time.Now().After(t) {
		delete(tcpCooldownTo, addr)
		return false
	}
	return true
}

func tcpRecordFailure(addr string) {
	tcpStateMu.Lock()
	defer tcpStateMu.Unlock()
	tcpCooldownTo[addr] = time.Now().Add(tcpFailCooldown)
}

func tcpRecordSuccess(addr string) {
	tcpStateMu.Lock()
	defer tcpStateMu.Unlock()
	delete(tcpCooldownTo, addr)
}

func tcpResetState() {
	tcpStateMu.Lock()
	defer tcpStateMu.Unlock()
	tcpCooldownTo = map[string]time.Time{}
}

func wsKeyFromDC(dc int) wsKey {
	abs := dc
	if abs < 0 {
		abs = -abs
	}
	return wsKey{dc: abs, isMedia: dc < 0}
}

func wsIsBlacklisted(dc int) bool {
	k := wsKeyFromDC(dc)
	wsStateMu.Lock()
	defer wsStateMu.Unlock()
	return wsBlacklist[k]
}

func wsCooldownActive(dc int) bool {
	k := wsKeyFromDC(dc)
	wsStateMu.Lock()
	defer wsStateMu.Unlock()
	t, ok := wsCooldownTo[k]
	if !ok {
		return false
	}
	if time.Now().After(t) {
		delete(wsCooldownTo, k)
		return false
	}
	return true
}

func wsRecordFailure(dc int, allRedirect bool) {
	k := wsKeyFromDC(dc)
	wsStateMu.Lock()
	defer wsStateMu.Unlock()
	if allRedirect {
		wsBlacklist[k] = true
		log.Warnf("MTProto WS %s blacklisted (all redirects)", k)
	}
	wsCooldownTo[k] = time.Now().Add(wsDCFailCooldown)
}

func wsRecordSuccess(dc int) {
	k := wsKeyFromDC(dc)
	wsStateMu.Lock()
	defer wsStateMu.Unlock()
	delete(wsCooldownTo, k)
	delete(wsBlacklist, k)
}

func wsResetState() {
	wsStateMu.Lock()
	defer wsStateMu.Unlock()
	wsBlacklist = map[wsKey]bool{}
	wsCooldownTo = map[wsKey]time.Time{}
}
