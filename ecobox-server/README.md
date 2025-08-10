# EcoBox Server

A Go-based local network server management and monitoring system that controls servers and their services, with Wake-on-LAN functionality and power state management.

## Features

- **Real-time Server Monitoring**: Monitor server power states and service availability
- **Wake-on-LAN Support**: Wake up servers remotely using magic packets
- **SSH Power Control**: Suspend servers via SSH commands
- **Parent-Child Relationships**: Automatically wake parent servers before children
- **Service Monitoring**: Track service availability on each server
- **Web Dashboard**: Modern, responsive web interface with real-time updates
- **WebSocket Updates**: Live updates without page refresh
- **Time Tracking**: Monitor server uptime and power state history
- **Action Logging**: Track all power management actions
- **System Information Collection**: Gather comprehensive system metrics and VM information
- **Power Management Capabilities**: Support for suspend, hibernate, WoL, and power monitoring

## Installation

1. Clone the repository:
   ```bash
   git clone <repository-url>
   cd ecobox-server
   ```

2. Install Go dependencies:
   ```bash
   go mod tidy
   ```

3. Build the application:
   ```bash
   go build -o dashboard ./cmd/dashboard
   ```

## Configuration

Create a `config.toml` file with your server configuration. See the provided `config.toml` example.

### Configuration Options

#### Dashboard Settings
- `port`: Web interface port (default: 8080)
- `update_interval`: Status check interval in seconds (default: 30)
- `wol_retry_interval`: WoL retry interval in seconds (default: 10)
- `wol_max_retries`: Maximum WoL retries (default: 5)
- `log_level`: Logging level ("debug", "info", "warn", "error")

#### Server Settings
- `id`: Unique server identifier
- `name`: Display name
- `hostname`: Network hostname or IP address
- `mac_address`: MAC address for Wake-on-LAN
- `parent_server_id`: ID of parent server (optional)
- `ssh_user`: SSH username (default: "root")
- `ssh_port`: SSH port (default: 22)
- `ssh_key_path`: Path to SSH private key (optional)

#### Service Settings
- `name`: Service display name
- `port`: Port number
- `type`: Service type ("ssh", "rdp", "vnc", "smb", "http", "https", "custom")

## Usage

1. Start the dashboard:
   ```bash
   ./dashboard -config config.toml
   ```

2. Open your web browser and navigate to `http://localhost:8080`

3. View server status, wake servers, and suspend servers from the web interface

## Requirements

### Wake-on-LAN
- Target servers must have Wake-on-LAN enabled in BIOS/UEFI
- Network adapters must support Wake-on-LAN
- Servers must be in the same network segment or proper routing configured

### SSH Suspend
- SSH server must be running on target servers
- SSH key-based authentication recommended
- User must have sudo privileges for suspend commands

### Network Access
- Dashboard must have network access to all monitored servers
- ICMP ping or TCP connectivity required for status monitoring

## API Endpoints

- `GET /api/servers` - List all servers
- `GET /api/servers/{id}` - Get specific server
- `POST /api/servers/{id}/wake` - Wake server
- `POST /api/servers/{id}/suspend` - Suspend server
- `GET /ws` - WebSocket endpoint for real-time updates

## Architecture

The application is structured as follows:

- **cmd/dashboard**: Main application entry point
- **internal/config**: Configuration loading and validation
- **internal/models**: Data models and types
- **internal/storage**: Data storage layer (memory-based)
- **internal/monitor**: Server monitoring and status checking
- **internal/control**: Power management (WoL and SSH)
- **internal/web**: HTTP server and WebSocket handling
- **web/static**: Static assets (CSS, JavaScript)

## Security Considerations

- Use SSH key-based authentication instead of passwords
- Run with minimal required privileges
- Consider network segmentation for management traffic
- Validate all configuration inputs
- Use HTTPS in production (add reverse proxy)

## Troubleshooting

### Wake-on-LAN Not Working
- Verify MAC address format in configuration
- Ensure Wake-on-LAN is enabled in BIOS/UEFI
- Check network adapter Wake-on-LAN settings
- Verify network connectivity and broadcast addresses

### SSH Suspend Failing
- Test SSH connectivity manually
- Verify SSH key permissions (600 for private key)
- Check sudo privileges for suspend commands
- Try different suspend commands (systemctl suspend, pm-suspend)

### Services Not Detected
- Verify port numbers in configuration
- Check firewall rules on target servers
- Ensure services are actually running and listening

## Development

### Prerequisites
- Go 1.21 or later
- Make (optional, for using Makefile)

### Building from Source

1. Clone the repository
2. Install dependencies: `make deps`
3. Build the application: `make build`
4. Run tests: `make test`
5. Start development server: `make dev`

### Project Structure
```
ecobox-server/
├── cmd/dashboard/          # Main application entry point
├── internal/              # Private application code
│   ├── config/           # Configuration loading and validation
│   ├── models/           # Data models and types
│   ├── storage/          # Data storage layer
│   ├── monitor/          # Server monitoring logic
│   ├── control/          # Power management (WoL, SSH)
│   └── web/              # Web server and handlers
├── web/                   # Static web assets
│   ├── static/css/       # CSS stylesheets
│   └── static/js/        # JavaScript files
├── config.toml           # Example configuration
├── Dockerfile            # Docker container definition
├── docker-compose.yml    # Docker Compose configuration
└── Makefile             # Build automation
```

### Docker Deployment

1. Build the Docker image:
   ```bash
   docker build -t ecobox-server .
   ```

2. Run with Docker Compose:
   ```bash
   docker-compose up -d
   ```

### Production Deployment

1. Build the application:
   ```bash
   make build
   ```

2. Install as a system service:
   ```bash
   sudo ./install.sh
   ```

This will:
- Create a dedicated user account
- Install the application to `/opt/ecobox-server`
- Set up a systemd service
- Start the service automatically

## Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

MIT License - see LICENSE file for details.
