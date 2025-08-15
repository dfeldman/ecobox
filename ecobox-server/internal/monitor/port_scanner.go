package monitor

import (
	"fmt"
	"net"
	"strconv"
	"time"

	"ecobox-server/internal/models"
	"github.com/sirupsen/logrus"
)

// PortInfo contains information about a port including its friendly name
type PortInfo struct {
	Port int
	Name string
}

// PortScanner handles TCP port scanning operations
type PortScanner struct {
	logger *logrus.Logger
}

// NewPortScanner creates a new port scanner instance
func NewPortScanner() *PortScanner {
	return &PortScanner{
		logger: logrus.New(),
	}
}

// ScanPort attempts TCP connection to hostname:port
func (ps *PortScanner) ScanPort(hostname string, port int, timeout time.Duration) bool {
	address := net.JoinHostPort(hostname, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return false
	}
	defer conn.Close()
	return true
}

// ScanServices updates status for each service based on port availability
func (ps *PortScanner) ScanServices(hostname string, services []models.Service) []models.Service {
	updatedServices := make([]models.Service, len(services))
	timeout := 3 * time.Second

	for i, service := range services {
		updatedServices[i] = service
		updatedServices[i].LastCheck = time.Now()

		if ps.ScanPort(hostname, service.Port, timeout) {
			updatedServices[i].Status = models.ServiceStatusUp
		} else {
			updatedServices[i].Status = models.ServiceStatusDown
		}
	}

	return updatedServices
}

// CommonHomelabPorts contains a comprehensive list of ports commonly found in homelab environments
var CommonHomelabPorts = []PortInfo{
	// Remote Access & Shell
	{22, "SSH"},
	{23, "Telnet"},
	{3389, "RDP"},

	// VNC Ports
	{5900, "VNC"},
	{5901, "VNC Display 1"},
	{5902, "VNC Display 2"},
	{5903, "VNC Display 3"},
	{5904, "VNC Display 4"},
	{5905, "VNC Display 5"},
	{5906, "VNC Display 6"},
	{5907, "VNC Display 7"},
	{5908, "VNC Display 8"},
	{5909, "VNC Display 9"},
	{5910, "VNC Display 10"},

	// Web Services
	{80, "HTTP"},
	{443, "HTTPS"},
	{8080, "HTTP Alt"},
	{8081, "HTTP Alt 2"},
	{8443, "HTTPS Alt"},
	{8006, "Proxmox Web UI"},
	{8123, "Home Assistant"},
	{3000, "Grafana"},
	{9090, "Prometheus"},
	{19999, "Netdata"},

	// File Sharing
	{21, "FTP"},
	{139, "NetBIOS"},
	{445, "SMB/CIFS"},
	{2049, "NFS"},
	{873, "rsync"},

	// Email
	{25, "SMTP"},
	{110, "POP3"},
	{143, "IMAP"},
	{465, "SMTPS"},
	{587, "SMTP Submission"},
	{993, "IMAPS"},
	{995, "POP3S"},

	// DNS
	{53, "DNS"},

	// Directory Services
	{389, "LDAP"},
	{636, "LDAPS"},
	{135, "RPC Endpoint Mapper"},
	{88, "Kerberos"},

	// Databases
	{1433, "SQL Server"},
	{3306, "MySQL/MariaDB"},
	{5432, "PostgreSQL"},
	{1521, "Oracle DB"},
	{27017, "MongoDB"},
	{6379, "Redis"},
	{5984, "CouchDB"},
	{9200, "Elasticsearch"},

	// Virtualization
	{902, "VMware ESXi"},
	{5986, "WinRM HTTPS"},
	{5985, "WinRM HTTP"},

	// Media Services
	{32400, "Plex Media Server"},
	{8096, "Jellyfin"},
	{8920, "Emby"},
	{7878, "Radarr"},
	{8989, "Sonarr"},
	{9117, "Jackett"},

	// Home Automation
	{1883, "MQTT"},
	{8883, "MQTT over TLS"},
	{1880, "Node-RED"},
	{8086, "InfluxDB"},

	// Network Services
	{67, "DHCP Server"},
	{68, "DHCP Client"},
	{161, "SNMP"},
	{162, "SNMP Trap"},
	{514, "Syslog"},
	{123, "NTP"},

	// Gaming
	{25565, "Minecraft"},
	{7777, "Terraria"},
	{27015, "Steam"},

	// Development
	{3001, "Development Server"},
	{4000, "Development Server Alt"},
	{5000, "Flask/Development"},
	{8000, "Development HTTP"},
	{9000, "Development Alt"},
	{3030, "Cockpit"},

	// Container Services
	{2376, "Docker TLS"},
	{2377, "Docker Swarm"},
	{9443, "Portainer"},

	// Backup Services
	{10000, "Webmin"},
	{8200, "Vault"},
	{9000, "Minio"},
}

// QuickScan performs a fast scan of common ports to detect if host is responsive
func (ps *PortScanner) QuickScan(hostname string) bool {
	// Select a few high-probability ports for quick detection
	quickPorts := []int{22, 80, 443, 3389, 5900}
	timeout := 1 * time.Second

	for _, port := range quickPorts {
		if ps.ScanPort(hostname, port, timeout) {
			return true
		}
	}
	return false
}

// ComprehensiveScan scans all common homelab ports and returns discovered services
func (ps *PortScanner) ComprehensiveScan(hostname string, serverID string) []models.Service {
	var discoveredServices []models.Service
	timeout := 2 * time.Second
	
	ps.logger.WithFields(logrus.Fields{
		"hostname": hostname,
		"server_id": serverID,
		"ports_to_scan": len(CommonHomelabPorts),
	}).Debug("Starting comprehensive port scan")
	
	// Use the CommonHomelabPorts for comprehensive scanning
	for _, portInfo := range CommonHomelabPorts {
		if ps.ScanPort(hostname, portInfo.Port, timeout) {
			service := models.Service{
				ID:        fmt.Sprintf("%s-port-%d", serverID, portInfo.Port),
				ServerID:  serverID,
				Name:      portInfo.Name, // Use the friendly name from PortInfo
				Port:      portInfo.Port,
				Type:      models.DetectServiceType(portInfo.Port),
				Status:    models.ServiceStatusUp,
				LastCheck: time.Now(),
				Source:    models.SourceDiscovered, // New source type for discovered services
			}
			discoveredServices = append(discoveredServices, service)
			ps.logger.WithFields(logrus.Fields{
				"port": portInfo.Port,
				"name": portInfo.Name,
				"type": service.Type,
			}).Debug("Discovered open port")
		}
	}
	
	ps.logger.WithField("discovered_count", len(discoveredServices)).Debug("Comprehensive scan completed")
	return discoveredServices
}

// ScanServicesWithDiscovery scans configured services and discovers additional ones
func (ps *PortScanner) ScanServicesWithDiscovery(hostname string, serverID string, configuredServices []models.Service) []models.Service {
	ps.logger.WithFields(logrus.Fields{
		"hostname": hostname,
		"server_id": serverID,
		"configured_service_count": len(configuredServices),
	}).Debug("Starting service scan with discovery")
	
	var allServices []models.Service
	configuredPorts := make(map[int]bool)
	
	// First, scan and update configured services
	updatedConfigured := ps.ScanServices(hostname, configuredServices)
	allServices = append(allServices, updatedConfigured...)
	ps.logger.WithField("configured_services_scanned", len(updatedConfigured)).Debug("Configured services scan completed")
	
	// Track which ports are already configured
	for _, service := range configuredServices {
		configuredPorts[service.Port] = true
	}
	
	// Discover additional services
	discoveredServices := ps.ComprehensiveScan(hostname, serverID)
	ps.logger.WithField("discovered_services", len(discoveredServices)).Debug("Comprehensive scan completed")
	
	// Only add discovered services that aren't already configured
	addedCount := 0
	for _, discovered := range discoveredServices {
		if !configuredPorts[discovered.Port] {
			allServices = append(allServices, discovered)
			addedCount++
		}
	}
	
	ps.logger.WithFields(logrus.Fields{
		"total_services": len(allServices),
		"configured_services": len(updatedConfigured),
		"new_discovered_services": addedCount,
	}).Debug("Service discovery completed")
	
	return allServices
}

// GetAllPorts returns a slice of all port numbers from CommonHomelabPorts
func GetAllPorts() []int {
	ports := make([]int, len(CommonHomelabPorts))
	for i, portInfo := range CommonHomelabPorts {
		ports[i] = portInfo.Port
	}
	return ports
}

// GetPortInfoByNumber returns the PortInfo for a given port number
func GetPortInfoByNumber(port int) (PortInfo, bool) {
	for _, portInfo := range CommonHomelabPorts {
		if portInfo.Port == port {
			return portInfo, true
		}
	}
	return PortInfo{Port: port, Name: fmt.Sprintf("Port %d", port)}, false
}

// GetPortName returns the friendly name for a given port from CommonHomelabPorts
func (ps *PortScanner) GetPortName(port int) string {
	for _, portInfo := range CommonHomelabPorts {
		if portInfo.Port == port {
			return portInfo.Name
		}
	}
	return fmt.Sprintf("Port %d", port)
}
