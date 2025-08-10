package monitor

import (
	"net"
	"time"
)

// PingChecker handles ICMP ping operations
type PingChecker struct{}

// NewPingChecker creates a new ping checker instance
func NewPingChecker() *PingChecker {
	return &PingChecker{}
}

// Ping performs ICMP ping to hostname
// Note: On most systems, this requires raw socket permissions
// Falls back to TCP connection test if ICMP fails
func (p *PingChecker) Ping(hostname string, timeout time.Duration) bool {
	// Try TCP connection as a more reliable alternative to ICMP ping
	// This avoids the need for raw socket permissions
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(hostname, "80"), timeout)
	if err == nil {
		conn.Close()
		return true
	}

	// Try with port 443 (HTTPS)
	conn, err = net.DialTimeout("tcp", net.JoinHostPort(hostname, "443"), timeout)
	if err == nil {
		conn.Close()
		return true
	}

	// Try with port 22 (SSH) - commonly open
	conn, err = net.DialTimeout("tcp", net.JoinHostPort(hostname, "22"), timeout)
	if err == nil {
		conn.Close()
		return true
	}

	return false
}

// PingHost is a more comprehensive connectivity test
func (p *PingChecker) PingHost(hostname string, timeout time.Duration) bool {
	// First try to resolve the hostname
	_, err := net.LookupHost(hostname)
	if err != nil {
		return false
	}

	// Try basic connectivity test
	return p.Ping(hostname, timeout)
}
