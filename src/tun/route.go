package tun

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/daniellavrushin/b4/log"
	"github.com/daniellavrushin/b4/tables"
)

func interfaceMTU(iface string) int {
	b, err := os.ReadFile("/sys/class/net/" + iface + "/mtu")
	if err != nil {
		return 0
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0
	}
	return n
}

type routeManager struct {
	tunName       string
	tunAddr       string
	tunAddrV6     string
	outIface      string
	outGateway    string
	mark          uint
	routeTable    int
	routes        []string
	skipTables    bool
	savedDefault  string
	savedRPFilter string
	fwdRulesAdded bool
	snatAdded     bool
	notrackAdded  bool

	mu      sync.Mutex
	srcIP   string
	current map[string]bool
}

func (r *routeManager) setupNAT() {
	markStr := fmt.Sprintf("0x%x", r.mark)

	if r.srcIP != "" {
		snat := []string{"-o", r.tunName, "-j", "SNAT", "--to-source", r.srcIP}
		if _, err := run(append([]string{"iptables", "-t", "nat", "-C", "POSTROUTING"}, snat...)...); err != nil {
			if _, err := run(append([]string{"iptables", "-t", "nat", "-A", "POSTROUTING"}, snat...)...); err != nil {
				log.Warnf("TUN: failed to add SNAT %s -> %s: %v", r.tunName, r.srcIP, err)
			} else {
				r.snatAdded = true
				log.Infof("TUN: SNAT installed (traffic into %s -> source %s)", r.tunName, r.srcIP)
			}
		} else {
			r.snatAdded = true
		}
	} else {
		log.Warnf("TUN: no source IP derived for %s; forwarded LAN traffic will not be NAT'd and replies will not return", r.outIface)
	}

	notrack := []string{"-m", "mark", "--mark", markStr, "-j", "CT", "--notrack"}
	if _, err := run(append([]string{"iptables", "-t", "raw", "-C", "OUTPUT"}, notrack...)...); err != nil {
		if _, err := run(append([]string{"iptables", "-t", "raw", "-A", "OUTPUT"}, notrack...)...); err != nil {
			log.Infof("TUN: NOTRACK not installed for mark %s (no raw table here); conntrack sysctls + SNAT cover it, so this is harmless: %v", markStr, err)
		} else {
			r.notrackAdded = true
			log.Infof("TUN: NOTRACK installed for re-injected packets (mark %s)", markStr)
		}
	} else {
		r.notrackAdded = true
	}
}

func (r *routeManager) removeSNAT() {
	if !r.snatAdded || r.srcIP == "" {
		return
	}
	for {
		if _, err := run("iptables", "-t", "nat", "-D", "POSTROUTING", "-o", r.tunName, "-j", "SNAT", "--to-source", r.srcIP); err != nil {
			break
		}
	}
	r.snatAdded = false
}

func (r *routeManager) teardownNAT() {
	r.removeSNAT()
	if r.notrackAdded {
		markStr := fmt.Sprintf("0x%x", r.mark)
		for {
			if _, err := run("iptables", "-t", "raw", "-D", "OUTPUT", "-m", "mark", "--mark", markStr, "-j", "CT", "--notrack"); err != nil {
				break
			}
		}
		r.notrackAdded = false
	}
}

func (r *routeManager) applyForwarding() int {
	added := 0
	for _, dir := range []string{"-i", "-o"} {
		if _, err := run("iptables", "-C", "FORWARD", dir, r.tunName, "-j", "ACCEPT"); err == nil {
			continue
		}
		if _, err := run("iptables", "-I", "FORWARD", dir, r.tunName, "-j", "ACCEPT"); err != nil {
			log.Warnf("TUN: failed to add FORWARD accept (%s %s): %v", dir, r.tunName, err)
		} else {
			added++
		}
	}
	r.fwdRulesAdded = true
	return added
}

func (r *routeManager) setupForwarding() {
	if r.applyForwarding() > 0 {
		log.Infof("TUN: FORWARD accept rules installed for %s", r.tunName)
	}
}

func (r *routeManager) teardownForwarding() {
	if !r.fwdRulesAdded {
		return
	}
	for _, dir := range []string{"-i", "-o"} {
		for {
			if _, err := run("iptables", "-D", "FORWARD", dir, r.tunName, "-j", "ACCEPT"); err != nil {
				break
			}
		}
	}
}

func (r *routeManager) selective() bool {
	return len(r.routes) > 0
}

func (r *routeManager) addRoute(prefix string) bool {
	if strings.Contains(prefix, ":") {
		log.Warnf("TUN: IPv6 target %s not routed (IPv6 TUN routing not yet supported); traffic stays on the normal path", prefix)
		return false
	}
	args := []string{"ip", "route", "replace", prefix, "dev", r.tunName}
	if r.srcIP != "" {
		args = append(args, "src", r.srcIP)
	}
	if _, err := run(args...); err != nil {
		log.Warnf("TUN: failed to add route %s: %v", prefix, err)
		return false
	}
	return true
}

func (r *routeManager) Resync(desired []string) {
	if !r.selective() {
		return
	}
	want := make(map[string]bool, len(desired))
	for _, p := range desired {
		p = strings.TrimSpace(p)
		if p != "" {
			want[p] = true
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	added, removed := 0, 0
	for p := range want {
		if !r.current[p] {
			if r.addRoute(p) {
				r.current[p] = true
				added++
			}
		}
	}
	for p := range r.current {
		if !want[p] {
			run("ip", "route", "del", p, "dev", r.tunName)
			delete(r.current, p)
			removed++
		}
	}
	if added > 0 || removed > 0 {
		log.Infof("TUN: routes resynced (+%d -%d, now %d)", added, removed, len(r.current))
	}
}

func (r *routeManager) Add(prefix string) {
	if !r.selective() {
		return
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.current[prefix] {
		return
	}
	if r.addRoute(prefix) {
		r.current[prefix] = true
		log.Tracef("TUN: learned route %s -> %s", prefix, r.tunName)
	}
}

func (r *routeManager) setup() error {
	out, err := run("ip", "-4", "route", "show", "default")
	if err != nil {
		return fmt.Errorf("failed to read current default route: %w", err)
	}
	r.savedDefault = strings.TrimSpace(strings.SplitN(out, "\n", 2)[0])
	log.Infof("TUN: saved default route: %s", r.savedDefault)

	if r.outGateway == "" {
		r.outGateway = extractGateway(r.savedDefault)
		if r.outGateway != "" {
			log.Infof("TUN: auto-detected gateway: %s", r.outGateway)
		} else {
			log.Infof("TUN: no gateway on default route, treating %s as point-to-point", r.outIface)
		}
	}

	srcIP := extractField(r.savedDefault, "src")
	if srcIP == "" {
		srcIP = interfacePrimaryIPv4(r.outIface)
	}
	r.srcIP = srcIP
	if r.current == nil {
		r.current = make(map[string]bool)
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
	mtu := interfaceMTU(r.outIface)
	if mtu <= 0 {
		mtu = 1500
	}
	if _, err := run("ip", "link", "set", r.tunName, "mtu", strconv.Itoa(mtu)); err != nil {
		log.Warnf("TUN: failed to set MTU %d: %v", mtu, err)
	} else {
		log.Infof("TUN: set %s MTU=%d (matching %s)", r.tunName, mtu, r.outIface)
	}

	if r.tunAddrV6 == "" {
		if err := os.WriteFile("/proc/sys/net/ipv6/conf/"+r.tunName+"/disable_ipv6", []byte("1\n"), 0644); err != nil {
			log.Tracef("TUN: could not disable IPv6 on %s: %v", r.tunName, err)
		}
	} else {
		log.Warnf("TUN: address_v6 %s is set, leaving IPv6 enabled on %s - note IPv6 is not yet forwarded in TUN mode, so v6 packets are dropped", r.tunAddrV6, r.tunName)
	}

	if r.skipTables {
		log.Infof("TUN: --skip-tables set; skipping rp_filter/FORWARD/SNAT/NOTRACK - manage NAT and forwarding yourself (b4 still sets up routing: device, capture, bypass table)")
	} else {
		r.loosenRPFilter()
		r.setupForwarding()
		r.setupNAT()
	}

	var capErr error
	if r.selective() {
		capErr = r.setupSelective(srcIP)
	} else {
		capErr = r.setupDefaultCapture(srcIP)
	}
	if capErr != nil {
		return capErr
	}
	return nil
}

func rpFilterPath(iface string) string {
	return "/proc/sys/net/ipv4/conf/" + iface + "/rp_filter"
}

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

func (r *routeManager) setupBypassTable() error {
	tableStr := fmt.Sprintf("%d", r.routeTable)
	markStr := fmt.Sprintf("0x%x", r.mark)

	if existing, err := run("ip", "route", "show", "table", tableStr); err == nil && strings.TrimSpace(existing) != "" {
		if !r.ownsBypassTable(markStr, tableStr) {
			return fmt.Errorf("route table %d is already in use (likely a system table; see /etc/iproute2/rt_tables) - set queue.tun.route_table to an unused id", r.routeTable)
		}
		log.Infof("TUN: reusing route table %d left by a previous run (flushing stale entries)", r.routeTable)
		run("ip", "route", "flush", "table", tableStr)
	}

	r.delFwmarkRule(markStr, tableStr)

	if _, err := run("ip", "rule", "add", "fwmark", markStr, "lookup", tableStr, "priority", "100"); err != nil {
		return fmt.Errorf("ip rule add (whole-default capture needs policy routing; a busybox 'ip' may reject custom tables - install full iproute2, e.g. 'apk add iproute2', or set queue.tun.route_table <= 255): %w", err)
	}
	return r.addBypassDefault(tableStr)
}

func ruleFieldValue(line, key string) string {
	fields := strings.Fields(line)
	for i := 0; i+1 < len(fields); i++ {
		if fields[i] == key {
			return fields[i+1]
		}
	}
	return ""
}

func (r *routeManager) ownsBypassTable(markStr, tableStr string) bool {
	out, err := run("ip", "rule", "show")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(out, "\n") {
		if ruleFieldValue(line, "lookup") != tableStr {
			continue
		}
		fw := ruleFieldValue(line, "fwmark")
		if fw == markStr || strings.HasPrefix(fw, markStr+"/") {
			return true
		}
	}
	return false
}

func (r *routeManager) delFwmarkRule(markStr, tableStr string) {
	for {
		if _, err := run("ip", "rule", "del", "fwmark", markStr, "lookup", tableStr); err != nil {
			return
		}
	}
}

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
		if r.addRoute(p) {
			r.current[p] = true
			added++
		}
	}
	if added == 0 {
		return fmt.Errorf("selective routing: no routes could be added")
	}
	log.Infof("TUN: selective routing %d/%d prefixes into %s (default route untouched, mark=0x%x table=%d)",
		added, len(r.routes), r.tunName, r.mark, r.routeTable)
	return nil
}

func (r *routeManager) replaceDefaultIntoTun() error {
	args := []string{"ip", "route", "replace", "default", "dev", r.tunName}
	if r.srcIP != "" {
		args = append(args, "src", r.srcIP)
	}
	if _, err := run(args...); err != nil {
		if r.srcIP == "" {
			return err
		}
		log.Warnf("TUN: default route with src %s rejected (%v); retrying without src", r.srcIP, err)
		if _, err2 := run("ip", "route", "replace", "default", "dev", r.tunName); err2 != nil {
			return err2
		}
		r.srcIP = ""
	}
	return nil
}

func (r *routeManager) setupDefaultCapture(srcIP string) error {
	if err := r.setupBypassTable(); err != nil {
		return err
	}

	if err := r.replaceDefaultIntoTun(); err != nil {
		return fmt.Errorf("ip route replace default: %w", err)
	}

	log.Infof("TUN: default-capture routing configured (tun=%s, out=%s gw=%q src=%q mark=0x%x table=%d)",
		r.tunName, r.outIface, r.outGateway, r.srcIP, r.mark, r.routeTable)

	return nil
}

func (r *routeManager) reconcile() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if newIP := interfacePrimaryIPv4(r.outIface); newIP != "" && newIP != r.srcIP {
		log.Infof("TUN: uplink %s address changed %q -> %q; updating SNAT and capture source", r.outIface, r.srcIP, newIP)
		r.removeSNAT()
		r.srcIP = newIP
		if !r.selective() {
			if err := r.replaceDefaultIntoTun(); err != nil {
				log.Warnf("TUN: reconcile failed to refresh default-capture src: %v", err)
			}
		}
	}

	if !r.skipTables {
		if n := r.applyForwarding(); n > 0 {
			log.Infof("TUN: reconcile restored %d FORWARD accept rule(s) for %s", n, r.tunName)
		}
		r.ensureNAT()
	}
	r.ensureBypass()
	if r.selective() {
		r.ensureSelectiveRoutes()
	} else {
		r.ensureDefaultCapture()
	}
}

func (r *routeManager) ensureNAT() {
	if r.srcIP != "" {
		snat := []string{"-o", r.tunName, "-j", "SNAT", "--to-source", r.srcIP}
		if _, err := run(append([]string{"iptables", "-t", "nat", "-C", "POSTROUTING"}, snat...)...); err != nil {
			if _, err := run(append([]string{"iptables", "-t", "nat", "-A", "POSTROUTING"}, snat...)...); err == nil {
				r.snatAdded = true
				log.Infof("TUN: reconcile restored SNAT (%s -> %s)", r.tunName, r.srcIP)
			}
		} else {
			r.snatAdded = true
		}
	}
	if r.notrackAdded {
		markStr := fmt.Sprintf("0x%x", r.mark)
		notrack := []string{"-m", "mark", "--mark", markStr, "-j", "CT", "--notrack"}
		if _, err := run(append([]string{"iptables", "-t", "raw", "-C", "OUTPUT"}, notrack...)...); err != nil {
			if _, err := run(append([]string{"iptables", "-t", "raw", "-A", "OUTPUT"}, notrack...)...); err == nil {
				log.Infof("TUN: reconcile restored NOTRACK (mark %s)", markStr)
			}
		}
	}
}

func (r *routeManager) ensureBypass() {
	markStr := fmt.Sprintf("0x%x", r.mark)
	tableStr := fmt.Sprintf("%d", r.routeTable)
	if !r.ownsBypassTable(markStr, tableStr) {
		if _, err := run("ip", "rule", "add", "fwmark", markStr, "lookup", tableStr, "priority", "100"); err != nil {
			log.Warnf("TUN: reconcile failed to restore fwmark rule: %v", err)
		} else {
			log.Infof("TUN: reconcile restored fwmark rule (mark %s -> table %s)", markStr, tableStr)
		}
	}
	if out, _ := run("ip", "route", "show", "table", tableStr); strings.TrimSpace(out) == "" {
		if err := r.addBypassDefault(tableStr); err != nil {
			log.Warnf("TUN: reconcile failed to restore bypass default route: %v", err)
		} else {
			log.Infof("TUN: reconcile restored bypass default route (table %s)", tableStr)
		}
	}
}

func (r *routeManager) ensureDefaultCapture() {
	out, _ := run("ip", "-4", "route", "show", "default")
	if strings.Contains(out, "dev "+r.tunName) {
		return
	}
	log.Infof("TUN: reconcile re-capturing default route into %s (it was reverted)", r.tunName)
	if err := r.replaceDefaultIntoTun(); err != nil {
		log.Warnf("TUN: reconcile failed to re-capture default route: %v", err)
	}
}

func (r *routeManager) ensureSelectiveRoutes() {
	for p := range r.current {
		r.addRoute(p)
	}
}

func interfacePrimaryIPv4(iface string) string {
	out, err := run("ip", "-4", "-o", "addr", "show", "dev", iface, "scope", "global")
	if err != nil {
		return ""
	}
	fields := strings.Fields(out)
	for i, f := range fields {
		if f == "inet" && i+1 < len(fields) {
			return strings.SplitN(fields[i+1], "/", 2)[0]
		}
	}
	return ""
}

func (r *routeManager) addBypassDefault(tableStr string) error {
	args := []string{"ip", "route", "replace", "default"}
	if r.outGateway != "" {
		args = append(args, "via", r.outGateway)
	}
	args = append(args, "dev", r.outIface, "table", tableStr)
	if _, err := run(args...); err != nil {
		return fmt.Errorf("ip route replace table (whole-default capture needs policy routing; a busybox 'ip' may reject custom tables - install full iproute2, e.g. 'apk add iproute2', or set queue.tun.route_table <= 255): %w", err)
	}
	return nil
}

func (r *routeManager) teardown() {
	markStr := fmt.Sprintf("0x%x", r.mark)
	tableStr := fmt.Sprintf("%d", r.routeTable)

	if !r.selective() && r.savedDefault != "" {
		args := append([]string{"ip", "route", "replace"}, strings.Fields(r.savedDefault)...)
		if _, err := run(args...); err != nil {
			log.Errorf("TUN: failed to restore default route: %v", err)
		} else {
			log.Infof("TUN: restored default route: %s", r.savedDefault)
		}
	}

	r.delFwmarkRule(markStr, tableStr)
	if _, err := run("ip", "route", "flush", "table", tableStr); err != nil {
		log.Warnf("TUN: failed to flush route table %s: %v", tableStr, err)
	}
	if interfaceExists(r.tunName) {
		if _, err := run("ip", "link", "del", r.tunName); err != nil {
			log.Warnf("TUN: failed to delete %s: %v", r.tunName, err)
		}
	} else {
		log.Tracef("TUN: %s already gone (removed when the device fd closed)", r.tunName)
	}

	if !r.skipTables {
		r.teardownForwarding()
		r.teardownNAT()
		r.restoreRPFilter()
	}

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
	if len(args) > 0 && (args[0] == "iptables" || args[0] == "ip6tables") {
		if w := tables.WaitArgs(args[0]); len(w) > 0 {
			newArgs := make([]string, 0, len(args)+len(w))
			newArgs = append(newArgs, args[0])
			newArgs = append(newArgs, w...)
			newArgs = append(newArgs, args[1:]...)
			args = newArgs
		}
	}
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}
