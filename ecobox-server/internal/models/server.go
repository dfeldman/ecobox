package models

import "time"

type Server struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Hostname       string       `json:"hostname"`
	MACAddress     string       `json:"mac_address"`
	CurrentState   PowerState   `json:"current_state"`
	DesiredState   PowerState   `json:"desired_state"`
	ParentServerID string       `json:"parent_server_id"`
	Initialized    bool         `json:"initialized"`

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
