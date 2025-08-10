package config

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

// LoadConfig loads and validates TOML configuration file
func LoadConfig(filepath string) (*Config, error) {
	var config Config

	// Check if file exists
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file not found: %s", filepath)
	}

	// Parse TOML file
	if _, err := toml.DecodeFile(filepath, &config); err != nil {
		return nil, fmt.Errorf("failed to parse configuration file: %w", err)
	}

	// Set defaults for missing values
	config.SetDefaults()

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &config, nil
}

// Validate validates the configuration for correctness
func (c *Config) Validate() error {
	// Validate dashboard configuration
	if c.Dashboard.Port < 1 || c.Dashboard.Port > 65535 {
		return fmt.Errorf("dashboard port must be between 1 and 65535, got %d", c.Dashboard.Port)
	}

	if c.Dashboard.UpdateInterval < 1 {
		return fmt.Errorf("update interval must be at least 1 second, got %d", c.Dashboard.UpdateInterval)
	}

	if c.Dashboard.WoLRetryInterval < 1 {
		return fmt.Errorf("WoL retry interval must be at least 1 second, got %d", c.Dashboard.WoLRetryInterval)
	}

	if c.Dashboard.WoLMaxRetries < 1 {
		return fmt.Errorf("WoL max retries must be at least 1, got %d", c.Dashboard.WoLMaxRetries)
	}

	validLogLevels := map[string]bool{
		"debug": true, "info": true, "warn": true, "error": true,
	}
	if !validLogLevels[c.Dashboard.LogLevel] {
		return fmt.Errorf("invalid log level '%s', must be one of: debug, info, warn, error", c.Dashboard.LogLevel)
	}

	// Validate IAP auth setting
	validIAPAuth := map[string]bool{
		"none": true, "tailscale": true, "authentik": true, "cloudflare": true,
	}
	if !validIAPAuth[c.Dashboard.IAPAuth] {
		return fmt.Errorf("invalid iap_auth '%s', must be one of: none, tailscale, authentik, cloudflare", c.Dashboard.IAPAuth)
	}

	// Validate servers
	serverIDs := make(map[string]bool)
	for _, server := range c.Servers {
		if server.ID == "" {
			return fmt.Errorf("server ID cannot be empty")
		}

		if serverIDs[server.ID] {
			return fmt.Errorf("duplicate server ID: %s", server.ID)
		}
		serverIDs[server.ID] = true

		if server.Name == "" {
			return fmt.Errorf("server name cannot be empty for server %s", server.ID)
		}

		if server.Hostname == "" {
			return fmt.Errorf("server hostname cannot be empty for server %s", server.ID)
		}

		// Validate MAC address format
		if err := validateMACAddress(server.MACAddress); err != nil {
			return fmt.Errorf("invalid MAC address for server %s: %w", server.ID, err)
		}

		// Validate SSH port
		if server.SSHPort < 1 || server.SSHPort > 65535 {
			return fmt.Errorf("SSH port must be between 1 and 65535 for server %s, got %d", server.ID, server.SSHPort)
		}

		// Validate services
		for _, service := range server.Services {
			if service.Name == "" {
				return fmt.Errorf("service name cannot be empty for server %s", server.ID)
			}
			if service.Port < 1 || service.Port > 65535 {
				return fmt.Errorf("service port must be between 1 and 65535 for server %s service %s, got %d", server.ID, service.Name, service.Port)
			}
		}
	}

	// Validate parent server references
	if err := c.validateParentReferences(serverIDs); err != nil {
		return err
	}

	return nil
}

// validateMACAddress validates MAC address format (XX:XX:XX:XX:XX:XX)
func validateMACAddress(mac string) error {
	if mac == "" {
		return fmt.Errorf("MAC address cannot be empty")
	}

	// Parse MAC address
	if _, err := net.ParseMAC(mac); err != nil {
		return fmt.Errorf("invalid MAC address format: %w", err)
	}

	// Ensure uppercase format with colons
	macRegex := regexp.MustCompile(`^([0-9A-F]{2}[:-]){5}([0-9A-F]{2})$`)
	upperMAC := strings.ToUpper(mac)
	if !macRegex.MatchString(upperMAC) {
		return fmt.Errorf("MAC address must be in format XX:XX:XX:XX:XX:XX or XX-XX-XX-XX-XX-XX")
	}

	return nil
}

// validateParentReferences checks for valid parent references and circular dependencies
func (c *Config) validateParentReferences(serverIDs map[string]bool) error {
	for _, server := range c.Servers {
		if server.ParentServerID == "" {
			continue // No parent is valid
		}

		// Check if parent server exists
		if !serverIDs[server.ParentServerID] {
			return fmt.Errorf("parent server '%s' not found for server '%s'", server.ParentServerID, server.ID)
		}

		// Check for circular references
		if err := c.checkCircularReference(server.ID, server.ParentServerID, make(map[string]bool)); err != nil {
			return err
		}
	}

	return nil
}

// checkCircularReference recursively checks for circular parent references
func (c *Config) checkCircularReference(originalID, currentParentID string, visited map[string]bool) error {
	if visited[currentParentID] {
		return fmt.Errorf("circular parent reference detected starting from server '%s'", originalID)
	}

	if currentParentID == originalID {
		return fmt.Errorf("server '%s' cannot be its own ancestor", originalID)
	}

	visited[currentParentID] = true

	// Find the parent server and check its parent
	for _, server := range c.Servers {
		if server.ID == currentParentID && server.ParentServerID != "" {
			return c.checkCircularReference(originalID, server.ParentServerID, visited)
		}
	}

	return nil
}
