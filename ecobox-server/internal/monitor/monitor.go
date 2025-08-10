package monitor

import (
	"sync"
	"time"

	"ecobox-server/internal/config"
	"ecobox-server/internal/control"
	"ecobox-server/internal/initializer"
	"ecobox-server/internal/models"
	"ecobox-server/internal/storage"
	"github.com/sirupsen/logrus"
)

// Monitor handles server monitoring and power state reconciliation
type Monitor struct {
	config         *config.Config
	storage        storage.Storage
	pingChecker    *PingChecker
	portScanner    *PortScanner
	powerManager   *control.PowerManager
	initManager    *initializer.Manager
	updateChan     chan ServerUpdate
	stopChan       chan struct{}
	logger         *logrus.Logger
	running        bool
	mu             sync.RWMutex
}

// ServerUpdate represents a server state update
type ServerUpdate struct {
	ServerID string               `json:"server_id"`
	State    models.PowerState    `json:"state"`
	Services []models.Service     `json:"services"`
	Server   *models.Server       `json:"server"`
}

// NewMonitor creates a new monitor instance
func NewMonitor(cfg *config.Config, storage storage.Storage, powerManager *control.PowerManager) *Monitor {
	initManager := initializer.NewManager(storage)
	
	return &Monitor{
		config:       cfg,
		storage:      storage,
		pingChecker:  NewPingChecker(),
		portScanner:  NewPortScanner(),
		powerManager: powerManager,
		initManager:  initManager,
		updateChan:   make(chan ServerUpdate, 100),
		stopChan:     make(chan struct{}),
		logger:       logrus.New(),
		running:      false,
	}
}

// Start begins monitoring background processes
func (m *Monitor) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		m.logger.Warn("Monitor is already running")
		return
	}

	m.running = true
	m.logger.Info("Starting server monitor")

	// Start status checking goroutine
	go m.statusCheckLoop()

	// Start power state reconciliation goroutine
	go m.reconcileLoop()
}

// Stop gracefully stops all monitoring processes
func (m *Monitor) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.logger.Info("Stopping server monitor")
	close(m.stopChan)
	m.running = false
}

// GetUpdates returns the update channel for server state changes
func (m *Monitor) GetUpdates() <-chan ServerUpdate {
	return m.updateChan
}

// statusCheckLoop runs the main monitoring loop
func (m *Monitor) statusCheckLoop() {
	ticker := time.NewTicker(time.Duration(m.config.Dashboard.UpdateInterval) * time.Second)
	defer ticker.Stop()

	// Perform initial check
	m.checkAllServers()

	for {
		select {
		case <-ticker.C:
			m.checkAllServers()
		case <-m.stopChan:
			m.logger.Info("Status check loop stopped")
			return
		}
	}
}

// reconcileLoop handles power state reconciliation
func (m *Monitor) reconcileLoop() {
	ticker := time.NewTicker(time.Duration(m.config.Dashboard.WoLRetryInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.reconcileAllServers()
		case <-m.stopChan:
			m.logger.Info("Reconcile loop stopped")
			return
		}
	}
}

// checkAllServers checks the status of all servers
func (m *Monitor) checkAllServers() {
	servers := m.storage.GetAllServers()
	
	for _, server := range servers {
		go m.checkServerStatus(server)
	}
}

// checkServerStatus determines and updates server power state
func (m *Monitor) checkServerStatus(server *models.Server) {
	oldState := server.CurrentState
	newState := m.determineServerState(server)
	
	// Update services status
	updatedServices := m.portScanner.ScanServices(server.Hostname, server.Services)
	server.Services = updatedServices

	// Update server state if changed
	if newState != oldState {
		m.logger.Infof("Server %s state changed: %s -> %s", server.Name, oldState, newState)
		
		if err := m.storage.UpdateServerState(server.ID, newState); err != nil {
			m.logger.Errorf("Failed to update server state for %s: %v", server.Name, err)
			return
		}
		
		// Update the server object
		server.CurrentState = newState
	}

	// Update server services
	if err := m.storage.UpdateServer(server); err != nil {
		m.logger.Errorf("Failed to update server %s: %v", server.Name, err)
	}

	// Send update notification
	select {
	case m.updateChan <- ServerUpdate{
		ServerID: server.ID,
		State:    newState,
		Services: updatedServices,
		Server:   server,
	}:
	default:
		// Channel is full, skip this update
	}
}

// determineServerState determines server state using ping and port scanning
func (m *Monitor) determineServerState(server *models.Server) models.PowerState {
	timeout := 5 * time.Second

	// First, try ICMP ping equivalent (TCP connectivity test)
	if m.pingChecker.PingHost(server.Hostname, timeout) {
		return models.PowerStateOn
	}

	// If ping fails, check if any services respond
	if m.portScanner.QuickScan(server.Hostname) {
		return models.PowerStateOn
	}

	// Check specific services
	hasRunningServices := false
	for _, service := range server.Services {
		if m.portScanner.ScanPort(server.Hostname, service.Port, timeout) {
			hasRunningServices = true
			break
		}
	}

	if hasRunningServices {
		return models.PowerStateOn
	}

	// If we can't detect the server, determine if it's off or suspended
	// This is a best guess - in reality, we can't easily distinguish between off and suspended
	// without additional information or wake-on-LAN testing
	if server.CurrentState == models.PowerStateSuspended {
		return models.PowerStateSuspended
	}

	return models.PowerStateOff
}

// reconcileAllServers handles power state reconciliation for all servers
func (m *Monitor) reconcileAllServers() {
	servers := m.storage.GetAllServers()
	
	for _, server := range servers {
		if server.CurrentState != server.DesiredState {
			go m.reconcileServerState(server)
		}
	}
}

// reconcileServerState reconciles a single server's power state
func (m *Monitor) reconcileServerState(server *models.Server) {
	m.logger.Infof("Reconciling power state for %s: current=%s, desired=%s, initialized=%t", 
		server.Name, server.CurrentState, server.DesiredState, server.Initialized)

	// Check if server needs initialization
	if !server.Initialized {
		m.logger.Infof("Server %s is not initialized, running initialization", server.Name)
		if err := m.initManager.InitializeServer(server); err != nil {
			m.logger.Errorf("Failed to initialize server %s: %v", server.Name, err)
			
			// Log failed initialization action
			action := models.ServerAction{
				Timestamp:   time.Now(),
				Action:      models.ActionTypeInitialize,
				Success:     false,
				ErrorMsg:    err.Error(),
				InitiatedBy: "system",
			}
			m.storage.AddServerAction(server.ID, action)
			return
		}
		// Server was updated by InitializeServer, so get the latest version
		updatedServer, err := m.storage.GetServer(server.ID)
		if err != nil {
			m.logger.Errorf("Failed to get updated server %s after initialization: %v", server.Name, err)
			return
		}
		server = updatedServer
	}

	// Perform power state reconciliation
	switch server.DesiredState {
	case models.PowerStateOn:
		if server.CurrentState == models.PowerStateOff || server.CurrentState == models.PowerStateSuspended {
			m.logger.Infof("Attempting to wake server %s", server.Name)
			
			action := models.ServerAction{
				Timestamp:   time.Now(),
				Action:      models.ActionTypeReconcile,
				Success:     false,
				InitiatedBy: "reconciler",
			}
			
			if err := m.powerManager.WakeServer(server); err != nil {
				m.logger.Errorf("Failed to wake server %s: %v", server.Name, err)
				action.ErrorMsg = err.Error()
			} else {
				m.logger.Infof("Successfully sent wake command to server %s", server.Name)
				action.Success = true
			}
			
			// Log the reconciliation action
			if err := m.storage.AddServerAction(server.ID, action); err != nil {
				m.logger.Errorf("Failed to log reconciliation action for %s: %v", server.Name, err)
			}
		}
		
	case models.PowerStateSuspended:
		if server.CurrentState == models.PowerStateOn {
			m.logger.Infof("Attempting to suspend server %s", server.Name)
			
			action := models.ServerAction{
				Timestamp:   time.Now(),
				Action:      models.ActionTypeReconcile,
				Success:     false,
				InitiatedBy: "reconciler",
			}
			
			if err := m.powerManager.SuspendServer(server); err != nil {
				m.logger.Errorf("Failed to suspend server %s: %v", server.Name, err)
				action.ErrorMsg = err.Error()
			} else {
				m.logger.Infof("Successfully sent suspend command to server %s", server.Name)
				action.Success = true
			}
			
			// Log the reconciliation action
			if err := m.storage.AddServerAction(server.ID, action); err != nil {
				m.logger.Errorf("Failed to log reconciliation action for %s: %v", server.Name, err)
			}
		}
	}
}

// SetLogger sets a custom logger
func (m *Monitor) SetLogger(logger *logrus.Logger) {
	m.logger = logger
	m.initManager.SetLogger(logger)
}

// IsRunning returns whether the monitor is currently running
func (m *Monitor) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// ForceCheck triggers an immediate check of all servers
func (m *Monitor) ForceCheck() {
	go m.checkAllServers()
}

// ForceReconcile triggers an immediate reconciliation of all servers
func (m *Monitor) ForceReconcile() {
	go m.reconcileAllServers()
}
