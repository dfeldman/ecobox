// Package proxmox provides a client for interacting with the Proxmox VE API
package proxmox

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client represents a Proxmox API client
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	APIToken   string // Format: "user@realm!tokenid=token-secret"
	Node       string // The node name to operate on
}

// NewClient creates a new Proxmox API client
func NewClient(host string, node string, apiToken string, insecureSkipVerify bool) *Client {
	return &Client{
		BaseURL:  fmt.Sprintf("https://%s:8006/api2/json", host),
		APIToken: apiToken,
		Node:     node,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: insecureSkipVerify,
				},
			},
		},
	}
}

// VM represents a virtual machine
type VM struct {
	VMID                int     `json:"vmid"`
	Name                string  `json:"name"`
	Status              string  `json:"status"`
	CPU                 float64 `json:"cpu"`
	CPUs                int     `json:"cpus"`
	Mem                 int64   `json:"mem"`
	MaxMem              int64   `json:"maxmem"`
	Disk                int64   `json:"disk"`
	MaxDisk             int64   `json:"maxdisk"`
	NetIn               int64   `json:"netin"`
	NetOut              int64   `json:"netout"`
	DiskRead            int64   `json:"diskread"`
	DiskWrite           int64   `json:"diskwrite"`
	Uptime              int64   `json:"uptime"`
	PID                 int     `json:"pid,omitempty"`
	QMPStatus           string  `json:"qmpstatus,omitempty"`
	Lock                string  `json:"lock,omitempty"`
	Tags                string  `json:"tags,omitempty"`
	Template            bool    `json:"template,omitempty"`
	RunningMachine      string  `json:"running-machine,omitempty"`
	RunningQemu         string  `json:"running-qemu,omitempty"`
}

// VMStatus represents detailed VM status
type VMStatus struct {
	VMID                int     `json:"vmid"`
	Name                string  `json:"name"`
	Status              string  `json:"status"`
	CPU                 float64 `json:"cpu"`
	CPUs                int     `json:"cpus"`
	Mem                 int64   `json:"mem"`
	MaxMem              int64   `json:"maxmem"`
	MemHost             int64   `json:"memhost,omitempty"`
	Disk                int64   `json:"disk"`
	MaxDisk             int64   `json:"maxdisk"`
	NetIn               int64   `json:"netin"`
	NetOut              int64   `json:"netout"`
	DiskRead            int64   `json:"diskread"`
	DiskWrite           int64   `json:"diskwrite"`
	Uptime              int64   `json:"uptime"`
	PID                 int     `json:"pid,omitempty"`
	QMPStatus           string  `json:"qmpstatus,omitempty"`
	Lock                string  `json:"lock,omitempty"`
	Tags                string  `json:"tags,omitempty"`
	Template            bool    `json:"template,omitempty"`
	RunningMachine      string  `json:"running-machine,omitempty"`
	RunningQemu         string  `json:"running-qemu,omitempty"`
	Serial              bool    `json:"serial,omitempty"`
	PressureCPUFull     float64 `json:"pressurecpufull,omitempty"`
	PressureCPUSome     float64 `json:"pressurecpusome,omitempty"`
	PressureIOFull      float64 `json:"pressueiofull,omitempty"`
	PressureIOSome      float64 `json:"pressureiosome,omitempty"`
	PressureMemoryFull  float64 `json:"pressurememoryfull,omitempty"`
	PressureMemorySome  float64 `json:"pressurememorysome,omitempty"`
}

// AgentNetworkInterface represents a network interface from the guest agent
type AgentNetworkInterface struct {
	Name         string `json:"name"`
	HardwareAddr string `json:"hardware-address,omitempty"`
	IPAddresses  []struct {
		IPAddressType string `json:"ip-address-type"`
		IPAddress     string `json:"ip-address"`
		Prefix        int    `json:"prefix"`
	} `json:"ip-addresses,omitempty"`
}

// AgentNetworkResponse represents the response from the agent network-get-interfaces command
type AgentNetworkResponse struct {
	Result []AgentNetworkInterface `json:"result"`
}

// TaskStatus represents the status of an async task
type TaskStatus struct {
	UPID       string `json:"upid"`
	Node       string `json:"node"`
	PID        int    `json:"pid"`
	PStart     int64  `json:"pstart"`
	StartTime  int64  `json:"starttime"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	ExitStatus string `json:"exitstatus,omitempty"`
}

// APIResponse represents a generic API response
type APIResponse struct {
	Data interface{} `json:"data"`
}

// ErrorResponse represents an error response from the API
type ErrorResponse struct {
	Errors map[string]string `json:"errors,omitempty"`
	Data   interface{}       `json:"data,omitempty"`
}

// doRequest performs an HTTP request to the Proxmox API
func (c *Client) doRequest(method, path string, body interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if body != nil {
		switch v := body.(type) {
		case url.Values:
			bodyReader = strings.NewReader(v.Encode())
		default:
			jsonData, err := json.Marshal(body)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
			bodyReader = bytes.NewReader(jsonData)
		}
	}

	req, err := http.NewRequest(method, c.BaseURL+path, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authorization header for API token
	req.Header.Set("Authorization", fmt.Sprintf("PVEAPIToken=%s", c.APIToken))
	
	if body != nil {
		if _, ok := body.(url.Values); ok {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		} else {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode >= 400 {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil && errResp.Errors != nil {
			return nil, fmt.Errorf("API error: %v", errResp.Errors)
		}
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// ListVMs returns a list of all VMs on the node
func (c *Client) ListVMs() ([]VM, error) {
	path := fmt.Sprintf("/nodes/%s/qemu", c.Node)
	
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data []VM `json:"data"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Data, nil
}

// GetVMStatus returns detailed status for a specific VM
func (c *Client) GetVMStatus(vmid int) (*VMStatus, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/current", c.Node, vmid)
	
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data VMStatus `json:"data"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response.Data, nil
}

// GetVMIPAddress returns the IP addresses of a VM using the QEMU guest agent
// Note: The QEMU guest agent must be installed and running in the VM
func (c *Client) GetVMIPAddress(vmid int) ([]string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/agent", c.Node, vmid)
	
	// First, check if agent is running with a ping
	pingData := url.Values{}
	pingData.Set("command", "ping")
	
	_, err := c.doRequest("POST", path, pingData)
	if err != nil {
		return nil, fmt.Errorf("guest agent not available: %w", err)
	}

	// Get network interfaces
	data := url.Values{}
	data.Set("command", "network-get-interfaces")
	
	respBody, err := c.doRequest("POST", path, data)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data AgentNetworkResponse `json:"data"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var ips []string
	for _, iface := range response.Data.Result {
		// Skip loopback interface
		if iface.Name == "lo" {
			continue
		}
		for _, addr := range iface.IPAddresses {
			if addr.IPAddressType == "ipv4" {
				ips = append(ips, addr.IPAddress)
			}
		}
	}

	return ips, nil
}

// StartVM starts a VM
func (c *Client) StartVM(vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/start", c.Node, vmid)
	
	respBody, err := c.doRequest("POST", path, nil)
	if err != nil {
		return "", err
	}

	var response struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Data, nil // Returns UPID (task ID)
}

// StopVM stops a VM (hard stop - like pulling the power cord)
func (c *Client) StopVM(vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/stop", c.Node, vmid)
	
	respBody, err := c.doRequest("POST", path, nil)
	if err != nil {
		return "", err
	}

	var response struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Data, nil // Returns UPID (task ID)
}

// ShutdownVM performs a clean shutdown of a VM (sends ACPI shutdown signal)
func (c *Client) ShutdownVM(vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/shutdown", c.Node, vmid)
	
	respBody, err := c.doRequest("POST", path, nil)
	if err != nil {
		return "", err
	}

	var response struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Data, nil // Returns UPID (task ID)
}

// RebootVM reboots a VM (clean reboot - sends ACPI reboot signal)
func (c *Client) RebootVM(vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/reboot", c.Node, vmid)
	
	respBody, err := c.doRequest("POST", path, nil)
	if err != nil {
		return "", err
	}

	var response struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Data, nil // Returns UPID (task ID)
}

// ResetVM performs a hard reset of a VM (like pressing the reset button)
func (c *Client) ResetVM(vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/reset", c.Node, vmid)
	
	respBody, err := c.doRequest("POST", path, nil)
	if err != nil {
		return "", err
	}

	var response struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Data, nil // Returns UPID (task ID)
}

// PauseVM pauses/suspends a VM
func (c *Client) PauseVM(vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/suspend", c.Node, vmid)
	
	respBody, err := c.doRequest("POST", path, nil)
	if err != nil {
		return "", err
	}

	var response struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Data, nil // Returns UPID (task ID)
}

// ResumeVM resumes a paused/suspended VM
func (c *Client) ResumeVM(vmid int) (string, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/resume", c.Node, vmid)
	
	respBody, err := c.doRequest("POST", path, nil)
	if err != nil {
		return "", err
	}

	var response struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Data, nil // Returns UPID (task ID)
}

// RRDData represents time-series data from Proxmox RRD
type RRDData struct {
	Time      float64 `json:"time"`
	CPU       float64 `json:"cpu,omitempty"`
	Mem       float64 `json:"mem,omitempty"`
	MaxMem    float64 `json:"maxmem,omitempty"`
	Disk      float64 `json:"disk,omitempty"`
	MaxDisk   float64 `json:"maxdisk,omitempty"`
	NetIn     float64 `json:"netin,omitempty"`    // bytes/second rate
	NetOut    float64 `json:"netout,omitempty"`   // bytes/second rate
	DiskRead  float64 `json:"diskread,omitempty"`  // bytes/second rate
	DiskWrite float64 `json:"diskwrite,omitempty"` // bytes/second rate
}

// GetVMRRDData gets time-series performance data for a VM
// timeframe can be: "hour", "day", "week", "month", "year"
// cf (consolidation function) can be: "AVERAGE" or "MAX"
func (c *Client) GetVMRRDData(vmid int, timeframe string, cf string) ([]RRDData, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/rrddata?timeframe=%s&cf=%s", 
		c.Node, vmid, timeframe, cf)
	
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data []RRDData `json:"data"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Data, nil
}

// VMMetrics represents calculated metrics for a VM
type VMMetrics struct {
	VMID           int
	Name           string
	CPUPercent     float64
	MemoryPercent  float64
	MemoryUsedGB   float64
	MemoryTotalGB  float64
	NetInRate      float64 // bytes/second
	NetOutRate     float64 // bytes/second
	DiskReadRate   float64 // bytes/second
	DiskWriteRate  float64 // bytes/second
	NetInTotal     int64   // total bytes since start
	NetOutTotal    int64   // total bytes since start
	DiskReadTotal  int64   // total bytes since start
	DiskWriteTotal int64   // total bytes since start
}

// GetVMMetricsWithRate gets VM metrics including calculated network/disk rates
// This function takes two measurements with a delay to calculate rates
func (c *Client) GetVMMetricsWithRate(vmid int, sampleInterval time.Duration) (*VMMetrics, error) {
	// First measurement
	status1, err := c.GetVMStatus(vmid)
	if err != nil {
		return nil, err
	}

	// Wait for sample interval
	time.Sleep(sampleInterval)

	// Second measurement
	status2, err := c.GetVMStatus(vmid)
	if err != nil {
		return nil, err
	}

	// Calculate rates
	intervalSeconds := sampleInterval.Seconds()
	
	metrics := &VMMetrics{
		VMID:           vmid,
		Name:           status2.Name,
		CPUPercent:     status2.CPU * 100,
		MemoryUsedGB:   float64(status2.Mem) / (1024 * 1024 * 1024),
		MemoryTotalGB:  float64(status2.MaxMem) / (1024 * 1024 * 1024),
		MemoryPercent:  float64(status2.Mem) / float64(status2.MaxMem) * 100,
		NetInTotal:     status2.NetIn,
		NetOutTotal:    status2.NetOut,
		DiskReadTotal:  status2.DiskRead,
		DiskWriteTotal: status2.DiskWrite,
	}

	// Calculate rates (bytes per second)
	if intervalSeconds > 0 {
		metrics.NetInRate = float64(status2.NetIn-status1.NetIn) / intervalSeconds
		metrics.NetOutRate = float64(status2.NetOut-status1.NetOut) / intervalSeconds
		metrics.DiskReadRate = float64(status2.DiskRead-status1.DiskRead) / intervalSeconds
		metrics.DiskWriteRate = float64(status2.DiskWrite-status1.DiskWrite) / intervalSeconds
	}

	return metrics, nil
}

// GetVMCurrentMetrics gets the most recent metrics from RRD data (includes rates)
func (c *Client) GetVMCurrentMetrics(vmid int) (*VMMetrics, error) {
	// Get basic status
	status, err := c.GetVMStatus(vmid)
	if err != nil {
		return nil, err
	}

	// Get RRD data for the last hour
	rrdData, err := c.GetVMRRDData(vmid, "hour", "AVERAGE")
	if err != nil {
		// If RRD fails, return basic metrics without rates
		return &VMMetrics{
			VMID:           vmid,
			Name:           status.Name,
			CPUPercent:     status.CPU * 100,
			MemoryUsedGB:   float64(status.Mem) / (1024 * 1024 * 1024),
			MemoryTotalGB:  float64(status.MaxMem) / (1024 * 1024 * 1024),
			MemoryPercent:  float64(status.Mem) / float64(status.MaxMem) * 100,
			NetInTotal:     status.NetIn,
			NetOutTotal:    status.NetOut,
			DiskReadTotal:  status.DiskRead,
			DiskWriteTotal: status.DiskWrite,
		}, nil
	}

	// Get the most recent RRD entry
	var latest *RRDData
	for i := len(rrdData) - 1; i >= 0; i-- {
		if rrdData[i].NetIn > 0 || rrdData[i].NetOut > 0 {
			latest = &rrdData[i]
			break
		}
	}

	metrics := &VMMetrics{
		VMID:           vmid,
		Name:           status.Name,
		CPUPercent:     status.CPU * 100,
		MemoryUsedGB:   float64(status.Mem) / (1024 * 1024 * 1024),
		MemoryTotalGB:  float64(status.MaxMem) / (1024 * 1024 * 1024),
		MemoryPercent:  float64(status.Mem) / float64(status.MaxMem) * 100,
		NetInTotal:     status.NetIn,
		NetOutTotal:    status.NetOut,
		DiskReadTotal:  status.DiskRead,
		DiskWriteTotal: status.DiskWrite,
	}

	if latest != nil {
		metrics.NetInRate = latest.NetIn
		metrics.NetOutRate = latest.NetOut
		metrics.DiskReadRate = latest.DiskRead
		metrics.DiskWriteRate = latest.DiskWrite
	}

	return metrics, nil
}

// GetTaskStatus gets the status of a task by its UPID
func (c *Client) GetTaskStatus(upid string) (*TaskStatus, error) {
	path := fmt.Sprintf("/nodes/%s/tasks/%s/status", c.Node, url.QueryEscape(upid))
	
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data TaskStatus `json:"data"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &response.Data, nil
}

// WaitForTask waits for a task to complete and returns its final status
func (c *Client) WaitForTask(upid string, timeout time.Duration) (*TaskStatus, error) {
	start := time.Now()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			status, err := c.GetTaskStatus(upid)
			if err != nil {
				return nil, err
			}

			// Task is complete if status is not "running"
			if status.Status != "running" {
				return status, nil
			}

			if time.Since(start) > timeout {
				return nil, fmt.Errorf("timeout waiting for task %s", upid)
			}
		case <-time.After(timeout):
			return nil, fmt.Errorf("timeout waiting for task %s", upid)
		}
	}
}

// GetVMConfig gets the configuration of a VM
func (c *Client) GetVMConfig(vmid int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", c.Node, vmid)
	
	respBody, err := c.doRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}

	var response struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response.Data, nil
}

// Helper function to format bytes in human-readable format
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Helper function to format bytes/second rate in human-readable format
func FormatBytesPerSec(bytesPerSec float64) string {
	const unit = 1024
	if bytesPerSec < unit {
		return fmt.Sprintf("%.1f B/s", bytesPerSec)
	}
	div, exp := float64(unit), 0
	for n := bytesPerSec / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB/s", bytesPerSec/div, "KMGTPE"[exp])
}

// Helper function to format uptime in human-readable format
func FormatUptime(seconds int64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, minutes, secs)
	} else if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, secs)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

