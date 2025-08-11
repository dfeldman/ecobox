package models

import "time"

type PowerState string

const (
	PowerStateOn        PowerState = "on"
	PowerStateOff       PowerState = "off"
	PowerStateSuspended PowerState = "suspended"
	PowerStateUnknown   PowerState = "unknown"
	PowerStateInitFailed PowerState = "init_failed"
)

type ServiceType string

const (
	ServiceTypeSSH     ServiceType = "ssh"
	ServiceTypeRDP     ServiceType = "rdp"
	ServiceTypeVNC     ServiceType = "vnc"
	ServiceTypeSMB     ServiceType = "smb"
	ServiceTypeHTTP    ServiceType = "http"
	ServiceTypeHTTPS   ServiceType = "https"
	ServiceTypeCustom  ServiceType = "custom"
)

type ServiceStatus string

const (
	ServiceStatusUp   ServiceStatus = "up"
	ServiceStatusDown ServiceStatus = "down"
)

type Source string

const (
	SourceConfig Source = "config"
	SourceAPI    Source = "api"
)

type ActionType string

const (
	ActionTypeWakeUp      ActionType = "wake"
	ActionTypeSuspend     ActionType = "suspend"
	ActionTypeInitialize  ActionType = "initialize"
	ActionTypeReconcile   ActionType = "reconcile"
)

// SystemType represents the type of system
type SystemType string

const (
	SystemTypeLinux   SystemType = "linux"
	SystemTypeWindows SystemType = "windows"
	SystemTypeProxmox SystemType = "proxmox"
	SystemTypeUnknown SystemType = "unknown"
)

// SystemInfo contains comprehensive system information collected from servers
type SystemInfo struct {
	// Basic system identification
	Type      SystemType `json:"type"`
	SystemID  string     `json:"system_id"`
	OSVersion string     `json:"os_version"`
	Hostname  string     `json:"hostname"`

	// Network configuration
	IPAddresses []NetworkInterface `json:"ip_addresses"`

	// Performance metrics (current values - time series stored separately)
	CPUUsage       float64     `json:"cpu_usage"`
	LoadAverage    []float64   `json:"load_average"`
	MemoryUsage    MemoryInfo  `json:"memory_usage"`
	NetworkUsage   NetworkInfo `json:"network_usage"`
	DiskUsage      DiskInfo    `json:"disk_usage"`

	// Power metrics (current values - time series stored separately)
	PowerMeterWatts    float64 `json:"power_meter_watts"`     // Actual measured power consumption
	PowerEstimateWatts float64 `json:"power_estimate_watts"`  // Software-estimated power consumption

	// Power management capabilities
	SuspendSupport    bool `json:"suspend_support"`
	HibernateSupport  bool `json:"hibernate_support"`
	PowerSwitchSupport bool `json:"power_switch_support"`   // Physical power switch control
	WakeOnLANSupport  bool `json:"wake_on_lan_support"`
	PowerMeterSupport bool `json:"power_meter_support"`     // Real physical power meter device
	PowerEstimateSupport bool `json:"power_estimate_support"` // Software power estimation

	// Wake-on-LAN configuration
	WakeOnLAN WOLInfo `json:"wake_on_lan"`

	// VM information (if this server hosts VMs)
	VMs []VMInfo `json:"vms"`

	// Collection metadata
	LastUpdated time.Time `json:"last_updated"`
}

// MemoryInfo contains memory usage information
type MemoryInfo struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
}

// NetworkInfo contains network usage information (current values)
type NetworkInfo struct {
	BytesRecv uint64  `json:"bytes_recv"`
	BytesSent uint64  `json:"bytes_sent"`
	MBpsRecv  float64 `json:"mbps_recv"`
	MBpsSent  float64 `json:"mbps_sent"`
}

// NetworkInterface contains network interface information
type NetworkInterface struct {
	Name       string `json:"name"`
	IPAddress  string `json:"ip_address"`
	MACAddress string `json:"mac_address"`
	IsIPv6     bool   `json:"is_ipv6"`
}

// DiskInfo contains disk usage information (current values)
type DiskInfo struct {
	Total       uint64  `json:"total"`
	Used        uint64  `json:"used"`
	Free        uint64  `json:"free"`
	UsedPercent float64 `json:"used_percent"`
	MountPoint  string  `json:"mount_point"`
}

// WOLInfo contains Wake-on-LAN information
type WOLInfo struct {
	Supported  bool     `json:"supported"`
	Interfaces []string `json:"interfaces"`
	Armed      bool     `json:"armed"`
}

// VMInfo contains information about a virtual machine running on this server
type VMInfo struct {
	Name      string `json:"name"`
	PrimaryIP string `json:"primary_ip,omitempty"` // May be unknown/empty
	Status    string `json:"status"`               // running, stopped, etc.
	VMID      string `json:"vm_id,omitempty"`      // Proxmox VMID or similar
}

// ProxmoxAPIKey contains Proxmox API key information  
type ProxmoxAPIKey struct {
	Username string `json:"username"`
	Realm    string `json:"realm"`
	TokenID  string `json:"token_id"`
	Secret   string `json:"secret"`
}
