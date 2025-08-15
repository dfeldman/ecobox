package models

import "time"

type Service struct {
	ID        string        `json:"id"`
	ServerID  string        `json:"server_id"`
	Name      string        `json:"name"`
	Port      int           `json:"port"`
	Type      ServiceType   `json:"type"`
	Status    ServiceStatus `json:"status"`
	LastCheck time.Time     `json:"last_check"`
	Source    Source        `json:"source"`
}

// KnownPorts maps common port numbers to their service types
var KnownPorts = map[int]ServiceType{
	// Remote Access & Shell
	21:   ServiceTypeFTP,
	22:   ServiceTypeSSH,
	23:   ServiceTypeTelnet,
	3389: ServiceTypeRDP,
	
	// VNC Ports
	5900: ServiceTypeVNC,
	5901: ServiceTypeVNC,
	5902: ServiceTypeVNC,
	5903: ServiceTypeVNC,
	5904: ServiceTypeVNC,
	5905: ServiceTypeVNC,
	5906: ServiceTypeVNC,
	5907: ServiceTypeVNC,
	5908: ServiceTypeVNC,
	5909: ServiceTypeVNC,
	5910: ServiceTypeVNC,
	
	// Web Services
	80:    ServiceTypeHTTP,
	443:   ServiceTypeHTTPS,
	8080:  ServiceTypeHTTP,
	8081:  ServiceTypeHTTP,
	8443:  ServiceTypeHTTPS,
	8006:  ServiceTypeProxmox,
	8123:  ServiceTypeHTTP,    // Home Assistant
	3000:  ServiceTypeHTTP,    // Grafana
	9090:  ServiceTypeHTTP,    // Prometheus
	19999: ServiceTypeHTTP,    // Netdata
	
	// File Sharing
	139:  ServiceTypeSMB,    // NetBIOS
	445:  ServiceTypeSMB,    // SMB/CIFS
	2049: ServiceTypeNFS,
	873:  ServiceTypeCustom, // rsync
	
	// Email
	25:  ServiceTypeMail, // SMTP
	110: ServiceTypeMail, // POP3
	143: ServiceTypeMail, // IMAP
	465: ServiceTypeMail, // SMTPS
	587: ServiceTypeMail, // SMTP Submission
	993: ServiceTypeMail, // IMAPS
	995: ServiceTypeMail, // POP3S
	
	// DNS
	53: ServiceTypeDNS,
	
	// Directory Services
	389: ServiceTypeLDAP,
	636: ServiceTypeLDAP,   // LDAPS
	135: ServiceTypeCustom, // RPC Endpoint Mapper
	88:  ServiceTypeLDAP,   // Kerberos
	
	// Databases
	1433:  ServiceTypeDB, // SQL Server
	3306:  ServiceTypeDB, // MySQL/MariaDB
	5432:  ServiceTypeDB, // PostgreSQL
	1521:  ServiceTypeDB, // Oracle DB
	27017: ServiceTypeDB, // MongoDB
	6379:  ServiceTypeDB, // Redis
	5984:  ServiceTypeDB, // CouchDB
	9200:  ServiceTypeDB, // Elasticsearch
	
	// Virtualization
	902:  ServiceTypeCustom, // VMware ESXi
	5986: ServiceTypeCustom, // WinRM HTTPS
	5985: ServiceTypeCustom, // WinRM HTTP
	
	// Media Services
	32400: ServiceTypeHTTP, // Plex Media Server
	8096:  ServiceTypeHTTP, // Jellyfin
	8920:  ServiceTypeHTTP, // Emby
	7878:  ServiceTypeHTTP, // Radarr
	8989:  ServiceTypeHTTP, // Sonarr
	9117:  ServiceTypeHTTP, // Jackett
	
	// Home Automation
	1883: ServiceTypeCustom, // MQTT
	8883: ServiceTypeCustom, // MQTT over TLS
	1880: ServiceTypeHTTP,   // Node-RED
	8086: ServiceTypeDB,     // InfluxDB
	
	// Network Services
	67:  ServiceTypeCustom, // DHCP Server
	68:  ServiceTypeCustom, // DHCP Client
	161: ServiceTypeCustom, // SNMP
	162: ServiceTypeCustom, // SNMP Trap
	514: ServiceTypeCustom, // Syslog
	123: ServiceTypeCustom, // NTP
	
	// Gaming
	25565: ServiceTypeCustom, // Minecraft
	7777:  ServiceTypeCustom, // Terraria
	27015: ServiceTypeCustom, // Steam
	
	// Development
	3001: ServiceTypeHTTP, // Development Server
	4000: ServiceTypeHTTP, // Development Server Alt
	5000: ServiceTypeHTTP, // Flask/Development
	8000: ServiceTypeHTTP, // Development HTTP
	9000: ServiceTypeHTTP, // Development Alt
	3030: ServiceTypeHTTP, // Cockpit
	
	// Container Services
	2376: ServiceTypeCustom, // Docker TLS
	2377: ServiceTypeCustom, // Docker Swarm
	9443: ServiceTypeHTTPS,  // Portainer
	
	// Backup Services
	10000: ServiceTypeHTTP,   // Webmin
	8200:  ServiceTypeHTTPS,  // Vault
}

// DetectServiceType attempts to determine service type from port number
func DetectServiceType(port int) ServiceType {
	if serviceType, exists := KnownPorts[port]; exists {
		return serviceType
	}
	return ServiceTypeCustom
}
