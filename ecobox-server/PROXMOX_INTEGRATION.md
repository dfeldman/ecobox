# Proxmox Integration

This document describes the Proxmox Virtual Environment (PVE) integration features added to the ecobox-server monitoring system.

## Overview

The system now provides comprehensive Proxmox integration that includes:

1. **Automatic API Key Setup**: Creates Proxmox API keys via SSH for authenticated API access
2. **VM Auto-Discovery**: Automatically discovers and creates server entries for Proxmox VMs
3. **API-Based Monitoring**: Uses Proxmox API instead of SSH for VM monitoring (faster, more reliable)
4. **Hybrid Monitoring**: Regular servers use SSH, Proxmox VMs use API, Proxmox hosts use both

## Features

### Automatic Proxmox Detection

The system automatically detects Proxmox hosts during the initialization process by:
- SSH-ing into the server and checking for `/etc/pve/version`
- Setting the system type to `proxmox` in the server's SystemInfo

### API Key Management

For each detected Proxmox host, the system:
1. Creates a unique API token using SSH command: `pveum user token add <user>@pam <tokenid> --privsep 0`
2. Stores the API key securely in the server's data structure
3. Uses the API key for all subsequent Proxmox API calls

### VM Auto-Discovery

The system periodically discovers VMs on each Proxmox host:
- Runs every `vm_discovery_interval` seconds (configurable, default: 300s)
- Lists all non-template VMs using the Proxmox API
- Automatically creates server entries for newly discovered VMs
- Updates existing VM server entries with current information

### Enhanced Monitoring

#### Proxmox Hosts
- **System stats**: Collected via SSH (CPU, memory, network, disk)
- **VM management**: Via Proxmox API
- **Power management**: Via SSH (suspend/wake)

#### Proxmox VMs
- **System stats**: Collected via Proxmox API (faster, more accurate)
- **Network status**: Port scanning on VM IP addresses
- **Power management**: Via Proxmox API (start/stop/suspend)

## Configuration

### VM Discovery Settings

Add to your `config.toml`:

```toml
[dashboard]
vm_discovery_interval = 300  # VM discovery interval in seconds (default: 300)
```

### Example Proxmox Host Configuration

```toml
[[servers]]
id = "proxmox-host"
name = "Proxmox Host"
hostname = "proxmox.example.com"
mac_address = "aa:bb:cc:dd:ee:ff"
ssh_user = "root"
ssh_port = 22
ssh_key_path = "/path/to/ssh/key"

[[servers.services]]
name = "Proxmox Web UI"
port = 8006
type = "https"
```

## Data Structure Changes

### Server Model Extensions

New fields added to the `Server` struct:

```go
// Proxmox-specific fields
ProxmoxAPIKey    *ProxmoxAPIKey `json:"proxmox_api_key,omitempty"`    // API key for Proxmox hosts
IsProxmoxVM      bool           `json:"is_proxmox_vm"`                // True if this is a Proxmox VM
ProxmoxVMID      int            `json:"proxmox_vm_id,omitempty"`      // VMID for Proxmox VMs
ProxmoxNodeName  string         `json:"proxmox_node_name,omitempty"`  // Node name for API calls
LastVMDiscovery  time.Time      `json:"last_vm_discovery"`            // Last VM discovery timestamp
```

### Auto-Created VM Servers

VMs discovered by the system have these characteristics:
- **ID**: `{parent-server-id}-vm-{vmid}` (e.g., "proxmox-host-vm-101")
- **Name**: VM name from Proxmox
- **Hostname**: VM's primary IP address (if available via QEMU agent)
- **ParentServerID**: Set to the Proxmox host's server ID
- **IsProxmoxVM**: `true`
- **ProxmoxVMID**: The VM's numeric ID
- **Source**: `api` (indicates auto-discovered)

## Monitoring Behavior

### Status Checks

1. **Regular servers**: SSH-based ping and port scanning
2. **Proxmox VMs**: API-based status checks, then port scanning if VM is running

### System Metrics Collection

1. **Regular servers**: SSH commands to collect system stats
2. **Proxmox VMs**: API calls to get VM resource usage (CPU, memory, network, disk)

### Power Management

1. **Regular servers**: SSH-based suspend/wake commands
2. **Proxmox VMs**: API-based start/stop/suspend operations

## Logging and Debugging

The system provides detailed logging for Proxmox operations:

- `INFO`: VM discovery results, API key creation success
- `DEBUG`: Detailed API calls, VM status checks
- `ERROR`: API failures, SSH connection issues, VM discovery failures

Enable debug logging by setting `log_level = "debug"` in your configuration.

## Metrics

### Proxmox-Specific Metrics

The system collects and stores these metrics for Proxmox VMs:
- **CPU Usage**: Percentage (0-100%)
- **Memory Usage**: Percentage of allocated memory
- **Network Traffic**: Total MB transferred
- **Disk Usage**: Percentage of allocated disk space
- **Uptime**: VM uptime in seconds

### Standard Metrics

All standard monitoring metrics are still collected:
- Power state changes
- Service availability
- System check success/failure rates
- Response times

## Security Considerations

1. **API Keys**: Stored in memory only, not persisted to disk
2. **SSH Access**: Required for initial setup and host monitoring
3. **TLS**: Proxmox API calls skip certificate verification (configurable)
4. **Permissions**: API tokens created with full privileges for the SSH user

## Troubleshooting

### Common Issues

1. **API Key Creation Fails**
   - Ensure SSH user has sudo privileges
   - Check that Proxmox is properly installed (`/etc/pve/version` exists)
   - Verify SSH connectivity and key authentication

2. **VM Discovery Not Working**
   - Check Proxmox API connectivity (port 8006)
   - Verify API key permissions
   - Check firewall settings

3. **VM Metrics Missing**
   - Ensure QEMU guest agent is installed in VMs for IP detection
   - Check VM is running and responsive
   - Verify Proxmox API access

### Log Analysis

Key log messages to look for:

```
INFO[0000] Setting up Proxmox API key                    server="Proxmox Host"
INFO[0000] Successfully set up Proxmox API key          server="Proxmox Host"
INFO[0000] Discovered Proxmox VMs                       server="Proxmox Host" vm_count=5
INFO[0000] Creating new Proxmox VM server entry         proxmox_host="Proxmox Host" vm_id=101 vm_name="Ubuntu VM"
```

## Integration with Existing Features

The Proxmox integration seamlessly works with existing features:
- **Web Dashboard**: Shows VM status alongside regular servers
- **Metrics Collection**: VM metrics stored in same format as host metrics
- **Power Management**: VM start/stop operations work through the UI
- **Service Monitoring**: Port scanning works on VM IP addresses
- **Authentication**: Uses same authentication system as the rest of the app

## Future Enhancements

Potential future improvements:
1. **Container Support**: Extend to Proxmox LXC containers
2. **Cluster Support**: Support for multi-node Proxmox clusters
3. **Advanced VM Management**: Snapshot creation, migration support
4. **Resource Planning**: CPU/memory allocation recommendations
5. **Cost Analysis**: Power usage estimation for VMs vs physical servers
