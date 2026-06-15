package tun

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/daniellavrushin/b4/log"
)

type routeManager struct {
	tunName       string
	tunAddr       string
	tunAddrV6     string
	outIface      string
	outGateway    string
	mark          uint
	routeTable    int
	routes        []string
	savedDefault  string
	savedRPFilter string
}

func (r *routeManager) selective() bool {
	return len(r.routes) > 0
}

func (r *routeManager) setup() error {
	out, err := run("ip", "route", "show", "default")
	if err != nil {
		return fmt.Errorf("failed to read current default route: %w", err)
	}
	r.savedDefault = strings.TrimSpace(out)
	log.Infof("TUN: saved default route: %s", r.savedDefault)

	if r.outGateway == "" {
		r.outGateway = extractGateway(r.savedDefault)
		if r.outGateway != "" {
			log.Infof("TUN: auto-detected gateway: %s", r.outGateway)
		} else {
			log.Infof("TUN: no gateway on default route, treating %s as point-to-point", r.outIface)
		}
	}

	// Source IP for router-originated traffic: without it the kernel picks the
	// TUN address as source and replies can't get back. Use the uplink's IP.
	srcIP := extractField(r.savedDefault, "src")
	if srcIP == "" {
		srcIP = interfacePrimaryIPv4(r.outIface)
	}

	if _, err := run("ip", "addr", "add", r.tunAddr, "dev", r.tunName); err != nil {
		return fmt.Errorf("ip addr add: %w", err)
	}
	if r.tunAddrV6 != "" {
		if _, err := run("ip", "-6", "addr", "add", r.tunAddrV6, "dev", r.tunName); err != nil {
			log.Warnf("TUN: failed to add IPv6 address: %v", err)
		}
	}
	if _, err := run("ip", "link", "set", r.tunName, "up"); err != nil {
		return fmt.Errorf("ip link set up: %w", err)
	}
	if _, err := run("ip", "link", "set", r.tunName, "mtu", "1500"); err != nil {
		log.Warnf("TUN: failed to set MTU: %v", err)
	}

	r.loosenRPFilter()

	if r.selective() {
		return r.setupSelective(srcIP)
	}
	return r.setupDefaultCapture(srcIP)
}

func rpFilterPath(iface string) string {
	return "/proc/sys/net/ipv4/conf/" + iface + "/rp_filter"
}

// loosenRPFilter switches the uplink to loose reverse-path filtering. With
// targets routed via the TUN, their replies arrive on the uplink whose reverse
// route points at the TUN - strict rp_filter (1) silently drops them. Loose
// mode (2) only requires the source be reachable via any interface.
func (r *routeManager) loosenRPFilter() {
	path := rpFilterPath(r.outIface)
	cur, err := os.ReadFile(path)
	if err != nil {
		log.Warnf("TUN: cannot read rp_filter for %s: %v", r.outIface, err)
		return
	}
	r.savedRPFilter = strings.TrimSpace(string(cur))
	if r.savedRPFilter == "2" {
		return
	}
	if err := os.WriteFile(path, []byte("2\n"), 0644); err != nil {
		log.Warnf("TUN: failed to set %s rp_filter=2 (loose): %v", r.outIface, err)
		r.savedRPFilter = ""
		return
	}
	log.Infof("TUN: set %s rp_filter=2 (loose) for asymmetric routing (was %s)", r.outIface, r.savedRPFilter)
}

func (r *routeManager) restoreRPFilter() {
	if r.savedRPFilter == "" {
		return
	}
	if err := os.WriteFile(rpFilterPath(r.outIface), []byte(r.savedRPFilter+"\n"), 0644); err != nil {
		log.Warnf("TUN: failed to restore %s rp_filter: %v", r.outIface, err)
	}
}

// setupBypassTable installs the fwmark policy-routing table that carries b4's
// own marked re-injected packets out the uplink. It contains only a default via
// the uplink (no TUN routes), so re-injected packets reach the internet without
// being pulled back into the TUN - in both selective and default-capture modes.
func (r *routeManager) setupBypassTable() error {
	tableStr := fmt.Sprintf("%d", r.routeTable)

	// Refuse to clobber a table that is already in use (e.g. on ASUS Merlin
	// table ids 100/200 are aliased to wan0/wan1 system tables).
	if existing, _ := run("ip", "route", "show", "table", tableStr); strings.TrimSpace(existing) != "" {
		return fmt.Errorf("route table %d is already in use (likely a system table; see /etc/iproute2/rt_tables) - set queue.tun.route_table to an unused id", r.routeTable)
	}

	markStr := fmt.Sprintf("0x%x", r.mark)
	run("ip", "rule", "del", "fwmark", markStr, "lookup", tableStr)

	if _, err := run("ip", "rule", "add", "fwmark", markStr, "lookup", tableStr, "priority", "100"); err != nil {
		return fmt.Errorf("ip rule add: %w", err)
	}
	return r.addBypassDefault(tableStr)
}

// setupSelective routes only the configured prefixes into the TUN, leaving the
// system default route untouched.
func (r *routeManager) setupSelective(srcIP string) error {
	if err := r.setupBypassTable(); err != nil {
		return err
	}

	added := 0
	for _, p := range r.routes {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		args := []string{"ip", "route", "add", p, "dev", r.tunName}
		if srcIP != "" && strings.Count(p, ":") == 0 {
			args = append(args, "src", srcIP)
		}
		if _, err := run(args...); err != nil {
			log.Warnf("TUN: failed to add selective route %s: %v", p, err)
			continue
		}
		added++
	}
	if added == 0 {
		return fmt.Errorf("selective routing: no routes could be added")
	}
	log.Infof("TUN: selective routing %d/%d prefixes into %s (default route untouched, mark=0x%x table=%d)",
		added, len(r.routes), r.tunName, r.mark, r.routeTable)
	return nil
}

// setupDefaultCapture replaces the whole default route with the TUN.
func (r *routeManager) setupDefaultCapture(srcIP string) error {
	if err := r.setupBypassTable(); err != nil {
		return err
	}

	replaceArgs := []string{"ip", "route", "replace", "default", "dev", r.tunName}
	if srcIP != "" {
		replaceArgs = append(replaceArgs, "src", srcIP)
	}
	if _, err := run(replaceArgs...); err != nil {
		return fmt.Errorf("ip route replace default: %w", err)
	}

	log.Infof("TUN: default-capture routing configured (tun=%s, out=%s gw=%q src=%q mark=0x%x table=%d)",
		r.tunName, r.outIface, r.outGateway, srcIP, r.mark, r.routeTable)

	return nil
}

// interfacePrimaryIPv4 returns the first global IPv4 address of iface, or "".
func interfacePrimaryIPv4(iface string) string {
	out, err := run("ip", "-4", "-o", "addr", "show", "dev", iface, "scope", "global")
	if err != nil {
		return ""
	}
	for _, field := range strings.Fields(out) {
		if strings.Contains(field, "/") && strings.Count(field, ".") == 3 {
			return strings.SplitN(field, "/", 2)[0]
		}
	}
	return ""
}

func (r *routeManager) addBypassDefault(tableStr string) error {
	args := []string{"ip", "route", "add", "default"}
	if r.outGateway != "" {
		args = append(args, "via", r.outGateway)
	}
	args = append(args, "dev", r.outIface, "table", tableStr)
	if _, err := run(args...); err != nil {
		return fmt.Errorf("ip route add table: %w", err)
	}
	return nil
}

func (r *routeManager) teardown() {
	markStr := fmt.Sprintf("0x%x", r.mark)
	tableStr := fmt.Sprintf("%d", r.routeTable)

	// Default-capture mode replaced the default route; selective mode never
	// touched it (only the TUN's own host routes, which vanish with the device).
	if !r.selective() && r.savedDefault != "" {
		args := append([]string{"ip", "route", "replace"}, strings.Fields(r.savedDefault)...)
		if _, err := run(args...); err != nil {
			log.Errorf("TUN: failed to restore default route: %v", err)
		} else {
			log.Infof("TUN: restored default route: %s", r.savedDefault)
		}
	}

	if _, err := run("ip", "rule", "del", "fwmark", markStr, "lookup", tableStr); err != nil {
		log.Warnf("TUN: failed to delete ip rule: %v", err)
	}
	if _, err := run("ip", "route", "flush", "table", tableStr); err != nil {
		log.Warnf("TUN: failed to flush route table %s: %v", tableStr, err)
	}
	if _, err := run("ip", "link", "del", r.tunName); err != nil {
		log.Warnf("TUN: failed to delete %s: %v", r.tunName, err)
	}

	r.restoreRPFilter()

	log.Infof("TUN: routing teardown complete")
}

func extractField(routeLine, keyword string) string {
	parts := strings.Fields(routeLine)
	for i, p := range parts {
		if p == keyword && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

func extractGateway(routeLine string) string {
	return extractField(routeLine, "via")
}

func run(args ...string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
