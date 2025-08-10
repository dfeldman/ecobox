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
	22:   ServiceTypeSSH,
	3389: ServiceTypeRDP,
	5900: ServiceTypeVNC,
	445:  ServiceTypeSMB,
	139:  ServiceTypeSMB,
	80:   ServiceTypeHTTP,
	443:  ServiceTypeHTTPS,
	8080: ServiceTypeHTTP,
	9000: ServiceTypeHTTP,
}

// DetectServiceType attempts to determine service type from port number
func DetectServiceType(port int) ServiceType {
	if serviceType, exists := KnownPorts[port]; exists {
		return serviceType
	}
	return ServiceTypeCustom
}
