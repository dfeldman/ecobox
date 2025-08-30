package models

import (
	"fmt"
	"time"
)

type Server struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Hostname       string       `json:"hostname"`
	MACAddress     string       `json:"mac_address"`
	CurrentState   PowerState   `json:"current_state"`
	DesiredState   PowerState   `json:"desired_state"`
	ParentServerID string       `json:"parent_server_id"`
	Initialized    bool         `json:"initialized"`

	// Initialization tracking
	InitRetryCount      int       `json:"init_retry_count"`
	LastInitAttempt     time.Time `json:"last_init_attempt"`
	LastSuccessfulInit  time.Time `json:"last_successful_init"`

	// System information collected from the server
	SystemInfo *SystemInfo `json:"system_info,omitempty"`

	// Time tracking (all in seconds)
	TotalOnTime       int64     `json:"total_on_time"`
	TotalSuspendedTime int64     `json:"total_suspended_time"`
	TotalOffTime      int64     `json:"total_off_time"`
	LastStateChange   time.Time `json:"last_state_change"`

	// Action log
	RecentActions []ServerAction `json:"recent_actions"`

	Services []Service `json:"services"`
	Source   Source    `json:"source"`

	// SSH credentials for suspend command
	SSHUser    string `json:"ssh_user"`
	SSHPort    int    `json:"ssh_port"`
	SSHKeyPath string `json:"ssh_key_path"`

	// Proxmox-specific fields
	ProxmoxAPIKey    *ProxmoxAPIKey `json:"proxmox_api_key,omitempty"`    // Only set if this is a Proxmox host
	IsProxmoxVM      bool           `json:"is_proxmox_vm"`                // True if this server is a Proxmox VM
	ProxmoxVMID      int            `json:"proxmox_vm_id,omitempty"`      // VMID if this is a Proxmox VM
	ProxmoxNodeName  string         `json:"proxmox_node_name,omitempty"`  // Node name for Proxmox operations
	LastVMDiscovery  time.Time      `json:"last_vm_discovery"`            // Last time we discovered VMs (for Proxmox hosts)

	// User-configurable power management options
	PowerOptions *PowerOptions `json:"power_options,omitempty"` // Power management settings from user input
}

type ServerAction struct {
	Timestamp   time.Time  `json:"timestamp"`
	Action      ActionType `json:"action"`
	Success     bool       `json:"success"`
	ErrorMsg    string     `json:"error_msg"`
	InitiatedBy string     `json:"initiated_by"` // "manual", "api", "scheduler", etc.
}

// GetCurrentUptime returns the current uptime in seconds based on state and last change
func (s *Server) GetCurrentUptime() int64 {
	if s.CurrentState != PowerStateOn {
		return 0
	}
	return int64(time.Since(s.LastStateChange).Seconds())
}

// GetTotalUptime returns total uptime including current session
func (s *Server) GetTotalUptime() int64 {
	total := s.TotalOnTime
	if s.CurrentState == PowerStateOn {
		total += int64(time.Since(s.LastStateChange).Seconds())
	}
	return total
}

// IsProxmoxHost returns true if this server is a Proxmox host with API key
func (s *Server) IsProxmoxHost() bool {
	return s.ProxmoxAPIKey != nil && s.SystemInfo != nil && s.SystemInfo.Type == SystemTypeProxmox
}

// GetProxmoxAPIToken returns the formatted API token for Proxmox API calls
func (s *Server) GetProxmoxAPIToken() string {
	if s.ProxmoxAPIKey == nil {
		return ""
	}
	return fmt.Sprintf("%s@%s!%s=%s",
		s.ProxmoxAPIKey.Username,
		s.ProxmoxAPIKey.Realm,
		s.ProxmoxAPIKey.TokenID,
		s.ProxmoxAPIKey.Secret)
}

// ShouldDiscoverVMs returns true if this Proxmox host should discover VMs
func (s *Server) ShouldDiscoverVMs(vmDiscoveryInterval time.Duration) bool {
	return s.IsProxmoxHost() &&
		(s.LastVMDiscovery.IsZero() || time.Since(s.LastVMDiscovery) >= vmDiscoveryInterval)
}

// EnsurePowerOptions ensures the server has PowerOptions initialized with defaults
func (s *Server) EnsurePowerOptions() {
	if s.PowerOptions == nil {
		s.PowerOptions = NewDefaultPowerOptions()
	}
}

// GetPowerOptions returns the server's power options, initializing with defaults if needed
func (s *Server) GetPowerOptions() *PowerOptions {
	s.EnsurePowerOptions()
	return s.PowerOptions
}

// ShouldAutoSuspend checks if the server should be auto-suspended based on current metrics
func (s *Server) ShouldAutoSuspend() bool {
	if s.PowerOptions == nil || s.SystemInfo == nil {
		return false
	}

	// Convert network usage from MBps to kbps
	netUsageKbps := (s.SystemInfo.NetworkUsage.MBpsRecv + s.SystemInfo.NetworkUsage.MBpsSent) * 1024.0 * 8.0

	// Use the first load average if available
	loadAvg := 0.0
	if len(s.SystemInfo.LoadAverage) > 0 {
		loadAvg = s.SystemInfo.LoadAverage[0]
	}

	return s.PowerOptions.ShouldAutoSuspend(
		s.SystemInfo.CPUUsage,
		s.SystemInfo.MemoryUsage.UsedPercent,
		loadAvg,
		netUsageKbps,
	)
}

// ShouldWakeUp checks if the server should wake up based on current time and schedule
func (s *Server) ShouldWakeUp(currentTime time.Time) bool {
	if s.PowerOptions == nil {
		return false
	}
	return s.PowerOptions.ShouldWakeUp(currentTime)
}
