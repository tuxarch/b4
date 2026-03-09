package config

import "github.com/spf13/cobra"

func (c *Config) BindFlags(cmd *cobra.Command) {
	// Config path
	cmd.Flags().StringVar(&c.ConfigPath, "config", c.ConfigPath, "Path to config file")

	// System configuration
	cmd.Flags().IntVar(&c.System.Tables.MonitorInterval, "tables-monitor-interval", c.System.Tables.MonitorInterval, "Tables monitor interval in seconds (default 10, 0 to disable)")
	cmd.Flags().BoolVar(&c.System.Tables.SkipSetup, "skip-tables", c.System.Tables.SkipSetup, "Skip iptables/nftables setup on startup")
	cmd.Flags().BoolVar(&c.System.Tables.Masquerade, "masquerade", c.System.Tables.Masquerade, "Enable NAT masquerade (useful for containers)")
	cmd.Flags().StringVar(&c.System.Tables.MasqueradeInterface, "masquerade-interface", c.System.Tables.MasqueradeInterface, "Restrict masquerade to this output interface (empty = all)")

	// Logging configuration
	cmd.Flags().BoolVarP(&c.System.Logging.Instaflush, "instaflush", "i", c.System.Logging.Instaflush, "Flush logs immediately")
	cmd.Flags().BoolVar(&c.System.Logging.Syslog, "syslog", c.System.Logging.Syslog, "Enable syslog output")
	cmd.Flags().StringVar(&c.System.Logging.ErrorFile, "error-file", c.System.Logging.ErrorFile, "Path to error log file (empty disables)")

	// Web Server configuration
	cmd.Flags().IntVar(&c.System.WebServer.Port, "web-port", c.System.WebServer.Port, "Port for internal web server (0 disables)")
}
