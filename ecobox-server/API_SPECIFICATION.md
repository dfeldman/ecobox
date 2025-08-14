# EcoBox Network Dashboard API Specification

## Authentication System

### Authentication Method
- **Type**: JWT Token-based authentication with HTTP-only cookies
- **Cookie Name**: `auth_token`
- **Cookie Settings**: HttpOnly, SameSite=Strict, MaxAge=365 days
- **Token Location**: Cookie header (automatically sent by browser)
- **No CSRF Protection**: The API relies on SameSite=Strict cookies and doesn't implement CSRF tokens

### Authentication Flow
1. User logs in via POST to `/login`
2. Server validates credentials and sets `auth_token` cookie
3. All subsequent requests include the cookie automatically
4. Server validates JWT token on each request
5. User data is available via `/api/auth/me`

## API Base URL
- **Base URL**: `{domain}/api`
- **WebSocket URL**: `{domain}/ws`

## Authentication Endpoints

### POST /login
**Purpose**: User login (form-based)
**Content-Type**: `application/x-www-form-urlencoded`
**Request Body**:
```
username=string&password=string
```
**Success Response**: HTTP 302 redirect to `/`
**Error Response**: Renders login page with error message

### POST /logout  
**Purpose**: User logout
**Headers**: `Accept: application/json` (for JSON response)
**Success Response**:
```json
{
  "success": true,
  "message": "Logged out successfully"
}
```

### GET /setup
**Purpose**: First-time setup page (only available if no admin user exists)
**Response**: HTML setup form

### POST /setup
**Purpose**: Complete first-time setup
**Content-Type**: `application/x-www-form-urlencoded`
**Request Body**:
```
password=string&confirm_password=string
```
**Success Response**: HTTP 302 redirect to `/login`

## API Endpoints (Protected)

All API endpoints require authentication via the `auth_token` cookie.

### GET /api/auth/me
**Purpose**: Get current authenticated user information
**Success Response**:
```json
{
  "success": true,
  "data": {
    "username": "admin",
    "created_at": "2025-01-01T00:00:00Z",
    "last_login": "2025-01-01T12:00:00Z", 
    "is_admin": true
  }
}
```
**Error Response** (401):
```json
{
  "success": false,
  "message": "Authentication required"
}
```

### POST /api/auth/password
**Purpose**: Change user password
**Request Body**:
```json
{
  "current_password": "old_password", // Optional for IAP users
  "new_password": "new_password",
  "confirm_password": "new_password"
}
```
**Success Response**:
```json
{
  "success": true,
  "message": "Password changed successfully"
}
```

### GET /api/auth/users *(Admin Only)*
**Purpose**: List all users
**Success Response**:
```json
{
  "success": true,
  "data": {
    "users": [
      {
        "username": "admin",
        "created_at": "2025-01-01T00:00:00Z",
        "last_login": "2025-01-01T12:00:00Z",
        "is_admin": true
      },
      {
        "username": "testuser", 
        "created_at": "2025-01-02T00:00:00Z",
        "last_login": "2025-01-02T08:00:00Z",
        "is_admin": false
      }
    ]
  }
}
```

### POST /api/auth/users *(Admin Only)*
**Purpose**: Create new user
**Request Body**:
```json
{
  "username": "newuser",
  "is_admin": false
}
```
**Success Response**:
```json
{
  "success": true,
  "message": "User created successfully",
  "data": {
    "user": {
      "username": "newuser",
      "created_at": "2025-01-01T00:00:00Z",
      "last_login": "0001-01-01T00:00:00Z",
      "is_admin": false
    },
    "initial_password": "randomly-generated-password"
  }
}
```

### DELETE /api/auth/users/{username} *(Admin Only)*
**Purpose**: Delete user
**Success Response**:
```json
{
  "success": true,
  "message": "User deleted successfully"
}
```
**Error Response** (400):
```json
{
  "success": false,
  "message": "Cannot delete your own account"
}
```

## Server Management Endpoints

### GET /api/servers
**Purpose**: Get all servers
**Success Response**:
```json
{
  "success": true,
  "data": [
    {
      "id": "server-1",
      "name": "Main Server",
      "hostname": "192.168.1.100", 
      "mac_address": "00:11:22:33:44:55",
      "current_state": "on",
      "desired_state": "on",
      "parent_server_id": "",
      "initialized": true,
      "init_retry_count": 0,
      "last_init_attempt": "2025-01-01T10:00:00Z",
      "last_successful_init": "2025-01-01T10:00:00Z",
      "system_info": {
        "type": "linux",
        "system_id": "unique-system-id",
        "os_version": "Ubuntu 22.04.3 LTS",
        "hostname": "main-server",
        "ip_addresses": [
          {
            "name": "eth0",
            "ip_address": "192.168.1.100", 
            "mac_address": "00:11:22:33:44:55",
            "is_ipv6": false
          }
        ],
        "cpu_usage": 15.5,
        "load_average": [1.2, 1.1, 1.0],
        "memory_usage": {
          "total": 16777216000,
          "used": 4294967296,
          "free": 12582148704,
          "used_percent": 25.6
        },
        "network_usage": {
          "bytes_recv": 1073741824,
          "bytes_sent": 536870912,
          "mbps_recv": 10.5,
          "mbps_sent": 5.2
        },
        "disk_usage": {
          "total": 1099511627776,
          "used": 274877906944,
          "free": 824633720832,
          "used_percent": 25.0,
          "mount_point": "/"
        },
        "power_meter_watts": 85.5,
        "power_estimate_watts": 82.0,
        "suspend_support": true,
        "hibernate_support": false,
        "power_switch_support": true,
        "wake_on_lan_support": true,
        "power_meter_support": true,
        "power_estimate_support": true,
        "wake_on_lan": {
          "supported": true,
          "interfaces": ["eth0"],
          "armed": true
        },
        "vms": [
          {
            "name": "VM-100",
            "primary_ip": "192.168.1.101",
            "status": "running",
            "vm_id": "100"
          }
        ],
        "last_updated": "2025-01-01T12:00:00Z"
      },
      "total_on_time": 86400,
      "total_suspended_time": 3600,
      "total_off_time": 7200,
      "last_state_change": "2025-01-01T08:00:00Z",
      "recent_actions": [
        {
          "timestamp": "2025-01-01T08:00:00Z",
          "action": "wake",
          "success": true,
          "error_msg": "",
          "initiated_by": "manual"
        }
      ],
      "services": [
        {
          "id": "ssh-22",
          "server_id": "server-1",
          "name": "SSH",
          "port": 22,
          "type": "ssh",
          "status": "up",
          "last_check": "2025-01-01T12:00:00Z",
          "source": "discovered"
        }
      ],
      "source": "config",
      "ssh_user": "admin",
      "ssh_port": 22,
      "ssh_key_path": "/path/to/key",
      "proxmox_api_key": {
        "username": "api-user",
        "realm": "pve",
        "token_id": "token1", 
        "secret": "secret-value"
      },
      "is_proxmox_vm": false,
      "proxmox_vm_id": 0,
      "proxmox_node_name": "pve-node1",
      "last_vm_discovery": "2025-01-01T11:00:00Z"
    }
  ]
}
```

### GET /api/servers/{id}
**Purpose**: Get specific server by ID
**Success Response**: Same as individual server object from `/api/servers`
**Error Response** (404):
```json
{
  "success": false,
  "message": "Server not found: server-id"
}
```

### POST /api/servers/{id}/wake
**Purpose**: Wake up a server
**Success Response**:
```json
{
  "success": true,
  "message": "Wake signal sent to Main Server"
}
```
**Error Response** (500):
```json
{
  "success": false,
  "message": "Failed to wake server: error details"
}
```

### POST /api/servers/{id}/suspend  
**Purpose**: Suspend a server
**Success Response**:
```json
{
  "success": true,
  "message": "Suspend command sent to Main Server"
}
```
**Error Response** (500):
```json
{
  "success": false,
  "message": "Failed to suspend server: error details"
}
```

## Metrics Endpoints

### GET /api/metrics
**Purpose**: Get historical metrics data for a server
**Query Parameters**:
- `server` (required): Server ID
- `start` (required): Start time in ISO 8601 format (RFC3339)
- `end` (required): End time in ISO 8601 format (RFC3339)

**Example Request**: 
```
GET /api/metrics?server=server-1&start=2025-01-01T00:00:00Z&end=2025-01-01T23:59:59Z
```

**Success Response**:
```json
{
  "success": true,
  "data": {
    "memory": [
      {
        "timestamp": "2025-01-01T00:00:00Z",
        "value": 25.6
      },
      {
        "timestamp": "2025-01-01T01:00:00Z", 
        "value": 28.2
      }
    ],
    "cpu": [
      {
        "timestamp": "2025-01-01T00:00:00Z",
        "value": 15.5
      },
      {
        "timestamp": "2025-01-01T01:00:00Z",
        "value": 18.3
      }
    ],
    "network": [
      {
        "timestamp": "2025-01-01T00:00:00Z", 
        "value": 10.5
      }
    ],
    "wattage": [
      {
        "timestamp": "2025-01-01T00:00:00Z",
        "value": 85.5
      }
    ]
  }
}
```

**Error Response** (400):
```json
{
  "success": false,
  "message": "Missing required parameter: server"
}
```

### GET /api/metrics/{server}/available
**Purpose**: Get available metrics for a server
**Success Response**:
```json
{
  "success": true,
  "data": {
    "frontend_metrics": {
      "memory": "Memory Usage (%)",
      "cpu": "CPU Usage (%)", 
      "network": "Network Usage (Mbps)",
      "wattage": "Power Usage (W)"
    },
    "all_metrics": {
      "memory": "Memory Usage (%)",
      "cpu": "CPU Usage (%)",
      "network": "Network Usage (Mbps)", 
      "wattage": "Power Usage (W)",
      "power_state_change": "Power State Changes",
      "wake_attempt": "Wake Attempts",
      "suspend_attempt": "Suspend Attempts"
    },
    "server": "server-name"
  }
}
```

## WebSocket Real-time Updates

### Connection
- **URL**: `ws://{domain}/ws` or `wss://{domain}/ws`
- **Authentication**: Requires `auth_token` cookie
- **Protocol**: Text messages with JSON payloads

### Message Format
All WebSocket messages follow this format:
```json
{
  "server_id": "server-1",
  "state": "on",
  "services": [
    {
      "id": "ssh-22",
      "server_id": "server-1", 
      "name": "SSH",
      "port": 22,
      "type": "ssh",
      "status": "up",
      "last_check": "2025-01-01T12:00:00Z",
      "source": "discovered"
    }
  ],
  "server": {
    // Full server object as defined in GET /api/servers
  },
  "metrics": {
    "memory": 25.6,
    "cpu": 15.5,
    "network": 10.5,
    "wattage": 85.5
  }
}
```

### Connection Lifecycle
1. **Connect**: Client establishes WebSocket connection
2. **Initial Data**: Server immediately sends current state for all servers
3. **Updates**: Server sends updates when server states, services, or metrics change
4. **Ping/Pong**: Client should handle ping/pong for keepalive (60s timeout)
5. **Reconnection**: Client should reconnect on disconnect with exponential backoff

## Data Types & Enums

### PowerState
- `"on"` - Server is powered on and responsive
- `"off"` - Server is powered off
- `"suspended"` - Server is suspended/sleeping
- `"unknown"` - Server state is unknown
- `"init_failed"` - Server initialization failed

### ServiceType  
- `"ssh"` - SSH service
- `"rdp"` - RDP service  
- `"vnc"` - VNC service
- `"smb"` - SMB/CIFS service
- `"http"` - HTTP service
- `"https"` - HTTPS service
- `"telnet"` - Telnet service
- `"nfs"` - NFS service
- `"ftp"` - FTP service
- `"database"` - Database service
- `"dns"` - DNS service
- `"mail"` - Mail service
- `"ldap"` - LDAP service
- `"proxmox"` - Proxmox service
- `"custom"` - Custom service

### ServiceStatus
- `"up"` - Service is responding
- `"down"` - Service is not responding

### SystemType
- `"linux"` - Linux system
- `"windows"` - Windows system
- `"proxmox"` - Proxmox VE host
- `"proxmox-vm"` - Proxmox VM
- `"unknown"` - Unknown system type

### Source
- `"config"` - Defined in configuration
- `"api"` - Created via API
- `"discovered"` - Auto-discovered

### ActionType
- `"wake"` - Wake/power on action
- `"suspend"` - Suspend action
- `"initialize"` - Initialization action
- `"reconcile"` - State reconciliation action

## Error Handling

### HTTP Status Codes
- `200` - Success
- `400` - Bad Request (validation error)
- `401` - Unauthorized (authentication required)  
- `403` - Forbidden (insufficient permissions)
- `404` - Not Found
- `500` - Internal Server Error
- `503` - Service Unavailable (metrics system down)

### Error Response Format
```json
{
  "success": false,
  "message": "Descriptive error message"
}
```

## Security Considerations

### Authentication
- JWT tokens in HTTP-only cookies
- SameSite=Strict cookie policy
- No CSRF protection implemented
- Admin-only endpoints check `is_admin` flag

### CORS
- Permissive CORS policy (`Access-Control-Allow-Origin: *`)
- Should be restricted in production

### Rate Limiting
- No rate limiting currently implemented
- Should be added for production use

## Implementation Notes for Vue.js

### State Management
- Use Pinia/Vuex for server state management
- Implement WebSocket connection management
- Handle authentication state globally

### API Client
- Create axios instance with proper error handling
- Implement request/response interceptors
- Handle authentication errors (redirect to login)

### WebSocket Integration  
- Implement automatic reconnection with exponential backoff
- Handle connection state (connecting, connected, disconnected)
- Merge WebSocket updates with existing state

### Real-time Updates
- Subscribe to WebSocket messages on component mount
- Update server state reactively
- Handle partial updates efficiently

### Error Handling
- Implement global error handling
- Show user-friendly error messages
- Handle network connectivity issues

### Authentication Flow
- Implement login/logout functionality
- Handle initial setup flow
- Protect routes based on authentication state
- Handle admin-only features

## Additional Notes

### Time Formats
- All timestamps use ISO 8601 format (RFC3339)
- Example: `"2025-01-01T12:00:00Z"`

### Metrics Time Periods
The API automatically selects appropriate aggregation periods based on time range:
- ≤ 2 hours: 1 minute intervals
- ≤ 12 hours: 5 minute intervals  
- ≤ 48 hours: 30 minute intervals
- ≤ 1 week: 1 hour intervals
- ≤ 1 month: 4 hour intervals
- > 1 month: 1 day intervals

### Proxmox Integration
- Proxmox hosts have `proxmox_api_key` field populated
- Proxmox VMs have `is_proxmox_vm: true` and `proxmox_vm_id` set
- VM discovery runs periodically on Proxmox hosts
- Parent-child relationships tracked via `parent_server_id`

### System Information
- `system_info` contains real-time system metrics
- Historical metrics available via `/api/metrics` endpoint
- Power consumption from both hardware meters and software estimates
- Wake-on-LAN configuration and support detection

This specification provides complete information needed to implement a Vue.js frontend that interfaces with the EcoBox Network Dashboard API.
