# EcoBox Backend Data Structures

## Complete Server Data Structure

This document outlines the complete data structures returned by the EcoBox Go backend API for servers and system information.

## Server Structure

The main `Server` object returned by `/api/servers` contains:

```go
type Server struct {
    // Core Identification
    ID             string       `json:"id"`                  // Unique server identifier
    Name           string       `json:"name"`                // Display name
    Hostname       string       `json:"hostname"`            // Network hostname/IP
    MACAddress     string       `json:"mac_address"`         // MAC address for WoL
    
    // Power State Management  
    CurrentState   PowerState   `json:"current_state"`       // "on", "off", "suspended", "unknown", "init_failed"
    DesiredState   PowerState   `json:"desired_state"`       // Target power state
    Initialized    bool         `json:"initialized"`         // Has been successfully initialized
    
    // Hierarchy (for VMs)
    ParentServerID string       `json:"parent_server_id"`    // ID of parent server (for VMs)
    
    // Initialization Tracking
    InitRetryCount      int       `json:"init_retry_count"`      // Number of init attempts
    LastInitAttempt     time.Time `json:"last_init_attempt"`     // When last init was tried
    LastSuccessfulInit  time.Time `json:"last_successful_init"`  // When last successful init occurred
    
    // System Information (the big one!)
    SystemInfo *SystemInfo `json:"system_info,omitempty"`    // Detailed system metrics and info
    
    // Time Tracking (in seconds)
    TotalOnTime       int64     `json:"total_on_time"`         // Total time server has been online
    TotalSuspendedTime int64     `json:"total_suspended_time"`  // Total time server has been suspended
    TotalOffTime      int64     `json:"total_off_time"`        // Total time server has been offline  
    LastStateChange   time.Time `json:"last_state_change"`     // When state last changed
    
    // Action History
    RecentActions []ServerAction `json:"recent_actions"`       // Recent power/management actions
    
    // Services Running on Server
    Services []Service `json:"services"`                      // Detected/configured services
    
    // Configuration Source
    Source   Source    `json:"source"`                        // "config", "api", "discovered"
    
    // SSH Configuration (for management)
    SSHUser    string `json:"ssh_user"`                       // SSH username
    SSHPort    int    `json:"ssh_port"`                       // SSH port (usually 22)
    SSHKeyPath string `json:"ssh_key_path"`                   // Path to SSH private key
    
    // Proxmox Integration
    ProxmoxAPIKey    *ProxmoxAPIKey `json:"proxmox_api_key,omitempty"`    // Proxmox API credentials (if host)
    IsProxmoxVM      bool           `json:"is_proxmox_vm"`                // True if this is a Proxmox VM
    ProxmoxVMID      int            `json:"proxmox_vm_id,omitempty"`      // VM ID in Proxmox
    ProxmoxNodeName  string         `json:"proxmox_node_name,omitempty"`  // Proxmox node name
    LastVMDiscovery  time.Time      `json:"last_vm_discovery"`            // Last VM discovery time
}
```

## SystemInfo Structure (The Most Important for Frontend)

This is where all the rich system metrics and capabilities are stored:

```go
type SystemInfo struct {
    // === Basic System Identification ===
    Type      SystemType `json:"type"`         // "linux", "windows", "proxmox", "proxmox-vm", "unknown"
    SystemID  string     `json:"system_id"`    // Unique system identifier
    OSVersion string     `json:"os_version"`   // OS version string (e.g. "Ubuntu 22.04.3 LTS")
    Hostname  string     `json:"hostname"`     // System hostname
    
    // === Network Configuration ===
    IPAddresses []NetworkInterface `json:"ip_addresses"`  // All network interfaces
    
    // === Real-Time Performance Metrics ===
    CPUUsage       float64     `json:"cpu_usage"`        // Current CPU usage percentage (0-100)
    LoadAverage    []float64   `json:"load_average"`     // System load averages [1min, 5min, 15min]
    MemoryUsage    MemoryInfo  `json:"memory_usage"`     // Current memory usage details
    NetworkUsage   NetworkInfo `json:"network_usage"`    // Current network I/O rates
    DiskUsage      DiskInfo    `json:"disk_usage"`       // Current disk usage details
    
    // === Power Metrics ===
    PowerMeterWatts    float64 `json:"power_meter_watts"`     // ACTUAL measured power (from smart plug/PDU)
    PowerEstimateWatts float64 `json:"power_estimate_watts"`  // SOFTWARE estimated power consumption
    
    // === Power Management Capabilities ===
    SuspendSupport         bool `json:"suspend_support"`         // Can suspend/resume
    HibernateSupport       bool `json:"hibernate_support"`       // Can hibernate  
    PowerSwitchSupport     bool `json:"power_switch_support"`    // Has physical power switch control
    WakeOnLANSupport       bool `json:"wake_on_lan_support"`     // Supports Wake-on-LAN
    PowerMeterSupport      bool `json:"power_meter_support"`     // Has real power meter device
    PowerEstimateSupport   bool `json:"power_estimate_support"`  // Has software power estimation
    
    // === Wake-on-LAN Configuration ===
    WakeOnLAN WOLInfo `json:"wake_on_lan"`    // WoL configuration details
    
    // === Virtual Machine Hosting ===
    VMs []VMInfo `json:"vms"`                 // VMs hosted on this server (if any)
    
    // === Metadata ===
    LastUpdated time.Time `json:"last_updated"`  // When this data was last collected
}
```

## Sub-Structures

### NetworkInterface
```go
type NetworkInterface struct {
    Name       string `json:"name"`        // Interface name (e.g. "eth0", "wlan0")
    IPAddress  string `json:"ip_address"`  // IP address  
    MACAddress string `json:"mac_address"` // MAC address
    IsIPv6     bool   `json:"is_ipv6"`     // True if IPv6 address
}
```

### MemoryInfo
```go
type MemoryInfo struct {
    Total       uint64  `json:"total"`        // Total RAM in bytes
    Used        uint64  `json:"used"`         // Used RAM in bytes
    Free        uint64  `json:"free"`         // Free RAM in bytes
    UsedPercent float64 `json:"used_percent"` // Usage percentage (0-100)
}
```

### NetworkInfo (Current I/O Rates)
```go
type NetworkInfo struct {
    BytesRecv uint64  `json:"bytes_recv"`  // Total bytes received
    BytesSent uint64  `json:"bytes_sent"`  // Total bytes sent
    MBpsRecv  float64 `json:"mbps_recv"`   // Current receive rate in MB/s
    MBpsSent  float64 `json:"mbps_sent"`   // Current send rate in MB/s  
}
```

### DiskInfo
```go
type DiskInfo struct {
    Total       uint64  `json:"total"`        // Total disk space in bytes
    Used        uint64  `json:"used"`         // Used disk space in bytes
    Free        uint64  `json:"free"`         // Free disk space in bytes
    UsedPercent float64 `json:"used_percent"` // Usage percentage (0-100)
    MountPoint  string  `json:"mount_point"`  // Mount point (e.g. "/", "/home")
}
```

### WOLInfo
```go
type WOLInfo struct {
    Supported  bool     `json:"supported"`   // Wake-on-LAN is supported
    Interfaces []string `json:"interfaces"`  // Network interfaces that support WoL
    Armed      bool     `json:"armed"`       // WoL is currently enabled/armed
}
```

### VMInfo 
```go
type VMInfo struct {
    Name      string `json:"name"`                // VM display name
    PrimaryIP string `json:"primary_ip,omitempty"` // Primary IP (may be empty)
    Status    string `json:"status"`              // "running", "stopped", etc.
    VMID      string `json:"vm_id,omitempty"`     // VM ID (Proxmox VMID, etc.)
}
```

### Service
```go
type Service struct {
    ID        string        `json:"id"`         // Unique service ID
    ServerID  string        `json:"server_id"`  // Parent server ID
    Name      string        `json:"name"`       // Service name/description
    Port      int           `json:"port"`       // Port number
    Type      ServiceType   `json:"type"`       // Service type (see below)
    Status    ServiceStatus `json:"status"`     // "up" or "down"
    LastCheck time.Time     `json:"last_check"` // When status was last checked
    Source    Source        `json:"source"`     // How service was discovered
}
```

### ServerAction
```go
type ServerAction struct {
    Timestamp   time.Time  `json:"timestamp"`    // When action occurred
    Action      ActionType `json:"action"`       // "wake", "suspend", "initialize", "reconcile" 
    Success     bool       `json:"success"`      // Whether action succeeded
    ErrorMsg    string     `json:"error_msg"`    // Error message if failed
    InitiatedBy string     `json:"initiated_by"` // "manual", "api", "scheduler", etc.
}
```

### ProxmoxAPIKey
```go
type ProxmoxAPIKey struct {
    Username string `json:"username"`  // Proxmox username
    Realm    string `json:"realm"`     // Authentication realm  
    TokenID  string `json:"token_id"`  // API token ID
    Secret   string `json:"secret"`    // API token secret
}
```

## Service Types Available

The system recognizes many service types for better categorization:

- **Remote Access**: `ssh`, `rdp`, `vnc`, `telnet`
- **Web Services**: `http`, `https`, `proxmox`  
- **File Sharing**: `smb`, `nfs`, `ftp`
- **Database**: `database` (MySQL, PostgreSQL, MongoDB, etc.)
- **Mail**: `mail` (SMTP, IMAP, POP3, etc.)
- **Directory**: `ldap`
- **DNS**: `dns`
- **Custom**: `custom` (anything else)

## What Frontend Can Display

### ✅ **Easily Displayable Data:**
- Server basic info (name, hostname, state)
- Power states and capabilities
- Real-time metrics (CPU, memory, network, disk usage percentages)
- Power consumption (both actual and estimated watts)
- Services with status (perfect for your new button layout!)
- VM list and status
- Network interfaces and IPs
- System type and OS version
- Recent actions/history
- Timestamps and uptime calculations

### ✅ **Rich Data Available:**
- Load averages (1, 5, 15 minute)
- Detailed memory/disk usage in bytes + percentages
- Network I/O rates in MB/s
- Wake-on-LAN configuration and support
- All power management capabilities
- Proxmox integration data
- Service port numbers and types
- Complete network interface details

### ⚠️ **Historical Data Limitations:**
- Current values only - no historical time series in this API
- Historical metrics are stored separately in CSV files in `/metrics/` directory
- Would need separate API endpoints for historical charts/graphs

### 🔄 **Real-Time Updates:**
- All current values are updated via WebSocket connections
- `LastUpdated` timestamps show data freshness
- WebSocket provides live updates of state changes

## Suggestions for Frontend Improvements

Based on this data structure, you could add these features:

1. **Power Management Dashboard** - Show both actual and estimated power consumption
2. **Detailed System Info Modal** - OS version, system ID, load averages
3. **Network Interface List** - All IPs and interfaces per server
4. **VM Management** - Expand/collapse VM lists under parent servers
5. **Service Details** - Click services to show port, type, last check time
6. **Power Capabilities** - Show what each server supports (suspend, WoL, etc.)
7. **Action History** - Recent power actions with success/failure
8. **Advanced Sorting** - By system type, power consumption, uptime, etc.

The data structure is very rich and provides much more than what's currently displayed in the frontend!
