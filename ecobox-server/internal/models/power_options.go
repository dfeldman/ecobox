package models

import "time"

// SuspendMethod represents the method used for suspending a server
type SuspendMethod string

const (
	SuspendMethodNone    SuspendMethod = "none"    // Never suspend the machine
	SuspendMethodSuspend SuspendMethod = "suspend" // Currently the only supported method
	// Future options: hibernate, powerdown
)

// DayOfWeek represents days of the week for scheduling
type DayOfWeek int

const (
	Sunday DayOfWeek = iota
	Monday
	Tuesday
	Wednesday
	Thursday
	Friday
	Saturday
)

// WakeTime represents a scheduled wake time
type WakeTime struct {
	Time       time.Time   `json:"time"`         // Time of day to wake (only hour/minute matter)
	DaysOfWeek []DayOfWeek `json:"days_of_week"` // Days when this wake time applies
}

// PowerOptions contains user-configurable power management options for a server
// This data is persisted and comes from user input in the web UI
type PowerOptions struct {
	// Basic power control options
	SuspendMethod SuspendMethod `json:"suspend_method"` // Method to use for suspending
	AllowShutdown bool          `json:"allow_shutdown"` // Allow manual shutdown (assumes we can start it back up)
	AllowRestart  bool          `json:"allow_restart"`  // Allow manual restart

	// Scheduled wake times
	WakeTimes []WakeTime `json:"wake_times"` // Times to automatically wake the machine

	// CPU-based auto-suspend
	UseCpuSuspend bool `json:"use_cpu_suspend"` // Enable CPU usage based suspend
	CpuSuspend    int  `json:"cpu_suspend"`     // CPU usage percentage below which to suspend

	// Memory-based auto-suspend  
	UseMemSuspend bool `json:"use_mem_suspend"` // Enable memory usage based suspend
	MemSuspend    int  `json:"mem_suspend"`     // Memory usage percentage below which to suspend

	// Load average based auto-suspend
	UseLoadSuspend bool `json:"use_load_suspend"` // Enable load average based suspend
	LoadSuspend    int  `json:"load_suspend"`     // Load average below which to suspend

	// Network usage based auto-suspend
	UseNetSuspend bool `json:"use_net_suspend"` // Enable network usage based suspend
	NetSuspend    int  `json:"net_suspend"`     // Network usage in kbps below which to suspend
}

// NewDefaultPowerOptions returns PowerOptions with sensible defaults
func NewDefaultPowerOptions() *PowerOptions {
	return &PowerOptions{
		SuspendMethod: SuspendMethodNone, // Conservative default - no auto-suspend
		AllowShutdown: false, // Conservative default
		AllowRestart:  false, // Conservative default
		WakeTimes:     []WakeTime{},
		UseCpuSuspend: false,
		CpuSuspend:    5,  // 5% CPU usage threshold
		UseMemSuspend: false,
		MemSuspend:    20, // 20% memory usage threshold
		UseLoadSuspend: false,
		LoadSuspend:    1,   // Load average of 1.0
		UseNetSuspend:  false,
		NetSuspend:     100, // 100 kbps threshold
	}
}

// ShouldAutoSuspend evaluates whether the server should be auto-suspended based on current metrics
// All enabled conditions must be true (AND logic) for auto-suspend to trigger
func (po *PowerOptions) ShouldAutoSuspend(cpuUsage float64, memUsage float64, loadAvg float64, netUsageKbps float64) bool {
	// If suspend method is None, never auto-suspend
	if po.SuspendMethod == SuspendMethodNone {
		return false
	}

	// If no conditions are enabled, never auto-suspend
	if !po.UseCpuSuspend && !po.UseMemSuspend && !po.UseLoadSuspend && !po.UseNetSuspend {
		return false
	}

	// Check each enabled condition - all must be true for suspend
	if po.UseCpuSuspend && cpuUsage >= float64(po.CpuSuspend) {
		return false
	}

	if po.UseMemSuspend && memUsage >= float64(po.MemSuspend) {
		return false
	}

	if po.UseLoadSuspend && loadAvg >= float64(po.LoadSuspend) {
		return false
	}

	if po.UseNetSuspend && netUsageKbps >= float64(po.NetSuspend) {
		return false
	}

	return true
}

// ShouldWakeUp checks if the server should wake up based on current time and wake schedule
func (po *PowerOptions) ShouldWakeUp(currentTime time.Time) bool {
	if len(po.WakeTimes) == 0 {
		return false
	}

	currentWeekday := DayOfWeek(currentTime.Weekday())
	currentHour := currentTime.Hour()
	currentMinute := currentTime.Minute()

	for _, wakeTime := range po.WakeTimes {
		// Check if current day is in the wake schedule
		dayMatches := false
		for _, day := range wakeTime.DaysOfWeek {
			if day == currentWeekday {
				dayMatches = true
				break
			}
		}

		if !dayMatches {
			continue
		}

		// Check if current time matches wake time (within same hour and minute)
		wakeHour := wakeTime.Time.Hour()
		wakeMinute := wakeTime.Time.Minute()

		if currentHour == wakeHour && currentMinute == wakeMinute {
			return true
		}
	}

	return false
}
