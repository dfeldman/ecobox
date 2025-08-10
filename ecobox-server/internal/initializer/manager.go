package initializer

import (
	"fmt"
	"net"
	"strings"
	"time"

	"ecobox-server/internal/models"
	"ecobox-server/internal/storage"
	"github.com/sirupsen/logrus"
)

// Manager handles server initialization tasks
type Manager struct {
	storage storage.Storage
	logger  *logrus.Logger
}

// NewManager creates a new initializer manager
func NewManager(storage storage.Storage) *Manager {
	return &Manager{
		storage: storage,
		logger:  logrus.New(),
	}
}

// SetLogger sets a custom logger
func (m *Manager) SetLogger(logger *logrus.Logger) {
	m.logger = logger
}

// InitializeServer performs initial setup for a server
func (m *Manager) InitializeServer(server *models.Server) error {
	m.logger.Infof("Initializing server %s (%s)", server.Name, server.Hostname)
	
	var errors []string
	success := true
	
	// Log the start of initialization
	action := models.ServerAction{
		Timestamp:   time.Now(),
		Action:      models.ActionTypeInitialize,
		Success:     false, // Will be updated at the end
		InitiatedBy: "system",
	}
	
	// 1. Collect IP address information
	if err := m.collectIPInfo(server); err != nil {
		errors = append(errors, fmt.Sprintf("IP collection failed: %v", err))
		success = false
		m.logger.Warnf("Failed to collect IP info for %s: %v", server.Name, err)
	} else {
		m.logger.Infof("Successfully collected IP info for %s", server.Name)
	}
	
	// 2. Validate/discover MAC address
	if err := m.validateMACAddress(server); err != nil {
		errors = append(errors, fmt.Sprintf("MAC validation failed: %v", err))
		success = false
		m.logger.Warnf("Failed to validate MAC address for %s: %v", server.Name, err)
	} else {
		m.logger.Infof("MAC address validated for %s", server.Name)
	}
	
	// 3. Check Wake-on-LAN capability
	if err := m.checkWoLCapability(server); err != nil {
		errors = append(errors, fmt.Sprintf("WoL check failed: %v", err))
		success = false
		m.logger.Warnf("Failed to check WoL capability for %s: %v", server.Name, err)
	} else {
		m.logger.Infof("Wake-on-LAN capability checked for %s", server.Name)
	}
	
	// Mark server as initialized
	server.Initialized = true
	
	// Update the action with results
	action.Success = success
	if !success {
		action.ErrorMsg = strings.Join(errors, "; ")
	}
	
	// Store the action
	if err := m.storage.AddServerAction(server.ID, action); err != nil {
		m.logger.Errorf("Failed to log initialization action for %s: %v", server.Name, err)
	}
	
	// Update the server
	if err := m.storage.UpdateServer(server); err != nil {
		return fmt.Errorf("failed to update server after initialization: %w", err)
	}
	
	if success {
		m.logger.Infof("Successfully initialized server %s", server.Name)
	} else {
		m.logger.Warnf("Server %s initialization completed with warnings: %s", server.Name, strings.Join(errors, "; "))
	}
	
	return nil
}

// collectIPInfo resolves and stores IP address information for the server
func (m *Manager) collectIPInfo(server *models.Server) error {
	// Attempt to resolve hostname to IP
	ips, err := net.LookupIP(server.Hostname)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %s: %w", server.Hostname, err)
	}
	
	if len(ips) == 0 {
		return fmt.Errorf("no IP addresses found for hostname %s", server.Hostname)
	}
	
	// For now, we just log the IPs. In the future, we might want to store them
	var ipStrings []string
	for _, ip := range ips {
		ipStrings = append(ipStrings, ip.String())
	}
	
	m.logger.Infof("Server %s resolves to IPs: %s", server.Name, strings.Join(ipStrings, ", "))
	return nil
}

// validateMACAddress ensures the MAC address is properly formatted
func (m *Manager) validateMACAddress(server *models.Server) error {
	if server.MACAddress == "" {
		return fmt.Errorf("MAC address is empty")
	}
	
	// Parse MAC address to validate format
	_, err := net.ParseMAC(server.MACAddress)
	if err != nil {
		return fmt.Errorf("invalid MAC address format: %w", err)
	}
	
	return nil
}

// checkWoLCapability attempts to determine if Wake-on-LAN is available
func (m *Manager) checkWoLCapability(server *models.Server) error {
	// For now, this is a placeholder that always succeeds
	// In a real implementation, you might:
	// 1. Check if the server responds to magic packets
	// 2. SSH into the server (if it's up) and check WoL settings
	// 3. Use network tools to detect WoL capability
	
	m.logger.Infof("Wake-on-LAN capability check for %s - assuming available", server.Name)
	
	// TODO: Implement actual WoL capability detection
	// This might involve:
	// - Checking BIOS/UEFI settings (if accessible)
	// - Testing magic packet response
	// - Querying network interface settings via SSH
	
	return nil
}
