package config

type Config struct {
	Dashboard DashboardConfig `toml:"dashboard"`
	Servers   []ServerConfig  `toml:"servers"`
}

type DashboardConfig struct {
	Port             int    `toml:"port"`              // Web interface port (default: 8080)
	UpdateInterval   int    `toml:"update_interval"`   // Status check interval in seconds (default: 30)
	WoLRetryInterval int    `toml:"wol_retry_interval"` // WoL retry interval in seconds (default: 10)
	WoLMaxRetries    int    `toml:"wol_max_retries"`   // Maximum WoL retries (default: 5)
	LogLevel         string `toml:"log_level"`         // "debug", "info", "warn", "error"

	// Authentication settings
	IAPAuth          string `toml:"iap_auth"`          // Identity-aware proxy: "tailscale", "authentik", "cloudflare", "none"
	SessionKeyFile   string `toml:"session_key_file"`  // Path to session key file (default: "sessionkey.conf")
	PasswordFile     string `toml:"password_file"`     // Path to password file (default: "passwd.conf")
}

type ServerConfig struct {
	ID             string          `toml:"id"`
	Name           string          `toml:"name"`
	Hostname       string          `toml:"hostname"`
	MACAddress     string          `toml:"mac_address"`
	ParentServerID string          `toml:"parent_server_id"`
	SSHUser        string          `toml:"ssh_user"`
	SSHPort        int             `toml:"ssh_port"`
	SSHKeyPath     string          `toml:"ssh_key_path"`
	Services       []ServiceConfig `toml:"services"`
}

type ServiceConfig struct {
	Name string `toml:"name"`
	Port int    `toml:"port"`
	Type string `toml:"type"`
}

// SetDefaults sets default values for missing configuration fields
func (c *Config) SetDefaults() {
	if c.Dashboard.Port == 0 {
		c.Dashboard.Port = 8080
	}
	if c.Dashboard.UpdateInterval == 0 {
		c.Dashboard.UpdateInterval = 30
	}
	if c.Dashboard.WoLRetryInterval == 0 {
		c.Dashboard.WoLRetryInterval = 10
	}
	if c.Dashboard.WoLMaxRetries == 0 {
		c.Dashboard.WoLMaxRetries = 5
	}
	if c.Dashboard.LogLevel == "" {
		c.Dashboard.LogLevel = "info"
	}

	// Set authentication defaults
	if c.Dashboard.IAPAuth == "" {
		c.Dashboard.IAPAuth = "none"
	}
	if c.Dashboard.SessionKeyFile == "" {
		c.Dashboard.SessionKeyFile = "sessionkey.conf"
	}
	if c.Dashboard.PasswordFile == "" {
		c.Dashboard.PasswordFile = "passwd.conf"
	}

	for i := range c.Servers {
		if c.Servers[i].SSHUser == "" {
			c.Servers[i].SSHUser = "root"
		}
		if c.Servers[i].SSHPort == 0 {
			c.Servers[i].SSHPort = 22
		}
	}
}
