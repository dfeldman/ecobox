# EcoBox Server Control Loop Documentation

## Overview

The EcoBox server monitoring and control system operates through several interconnected control loops that manage power states, system monitoring, service discovery, and Proxmox VM management. This document provides a comprehensive overview of how these loops work, their timing, and how they handle different types of machines.

## Architecture

The control system is centered around the `Monitor` struct in `/internal/monitor/monitor.go`, which orchestrates multiple concurrent goroutines that handle different aspects of server management:

1. **Status Check Loop** - Monitors server power states and service availability
2. **Reconciliation Loop** - Ensures desired and actual power states match
3. **System Check Loop** - Gathers system metrics (CPU, memory, network)
4. **Initialization Check Loop** - Handles server discovery and setup
5. **Proxmox Discovery Loop** - Manages VM discovery on Proxmox hosts

## Main Control Loops

### 1. Status Check Loop (`statusCheckLoop`)

**Purpose**: Primary monitoring loop that checks power states and service availability

**Timing**: Runs every `UpdateInterval` seconds (default: 30 seconds)

**Process**:
1. Gets all servers from storage
2. For each server, spawns a goroutine to check status
3. Determines power state using ping and port scanning
4. Updates service status (with discovery for online servers)
5. Records metrics and sends update notifications

**Machine Type Differences**:
- **Regular Servers**: Uses ICMP ping + port scanning + SSH connectivity tests
- **Proxmox VMs**: Uses Proxmox API to get VM status and metrics
- **Proxmox Hosts**: Treated as regular servers but also triggers VM discovery

### 2. Reconciliation Loop (`reconcileLoop`)

**Purpose**: Ensures server desired states match current states through power management actions

**Timing**: Runs every `WoLRetryInterval` seconds (default: 10 seconds)

**Process**:
1. Identifies servers where `CurrentState != DesiredState`
2. Attempts initialization if needed (with retry logic)
3. Executes power management actions:
   - **Wake**: Sends Wake-on-LAN packets (regular servers) or API calls (Proxmox VMs)
   - **Suspend**: Executes SSH suspend commands or Proxmox API shutdown

**Machine Type Differences**:
- **Regular Servers**: Wake-on-LAN + SSH suspend commands
- **Proxmox VMs**: Proxmox API `start`/`shutdown`/`stop` operations
- **Parent/Child Hierarchy**: Automatically wakes parent servers before children

### 3. System Check Loop (`systemCheckLoop`)

**Purpose**: Gathers detailed system metrics and information

**Timing**: Runs every `SystemCheckInterval` seconds (default: 300 seconds / 5 minutes)

**Process**:
1. Only processes servers that are online and initialized
2. Rate-limits checks to prevent excessive SSH connections
3. Collects system metrics (CPU, memory, network usage)
4. Records metrics to time-series storage

**Machine Type Differences**:
- **Regular Servers**: SSH-based system information gathering using various commands
- **Proxmox VMs**: API-based metrics from Proxmox host
- **Proxmox Hosts**: SSH + special Proxmox-specific detection

### 4. Initialization Check Loop (`initializationCheckLoop`)

**Purpose**: Handles server discovery, SSH setup, and system information gathering

**Timing**: Runs every `InitCheckInterval` seconds (default: 3600 seconds / 1 hour)

**Process**:
1. Tests SSH connectivity
2. Detects operating system and system type
3. Discovers network interfaces and MAC addresses
4. Sets up system capabilities
5. For Proxmox hosts: Creates API keys and discovers node names

**Retry Logic**:
- Maximum `maxInitRetries` attempts (default: 3)
- Failed servers marked as `PowerStateInitFailed`
- Automatic retry after `reinitInterval` (default: 1 hour)
- Periodic re-initialization of successful servers

### 5. Proxmox Discovery Loop (`proxmoxDiscoveryLoop`)

**Purpose**: Discovers and manages Proxmox VMs automatically

**Timing**: Runs every `VMDiscoveryInterval` seconds (default: 300 seconds / 5 minutes)

**Process**:
1. Identifies Proxmox hosts (servers with `SystemType == Proxmox`)
2. Sets up API keys if missing
3. Discovers VMs via Proxmox API
4. Creates/updates server entries for discovered VMs
5. Updates VM power states and IP addresses

## Startup Sequence

When the system starts (`cmd/dashboard/main.go`):

1. **Configuration Loading**: Loads `config.toml` with timing parameters
2. **Storage Initialization**: Sets up in-memory storage
3. **Server Loading**: Loads servers from configuration
4. **Component Creation**: 
   - PowerManager (handles Wake-on-LAN and SSH suspend)
   - AuthManager (handles authentication)
   - Monitor (orchestrates all loops)
   - WebServer (provides API/UI)
5. **Monitor Start**: Launches all control loops concurrently
6. **Web Server Start**: Starts HTTP server for API access

## Machine Type Handling

### Regular Physical/Virtual Servers

**Detection**: 
- SSH connectivity test
- Operating system detection via SSH
- Network interface discovery
- MAC address extraction for Wake-on-LAN

**Power Management**:
- **Wake**: Wake-on-LAN magic packets to broadcast addresses
- **Suspend**: SSH commands (`systemctl suspend`, `pm-suspend`, etc.)

**Monitoring**:
- ICMP ping equivalent (TCP connectivity test)
- Port scanning for service discovery
- SSH-based system metrics collection

### Proxmox Virtual Machines

**Detection**:
- Discovered automatically by parent Proxmox host
- Created with `IsProxmoxVM=true` and `ProxmoxVMID`

**Power Management**:
- **Wake**: Proxmox API `start` command
- **Suspend**: Proxmox API `shutdown` (graceful) or `stop` (forced)

**Monitoring**:
- Proxmox API for power state and metrics
- QEMU guest agent for IP address discovery
- Port scanning if IP address available

### Proxmox Hosts

**Detection**:
- SSH-based system type detection identifies Proxmox
- API key automatically created via SSH
- Node name discovered via API

**Special Features**:
- **VM Discovery**: Automatically finds and manages VMs
- **API Management**: Creates and stores API tokens
- **Hierarchy Management**: Acts as parent for discovered VMs

## Timing Configuration

All timing parameters are configurable in `config.toml`:

```toml
[dashboard]
update_interval = 30      # Status checks (seconds)
wol_retry_interval = 10   # Power reconciliation (seconds)
system_check_interval = 300   # System metrics (seconds)
init_check_interval = 3600    # Initialization checks (seconds)
vm_discovery_interval = 300   # Proxmox VM discovery (seconds)
metrics_flush_interval = 300  # Metrics persistence (seconds)
```

## Error Handling and Resilience

### Initialization Failures
- Retry logic with exponential backoff
- Maximum retry limits to prevent infinite loops
- Failed servers marked as `init_failed` state
- Automatic retry after cooldown period

### Network Connectivity Issues
- Multiple detection methods (ping, port scan, SSH)
- Graceful degradation when some methods fail
- Rate limiting to prevent connection flooding

### Proxmox API Failures
- Fallback to SSH-based operations when possible
- API key recreation on authentication failures
- VM state preservation during host downtime

### Parent/Child Dependencies
- Automatic parent server wake before children
- Circular dependency detection
- Graceful handling of missing parent servers

## Metrics and Observability

The system records comprehensive metrics for:

- **Power State Changes**: Transitions, timing, success rates
- **System Performance**: CPU, memory, network usage
- **Operation Timing**: Duration of checks, API calls, SSH operations
- **Error Rates**: Failed operations, connectivity issues
- **Service Availability**: Port scan results, service discovery

Metrics are stored in time-series format with configurable retention and can be accessed via the web API.

## Configuration Examples

### Basic Physical Server
```toml
[[servers]]
id = "server1"
name = "Main Server"
hostname = "192.168.1.100"
mac_address = "aa:bb:cc:dd:ee:ff"
ssh_user = "admin"
ssh_port = 22
ssh_key_path = "/path/to/key"
```

### Server with Parent (Wake parent first)
```toml
[[servers]]
id = "vm1"
name = "VM on Server"
hostname = "192.168.1.101"
parent_server_id = "server1"
ssh_user = "admin"
```

### Proxmox Host (Auto-discovers VMs)
```toml
[[servers]]
id = "proxmox1"
name = "Proxmox Host"
hostname = "192.168.1.10"
mac_address = "11:22:33:44:55:66"
ssh_user = "root"
ssh_key_path = "/path/to/proxmox/key"
```

## Recent Issues and Fixes

### Issue 1: Suspended VM Initialization Problem (Fixed)

**Problem**: If a VM was suspended at startup, initialization would fail because SSH couldn't connect. However, the system would then refuse to wake the VM because it wasn't properly initialized, creating a chicken-and-egg problem.

**Solution**: 
- Modified reconciliation logic to allow power management operations for suspended/off servers even if they're not initialized
- Proxmox VMs don't require SSH initialization for power operations (they use API)
- Added logic to handle different initialization requirements based on server state

### Issue 2: No Power State Change Progress Indicator (Fixed)

**Problem**: When a power management action was triggered (wake/suspend), users had no way to see that the action was in progress until it succeeded or failed.

**Solution**:
- Added transitioning power states: `PowerStateWaking` and `PowerStateSuspending`
- These states are set immediately when a power action begins
- States are cleared when the operation completes successfully or times out
- Added timeout logic (5 minutes for wake, 2 minutes for suspend)
- Added comprehensive metrics for transitioning states

### Issue 3: Separate Proxmox VM Operations (NEW FEATURE)

**Enhancement**: Added distinct operations for Proxmox VMs to provide better control and user clarity.

**New Power States**:
- **`stopped`** - VM is completely stopped (full shutdown, instant restart)
- **`suspended`** - VM is paused/suspended (RAM preserved, instant resume)
- **`stopping`** - Stop operation in progress (transitioning)

**New API Endpoints**:
- **`POST /api/servers/{id}/shutdown`** - Clean shutdown (graceful, for all server types)
- **`POST /api/servers/{id}/stop`** - Force stop (hard stop, Proxmox VMs only)
- **`POST /api/servers/{id}/suspend`** - Suspend/pause (preserves RAM state)

**Operation Differences**:

| Operation | Regular Servers | Proxmox VMs | Result State | Resume Method |
|-----------|-----------------|-------------|--------------|---------------|
| **Suspend** | SSH suspend commands | Proxmox pause API | `suspended` | Wake-on-LAN / Resume API |
| **Shutdown** | SSH suspend (same as suspend) | Proxmox shutdown API | `stopped` | Wake-on-LAN / Start API |
| **Stop** | Not supported | Proxmox stop API (force) | `stopped` | Start API |

**Key Benefits**:
- ✅ **User Clarity** - Distinct operations with clear terminology
- ✅ **Proper State Management** - Stopped vs suspended states properly tracked
- ✅ **Smart Resume** - System automatically chooses Resume vs Start based on previous state
- ✅ **Graceful vs Force** - Clean shutdown option with force stop fallback
- ✅ **RAM Preservation** - True suspend/resume for VMs when needed

## Power State Transitions

The system now supports the following power states:

- **`on`** - Server is running and accessible
- **`off`** - Server is powered off
- **`suspended`** - Server is suspended/hibernated
- **`unknown`** - Server state cannot be determined
- **`init_failed`** - Server initialization has failed
- **`waking`** - Wake operation in progress (transitioning)
- **`suspending`** - Suspend operation in progress (transitioning)

### Transitioning State Behavior

When a power operation is initiated:

1. **Wake Operation**:
   - State immediately changes to `waking`
   - System monitors for server to become accessible
   - Success: State changes to `on` when server responds
   - Timeout: After 5 minutes, reverts to `off` state

2. **Suspend Operation**:
   - State immediately changes to `suspending` 
   - System monitors for server to become inaccessible
   - Success: State changes to `suspended` when server stops responding
   - Timeout: After 2 minutes, reverts to `on` state

### Initialization Logic Updates

The system now handles initialization more intelligently:

- **Online servers**: Require full SSH initialization for monitoring
- **Suspended/Off servers**: Can be powered on without initialization
- **Proxmox VMs**: Don't require SSH initialization (use API instead)
- **Unknown state servers**: Attempt initialization before power operations
