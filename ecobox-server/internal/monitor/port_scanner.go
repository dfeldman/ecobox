package monitor

import (
	"net"
	"strconv"
	"time"

	"ecobox-server/internal/models"
)

// PortScanner handles TCP port scanning operations
type PortScanner struct{}

// NewPortScanner creates a new port scanner instance
func NewPortScanner() *PortScanner {
	return &PortScanner{}
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

// QuickScan performs a fast scan of common ports to detect if host is responsive
func (ps *PortScanner) QuickScan(hostname string) bool {
	commonPorts := []int{22, 80, 443, 3389, 5900}
	timeout := 1 * time.Second

	for _, port := range commonPorts {
		if ps.ScanPort(hostname, port, timeout) {
			return true
		}
	}
	return false
}
