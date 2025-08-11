# Proxmox VE API Go Package

A Go package for interacting with the Proxmox Virtual Environment (PVE) API, focused on VM management and monitoring.

## Features

- List all VMs on a node
- Get detailed VM status and configuration
- Retrieve VM IP addresses (via QEMU Guest Agent)
- Get real-time performance metrics including:
  - CPU usage
  - Memory usage
  - **Network I/O rates (bytes/second)**
  - **Disk I/O rates (bytes/second)**
  - Total network/disk bytes transferred
- VM power operations (start, stop, shutdown, reboot, reset, pause, resume)
- Task management with status tracking
- Support for API token authentication

## Installation

```bash
go get github.com/yourusername/proxmox-go
```

## Authentication

This package uses API token authentication. To create an API token in Proxmox:

1. Go to Datacenter → Permissions → API Tokens
2. Add a new token for your user
3. Note the token ID and secret
4. Format: `user@realm!tokenid=token-secret`

Example: `automation@pve!mytoken=550a7f46-aab6-4f9c-8d5a-3fdf1e50d2e3`

## Usage

### Initialize Client

```go
client := proxmox.NewClient(
    "192.168.1.100",  // Proxmox host
    "pve",            // Node name
    "user@pve!token=secret", // API token
    true,             // Skip TLS verification
)
```

### List VMs

```go
vms, err := client.ListVMs()
for _, vm := range vms {
    fmt.Printf("VM %d: %s (Status: %s)\n", vm.VMID, vm.Name, vm.Status)
}
```

### Get Network/Disk I/O Rates

The API provides cumulative bytes since VM start, not bytes/second. This package offers three methods to get rates:

#### Method 1: Calculate rates by sampling
```go
// Takes two measurements with 5-second interval
metrics, err := client.GetVMMetricsWithRate(vmid, 5*time.Second)
fmt.Printf("Network Rate: In=%s, Out=%s\n",
    proxmox.FormatBytesPerSec(metrics.NetInRate),
    proxmox.FormatBytesPerSec(metrics.NetOutRate))
```

#### Method 2: Get rates from Proxmox RRD data
```go
// Get historical data with rates already calculated
rrdData, err := client.GetVMRRDData(vmid, "hour", "AVERAGE")
// Returns array of data points with NetIn/NetOut as bytes/second
```

#### Method 3: Get current rates from latest RRD data
```go
metrics, err := client.GetVMCurrentMetrics(vmid)
fmt.Printf("Current Rate: In=%s, Out=%s\n",
    proxmox.FormatBytesPerSec(metrics.NetInRate),
    proxmox.FormatBytesPerSec(metrics.NetOutRate))
```

### VM Power Operations

```go
// Start VM
upid, err := client.StartVM(vmid)

// Graceful shutdown (ACPI)
upid, err := client.ShutdownVM(vmid)

// Hard stop
upid, err := client.StopVM(vmid)

// Reboot (ACPI)
upid, err := client.RebootVM(vmid)

// Hard reset
upid, err := client.ResetVM(vmid)

// Pause/Suspend
upid, err := client.PauseVM(vmid)

// Resume
upid, err := client.ResumeVM(vmid)
```

### Get VM IP Addresses

Requires QEMU Guest Agent installed and running in the VM:

```go
ips, err := client.GetVMIPAddress(vmid)
```

### Task Management

Power operations return a UPID (task ID). You can wait for comp# Proxmox VE API Go Package

A Go package for interacting with the Proxmox Virtual Environment (PVE) API
