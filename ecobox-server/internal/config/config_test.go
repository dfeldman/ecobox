package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	configContent := `[dashboard]
port = 8080
update_interval = 30
log_level = "info"

[[servers]]
id = "test-server"
name = "Test Server"
hostname = "192.168.1.100"
mac_address = "AA:BB:CC:DD:EE:FF"

  [[servers.services]]
  name = "SSH"
  port = 22
  type = "ssh"
`

	tmpFile, err := os.CreateTemp("", "config-*.toml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(configContent); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	tmpFile.Close()

	// Test loading the config
	cfg, err := LoadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify configuration values
	if cfg.Dashboard.Port != 8080 {
		t.Errorf("Expected port 8080, got %d", cfg.Dashboard.Port)
	}

	if len(cfg.Servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(cfg.Servers))
	}

	server := cfg.Servers[0]
	if server.ID != "test-server" {
		t.Errorf("Expected server ID 'test-server', got '%s'", server.ID)
	}

	if server.MACAddress != "AA:BB:CC:DD:EE:FF" {
		t.Errorf("Expected MAC address 'AA:BB:CC:DD:EE:FF', got '%s'", server.MACAddress)
	}
}

func TestConfigValidation(t *testing.T) {
	cfg := &Config{
		Dashboard: DashboardConfig{
			Port:             8080,
			UpdateInterval:   30,
			WoLRetryInterval: 10,
			WoLMaxRetries:    5,
			LogLevel:         "info",
		},
		Servers: []ServerConfig{
			{
				ID:         "test-server",
				Name:       "Test Server",
				Hostname:   "192.168.1.100",
				MACAddress: "AA:BB:CC:DD:EE:FF",
				SSHUser:    "root",
				SSHPort:    22,
			},
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Valid configuration failed validation: %v", err)
	}
}

func TestInvalidMACAddress(t *testing.T) {
	cfg := &Config{
		Dashboard: DashboardConfig{
			Port:             8080,
			UpdateInterval:   30,
			WoLRetryInterval: 10,
			WoLMaxRetries:    5,
			LogLevel:         "info",
		},
		Servers: []ServerConfig{
			{
				ID:         "test-server",
				Name:       "Test Server",
				Hostname:   "192.168.1.100",
				MACAddress: "invalid-mac",
				SSHUser:    "root",
				SSHPort:    22,
			},
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Error("Expected validation error for invalid MAC address")
	}
}
