package tun

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/daniellavrushin/b4/log"
)

// routeManager handles setting up and tearing down routing rules for TUN mode.
type routeManager struct {
	tunName      string
	tunAddr      string // e.g. "10.255.0.1/30"
	outIface     string // e.g. "eth0"
	outGateway   string // e.g. "192.168.1.1"
	mark         uint
	routeTable   int
	savedDefault string // original default route for restoration
}

func newRouteManager(tunName, tunAddr, outIface, outGateway string, mark uint, routeTable int) *routeManager {
	return &routeManager{
		tunName:    tunName,
		tunAddr:    tunAddr,
		outIface:   outIface,
		outGateway: outGateway,
		mark:       mark,
		routeTable: routeTable,
	}
}

// setup configures routing so traffic flows through the TUN device,
// while b4's own outbound packets (marked with fwmark) bypass the TUN.
func (r *routeManager) setup() error {
	// Save current default route for restoration
	out, err := run("ip", "route", "show", "default")
	if err != nil {
		return fmt.Errorf("failed to read current default route: %w", err)
	}
	r.savedDefault = strings.TrimSpace(out)
	log.Infof("TUN: saved default route: %s", r.savedDefault)

	// Auto-detect gateway if not specified
	if r.outGateway == "" {
		gw := extractGateway(r.savedDefault)
		if gw == "" {
			return fmt.Errorf("could not auto-detect gateway from default route: %s", r.savedDefault)
		}
		r.outGateway = gw
		log.Infof("TUN: auto-detected gateway: %s", r.outGateway)
	}

	// 1. Configure TUN device
	if _, err := run("ip", "addr", "add", r.tunAddr, "dev", r.tunName); err != nil {
		return fmt.Errorf("ip addr add: %w", err)
	}
	if _, err := run("ip", "link", "set", r.tunName, "up"); err != nil {
		return fmt.Errorf("ip link set up: %w", err)
	}
	if _, err := run("ip", "link", "set", r.tunName, "mtu", "1500"); err != nil {
		log.Warnf("TUN: failed to set MTU: %v", err)
	}

	// 2. Policy routing: marked packets use a separate table that routes via the real interface
	markStr := fmt.Sprintf("0x%x", r.mark)
	tableStr := fmt.Sprintf("%d", r.routeTable)

	// Clean up stale rules/routes from a previous run that didn't shut down cleanly
	run("ip", "rule", "del", "fwmark", markStr, "lookup", tableStr)
	run("ip", "route", "flush", "table", tableStr)

	if _, err := run("ip", "rule", "add", "fwmark", markStr, "lookup", tableStr, "priority", "100"); err != nil {
		return fmt.Errorf("ip rule add: %w", err)
	}
	if _, err := run("ip", "route", "add", "default", "via", r.outGateway, "dev", r.outIface, "table", tableStr); err != nil {
		return fmt.Errorf("ip route add table: %w", err)
	}

	// 3. Replace default route to go through TUN, preserving the original source IP
	//    so the kernel doesn't assign the TUN address (e.g. 10.255.0.1) as source
	srcIP := extractField(r.savedDefault, "src")
	if srcIP != "" {
		if _, err := run("ip", "route", "replace", "default", "dev", r.tunName, "src", srcIP); err != nil {
			return fmt.Errorf("ip route replace default: %w", err)
		}
	} else {
		if _, err := run("ip", "route", "replace", "default", "dev", r.tunName); err != nil {
			return fmt.Errorf("ip route replace default: %w", err)
		}
	}

	log.Infof("TUN: routing configured (tun=%s, out=%s via %s, mark=0x%x, table=%d)",
		r.tunName, r.outIface, r.outGateway, r.mark, r.routeTable)

	return nil
}

// teardown restores the original routing configuration.
func (r *routeManager) teardown() {
	markStr := fmt.Sprintf("0x%x", r.mark)
	tableStr := fmt.Sprintf("%d", r.routeTable)

	// Restore original default route
	if r.savedDefault != "" {
		args := append([]string{"ip", "route", "replace"}, strings.Fields(r.savedDefault)...)
		if _, err := run(args...); err != nil {
			log.Errorf("TUN: failed to restore default route: %v", err)
		} else {
			log.Infof("TUN: restored default route: %s", r.savedDefault)
		}
	}

	// Clean up policy routing
	if _, err := run("ip", "rule", "del", "fwmark", markStr, "lookup", tableStr); err != nil {
		log.Warnf("TUN: failed to delete ip rule: %v", err)
	}
	if _, err := run("ip", "route", "flush", "table", tableStr); err != nil {
		log.Warnf("TUN: failed to flush route table %s: %v", tableStr, err)
	}

	// Remove TUN device
	if _, err := run("ip", "link", "del", r.tunName); err != nil {
		log.Warnf("TUN: failed to delete %s: %v", r.tunName, err)
	}

	log.Infof("TUN: routing teardown complete")
}

// extractField parses a route line for a keyword and returns the next token.
// e.g. extractField("default via 1.2.3.4 dev eth0 src 10.0.0.1", "via") => "1.2.3.4"
func extractField(routeLine, keyword string) string {
	parts := strings.Fields(routeLine)
	for i, p := range parts {
		if p == keyword && i+1 < len(parts) {
			return parts[i+1]
		}
	}
	return ""
}

// extractGateway parses "default via X.X.X.X dev Y" to get the gateway IP.
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
