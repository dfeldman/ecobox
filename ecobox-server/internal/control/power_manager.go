package control

import (
	"fmt"
	"time"

	"ecobox-server/internal/models"
	"ecobox-server/internal/proxmox"
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

	// Handle Proxmox VMs differently
	if server.IsProxmoxVM {
		return pm.wakeProxmoxVM(server)
	}

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

	// Check if server is in a state that can be suspended
	if server.CurrentState != models.PowerStateOn && server.CurrentState != models.PowerStateWaking {
		return fmt.Errorf("server %s cannot be suspended from current state: %s", server.Name, server.CurrentState)
	}

	// Handle Proxmox VMs differently
	if server.IsProxmoxVM {
		return pm.suspendProxmoxVM(server)
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

// ShutdownServer handles clean shutdown requests 
func (pm *PowerManager) ShutdownServer(server *models.Server) error {
	pm.logger.Infof("Shutdown request for server: %s", server.Name)

	// Check if server is in a state that can be shut down
	if server.CurrentState != models.PowerStateOn && server.CurrentState != models.PowerStateWaking {
		return fmt.Errorf("server %s cannot be shut down from current state: %s", server.Name, server.CurrentState)
	}

	// Handle Proxmox VMs differently
	if server.IsProxmoxVM {
		return pm.shutdownProxmoxVM(server)
	}

	// For regular servers, shutdown is the same as suspend for now
	// In the future, we could implement SSH shutdown commands
	return pm.SuspendServer(server)
}

// StopServer handles force stop requests (Proxmox VMs only)
func (pm *PowerManager) StopServer(server *models.Server) error {
	pm.logger.Infof("Stop request for server: %s", server.Name)

	// Only supported for Proxmox VMs
	if !server.IsProxmoxVM {
		return fmt.Errorf("force stop is only supported for Proxmox VMs")
	}

	// Check if server is in a state that can be stopped
	if server.CurrentState != models.PowerStateOn && 
	   server.CurrentState != models.PowerStateWaking && 
	   server.CurrentState != models.PowerStateSuspended {
		return fmt.Errorf("server %s cannot be stopped from current state: %s", server.Name, server.CurrentState)
	}

	return pm.stopProxmoxVM(server)
}

// wakeProxmoxVM starts a Proxmox VM using the API
func (pm *PowerManager) wakeProxmoxVM(server *models.Server) error {
	pm.logger.Infof("Starting Proxmox VM: %s (VMID: %d)", server.Name, server.ProxmoxVMID)

	// Get the parent Proxmox host
	parentServer, err := pm.storage.GetServer(server.ParentServerID)
	if err != nil {
		return fmt.Errorf("failed to get parent Proxmox host %s: %w", server.ParentServerID, err)
	}

	// Ensure parent is awake first
	if parentServer.CurrentState != models.PowerStateOn {
		pm.logger.Infof("Waking parent Proxmox host %s first", parentServer.Name)
		if err := pm.WakeServer(parentServer); err != nil {
			return fmt.Errorf("failed to wake parent Proxmox host: %w", err)
		}
		// Wait for parent to become available
		time.Sleep(10 * time.Second)
	}

	// Make sure parent has API key
	if parentServer.ProxmoxAPIKey == nil {
		return fmt.Errorf("parent Proxmox host %s has no API key", parentServer.Name)
	}

	// Create Proxmox client
	client := proxmox.NewClient(
		parentServer.Hostname,
		parentServer.ProxmoxNodeName,
		parentServer.GetProxmoxAPIToken(),
		true, // Skip TLS verification
	)

	var taskID string

	// Choose the appropriate wake method based on current state
	if server.CurrentState == models.PowerStateSuspended {
		// Resume from suspended/paused state
		pm.logger.Infof("Resuming suspended Proxmox VM %s", server.Name)
		taskID, err = client.ResumeVM(server.ProxmoxVMID)
	} else {
		// Start from stopped state (or unknown)
		pm.logger.Infof("Starting stopped Proxmox VM %s", server.Name)
		taskID, err = client.StartVM(server.ProxmoxVMID)
	}
	
	// Log the action
	action := models.ServerAction{
		Timestamp:   time.Now(),
		Action:      models.ActionTypeWakeUp,
		Success:     err == nil,
		InitiatedBy: "manual",
	}

	if err != nil {
		action.ErrorMsg = err.Error()
		pm.logger.Errorf("Failed to start Proxmox VM %s: %v", server.Name, err)
	} else {
		pm.logger.Infof("Successfully started Proxmox VM %s (Task: %s)", server.Name, taskID)
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

// suspendProxmoxVM pauses/suspends a Proxmox VM using the API (preserves RAM)
func (pm *PowerManager) suspendProxmoxVM(server *models.Server) error {
	pm.logger.Infof("Suspending/Pausing Proxmox VM: %s (VMID: %d)", server.Name, server.ProxmoxVMID)

	// Get the parent Proxmox host
	parentServer, err := pm.storage.GetServer(server.ParentServerID)
	if err != nil {
		return fmt.Errorf("failed to get parent Proxmox host %s: %w", server.ParentServerID, err)
	}

	// Make sure parent has API key and is accessible
	if parentServer.ProxmoxAPIKey == nil {
		return fmt.Errorf("parent Proxmox host %s has no API key", parentServer.Name)
	}

	if parentServer.CurrentState != models.PowerStateOn {
		return fmt.Errorf("parent Proxmox host %s is not online", parentServer.Name)
	}

	// Create Proxmox client
	client := proxmox.NewClient(
		parentServer.Hostname,
		parentServer.ProxmoxNodeName,
		parentServer.GetProxmoxAPIToken(),
		true, // Skip TLS verification
	)

	// Pause/suspend the VM (preserves RAM state)
	taskID, err := client.PauseVM(server.ProxmoxVMID)
	
	// Log the action
	action := models.ServerAction{
		Timestamp:   time.Now(),
		Action:      models.ActionTypeSuspend,
		Success:     err == nil,
		InitiatedBy: "manual",
	}

	if err != nil {
		action.ErrorMsg = err.Error()
		pm.logger.Errorf("Failed to suspend Proxmox VM %s: %v", server.Name, err)
	} else {
		pm.logger.Infof("Successfully suspended Proxmox VM %s (Task: %s)", server.Name, taskID)
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

// shutdownProxmoxVM performs a clean shutdown of a Proxmox VM using the API
func (pm *PowerManager) shutdownProxmoxVM(server *models.Server) error {
	pm.logger.Infof("Shutting down Proxmox VM: %s (VMID: %d)", server.Name, server.ProxmoxVMID)

	// Get the parent Proxmox host
	parentServer, err := pm.storage.GetServer(server.ParentServerID)
	if err != nil {
		return fmt.Errorf("failed to get parent Proxmox host %s: %w", server.ParentServerID, err)
	}

	// Make sure parent has API key and is accessible
	if parentServer.ProxmoxAPIKey == nil {
		return fmt.Errorf("parent Proxmox host %s has no API key", parentServer.Name)
	}

	if parentServer.CurrentState != models.PowerStateOn {
		return fmt.Errorf("parent Proxmox host %s is not online", parentServer.Name)
	}

	// Create Proxmox client
	client := proxmox.NewClient(
		parentServer.Hostname,
		parentServer.ProxmoxNodeName,
		parentServer.GetProxmoxAPIToken(),
		true, // Skip TLS verification
	)

	// Graceful shutdown the VM
	taskID, err := client.ShutdownVM(server.ProxmoxVMID)
	
	// Log the action
	action := models.ServerAction{
		Timestamp:   time.Now(),
		Action:      models.ActionTypeShutdown,
		Success:     err == nil,
		InitiatedBy: "manual",
	}

	if err != nil {
		action.ErrorMsg = err.Error()
		pm.logger.Errorf("Failed to shutdown Proxmox VM %s: %v", server.Name, err)
	} else {
		pm.logger.Infof("Successfully shutdown Proxmox VM %s (Task: %s)", server.Name, taskID)
		// Update desired state to stopped
		if updateErr := pm.storage.UpdateServerState(server.ID, models.PowerStateStopped); updateErr != nil {
			pm.logger.Errorf("Failed to update server state: %v", updateErr)
		}
	}

	// Add action to server history
	if actionErr := pm.storage.AddServerAction(server.ID, action); actionErr != nil {
		pm.logger.Errorf("Failed to log server action: %v", actionErr)
	}

	return err
}

// stopProxmoxVM performs a hard stop of a Proxmox VM using the API
func (pm *PowerManager) stopProxmoxVM(server *models.Server) error {
	pm.logger.Infof("Force stopping Proxmox VM: %s (VMID: %d)", server.Name, server.ProxmoxVMID)

	// Get the parent Proxmox host
	parentServer, err := pm.storage.GetServer(server.ParentServerID)
	if err != nil {
		return fmt.Errorf("failed to get parent Proxmox host %s: %w", server.ParentServerID, err)
	}

	// Make sure parent has API key and is accessible
	if parentServer.ProxmoxAPIKey == nil {
		return fmt.Errorf("parent Proxmox host %s has no API key", parentServer.Name)
	}

	if parentServer.CurrentState != models.PowerStateOn {
		return fmt.Errorf("parent Proxmox host %s is not online", parentServer.Name)
	}

	// Create Proxmox client
	client := proxmox.NewClient(
		parentServer.Hostname,
		parentServer.ProxmoxNodeName,
		parentServer.GetProxmoxAPIToken(),
		true, // Skip TLS verification
	)

	// Force stop the VM
	taskID, err := client.StopVM(server.ProxmoxVMID)
	
	// Log the action
	action := models.ServerAction{
		Timestamp:   time.Now(),
		Action:      models.ActionTypeStop,
		Success:     err == nil,
		InitiatedBy: "manual",
	}

	if err != nil {
		action.ErrorMsg = err.Error()
		pm.logger.Errorf("Failed to stop Proxmox VM %s: %v", server.Name, err)
	} else {
		pm.logger.Infof("Successfully stopped Proxmox VM %s (Task: %s)", server.Name, taskID)
		// Update desired state to stopped
		if updateErr := pm.storage.UpdateServerState(server.ID, models.PowerStateStopped); updateErr != nil {
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
