package metrics

// StandardMetrics defines the consistent metric names used across the system
// These names are shared between backend recording and frontend display
var StandardMetrics = struct {
	// System resource metrics (what frontend expects)
	Memory  string
	CPU     string
	Network string
	Wattage string
	
	// Power management metrics
	PowerStateChange   string
	PowerStateOn       string
	PowerStateOff      string
	PowerStateStopped  string  // New: VM stopped state
	PowerStateSuspended string
	PowerStateInitFailed string
	PowerStateWaking     string  // New transitioning state
	PowerStateSuspending string  // New transitioning state
	PowerStateStopping   string  // New transitioning state
	
	// Wake/suspend operations
	WakeAttempt       string
	WakeSuccess       string
	WakeFailure       string
	WakeDuration      string
	SuspendAttempt    string
	SuspendSuccess    string
	SuspendFailure    string
	SuspendDuration   string
	
	// Initialization metrics
	InitAttempt         string
	InitSuccess         string
	InitFailure         string
	InitDuration        string
	InitRetryCount      string
	InitStateReset      string
	InitMaxRetriesExceeded string
	
	// Service monitoring
	ServiceAvailability string
	StateUpdateError    string
	
	// System checks
	SystemCheckAttempt   string
	SystemCheckSuccess   string
	SystemCheckFailure   string
	SystemCheckDuration  string
	
	// System overview metrics
	MonitoringCycle     string
	MonitoringServerCount string
	SystemCheckCycle    string
	TotalServers        string
	OnlineServers       string
	CheckedServers      string
}{
	// System resource metrics (frontend expects these exact names)
	Memory:  "memory",
	CPU:     "cpu", 
	Network: "network",
	Wattage: "wattage",
	
	// Power management metrics
	PowerStateChange:     "power_state_change",
	PowerStateOn:         "power_state_on",
	PowerStateOff:        "power_state_off", 
	PowerStateStopped:    "power_state_stopped",    // New: VM stopped state
	PowerStateSuspended:  "power_state_suspended",
	PowerStateInitFailed: "power_state_init_failed",
	PowerStateWaking:     "power_state_waking",     // New transitioning state
	PowerStateSuspending: "power_state_suspending", // New transitioning state
	PowerStateStopping:   "power_state_stopping",   // New transitioning state
	
	// Wake/suspend operations
	WakeAttempt:     "wake_attempt",
	WakeSuccess:     "wake_success", 
	WakeFailure:     "wake_failure",
	WakeDuration:    "wake_duration_seconds",
	SuspendAttempt:  "suspend_attempt",
	SuspendSuccess:  "suspend_success",
	SuspendFailure:  "suspend_failure", 
	SuspendDuration: "suspend_duration_seconds",
	
	// Initialization metrics
	InitAttempt:            "init_attempt",
	InitSuccess:            "init_success",
	InitFailure:            "init_failure", 
	InitDuration:           "init_duration_seconds",
	InitRetryCount:         "init_retry_count",
	InitStateReset:         "init_state_reset",
	InitMaxRetriesExceeded: "init_max_retries_exceeded",
	
	// Service monitoring  
	ServiceAvailability: "service_availability_percent",
	StateUpdateError:    "state_update_error",
	
	// System checks
	SystemCheckAttempt:  "system_check_attempt",
	SystemCheckSuccess:  "system_check_success", 
	SystemCheckFailure:  "system_check_failure",
	SystemCheckDuration: "system_check_duration_seconds",
	
	// System overview metrics
	MonitoringCycle:       "monitoring_cycle",
	MonitoringServerCount: "monitoring_server_count",
	SystemCheckCycle:      "system_check_cycle", 
	TotalServers:          "total_servers",
	OnlineServers:         "online_servers",
	CheckedServers:        "checked_servers",
}

// FrontendMetrics returns the list of metrics that the frontend can display
func FrontendMetrics() []string {
	return []string{
		StandardMetrics.Memory,
		StandardMetrics.CPU,
		StandardMetrics.Network,
		StandardMetrics.Wattage,
	}
}

// AllMetrics returns all available metric names for documentation/discovery
func AllMetrics() []string {
	return []string{
		// System resources
		StandardMetrics.Memory,
		StandardMetrics.CPU,
		StandardMetrics.Network,
		StandardMetrics.Wattage,
		
		// Power management
		StandardMetrics.PowerStateChange,
		StandardMetrics.PowerStateOn,
		StandardMetrics.PowerStateOff,
		StandardMetrics.PowerStateStopped,    // New: VM stopped state
		StandardMetrics.PowerStateSuspended,
		StandardMetrics.PowerStateInitFailed,
		StandardMetrics.PowerStateWaking,     // New transitioning state
		StandardMetrics.PowerStateSuspending, // New transitioning state
		StandardMetrics.PowerStateStopping,   // New transitioning state
		
		// Wake/suspend operations
		StandardMetrics.WakeAttempt,
		StandardMetrics.WakeSuccess,
		StandardMetrics.WakeFailure,
		StandardMetrics.WakeDuration,
		StandardMetrics.SuspendAttempt,
		StandardMetrics.SuspendSuccess,
		StandardMetrics.SuspendFailure,
		StandardMetrics.SuspendDuration,
		
		// Initialization
		StandardMetrics.InitAttempt,
		StandardMetrics.InitSuccess,
		StandardMetrics.InitFailure,
		StandardMetrics.InitDuration,
		StandardMetrics.InitRetryCount,
		StandardMetrics.InitStateReset,
		StandardMetrics.InitMaxRetriesExceeded,
		
		// Service monitoring
		StandardMetrics.ServiceAvailability,
		StandardMetrics.StateUpdateError,
		
		// System checks
		StandardMetrics.SystemCheckAttempt,
		StandardMetrics.SystemCheckSuccess,
		StandardMetrics.SystemCheckFailure,
		StandardMetrics.SystemCheckDuration,
		
		// System overview
		StandardMetrics.MonitoringCycle,
		StandardMetrics.MonitoringServerCount,
		StandardMetrics.SystemCheckCycle,
		StandardMetrics.TotalServers,
		StandardMetrics.OnlineServers,
		StandardMetrics.CheckedServers,
	}
}
