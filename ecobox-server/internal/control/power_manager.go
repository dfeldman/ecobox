package control

import (
	"fmt"
	"time"

	"ecobox-server/internal/models"
	"ecobox-server/internal/storage"
	"github.com/sirupsen/logrus"
)

// PowerManager handles server power state management
type PowerManager struct {
	wolSender *WoLSender
	sshClient *SSHClient
	storage   storage.Storage
	logger    *logrus.Logger
}

// NewPowerManager creates a new power manager instance
func NewPowerManager(storage storage.Storage) *PowerManager {
	return &PowerManager{
		wolSender: NewWoLSender(),
		sshClient: NewSSHClient(),
		storage:   storage,
		logger:    logrus.New(),
	}
}

// WakeServer handles wake requests with parent server support
func (pm *PowerManager) WakeServer(server *models.Server) error {
	pm.logger.Infof("Wake request for server: %s", server.Name)

	// If server has a parent, wake parent first
	if server.ParentServerID != "" {
		parentServer, err := pm.storage.GetServer(server.ParentServerID)
		if err != nil {
			return fmt.Errorf("failed to get parent server %s: %w", server.ParentServerID, err)
		}

		// Recursively wake parent if it's not already on
		if parentServer.CurrentState != models.PowerStateOn {
			pm.logger.Infof("Waking parent server %s first", parentServer.Name)
			if err := pm.WakeServer(parentServer); err != nil {
				return fmt.Errorf("failed to wake parent server: %w", err)
			}

			// Wait a bit for parent to become available
			time.Sleep(5 * time.Second)
		}
	}

	// Send WoL packet to the server
	err := pm.wolSender.SendMagicPacketMultiple(server.MACAddress, []string{
		"255.255.255.255:9",
		"255.255.255.255:7",
	})

	// Log the action
	action := models.ServerAction{
		Timestamp:   time.Now(),
		Action:      models.ActionTypeWakeUp,
		Success:     err == nil,
		InitiatedBy: "manual",
	}

	if err != nil {
		action.ErrorMsg = err.Error()
		pm.logger.Errorf("Failed to send WoL packet to %s: %v", server.Name, err)
	} else {
		pm.logger.Infof("WoL packet sent to %s", server.Name)
		// Update desired state
		if updateErr := pm.storage.UpdateServerState(server.ID, models.PowerStateUnknown); updateErr != nil {
			pm.logger.Errorf("Failed to update server state: %v", updateErr)
		}
	}

	// Add action to server history
	if actionErr := pm.storage.AddServerAction(server.ID, action); actionErr != nil {
		pm.logger.Errorf("Failed to log server action: %v", actionErr)
	}

	return err
}

// SuspendServer handles suspend requests via SSH
func (pm *PowerManager) SuspendServer(server *models.Server) error {
	pm.logger.Infof("Suspend request for server: %s", server.Name)

	// Check if server is currently on
	if server.CurrentState != models.PowerStateOn {
		return fmt.Errorf("server %s is not currently on (state: %s)", server.Name, server.CurrentState)
	}

	// Execute suspend command via SSH
	var err error
	suspendCommands := []string{
		"systemctl suspend",
		"pm-suspend",
		"echo mem > /proc/sys/power/state",
	}

	// Try different suspend commands
	for _, cmd := range suspendCommands {
		err = pm.sshClient.ExecuteCommand(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath, cmd)
		if err == nil {
			break
		}
		pm.logger.Warnf("Suspend command '%s' failed for %s: %v", cmd, server.Name, err)
	}

	// Log the action
	action := models.ServerAction{
		Timestamp:   time.Now(),
		Action:      models.ActionTypeSuspend,
		Success:     err == nil,
		InitiatedBy: "manual",
	}

	if err != nil {
		action.ErrorMsg = fmt.Sprintf("All suspend commands failed. Last error: %v", err)
		pm.logger.Errorf("Failed to suspend %s: %v", server.Name, err)
	} else {
		pm.logger.Infof("Successfully sent suspend command to %s", server.Name)
		// Update desired state
		if updateErr := pm.storage.UpdateServerState(server.ID, models.PowerStateSuspended); updateErr != nil {
			pm.logger.Errorf("Failed to update server state: %v", updateErr)
		}
	}

	// Add action to server history
	if actionErr := pm.storage.AddServerAction(server.ID, action); actionErr != nil {
		pm.logger.Errorf("Failed to log server action: %v", actionErr)
	}

	return err
}

// GetRootServer finds the root server in a hierarchy
func (pm *PowerManager) GetRootServer(server *models.Server) (*models.Server, error) {
	current := server
	visited := make(map[string]bool)

	for current.ParentServerID != "" {
		// Check for circular reference
		if visited[current.ID] {
			return nil, fmt.Errorf("circular parent reference detected starting from server %s", server.ID)
		}
		visited[current.ID] = true

		// Get parent server
		parent, err := pm.storage.GetServer(current.ParentServerID)
		if err != nil {
			return nil, fmt.Errorf("failed to get parent server %s: %w", current.ParentServerID, err)
		}

		current = parent
	}

	return current, nil
}

// TestServerConnectivity tests if server can be reached via SSH
func (pm *PowerManager) TestServerConnectivity(server *models.Server) error {
	return pm.sshClient.TestConnection(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath)
}

// SetLogger allows setting a custom logger
func (pm *PowerManager) SetLogger(logger *logrus.Logger) {
	pm.logger = logger
}
