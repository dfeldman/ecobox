package monitor

import (
	"sync"
	"time"

	"ecobox-server/internal/config"
	"ecobox-server/internal/control"
	"ecobox-server/internal/initializer"
	"ecobox-server/internal/metrics"
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
	systemMonitor  *control.SystemMonitor
	initManager    *initializer.Manager
	metricsManager *metrics.Manager
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
		config:            cfg,
		storage:           storage,
		pingChecker:       NewPingChecker(),
		portScanner:       NewPortScanner(),
		powerManager:      powerManager,
		systemMonitor:     systemMonitor,
		initManager:       initManager,
		metricsManager:    metricsManager,
		updateChan:        make(chan ServerUpdate, 100),
		stopChan:          make(chan struct{}),
		logger:            logrus.New(),
		running:           false,
		lastSystemCheck:   make(map[string]time.Time),
		lastInitCheck:     make(map[string]time.Time),
		maxInitRetries:    3,
		reinitInterval:    1 * time.Hour, // Clear init state and retry every hour
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
	newState := m.determineServerState(server)
	
	// Update services status
	updatedServices := m.portScanner.ScanServices(server.Hostname, server.Services)
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
		case models.PowerStateSuspended:
			m.recordMetric(server.ID, metrics.StandardMetrics.PowerStateSuspended, 1)
		case models.PowerStateInitFailed:
			m.recordMetric(server.ID, metrics.StandardMetrics.PowerStateInitFailed, 1)
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
	if m.shouldAttemptInitialization(server) {
		if m.attemptServerInitialization(server) {
			// Server was updated by initialization, so get the latest version
			updatedServer, err := m.storage.GetServer(server.ID)
			if err != nil {
				m.logger.Errorf("Failed to get updated server %s after initialization: %v", server.Name, err)
				return
			}
			server = updatedServer
		} else {
			// Initialization failed, don't continue with power state reconciliation
			return
		}
	}

	// Perform power state reconciliation
	switch server.DesiredState {
	case models.PowerStateOn:
		if server.CurrentState == models.PowerStateOff || server.CurrentState == models.PowerStateSuspended {
			m.logger.Infof("Attempting to wake server %s", server.Name)
			
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
			} else {
				m.logger.Infof("Successfully sent wake command to server %s", server.Name)
				m.recordMetric(server.ID, metrics.StandardMetrics.WakeSuccess, 1)
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
			} else {
				m.logger.Infof("Successfully sent suspend command to server %s", server.Name)
				m.recordMetric(server.ID, metrics.StandardMetrics.SuspendSuccess, 1)
				action.Success = true
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

// performSystemChecks runs system information gathering for all appropriate servers
func (m *Monitor) performSystemChecks() {
	servers := m.storage.GetAllServers()
	
	// Record overall system check metrics
	m.recordMetric("_system", metrics.StandardMetrics.SystemCheckCycle, 1)
	m.recordMetric("_system", metrics.StandardMetrics.TotalServers, float64(len(servers)))
	
	onlineServers := 0
	checkedServers := 0
	
	for _, server := range servers {
		// Skip if server is offline, not initialized, or doesn't have SSH info
		if server.CurrentState != models.PowerStateOn || !server.Initialized || 
		   server.SSHUser == "" || server.Hostname == "" {
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

		// Perform system check in a separate goroutine to avoid blocking
		go func(srv *models.Server) {
			m.logger.WithField("server", srv.Name).Debug("Starting system information check")
			
			// Record per-server system check metrics
			m.recordMetric(srv.ID, metrics.StandardMetrics.SystemCheckAttempt, 1)
			
			startTime := time.Now()
			systemMetrics, err := m.systemMonitor.PerformSystemCheck(srv)
			checkDuration := time.Since(startTime)
			
			// Record timing metrics
			m.recordMetric(srv.ID, metrics.StandardMetrics.SystemCheckDuration, checkDuration.Seconds())
			
			if err != nil {
				m.logger.WithFields(logrus.Fields{
					"server": srv.Name,
					"error":  err,
				}).Debug("System check failed")
				m.recordMetric(srv.ID, metrics.StandardMetrics.SystemCheckFailure, 1)
			} else {
				m.logger.WithField("server", srv.Name).Debug("System check completed successfully")
				m.recordMetric(srv.ID, metrics.StandardMetrics.SystemCheckSuccess, 1)
				
				// Record the collected metrics if any were successfully retrieved
				if systemMetrics != nil {
					if systemMetrics.CPUUsage != nil {
						m.recordMetric(srv.ID, metrics.StandardMetrics.CPU, *systemMetrics.CPUUsage)
					}
					if systemMetrics.MemoryPercent != nil {
						m.recordMetric(srv.ID, metrics.StandardMetrics.Memory, *systemMetrics.MemoryPercent)
					}
					if systemMetrics.NetworkRxMbps != nil && systemMetrics.NetworkTxMbps != nil {
						// Record total network as sum of rx + tx
						totalNetwork := *systemMetrics.NetworkRxMbps + *systemMetrics.NetworkTxMbps
						m.recordMetric(srv.ID, metrics.StandardMetrics.Network, totalNetwork)
					}
					// Note: Wattage will be implemented later with power meter API integration
					// if systemMetrics.DiskPercent != nil {
					//     m.recordMetric(srv.Name, "disk_usage_percent", *systemMetrics.DiskPercent)
					// }
					// if systemMetrics.LoadAverage1m != nil {
					//     m.recordMetric(srv.Name, "load_average_1m", *systemMetrics.LoadAverage1m)
					// }
				}
			}
			
			// Update last check time
			m.mu.Lock()
			m.lastSystemCheck[srv.ID] = time.Now()
			m.mu.Unlock()
		}(server)
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
		 }
			
			// Update last check time
			m.mu.Lock()
			m.lastInitCheck[srv.ID] = time.Now()
			m.mu.Unlock()
		}(server)
	}
}

// GetMetricsManager returns the metrics manager for external use
func (m *Monitor) GetMetricsManager() *metrics.Manager {
	return m.metricsManager
}

// SetSystemMonitorLogger sets the logger for the system monitor
func (m *Monitor) SetSystemMonitorLogger(logger *logrus.Logger) {
	// The system monitor doesn't have a SetLogger method in our implementation
	// but if it did, we would call it here
}

// GetTrackedMetrics returns a list of metrics being tracked by the monitor
func (m *Monitor) GetTrackedMetrics() map[string][]string {
	return map[string][]string{
		"Initialization": {
			"init_attempt",              // Number of initialization attempts
			"init_success",              // Number of successful initializations  
			"init_failure",              // Number of failed initializations
			"init_state_reset",          // Number of times init state was reset for retry
			"init_max_retries_exceeded", // Number of times max retries was exceeded
			"init_duration_seconds",     // Time taken for each initialization attempt
			"init_success_duration_seconds", // Time taken for successful initializations
			"init_retry_count",          // Current retry count for each attempt
			"init_success_retry_count",  // Retry count when initialization succeeded
			"init_final_retry_count",    // Final retry count when max retries exceeded
		},
		"Power Management": {
			"wake_attempt",       // Number of wake attempts
			"wake_success",       // Number of successful wakes
			"wake_failure",       // Number of failed wakes
			"wake_duration_seconds", // Time taken for wake operations
			"suspend_attempt",    // Number of suspend attempts
			"suspend_success",    // Number of successful suspends
			"suspend_failure",    // Number of failed suspends
			"suspend_duration_seconds", // Time taken for suspend operations
		},
		"State Monitoring": {
			"power_state_change",       // Number of power state changes
			"power_state_on",           // Transitions to 'on' state
			"power_state_off",          // Transitions to 'off' state
			"power_state_suspended",    // Transitions to 'suspended' state
			"power_state_unknown",      // Transitions to 'unknown' state
			"power_state_init_failed",  // Transitions to 'init_failed' state
			"state_update_error",       // Errors updating server state
		},
		"Service Monitoring": {
			"service_availability_percent", // Percentage of services online
		},
		"System Checks": {
			"system_check_attempt",         // Number of system check attempts
			"system_check_success",         // Number of successful system checks
			"system_check_failure",         // Number of failed system checks
			"system_check_duration_seconds", // Time taken for system checks
		},
		"System Overview": {
			"monitoring_cycle",      // Number of monitoring cycles completed
			"monitoring_server_count", // Number of servers being monitored
			"system_check_cycle",    // Number of system check cycles completed
			"total_servers",         // Total number of configured servers
			"online_servers",        // Number of servers currently online
			"checked_servers",       // Number of servers that had system checks performed
		},
	}
}
