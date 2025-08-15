package monitor

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"ecobox-server/internal/command"
	"ecobox-server/internal/config"
	"ecobox-server/internal/control"
	"ecobox-server/internal/initializer"
	"ecobox-server/internal/metrics"
	"ecobox-server/internal/models"
	"ecobox-server/internal/proxmox"
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
	systemMonitor  *control.SystemMonitor
	initManager    *initializer.Manager
	metricsManager *metrics.Manager
	commander      *command.Commander
	updateChan     chan ServerUpdate
	stopChan       chan struct{}
	logger         *logrus.Logger
	running        bool
	mu             sync.RWMutex
	
	// Timing control for system checks
	lastSystemCheck  map[string]time.Time
	lastInitCheck    map[string]time.Time
	
	// Initialization constants
	maxInitRetries       int           // Maximum number of initialization attempts before giving up
	reinitInterval       time.Duration // How often to clear init state and retry (even for failed servers)
	
	// Proxmox-specific settings
	vmDiscoveryInterval  time.Duration // How often to discover VMs on Proxmox hosts
}

// ServerUpdate represents a server state update
type ServerUpdate struct {
	ServerID string               `json:"server_id"`
	State    models.PowerState    `json:"state"`
	Services []models.Service     `json:"services"`
	Server   *models.Server       `json:"server"`
	Metrics  map[string]float64   `json:"metrics,omitempty"`
}

// NewMonitor creates a new monitor instance
func NewMonitor(cfg *config.Config, storage storage.Storage, powerManager *control.PowerManager) *Monitor {
	initManager := initializer.NewManager(storage)
	systemMonitor := control.NewSystemMonitor(storage, logrus.New())
	
	// Create SSH client for command execution
	sshClient := control.NewSSHClient()
	commander := command.NewCommander(sshClient, logrus.New())
	
	// Initialize metrics manager
	metricsConfig := metrics.ManagerConfig{
		BaseDataDir:   cfg.Dashboard.MetricsDataDir,
		FlushInterval: time.Duration(cfg.Dashboard.MetricsFlushInterval) * time.Second,
	}
	metricsManager, err := metrics.NewManager(metricsConfig)
	if err != nil {
		logrus.Errorf("Failed to initialize metrics manager: %v", err)
		// Continue without metrics rather than failing completely
		metricsManager = nil
	}
	
	return &Monitor{
		config:              cfg,
		storage:             storage,
		pingChecker:         NewPingChecker(),
		portScanner:         NewPortScanner(),
		powerManager:        powerManager,
		systemMonitor:       systemMonitor,
		initManager:         initManager,
		metricsManager:      metricsManager,
		commander:           commander,
		updateChan:          make(chan ServerUpdate, 100),
		stopChan:            make(chan struct{}),
		logger:              logrus.New(),
		running:             false,
		lastSystemCheck:     make(map[string]time.Time),
		lastInitCheck:       make(map[string]time.Time),
		maxInitRetries:      3,
		reinitInterval:      1 * time.Hour, // Clear init state and retry every hour
		vmDiscoveryInterval: time.Duration(cfg.Dashboard.VMDiscoveryInterval) * time.Second,
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
	
	// Start system monitoring goroutines
	go m.systemCheckLoop()
	go m.initializationCheckLoop()
	
	// Start Proxmox VM discovery loop
	go m.proxmoxDiscoveryLoop()
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
	
	// Close metrics manager
	if m.metricsManager != nil {
		if err := m.metricsManager.Close(); err != nil {
			m.logger.Errorf("Failed to close metrics manager: %v", err)
		}
	}
}

// GetUpdates returns the update channel for server state changes
func (m *Monitor) GetUpdates() <-chan ServerUpdate {
	return m.updateChan
}

// GetMetricsManager returns the metrics manager for external access
func (m *Monitor) GetMetricsManager() *metrics.Manager {
	return m.metricsManager
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
	
	// Record monitoring cycle metrics
	m.recordMetric("_system", metrics.StandardMetrics.MonitoringCycle, 1)
	m.recordMetric("_system", metrics.StandardMetrics.MonitoringServerCount, float64(len(servers)))
	
	for _, server := range servers {
		go m.checkServerStatus(server)
	}
}

// checkServerStatus determines and updates server power state
func (m *Monitor) checkServerStatus(server *models.Server) {
	oldState := server.CurrentState
	var newState models.PowerState
	var updatedServices []models.Service
	
	// Handle Proxmox VMs differently from regular servers
	if server.IsProxmoxVM {
		newState, updatedServices = m.checkProxmoxVMStatus(server)
	} else {
		newState = m.determineServerState(server)
		// Update services status for regular servers - now includes discovery
		if newState == models.PowerStateOn {
			// Only perform comprehensive scanning if server is online
			updatedServices = m.portScanner.ScanServicesWithDiscovery(server.Hostname, server.ID, server.Services)
		} else {
			// If server is offline, just scan configured services
			updatedServices = m.portScanner.ScanServices(server.Hostname, server.Services)
		}
	}
	
	server.Services = updatedServices

	// Record metrics for state changes
	if newState != oldState {
		m.logger.Infof("Server %s state changed: %s -> %s", server.Name, oldState, newState)
		
		// Record state change metrics
		m.recordMetric(server.ID, metrics.StandardMetrics.PowerStateChange, 1)
		switch newState {
		case models.PowerStateOn:
			m.recordMetric(server.ID, metrics.StandardMetrics.PowerStateOn, 1)
		case models.PowerStateOff:
			m.recordMetric(server.ID, metrics.StandardMetrics.PowerStateOff, 1)
		case models.PowerStateStopped:
			m.recordMetric(server.ID, metrics.StandardMetrics.PowerStateStopped, 1)
		case models.PowerStateSuspended:
			m.recordMetric(server.ID, metrics.StandardMetrics.PowerStateSuspended, 1)
		case models.PowerStateInitFailed:
			m.recordMetric(server.ID, metrics.StandardMetrics.PowerStateInitFailed, 1)
		case models.PowerStateWaking:
			m.recordMetric(server.ID, metrics.StandardMetrics.PowerStateWaking, 1)
		case models.PowerStateSuspending:
			m.recordMetric(server.ID, metrics.StandardMetrics.PowerStateSuspending, 1)
		case models.PowerStateStopping:
			m.recordMetric(server.ID, metrics.StandardMetrics.PowerStateStopping, 1)
		}
		
		if err := m.storage.UpdateServerState(server.ID, newState); err != nil {
			m.logger.Errorf("Failed to update server state for %s: %v", server.Name, err)
			m.recordMetric(server.ID, metrics.StandardMetrics.StateUpdateError, 1)
			return
		}
		
		// Update the server object
		server.CurrentState = newState
	}

	// Record service availability metrics
	onlineServices := 0
	for _, service := range updatedServices {
		if service.Status == models.ServiceStatusUp {
			onlineServices++
		}
	}
	if len(updatedServices) > 0 {
		serviceAvailabilityPct := (float64(onlineServices) / float64(len(updatedServices))) * 100
		m.recordMetric(server.ID, metrics.StandardMetrics.ServiceAvailability, serviceAvailabilityPct)
	}

	// Update server services
	if err := m.storage.UpdateServer(server); err != nil {
		m.logger.Errorf("Failed to update server %s: %v", server.Name, err)
	}

	// Get current metrics for this server
	metrics, err := m.metricsManager.GetLatestValues(server.ID)
	if err != nil {
		m.logger.Errorf("Failed to get latest metrics for server %s: %v", server.ID, err)
		metrics = make(map[string]float64) // Use empty metrics on error
	} else {
		m.logger.Debugf("Got %d metrics for server %s: %+v", len(metrics), server.ID, metrics)
	}

	// Send update notification
	select {
	case m.updateChan <- ServerUpdate{
		ServerID: server.ID,
		State:    newState,
		Services: updatedServices,
		Server:   server,
		Metrics:  metrics,
	}:
	default:
		// Channel is full, skip this update
	}
}

// determineServerState determines server state using ping and port scanning
func (m *Monitor) determineServerState(server *models.Server) models.PowerState {
	timeout := 5 * time.Second

	// Don't change state if server is in init_failed and hasn't been reset
	if server.CurrentState == models.PowerStateInitFailed {
		// Check if enough time has passed to allow state detection again
		if !server.LastInitAttempt.IsZero() && time.Since(server.LastInitAttempt) < m.reinitInterval {
			return models.PowerStateInitFailed
		}
	}

	// Handle transitioning states - check if the transition completed
	switch server.CurrentState {
	case models.PowerStateWaking:
		// Server was waking - check if it's now online
		if m.pingChecker.PingHost(server.Hostname, timeout) || m.portScanner.QuickScan(server.Hostname) {
			m.logger.Infof("Server %s successfully completed wake operation", server.Name)
			return models.PowerStateOn
		}
		// Still waking - check if we should timeout the wake operation
		if !server.LastStateChange.IsZero() && time.Since(server.LastStateChange) > 5*time.Minute {
			m.logger.Warnf("Server %s wake operation timed out, reverting to off state", server.Name)
			return models.PowerStateOff
		}
		// Keep waking state
		return models.PowerStateWaking
		
	case models.PowerStateSuspending:
		// Server was suspending - check if it's now offline
		if !m.pingChecker.PingHost(server.Hostname, timeout) && !m.portScanner.QuickScan(server.Hostname) {
			m.logger.Infof("Server %s successfully completed suspend operation", server.Name)
			return models.PowerStateSuspended
		}
		// Still suspending - check if we should timeout the suspend operation
		if !server.LastStateChange.IsZero() && time.Since(server.LastStateChange) > 2*time.Minute {
			m.logger.Warnf("Server %s suspend operation timed out, reverting to on state", server.Name)
			return models.PowerStateOn
		}
		// Keep suspending state
		return models.PowerStateSuspending
		
	case models.PowerStateStopping:
		// Server was stopping - check if it's now offline
		if !m.pingChecker.PingHost(server.Hostname, timeout) && !m.portScanner.QuickScan(server.Hostname) {
			m.logger.Infof("Server %s successfully completed stop operation", server.Name)
			return models.PowerStateStopped
		}
		// Still stopping - check if we should timeout the stop operation
		if !server.LastStateChange.IsZero() && time.Since(server.LastStateChange) > 2*time.Minute {
			m.logger.Warnf("Server %s stop operation timed out, reverting to on state", server.Name)
			return models.PowerStateOn
		}
		// Keep stopping state
		return models.PowerStateStopping
	}

	// Normal state detection logic
	// First, try ICMP ping equivalent (TCP connectivity test)
	if m.pingChecker.PingHost(server.Hostname, timeout) {
		return models.PowerStateOn
	}

	// If ping fails, check if any services respond
	if m.portScanner.QuickScan(server.Hostname) {
		return models.PowerStateOn
	}

	// Check configured services first
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
		shouldReconcile := false
		
		// Always reconcile if desired state doesn't match current state
		if server.CurrentState != server.DesiredState {
			shouldReconcile = true
		}
		
		// Also reconcile if server needs initialization (including periodic re-init)
		if m.shouldAttemptInitialization(server) {
			shouldReconcile = true
		}
		
		if shouldReconcile {
			go m.reconcileServerState(server)
		}
	}
}

// reconcileServerState reconciles a single server's power state
func (m *Monitor) reconcileServerState(server *models.Server) {
	m.logger.Infof("Reconciling power state for %s: current=%s, desired=%s, initialized=%t", 
		server.Name, server.CurrentState, server.DesiredState, server.Initialized)

	// Check if server needs initialization
	// However, allow power management for suspended/off servers even if not initialized
	needsInit := m.shouldAttemptInitialization(server)
	canDoPowerOps := server.Initialized || 
		server.CurrentState == models.PowerStateSuspended || 
		server.CurrentState == models.PowerStateOff ||
		server.IsProxmoxVM  // Proxmox VMs don't need SSH initialization for power ops
	
	if needsInit && server.CurrentState == models.PowerStateOn {
		// Only require initialization for online servers (we need SSH for monitoring)
		if m.attemptServerInitialization(server) {
			// Server was updated by initialization, so get the latest version
			updatedServer, err := m.storage.GetServer(server.ID)
			if err != nil {
				m.logger.Errorf("Failed to get updated server %s after initialization: %v", server.Name, err)
				return
			}
			server = updatedServer
		} else {
			// Initialization failed for online server, don't continue
			return
		}
	} else if needsInit && !canDoPowerOps {
		// Server is in unknown state and not initialized - try init anyway
		m.logger.Infof("Server %s not initialized and state unknown, attempting initialization", server.Name)
		if !m.attemptServerInitialization(server) {
			// Initialization failed, don't continue
			return
		}
		// Get updated server after init
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
		if server.CurrentState == models.PowerStateOff || 
		   server.CurrentState == models.PowerStateStopped ||
		   server.CurrentState == models.PowerStateSuspended ||
		   server.CurrentState == models.PowerStateUnknown {
			m.logger.Infof("Attempting to wake server %s (current: %s)", server.Name, server.CurrentState)
			
			// Set transitioning state to indicate wake operation in progress
			if err := m.storage.UpdateServerState(server.ID, models.PowerStateWaking); err != nil {
				m.logger.Errorf("Failed to set waking state for server %s: %v", server.Name, err)
			} else {
				server.CurrentState = models.PowerStateWaking
			}
			
			// Record wake attempt metric
			m.recordMetric(server.ID, metrics.StandardMetrics.WakeAttempt, 1)
			
			action := models.ServerAction{
				Timestamp:   time.Now(),
				Action:      models.ActionTypeReconcile,
				Success:     false,
				InitiatedBy: "reconciler",
			}
			
			startTime := time.Now()
			err := m.powerManager.WakeServer(server)
			wakeDuration := time.Since(startTime)
			
			// Record timing metrics
			m.recordMetric(server.ID, metrics.StandardMetrics.WakeDuration, wakeDuration.Seconds())
			
			if err != nil {
				m.logger.Errorf("Failed to wake server %s: %v", server.Name, err)
				m.recordMetric(server.ID, metrics.StandardMetrics.WakeFailure, 1)
				action.ErrorMsg = err.Error()
				
				// Revert to previous state on failure (best guess)
				previousState := models.PowerStateUnknown
				if server.CurrentState == models.PowerStateWaking {
					// Try to determine what state it was before
					if server.IsProxmoxVM {
						// For VMs, we can check via API
						previousState = models.PowerStateOff
					} else {
						// For physical servers, assume it was off/suspended
						previousState = models.PowerStateOff
					}
				}
				if updateErr := m.storage.UpdateServerState(server.ID, previousState); updateErr != nil {
					m.logger.Errorf("Failed to revert server state after wake failure for %s: %v", server.Name, updateErr)
				}
			} else {
				m.logger.Infof("Successfully sent wake command to server %s", server.Name)
				m.recordMetric(server.ID, metrics.StandardMetrics.WakeSuccess, 1)
				action.Success = true
				
				// Keep waking state - will be updated by status check when server comes online
			}
			
			// Log the reconciliation action
			if err := m.storage.AddServerAction(server.ID, action); err != nil {
				m.logger.Errorf("Failed to log reconciliation action for %s: %v", server.Name, err)
			}
		}
		
	case models.PowerStateSuspended:
		if server.CurrentState == models.PowerStateOn {
			m.logger.Infof("Attempting to suspend server %s", server.Name)
			
			// Set transitioning state to indicate suspend operation in progress
			if err := m.storage.UpdateServerState(server.ID, models.PowerStateSuspending); err != nil {
				m.logger.Errorf("Failed to set suspending state for server %s: %v", server.Name, err)
			} else {
				server.CurrentState = models.PowerStateSuspending
			}
			
			// Record suspend attempt metric
			m.recordMetric(server.ID, metrics.StandardMetrics.SuspendAttempt, 1)
			
			action := models.ServerAction{
				Timestamp:   time.Now(),
				Action:      models.ActionTypeReconcile,
				Success:     false,
				InitiatedBy: "reconciler",
			}
			
			startTime := time.Now()
			err := m.powerManager.SuspendServer(server)
			suspendDuration := time.Since(startTime)
			
			// Record timing metrics
			m.recordMetric(server.ID, metrics.StandardMetrics.SuspendDuration, suspendDuration.Seconds())
			
			if err != nil {
				m.logger.Errorf("Failed to suspend server %s: %v", server.Name, err)
				m.recordMetric(server.ID, metrics.StandardMetrics.SuspendFailure, 1)
				action.ErrorMsg = err.Error()
				
				// Revert to online state on failure
				if updateErr := m.storage.UpdateServerState(server.ID, models.PowerStateOn); updateErr != nil {
					m.logger.Errorf("Failed to revert server state after suspend failure for %s: %v", server.Name, updateErr)
				}
			} else {
				m.logger.Infof("Successfully sent suspend command to server %s", server.Name)
				m.recordMetric(server.ID, metrics.StandardMetrics.SuspendSuccess, 1)
				action.Success = true
				
				// Keep suspending state - will be updated by status check when server goes offline
			}
			
			// Log the reconciliation action
			if err := m.storage.AddServerAction(server.ID, action); err != nil {
				m.logger.Errorf("Failed to log reconciliation action for %s: %v", server.Name, err)
			}
		}
		
	case models.PowerStateStopped:
		// Only meaningful for Proxmox VMs - clean shutdown (graceful)
		if server.CurrentState == models.PowerStateOn && server.IsProxmoxVM {
			m.logger.Infof("Attempting to shutdown Proxmox VM %s", server.Name)
			
			// Set transitioning state to indicate shutdown operation in progress
			if err := m.storage.UpdateServerState(server.ID, models.PowerStateStopping); err != nil {
				m.logger.Errorf("Failed to set stopping state for server %s: %v", server.Name, err)
			} else {
				server.CurrentState = models.PowerStateStopping
			}
			
			// Record shutdown attempt metric
			m.recordMetric(server.ID, metrics.StandardMetrics.SuspendAttempt, 1) // Reuse suspend metric for now
			
			action := models.ServerAction{
				Timestamp:   time.Now(),
				Action:      models.ActionTypeReconcile,
				Success:     false,
				InitiatedBy: "reconciler",
			}
			
			startTime := time.Now()
			err := m.powerManager.ShutdownServer(server)
			shutdownDuration := time.Since(startTime)
			
			// Record timing metrics
			m.recordMetric(server.ID, metrics.StandardMetrics.SuspendDuration, shutdownDuration.Seconds())
			
			if err != nil {
				m.logger.Errorf("Failed to shutdown server %s: %v", server.Name, err)
				m.recordMetric(server.ID, metrics.StandardMetrics.SuspendFailure, 1)
				action.ErrorMsg = err.Error()
				
				// Revert to online state on failure
				if updateErr := m.storage.UpdateServerState(server.ID, models.PowerStateOn); updateErr != nil {
					m.logger.Errorf("Failed to revert server state after shutdown failure for %s: %v", server.Name, updateErr)
				}
			} else {
				m.logger.Infof("Successfully sent shutdown command to server %s", server.Name)
				m.recordMetric(server.ID, metrics.StandardMetrics.SuspendSuccess, 1)
				action.Success = true
				
				// Keep stopping state - will be updated by status check when server goes offline
			}
			
			// Log the reconciliation action
			if err := m.storage.AddServerAction(server.ID, action); err != nil {
				m.logger.Errorf("Failed to log reconciliation action for %s: %v", server.Name, err)
			}
		}
	}
}

// shouldAttemptInitialization determines if we should try to initialize a server
func (m *Monitor) shouldAttemptInitialization(server *models.Server) bool {
	// Skip initialization for Proxmox VMs - they get data from API, not SSH
	if server.IsProxmoxVM {
		return false
	}
	
	// Always attempt if not initialized and not in failed state
	if !server.Initialized && server.CurrentState != models.PowerStateInitFailed {
		return true
	}

	// Check if enough time has passed since last successful init to do periodic re-initialization
	if server.Initialized && !server.LastSuccessfulInit.IsZero() {
		timeSinceLastInit := time.Since(server.LastSuccessfulInit)
		if timeSinceLastInit >= m.reinitInterval {
			m.logger.Infof("Server %s is due for periodic re-initialization (last successful init: %v ago)", 
				server.Name, timeSinceLastInit)
			return true
		}
	}

	// Check if enough time has passed since failure to retry failed initialization
	if server.CurrentState == models.PowerStateInitFailed && !server.LastInitAttempt.IsZero() {
		timeSinceLastAttempt := time.Since(server.LastInitAttempt)
		if timeSinceLastAttempt >= m.reinitInterval {
			m.logger.Infof("Server %s initialization failed but enough time has passed to retry (last attempt: %v ago)", 
				server.Name, timeSinceLastAttempt)
			return true
		}
	}

	return false
}

// attemptServerInitialization attempts to initialize a server with retry logic
func (m *Monitor) attemptServerInitialization(server *models.Server) bool {
	// Check if we should reset the retry count (for periodic re-init)
	if server.Initialized || (!server.LastInitAttempt.IsZero() && time.Since(server.LastInitAttempt) >= m.reinitInterval) {
		m.logger.Infof("Resetting initialization state for server %s", server.Name)
		
		// Record metrics for initialization reset
		m.recordMetric(server.ID, metrics.StandardMetrics.InitStateReset, 1)
		
		// Reset initialization tracking
		server.InitRetryCount = 0
		server.Initialized = false
		
		// Update server state if it was in failed state
		if server.CurrentState == models.PowerStateInitFailed {
			// Set to unknown so it can be properly detected
			server.CurrentState = models.PowerStateUnknown
			if err := m.storage.UpdateServerState(server.ID, models.PowerStateUnknown); err != nil {
				m.logger.Errorf("Failed to update server state for %s: %v", server.Name, err)
			}
		}
		
		// Log the reset action
		action := models.ServerAction{
			Timestamp:   time.Now(),
			Action:      models.ActionTypeInitialize,
			Success:     true,
			InitiatedBy: "system",
			ErrorMsg:    "Reset initialization state for periodic re-init",
		}
		if err := m.storage.AddServerAction(server.ID, action); err != nil {
			m.logger.Errorf("Failed to log init reset action for %s: %v", server.Name, err)
		}
	}

	// Check if we've exceeded retry limit
	if server.InitRetryCount >= m.maxInitRetries {
		m.logger.Warnf("Server %s has exceeded maximum initialization attempts (%d), setting to init_failed state", 
			server.Name, m.maxInitRetries)
		
		// Record metrics for max retries exceeded
		m.recordMetric(server.ID, metrics.StandardMetrics.InitMaxRetriesExceeded, 1)
		m.recordMetric(server.ID, metrics.StandardMetrics.InitRetryCount, float64(server.InitRetryCount))
		
		// Update server state to init failed
		if err := m.storage.UpdateServerState(server.ID, models.PowerStateInitFailed); err != nil {
			m.logger.Errorf("Failed to update server state to init_failed for %s: %v", server.Name, err)
		}
		
		// Log the failed state action
		action := models.ServerAction{
			Timestamp:   time.Now(),
			Action:      models.ActionTypeInitialize,
			Success:     false,
			InitiatedBy: "system",
			ErrorMsg:    "Exceeded maximum initialization attempts",
		}
		if err := m.storage.AddServerAction(server.ID, action); err != nil {
			m.logger.Errorf("Failed to log init failed action for %s: %v", server.Name, err)
		}
		
		return false
	}

	// Attempt initialization
	m.logger.Infof("Server %s attempting initialization (attempt %d/%d)", 
		server.Name, server.InitRetryCount+1, m.maxInitRetries)
	
	// Record metrics for initialization attempt
	m.recordMetric(server.ID, metrics.StandardMetrics.InitAttempt, 1)
	m.recordMetric(server.ID, metrics.StandardMetrics.InitRetryCount, float64(server.InitRetryCount))
	
	// Update retry count and attempt time
	server.InitRetryCount++
	server.LastInitAttempt = time.Now()
	
	// Save updated retry tracking
	if err := m.storage.UpdateServer(server); err != nil {
		m.logger.Errorf("Failed to update server retry tracking for %s: %v", server.Name, err)
	}

	// Measure initialization time
	startTime := time.Now()
	err := m.initManager.InitializeServer(server)
	initDuration := time.Since(startTime)
	
	// Record timing metrics
	m.recordMetric(server.ID, metrics.StandardMetrics.InitDuration, initDuration.Seconds())

	if err != nil {
		m.logger.Errorf("Failed to initialize server %s (attempt %d/%d): %v", 
			server.Name, server.InitRetryCount, m.maxInitRetries, err)
		
		// Record failure metrics
		m.recordMetric(server.ID, metrics.StandardMetrics.InitFailure, 1)
		
		// Log failed initialization action
		action := models.ServerAction{
			Timestamp:   time.Now(),
			Action:      models.ActionTypeInitialize,
			Success:     false,
			ErrorMsg:    err.Error(),
			InitiatedBy: "system",
		}
		if err := m.storage.AddServerAction(server.ID, action); err != nil {
			m.logger.Errorf("Failed to log initialization failure for %s: %v", server.Name, err)
		}
		
		return false
	}

	// Initialization succeeded
	m.logger.Infof("Successfully initialized server %s on attempt %d", server.Name, server.InitRetryCount)
	
	// Record success metrics
	m.recordMetric(server.ID, metrics.StandardMetrics.InitSuccess, 1)
	m.recordMetric(server.ID, metrics.StandardMetrics.InitDuration, initDuration.Seconds())
	m.recordMetric(server.ID, metrics.StandardMetrics.InitRetryCount, float64(server.InitRetryCount))
	
	// Update success tracking
	server.LastSuccessfulInit = time.Now()
	if err := m.storage.UpdateServer(server); err != nil {
		m.logger.Errorf("Failed to update server success tracking for %s: %v", server.Name, err)
	}
	
	// Log successful initialization action
	action := models.ServerAction{
		Timestamp:   time.Now(),
		Action:      models.ActionTypeInitialize,
		Success:     true,
		InitiatedBy: "system",
	}
	if err := m.storage.AddServerAction(server.ID, action); err != nil {
		m.logger.Errorf("Failed to log initialization success for %s: %v", server.Name, err)
	}
	
	return true
}

// recordInitializationMetric records initialization-related metrics
func (m *Monitor) recordInitializationMetric(serverName, metricName string, value float64) {
	if m.metricsManager == nil {
		return // Metrics manager not available
	}
	
	if err := m.metricsManager.Push(serverName, metricName, value); err != nil {
		m.logger.Errorf("Failed to record metric %s for server %s: %v", metricName, serverName, err)
	}
}

// recordMetric is a convenience wrapper using the standard metrics names
func (m *Monitor) recordMetric(serverName, metricName string, value float64) {
	m.recordInitializationMetric(serverName, metricName, value)
}

// SetLogger sets a custom logger
func (m *Monitor) SetLogger(logger *logrus.Logger) {
	m.logger = logger
	m.initManager.SetLogger(logger)
	m.commander.SetLogger(logger)
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

// systemCheckLoop handles periodic system information gathering for online servers
func (m *Monitor) systemCheckLoop() {
	interval := time.Duration(m.config.Dashboard.SystemCheckInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	m.logger.WithField("interval", interval).Info("Starting system check loop")

	// Perform initial system checks
	m.performSystemChecks()

	for {
		select {
		case <-ticker.C:
			m.performSystemChecks()
		case <-m.stopChan:
			m.logger.Info("System check loop stopped")
			return
		}
	}
}

// initializationCheckLoop handles periodic initialization checks and re-initialization
func (m *Monitor) initializationCheckLoop() {
	interval := time.Duration(m.config.Dashboard.InitCheckInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	m.logger.WithField("interval", interval).Info("Starting initialization check loop")

	// Perform initial initialization checks
	m.performInitializationChecks()

	for {
		select {
		case <-ticker.C:
			m.performInitializationChecks()
		case <-m.stopChan:
			m.logger.Info("Initialization check loop stopped")
			return
		}
	}
}

// proxmoxDiscoveryLoop runs periodically to discover and manage Proxmox VMs
func (m *Monitor) proxmoxDiscoveryLoop() {
	m.logger.Info("Starting Proxmox VM discovery loop")
	ticker := time.NewTicker(m.vmDiscoveryInterval)
	defer ticker.Stop()

	// Perform initial discovery
	m.performProxmoxDiscovery()

	for {
		select {
		case <-ticker.C:
			m.performProxmoxDiscovery()
		case <-m.stopChan:
			m.logger.Info("Proxmox discovery loop stopped")
			return
		}
	}
}

// performProxmoxDiscovery discovers and manages Proxmox hosts and their VMs
func (m *Monitor) performProxmoxDiscovery() {
	servers := m.storage.GetAllServers()
	m.logger.Debugf("Performing Proxmox discovery on %d servers", len(servers))
	
	for _, server := range servers {
		m.logger.Debugf("Checking server %s: initialized=%v, state=%v, systemType=%v", 
			server.Name, server.Initialized, server.CurrentState, 
			func() interface{} { if server.SystemInfo != nil { return server.SystemInfo.Type } else { return "nil" } }())
			
		// Skip if not initialized or not online
		if !server.Initialized || server.CurrentState != models.PowerStateOn {
			m.logger.Debugf("Skipping server %s: not initialized or not online", server.Name)
			continue
		}
		
		// Check if this is a potential Proxmox host that needs API key setup
		// Only create API key if it doesn't exist (don't recreate if ForceReinitialization is set)
		needsAPIKeySetup := server.SystemInfo != nil && server.SystemInfo.Type == models.SystemTypeProxmox && 
			server.ProxmoxAPIKey == nil
		if needsAPIKeySetup {
			if server.ProxmoxAPIKey == nil {
				m.logger.Infof("Server %s is Proxmox and needs API key setup", server.Name)
			} else {
				m.logger.Infof("Server %s is Proxmox and forcing API key recreation due to config", server.Name)
			}
			m.setupProxmoxAPIKey(server)
		}
		
		// Discover VMs if this is a Proxmox host that should discover VMs
		if server.ShouldDiscoverVMs(m.vmDiscoveryInterval) {
			m.logger.Infof("Server %s should discover VMs", server.Name)
			m.discoverProxmoxVMs(server)
		}
	}
}

// setupProxmoxAPIKey creates and stores an API key for a Proxmox host
func (m *Monitor) setupProxmoxAPIKey(server *models.Server) {
	m.logger.WithField("server", server.Name).Info("Setting up Proxmox API key")
	
	// Create API key using SSH
	apiKey, err := m.commander.CreateProxmoxAPIKey(
		server.Hostname,
		server.SSHPort,
		server.SSHUser,
		server.SSHKeyPath,
	)
	if err != nil {
		m.logger.WithField("server", server.Name).
			WithError(err).
			Error("Failed to create Proxmox API key")
		return
	}
	
	// Store API key in server
	server.ProxmoxAPIKey = apiKey
	
	// Discover the actual node name by querying the API
	client := proxmox.NewClient(
		server.Hostname,
		"", // Empty node name for now
		server.GetProxmoxAPIToken(),
		true, // Skip TLS verification
	)
	
	nodes, err := client.ListNodes()
	if err != nil || len(nodes) == 0 {
		m.logger.WithField("server", server.Name).
			WithError(err).
			Warn("Failed to discover Proxmox node name, using hostname as fallback")
		server.ProxmoxNodeName = server.Hostname
	} else {
		// Use the first (and usually only) node name
		server.ProxmoxNodeName = nodes[0].Node
		m.logger.WithField("server", server.Name).
			WithField("node_name", server.ProxmoxNodeName).
			Info("Discovered Proxmox node name")
	}
	
	if err := m.storage.UpdateServer(server); err != nil {
		m.logger.WithField("server", server.Name).
			WithError(err).
			Error("Failed to store Proxmox API key")
		return
	}
	
	m.logger.WithField("server", server.Name).
		WithField("node_name", server.ProxmoxNodeName).
		Info("Successfully set up Proxmox API key")
}

// discoverProxmoxVMs discovers VMs on a Proxmox host and creates/updates server entries
func (m *Monitor) discoverProxmoxVMs(server *models.Server) {
	m.logger.WithField("server", server.Name).Debug("Discovering Proxmox VMs")
	
	// Create Proxmox client
	client := proxmox.NewClient(
		server.Hostname,
		server.ProxmoxNodeName,
		server.GetProxmoxAPIToken(),
		true, // Skip TLS verification for self-signed certs
	)
	
	// List VMs
	vms, err := client.ListVMs()
	if err != nil {
		m.logger.WithField("server", server.Name).
			WithError(err).
			Error("Failed to list Proxmox VMs")
		return
	}
	
	m.logger.WithField("server", server.Name).
		WithField("vm_count", len(vms)).
		Info("Discovered Proxmox VMs")
	
	// Process each VM
	for _, vm := range vms {
		// Skip templates
		if vm.Template {
			continue
		}
		
		m.createOrUpdateProxmoxVM(server, vm, client)
	}
	
	// Also populate the parent server's system_info.vms array for the frontend
	m.populateSystemInfoVMs(server, vms, client)
	
	// Update last discovery time
	server.LastVMDiscovery = time.Now()
	if err := m.storage.UpdateServer(server); err != nil {
		m.logger.WithField("server", server.Name).
			WithError(err).
			Error("Failed to update VM discovery time")
	}
}

// createOrUpdateProxmoxVM creates or updates a server entry for a Proxmox VM
func (m *Monitor) createOrUpdateProxmoxVM(proxmoxHost *models.Server, vm proxmox.VM, client *proxmox.Client) {
	// Generate unique ID for this VM
	vmServerID := fmt.Sprintf("%s-vm-%d", proxmoxHost.ID, vm.VMID)
	
	// Check if server already exists
	existingServer, err := m.storage.GetServer(vmServerID)
	if err != nil {
		// Server doesn't exist, create new one
		m.createProxmoxVMServer(proxmoxHost, vm, vmServerID, client)
		return
	}
	
	// Update existing VM server
	m.updateProxmoxVMServer(existingServer, vm, client)
}

// createProxmoxVMServer creates a new server entry for a Proxmox VM
func (m *Monitor) createProxmoxVMServer(proxmoxHost *models.Server, vm proxmox.VM, serverID string, client *proxmox.Client) {
	m.logger.WithField("proxmox_host", proxmoxHost.Name).
		WithField("vm_id", vm.VMID).
		WithField("vm_name", vm.Name).
		Info("Creating new Proxmox VM server entry")
	
	// Get VM IP addresses if possible
	var hostname string
	ips, err := client.GetVMIPAddress(vm.VMID)
	if err == nil && len(ips) > 0 {
		hostname = ips[0] // Use first IP as hostname
		m.logger.WithField("vm_name", vm.Name).WithField("vm_id", vm.VMID).
			WithField("ip", hostname).Debug("VM has IP address from guest agent")
	} else {
		// Fallback to VM name or ID - this indicates no guest agent
		hostname = vm.Name
		if hostname == "" {
			hostname = fmt.Sprintf("vm-%d", vm.VMID)
		}
		m.logger.WithField("vm_name", vm.Name).WithField("vm_id", vm.VMID).
			WithField("fallback_hostname", hostname).
			Info("VM has no IP address - guest agent not installed or not running")
	}
	
	// Create server entry
	vmServer := &models.Server{
		ID:             serverID,
		Name:           vm.Name,
		Hostname:       hostname,
		CurrentState:   m.convertProxmoxStatusToPowerState(vm.Status),
		DesiredState:   models.PowerStateUnknown, // VMs don't need power management via SSH
		ParentServerID: proxmoxHost.ID,
		IsProxmoxVM:    true,
		ProxmoxVMID:    vm.VMID,
		ProxmoxNodeName: proxmoxHost.ProxmoxNodeName,
		Source:         models.SourceAPI,
		LastStateChange: time.Now(),
		Services:       []models.Service{}, // VMs don't need SSH services
		Initialized:    true, // VMs don't need SSH initialization
		LastSuccessfulInit: time.Now(),
	}
	
	// Store the new VM server
	if err := m.storage.AddServer(vmServer); err != nil {
		m.logger.WithField("server_id", serverID).
			WithError(err).
			Error("Failed to add Proxmox VM server")
		return
	}
	
	m.logger.WithField("server_id", serverID).
		WithField("vm_name", vm.Name).
		Info("Successfully created Proxmox VM server entry")
}

// updateProxmoxVMServer updates an existing Proxmox VM server entry
func (m *Monitor) updateProxmoxVMServer(vmServer *models.Server, vm proxmox.VM, client *proxmox.Client) {
	// Update VM-specific information
	vmServer.Name = vm.Name
	
	// Update hostname if we can get IP addresses
	ips, err := client.GetVMIPAddress(vm.VMID)
	if err == nil && len(ips) > 0 && vmServer.Hostname != ips[0] {
		vmServer.Hostname = ips[0]
		m.logger.WithField("server", vmServer.Name).
			WithField("new_ip", ips[0]).
			Info("Updated Proxmox VM IP address")
	}
	
	// Update power state based on VM status
	newState := m.convertProxmoxStatusToPowerState(vm.Status)
	if newState != vmServer.CurrentState {
		vmServer.CurrentState = newState
		vmServer.LastStateChange = time.Now()
		
		m.logger.WithField("server", vmServer.Name).
			WithField("old_state", vmServer.CurrentState).
			WithField("new_state", newState).
			Info("Proxmox VM state changed")
	}
	
	// Store updated server
	if err := m.storage.UpdateServer(vmServer); err != nil {
		m.logger.WithField("server", vmServer.Name).
			WithError(err).
			Error("Failed to update Proxmox VM server")
	}
}

// convertProxmoxStatusToPowerState converts Proxmox VM status to our PowerState
func (m *Monitor) convertProxmoxStatusToPowerState(proxmoxStatus string) models.PowerState {
	switch strings.ToLower(proxmoxStatus) {
	case "running":
		return models.PowerStateOn
	case "stopped":
		return models.PowerStateStopped  // VM is stopped (full shutdown)
	case "suspended", "paused":
		return models.PowerStateSuspended // VM is suspended/paused (RAM preserved)
	default:
		return models.PowerStateUnknown
	}
}

// performSystemChecks runs system information gathering for all appropriate servers
func (m *Monitor) performSystemChecks() {
	servers := m.storage.GetAllServers()
	
	// Debug logging to see what servers are in memory
	m.logger.WithField("total_servers", len(servers)).Debug("Performing system checks")
	for _, server := range servers {
		m.logger.WithFields(logrus.Fields{
			"name":           server.Name,
			"hostname":       server.Hostname,
			"current_state":  server.CurrentState,
			"initialized":    server.Initialized,
			"is_proxmox_vm":  server.IsProxmoxVM,
			"parent_server":  server.ParentServerID,
		}).Debug("Server in memory")
	}
	
	// Record overall system check metrics
	m.recordMetric("_system", metrics.StandardMetrics.SystemCheckCycle, 1)
	m.recordMetric("_system", metrics.StandardMetrics.TotalServers, float64(len(servers)))
	
	onlineServers := 0
	checkedServers := 0
	
	for _, server := range servers {
		// Skip if server is offline or not initialized
		if server.CurrentState != models.PowerStateOn || !server.Initialized {
			continue
		}
		
		onlineServers++

		// Check if enough time has passed since last system check
		m.mu.RLock()
		lastCheck, exists := m.lastSystemCheck[server.ID]
		m.mu.RUnlock()
		
		if exists {
			timeSinceCheck := time.Since(lastCheck)
			minInterval := time.Duration(m.config.Dashboard.SystemCheckInterval) * time.Second
			if timeSinceCheck < minInterval {
				continue
			}
		}
		
		checkedServers++

		// Handle Proxmox VMs and regular servers differently
		if server.IsProxmoxVM {
			go m.performProxmoxVMSystemCheck(server)
		} else {
			// Skip servers without SSH info
			if server.SSHUser == "" || server.Hostname == "" {
				continue
			}
			go m.performRegularServerSystemCheck(server)
		}
	}
	
	// Record summary metrics
	m.recordMetric("_system", metrics.StandardMetrics.OnlineServers, float64(onlineServers))
	m.recordMetric("_system", metrics.StandardMetrics.CheckedServers, float64(checkedServers))
}

// performInitializationChecks runs initialization checks for servers that need them
func (m *Monitor) performInitializationChecks() {
	servers := m.storage.GetAllServers()
	
	for _, server := range servers {
		// Use the centralized initialization logic
		if !m.shouldAttemptInitialization(server) {
			continue
		}

		// Skip if no SSH info available
		if server.SSHUser == "" || server.Hostname == "" {
			continue
		}

		// Only attempt initialization if server appears to be online
		if server.CurrentState == models.PowerStateOff {
			continue
		}

		// Perform initialization check in a separate goroutine
		go func(srv *models.Server) {
		 m.logger.WithField("server", srv.Name).Debug("Starting initialization check")
			
		 if err := m.systemMonitor.PerformInitializationCheck(srv); err != nil {
			 m.logger.WithFields(logrus.Fields{
				 "server": srv.Name,
				 "error":  err,
			 }).Debug("Initialization check failed")
		 } else {
			 m.logger.WithField("server", srv.Name).Info("Initialization check completed successfully")
			 
			 // Trigger immediate Proxmox discovery if this is a Proxmox server
			 if srv.SystemInfo != nil && srv.SystemInfo.Type == models.SystemTypeProxmox {
				 go func() {
					 m.logger.WithField("server", srv.Name).Info("Triggering immediate Proxmox discovery after initialization")
					 m.performProxmoxDiscovery()
				 }()
			 }
		 }
			
			// Update last check time
			m.mu.Lock()
			m.lastInitCheck[srv.ID] = time.Now()
			m.mu.Unlock()
		}(server)
	}
}

// performRegularServerSystemCheck performs system check via SSH for regular servers
func (m *Monitor) performRegularServerSystemCheck(server *models.Server) {
	m.logger.WithField("server", server.Name).Debug("Starting SSH-based system information check")
	
	// Record per-server system check metrics
	m.recordMetric(server.ID, metrics.StandardMetrics.SystemCheckAttempt, 1)
	
	startTime := time.Now()
	systemMetrics, err := m.systemMonitor.PerformSystemCheck(server)
	checkDuration := time.Since(startTime)
	
	// Record timing metrics
	m.recordMetric(server.ID, metrics.StandardMetrics.SystemCheckDuration, checkDuration.Seconds())
	
	if err != nil {
		m.logger.WithFields(logrus.Fields{
			"server": server.Name,
			"error":  err,
		}).Debug("System check failed")
		m.recordMetric(server.ID, metrics.StandardMetrics.SystemCheckFailure, 1)
	} else {
		m.logger.WithField("server", server.Name).Debug("System check completed successfully")
		m.recordMetric(server.ID, metrics.StandardMetrics.SystemCheckSuccess, 1)
		
		// Record the collected metrics if any were successfully retrieved
		if systemMetrics != nil {
			if systemMetrics.CPUUsage != nil {
				m.recordMetric(server.ID, metrics.StandardMetrics.CPU, *systemMetrics.CPUUsage)
			}
			if systemMetrics.MemoryPercent != nil {
				m.recordMetric(server.ID, metrics.StandardMetrics.Memory, *systemMetrics.MemoryPercent)
			}
			if systemMetrics.NetworkRxMbps != nil && systemMetrics.NetworkTxMbps != nil {
				// Record total network as sum of rx + tx
				totalNetwork := *systemMetrics.NetworkRxMbps + *systemMetrics.NetworkTxMbps
				m.recordMetric(server.ID, metrics.StandardMetrics.Network, totalNetwork)
			}
		}
	}
	
	// Update last check time
	m.mu.Lock()
	m.lastSystemCheck[server.ID] = time.Now()
	m.mu.Unlock()
}

// performProxmoxVMSystemCheck performs system check via Proxmox API for VMs
func (m *Monitor) performProxmoxVMSystemCheck(server *models.Server) {
	m.logger.WithField("server", server.Name).Debug("Starting Proxmox API-based system information check")
	
	// Record per-server system check metrics
	m.recordMetric(server.ID, metrics.StandardMetrics.SystemCheckAttempt, 1)
	
	startTime := time.Now()
	
	// Get the parent Proxmox host
	parentServer, err := m.storage.GetServer(server.ParentServerID)
	if err != nil {
		m.logger.WithField("server", server.Name).
			WithError(err).
			Error("Failed to get parent Proxmox host for system check")
		m.recordMetric(server.ID, metrics.StandardMetrics.SystemCheckFailure, 1)
		return
	}
	
	// Make sure parent has API key
	if parentServer.ProxmoxAPIKey == nil {
		m.logger.WithField("server", server.Name).
			WithField("parent", parentServer.Name).
			Debug("Parent Proxmox host has no API key, skipping system check")
		m.recordMetric(server.ID, metrics.StandardMetrics.SystemCheckFailure, 1)
		return
	}
	
	// Create Proxmox client
	client := proxmox.NewClient(
		parentServer.Hostname,
		parentServer.ProxmoxNodeName,
		parentServer.GetProxmoxAPIToken(),
		true, // Skip TLS verification
	)
	
	// Get VM status with detailed metrics
	vmStatus, err := client.GetVMStatus(server.ProxmoxVMID)
	checkDuration := time.Since(startTime)
	
	// Record timing metrics
	m.recordMetric(server.ID, metrics.StandardMetrics.SystemCheckDuration, checkDuration.Seconds())
	
	if err != nil {
		m.logger.WithField("server", server.Name).
			WithField("vm_id", server.ProxmoxVMID).
			WithError(err).
			Debug("Proxmox VM system check failed")
		m.recordMetric(server.ID, metrics.StandardMetrics.SystemCheckFailure, 1)
	} else {
		m.logger.WithField("server", server.Name).Debug("Proxmox VM system check completed successfully")
		m.recordMetric(server.ID, metrics.StandardMetrics.SystemCheckSuccess, 1)
		
		// Convert Proxmox metrics to our standard format and record them
		if vmStatus.CPU >= 0 && vmStatus.CPU <= 1 {
			cpuPercent := vmStatus.CPU * 100
			m.recordMetric(server.ID, metrics.StandardMetrics.CPU, cpuPercent)
		}
		
		if vmStatus.MaxMem > 0 {
			memoryPercent := (float64(vmStatus.Mem) / float64(vmStatus.MaxMem)) * 100
			m.recordMetric(server.ID, metrics.StandardMetrics.Memory, memoryPercent)
		}
		
		// Network metrics (convert from bytes to Mbps)
		if vmStatus.NetIn >= 0 && vmStatus.NetOut >= 0 {
			// These are cumulative bytes, so we can't directly convert to Mbps
			// For now, just record the raw values as placeholders
			// TODO: Calculate rate by comparing with previous values
			networkTotal := float64(vmStatus.NetIn + vmStatus.NetOut) / (1024 * 1024) // Convert to MB
			m.recordMetric(server.ID, metrics.StandardMetrics.Network, networkTotal)
		}
		
		// Additional Proxmox-specific metrics
		if vmStatus.Uptime > 0 {
			m.recordMetric(server.ID, "uptime_seconds", float64(vmStatus.Uptime))
		}
		
		if vmStatus.MaxDisk > 0 {
			diskPercent := (float64(vmStatus.Disk) / float64(vmStatus.MaxDisk)) * 100
			m.recordMetric(server.ID, "disk_usage_percent", diskPercent)
		}
	}
	
	// Update last check time
	m.mu.Lock()
	m.lastSystemCheck[server.ID] = time.Now()
	m.mu.Unlock()
}

// checkProxmoxVMStatus checks the status of a Proxmox VM using the API
func (m *Monitor) checkProxmoxVMStatus(server *models.Server) (models.PowerState, []models.Service) {
	// Get the parent Proxmox host
	parentServer, err := m.storage.GetServer(server.ParentServerID)
	if err != nil {
		m.logger.WithField("server", server.Name).
			WithError(err).
			Error("Failed to get parent Proxmox host")
		return models.PowerStateUnknown, server.Services
	}
	
	// Make sure parent has API key
	if parentServer.ProxmoxAPIKey == nil {
		m.logger.WithField("server", server.Name).
			WithField("parent", parentServer.Name).
			Warn("Parent Proxmox host has no API key")
		return models.PowerStateUnknown, server.Services
	}
	
	// Create Proxmox client
	client := proxmox.NewClient(
		parentServer.Hostname,
		parentServer.ProxmoxNodeName,
		parentServer.GetProxmoxAPIToken(),
		true, // Skip TLS verification
	)
	
	// Get VM status
	vmStatus, err := client.GetVMStatus(server.ProxmoxVMID)
	if err != nil {
		m.logger.WithField("server", server.Name).
			WithField("vm_id", server.ProxmoxVMID).
			WithError(err).
			Error("Failed to get Proxmox VM status")
		return models.PowerStateUnknown, server.Services
	}
	
	// Convert status to power state - but handle transitioning states
	apiState := m.convertProxmoxStatusToPowerState(vmStatus.Status)
	var newState models.PowerState
	
	// Handle transitioning states
	switch server.CurrentState {
	case models.PowerStateWaking:
		if apiState == models.PowerStateOn {
			m.logger.WithField("server", server.Name).Info("Proxmox VM successfully completed wake operation")
			newState = models.PowerStateOn
		} else {
			// Still waking or failed - check timeout
			if !server.LastStateChange.IsZero() && time.Since(server.LastStateChange) > 5*time.Minute {
				m.logger.WithField("server", server.Name).Warn("Proxmox VM wake operation timed out")
				newState = apiState // Use actual API state
			} else {
				newState = models.PowerStateWaking // Keep transitioning
			}
		}
		
	case models.PowerStateSuspending:
		if apiState == models.PowerStateSuspended {
			m.logger.WithField("server", server.Name).Info("Proxmox VM successfully completed suspend operation")
			newState = apiState
		} else {
			// Still suspending or failed - check timeout
			if !server.LastStateChange.IsZero() && time.Since(server.LastStateChange) > 2*time.Minute {
				m.logger.WithField("server", server.Name).Warn("Proxmox VM suspend operation timed out")
				newState = apiState // Use actual API state
			} else {
				newState = models.PowerStateSuspending // Keep transitioning
			}
		}
		
	case models.PowerStateStopping:
		if apiState == models.PowerStateStopped {
			m.logger.WithField("server", server.Name).Info("Proxmox VM successfully completed stop operation")
			newState = apiState
		} else {
			// Still stopping or failed - check timeout
			if !server.LastStateChange.IsZero() && time.Since(server.LastStateChange) > 2*time.Minute {
				m.logger.WithField("server", server.Name).Warn("Proxmox VM stop operation timed out")
				newState = apiState // Use actual API state
			} else {
				newState = models.PowerStateStopping // Keep transitioning
			}
		}
		
	default:
		// Not in transitioning state, use API state directly
		newState = apiState
	}
	
	// For VMs that are running, try to scan services with discovery
	var updatedServices []models.Service
	if newState == models.PowerStateOn && server.Hostname != "" {
		m.logger.WithField("server", server.Name).WithField("hostname", server.Hostname).
			Debug("Scanning VM services with discovery")
		// Use comprehensive scanning with discovery for VMs too
		updatedServices = m.portScanner.ScanServicesWithDiscovery(server.Hostname, server.ID, server.Services)
		m.logger.WithField("server", server.Name).WithField("service_count", len(updatedServices)).
			Debug("VM service scan completed")
	} else {
		m.logger.WithField("server", server.Name).WithField("state", newState).WithField("hostname", server.Hostname).
			Debug("VM not running or no hostname, marking services down")
		// Mark all services as down if VM is not running
		updatedServices = make([]models.Service, len(server.Services))
		for i, service := range server.Services {
			updatedServices[i] = service
			updatedServices[i].Status = models.ServiceStatusDown
			updatedServices[i].LastCheck = time.Now()
		}
	}
	
	// Try to update VM IP if it's running and we have QEMU agent
	if newState == models.PowerStateOn {
		ips, err := client.GetVMIPAddress(server.ProxmoxVMID)
		if err == nil && len(ips) > 0 && server.Hostname != ips[0] {
			server.Hostname = ips[0]
			m.logger.WithField("server", server.Name).
				WithField("new_ip", ips[0]).
				Debug("Updated VM IP address")
		}
	}
	
	return newState, updatedServices
}

// populateSystemInfoVMs populates the parent server's system_info.vms array for frontend display
func (m *Monitor) populateSystemInfoVMs(server *models.Server, vms []proxmox.VM, client *proxmox.Client) {
	if server.SystemInfo == nil {
		server.SystemInfo = &models.SystemInfo{
			Type: models.SystemTypeProxmox,
		}
	}
	
	// Clear existing VMs array
	server.SystemInfo.VMs = []models.VMInfo{}
	
	// Convert Proxmox VMs to VMInfo for frontend
	for _, vm := range vms {
		// Skip templates
		if vm.Template {
			continue
		}
		
		// Get VM IP addresses if possible
		var primaryIP string
		ips, err := client.GetVMIPAddress(vm.VMID)
		if err == nil && len(ips) > 0 {
			primaryIP = ips[0]
		}
		
		// Convert Proxmox VM status to our VMInfo format
		vmInfo := models.VMInfo{
			VMID:      fmt.Sprintf("%d", vm.VMID), // Convert int to string
			Name:      vm.Name,
			Status:    vm.Status,
			PrimaryIP: primaryIP,
		}
		
		server.SystemInfo.VMs = append(server.SystemInfo.VMs, vmInfo)
	}
	
	m.logger.WithField("server", server.Name).
		WithField("vm_count", len(server.SystemInfo.VMs)).
		Debug("Populated system_info VMs array for frontend")
}
