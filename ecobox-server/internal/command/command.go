package command

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"ecobox-server/internal/models"
	"github.com/sirupsen/logrus"
)

// SSHExecutor interface for SSH command execution
type SSHExecutor interface {
	ExecuteCommand(host string, port int, user string, keyPath string, command string) error
	ExecuteCommandWithOutput(host string, port int, user string, keyPath string, command string) (string, error)
}

// Commander provides system management operations
type Commander struct {
	executor SSHExecutor
	logger   *logrus.Logger
}

// NewCommander creates a new Commander instance
func NewCommander(executor SSHExecutor, logger *logrus.Logger) *Commander {
	if logger == nil {
		logger = logrus.New()
	}
	return &Commander{
		executor: executor,
		logger:   logger,
	}
}

// CommandError represents a detailed command error
type CommandError struct {
	Type    string
	Message string
	Command string
	Output  string
	Err     error
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// TestConnection tests if SSH connection can be established
func (c *Commander) TestConnection(host string, port int, user string, keyPath string) error {
	c.logger.WithFields(logrus.Fields{
		"host": host,
		"port": port,
		"user": user,
	}).Debug("Testing SSH connection")

	output, err := c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, "echo 'Connection successful'")
	if err != nil {
		return c.handleSSHError(err, "echo", output)
	}

	c.logger.Info("Connection test successful")
	return nil
}

// DetectSystemType detects the type of system
func (c *Commander) DetectSystemType(host string, port int, user string, keyPath string) (models.SystemType, error) {
	c.logger.Debug("Detecting system type")

	// Check for Proxmox
	output, err := c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, "test -f /etc/pve/version && cat /etc/pve/version")
	if err == nil && strings.Contains(output, "pve-manager") {
		c.logger.Info("Detected Proxmox system")
		return models.SystemTypeProxmox, nil
	}

	// Check for Linux
	output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, "uname -s")
	if err == nil {
		if strings.Contains(strings.ToLower(output), "linux") {
			c.logger.Info("Detected Linux system")
			return models.SystemTypeLinux, nil
		}
	}

	// Check for Windows (via PowerShell through SSH)
	output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, "powershell.exe -Command \"$PSVersionTable.PSVersion\"")
	if err == nil && strings.Contains(output, "Major") {
		c.logger.Info("Detected Windows system")
		return models.SystemTypeWindows, nil
	}

	c.logger.Warn("Unable to detect system type")
	return models.SystemTypeUnknown, &CommandError{
		Type:    "DetectionError",
		Message: "Unable to detect system type",
		Command: "system detection",
		Output:  output,
	}
}

// GetCPUUsage gets CPU usage for all cores combined
func (c *Commander) GetCPUUsage(host string, port int, user string, keyPath string, systemType models.SystemType) (float64, error) {
	c.logger.Debug("Getting CPU usage")

	var cmd string
	var output string
	var err error

	switch systemType {
	case models.SystemTypeLinux, models.SystemTypeProxmox:
		// Use a simpler approach with vmstat for CPU usage
		cmd = "vmstat 1 2 | tail -1 | awk '{print 100-$15}'"
		output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
	case models.SystemTypeWindows:
		cmd = "powershell.exe -Command \"Get-CimInstance Win32_Processor | Measure-Object -Property LoadPercentage -Average | Select-Object -ExpandProperty Average\""
		output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
	default:
		return 0, &CommandError{
			Type:    "UnsupportedError",
			Message: fmt.Sprintf("CPU usage not supported for system type: %s", systemType),
		}
	}

	if err != nil {
		return 0, c.handleSSHError(err, cmd, output)
	}

	usage, err := strconv.ParseFloat(strings.TrimSpace(output), 64)
	if err != nil {
		return 0, &CommandError{
			Type:    "ParseError",
			Message: "Failed to parse CPU usage",
			Command: cmd,
			Output:  output,
			Err:     err,
		}
	}

	return usage, nil
}

// GetLoadAverage gets system load average
func (c *Commander) GetLoadAverage(host string, port int, user string, keyPath string, systemType models.SystemType) ([]float64, error) {
	c.logger.Debug("Getting load average")

	switch systemType {
	case models.SystemTypeLinux, models.SystemTypeProxmox:
		cmd := "cat /proc/loadavg"
		output, err := c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			return nil, c.handleSSHError(err, cmd, output)
		}

		parts := strings.Fields(output)
		if len(parts) < 3 {
			return nil, &CommandError{
				Type:    "ParseError",
				Message: "Invalid load average format",
				Command: cmd,
				Output:  output,
			}
		}

		loads := make([]float64, 3)
		for i := 0; i < 3; i++ {
			loads[i], err = strconv.ParseFloat(parts[i], 64)
			if err != nil {
				return nil, &CommandError{
					Type:    "ParseError",
					Message: fmt.Sprintf("Failed to parse load average value %d", i),
					Command: cmd,
					Output:  output,
					Err:     err,
				}
			}
		}
		return loads, nil

	case models.SystemTypeWindows:
		// Windows doesn't have load average, but we can get processor queue length as an approximation
		cmd := `powershell.exe -Command "(Get-Counter '\System\Processor Queue Length').CounterSamples[0].CookedValue"`
		output, err := c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			// If counter fails, return CPU usage as a single value approximation
			cpu, cpuErr := c.GetCPUUsage(host, port, user, keyPath, systemType)
			if cpuErr != nil {
				return nil, c.handleSSHError(err, cmd, output)
			}
			// Return CPU usage percentage / 100 as a rough load approximation
			load := cpu / 100.0
			return []float64{load, load, load}, nil
		}

		queueLength, err := strconv.ParseFloat(strings.TrimSpace(output), 64)
		if err != nil {
			// Fallback to CPU usage
			cpu, _ := c.GetCPUUsage(host, port, user, keyPath, systemType)
			load := cpu / 100.0
			return []float64{load, load, load}, nil
		}

		// Return queue length as all three values (Windows doesn't have 1/5/15 min averages)
		return []float64{queueLength, queueLength, queueLength}, nil

	default:
		return nil, &CommandError{
			Type:    "UnsupportedError",
			Message: fmt.Sprintf("Load average not supported for system type: %s", systemType),
		}
	}
}

// GetMemoryUsage gets memory usage information
func (c *Commander) GetMemoryUsage(host string, port int, user string, keyPath string, systemType models.SystemType) (*models.MemoryInfo, error) {
	c.logger.Debug("Getting memory usage")

	var cmd string
	var output string
	var err error

	switch systemType {
	case models.SystemTypeLinux, models.SystemTypeProxmox:
		cmd = "sudo free -b | grep '^Mem:'"
		output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			return nil, c.handleSSHError(err, cmd, output)
		}

		parts := strings.Fields(output)
		if len(parts) < 3 {
			return nil, &CommandError{
				Type:    "ParseError",
				Message: "Invalid memory info format",
				Command: cmd,
				Output:  output,
			}
		}

		total, _ := strconv.ParseUint(parts[1], 10, 64)
		used, _ := strconv.ParseUint(parts[2], 10, 64)
		free := total - used
		usedPercent := float64(used) / float64(total) * 100

		return &models.MemoryInfo{
			Total:       total,
			Used:        used,
			Free:        free,
			UsedPercent: usedPercent,
		}, nil

	case models.SystemTypeWindows:
		cmd = `powershell.exe -Command "Get-CimInstance Win32_OperatingSystem | Select-Object TotalVisibleMemorySize, FreePhysicalMemory | ConvertTo-Json"`
		output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			return nil, c.handleSSHError(err, cmd, output)
		}

		// Parse Windows memory info (values are in KB)
		var winMem struct {
			TotalVisibleMemorySize uint64
			FreePhysicalMemory     uint64
		}
		
		if err := json.Unmarshal([]byte(output), &winMem); err != nil {
			// Fallback to parsing text output
			cmd = `powershell.exe -Command "$mem = Get-CimInstance Win32_OperatingSystem; Write-Output $mem.TotalVisibleMemorySize, $mem.FreePhysicalMemory"`
			output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
			if err != nil {
				return nil, c.handleSSHError(err, cmd, output)
			}
			
			lines := strings.Split(strings.TrimSpace(output), "\n")
			if len(lines) >= 2 {
				winMem.TotalVisibleMemorySize, _ = strconv.ParseUint(strings.TrimSpace(lines[0]), 10, 64)
				winMem.FreePhysicalMemory, _ = strconv.ParseUint(strings.TrimSpace(lines[1]), 10, 64)
			}
		}

		total := winMem.TotalVisibleMemorySize * 1024 // Convert KB to bytes
		free := winMem.FreePhysicalMemory * 1024
		used := total - free
		usedPercent := float64(used) / float64(total) * 100

		return &models.MemoryInfo{
			Total:       total,
			Used:        used,
			Free:        free,
			UsedPercent: usedPercent,
		}, nil

	default:
		return nil, &CommandError{
			Type:    "UnsupportedError",
			Message: fmt.Sprintf("Memory usage not supported for system type: %s", systemType),
		}
	}
}

// GetNetworkUsage gets network usage in MB/s
func (c *Commander) GetNetworkUsage(host string, port int, user string, keyPath string, systemType models.SystemType) (*models.NetworkInfo, error) {
	c.logger.Debug("Getting network usage")

	switch systemType {
	case models.SystemTypeLinux, models.SystemTypeProxmox:
		// Get initial readings
		cmd1 := "cat /proc/net/dev | grep -v lo: | awk 'NR>2 {rx+=$2; tx+=$10} END {print rx, tx}'"
		output1, err := c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd1)
		if err != nil {
			return nil, c.handleSSHError(err, cmd1, output1)
		}

		parts1 := strings.Fields(output1)
		if len(parts1) != 2 {
			return nil, &CommandError{
				Type:    "ParseError",
				Message: "Invalid network stats format",
				Command: cmd1,
				Output:  output1,
			}
		}

		rx1, _ := strconv.ParseUint(parts1[0], 10, 64)
		tx1, _ := strconv.ParseUint(parts1[1], 10, 64)

		// Wait 1 second
		time.Sleep(1 * time.Second)

		// Get second reading
		output2, err := c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd1)
		if err != nil {
			return nil, c.handleSSHError(err, cmd1, output2)
		}

		parts2 := strings.Fields(output2)
		if len(parts2) != 2 {
			return nil, &CommandError{
				Type:    "ParseError",
				Message: "Invalid network stats format",
				Command: cmd1,
				Output:  output2,
			}
		}

		rx2, _ := strconv.ParseUint(parts2[0], 10, 64)
		tx2, _ := strconv.ParseUint(parts2[1], 10, 64)

		// Calculate MB/s
		mbpsRecv := float64(rx2-rx1) / 1024 / 1024
		mbpsSent := float64(tx2-tx1) / 1024 / 1024

		return &models.NetworkInfo{
			BytesRecv: rx2,
			BytesSent: tx2,
			MBpsRecv:  mbpsRecv,
			MBpsSent:  mbpsSent,
		}, nil

	case models.SystemTypeWindows:
		// Get initial network stats
		cmd := `powershell.exe -Command "Get-NetAdapterStatistics | Measure-Object -Property ReceivedBytes, SentBytes -Sum | Select-Object -ExpandProperty Sum"`
		output1, err := c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			return nil, c.handleSSHError(err, cmd, output1)
		}

		lines1 := strings.Split(strings.TrimSpace(output1), "\n")
		if len(lines1) < 2 {
			return nil, &CommandError{
				Type:    "ParseError",
				Message: "Invalid Windows network stats format",
				Command: cmd,
				Output:  output1,
			}
		}

		rx1, _ := strconv.ParseUint(strings.TrimSpace(lines1[0]), 10, 64)
		tx1, _ := strconv.ParseUint(strings.TrimSpace(lines1[1]), 10, 64)

		// Wait 1 second
		time.Sleep(1 * time.Second)

		// Get second reading
		output2, err := c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			return nil, c.handleSSHError(err, cmd, output2)
		}

		lines2 := strings.Split(strings.TrimSpace(output2), "\n")
		if len(lines2) < 2 {
			return nil, &CommandError{
				Type:    "ParseError",
				Message: "Invalid Windows network stats format",
				Command: cmd,
				Output:  output2,
			}
		}

		rx2, _ := strconv.ParseUint(strings.TrimSpace(lines2[0]), 10, 64)
		tx2, _ := strconv.ParseUint(strings.TrimSpace(lines2[1]), 10, 64)

		// Calculate MB/s
		mbpsRecv := float64(rx2-rx1) / 1024 / 1024
		mbpsSent := float64(tx2-tx1) / 1024 / 1024

		return &models.NetworkInfo{
			BytesRecv: rx2,
			BytesSent: tx2,
			MBpsRecv:  mbpsRecv,
			MBpsSent:  mbpsSent,
		}, nil

	default:
		return nil, &CommandError{
			Type:    "UnsupportedError",
			Message: fmt.Sprintf("Network usage not supported for system type: %s", systemType),
		}
	}
}

// GetNetworkInterfaces gets all IP addresses and MAC addresses
func (c *Commander) GetNetworkInterfaces(host string, port int, user string, keyPath string, systemType models.SystemType) ([]models.NetworkInterface, error) {
	c.logger.Debug("Getting network interfaces")

	var cmd string
	var output string
	var err error

	switch systemType {
	case models.SystemTypeLinux, models.SystemTypeProxmox:
		cmd = "sudo ip -j addr show"
		output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			// Fallback to non-JSON format
			cmd = "sudo ip addr show | grep -E 'inet |inet6 |link/ether'"
			output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
			if err != nil {
				return nil, c.handleSSHError(err, cmd, output)
			}
			return c.parseNetworkInterfacesText(output), nil
		}
		return c.parseNetworkInterfacesJSON(output)

	case models.SystemTypeWindows:
		// Get network adapter configuration with IP and MAC addresses
		cmd = `powershell.exe -Command "Get-NetAdapter | Where-Object {$_.Status -eq 'Up'} | ForEach-Object { $adapter = $_; Get-NetIPAddress -InterfaceIndex $adapter.ifIndex -ErrorAction SilentlyContinue | ForEach-Object { [PSCustomObject]@{Name=$adapter.Name; MAC=$adapter.MacAddress; IP=$_.IPAddress; Family=$_.AddressFamily} } } | ConvertTo-Json -Compress"`
		output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			// Fallback to WMI query
			cmd = `powershell.exe -Command "Get-CimInstance Win32_NetworkAdapterConfiguration | Where-Object {$_.IPEnabled -eq $true} | Select-Object Description, MACAddress, IPAddress | ConvertTo-Json -Compress"`
			output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
			if err != nil {
				return nil, c.handleSSHError(err, cmd, output)
			}
			return c.parseWindowsNetworkInterfacesWMI(output)
		}
		return c.parseWindowsNetworkInterfaces(output)

	default:
		return nil, &CommandError{
			Type:    "UnsupportedError",
			Message: fmt.Sprintf("Network interfaces not supported for system type: %s", systemType),
		}
	}
}

// GetSystemID gets system ID from /etc/machine-id or Windows
func (c *Commander) GetSystemID(host string, port int, user string, keyPath string, systemType models.SystemType) (string, error) {
	c.logger.Debug("Getting system ID")

	var cmd string
	var output string
	var err error

	switch systemType {
	case models.SystemTypeLinux, models.SystemTypeProxmox:
		cmd = "sudo cat /etc/machine-id"
		output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			return "", c.handleSSHError(err, cmd, output)
		}
		return strings.TrimSpace(output), nil

	case models.SystemTypeWindows:
		cmd = `powershell.exe -Command "(Get-CimInstance Win32_ComputerSystemProduct).UUID"`
		output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			return "", c.handleSSHError(err, cmd, output)
		}
		return strings.TrimSpace(output), nil

	default:
		return "", &CommandError{
			Type:    "UnsupportedError",
			Message: fmt.Sprintf("System ID not supported for system type: %s", systemType),
		}
	}
}

// GetOSVersion gets OS version information
func (c *Commander) GetOSVersion(host string, port int, user string, keyPath string, systemType models.SystemType) (string, error) {
	c.logger.Debug("Getting OS version")

	var cmd string
	var output string
	var err error

	switch systemType {
	case models.SystemTypeLinux, models.SystemTypeProxmox:
		cmd = "cat /etc/os-release | grep -E '^(NAME|VERSION)='"
		output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			return "", c.handleSSHError(err, cmd, output)
		}

		lines := strings.Split(output, "\n")
		info := make(map[string]string)
		for _, line := range lines {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := strings.Trim(parts[1], "\"")
				info[key] = value
			}
		}
		return fmt.Sprintf("%s %s", info["NAME"], info["VERSION"]), nil

	case models.SystemTypeWindows:
		cmd = `powershell.exe -Command "(Get-CimInstance Win32_OperatingSystem).Caption + ' ' + (Get-CimInstance Win32_OperatingSystem).Version"`
		output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			return "", c.handleSSHError(err, cmd, output)
		}
		return strings.TrimSpace(output), nil

	default:
		return "", &CommandError{
			Type:    "UnsupportedError",
			Message: fmt.Sprintf("OS version not supported for system type: %s", systemType),
		}
	}
}

// GetDiskUsage gets disk space information
func (c *Commander) GetDiskUsage(host string, port int, user string, keyPath string, systemType models.SystemType) (*models.DiskInfo, error) {
	c.logger.Debug("Getting disk usage")

	var cmd string
	var output string
	var err error

	switch systemType {
	case models.SystemTypeLinux, models.SystemTypeProxmox:
		cmd = "sudo df -B1 / | tail -1"
		output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			return nil, c.handleSSHError(err, cmd, output)
		}

		parts := strings.Fields(output)
		if len(parts) < 4 {
			return nil, &CommandError{
				Type:    "ParseError",
				Message: "Invalid disk usage format",
				Command: cmd,
				Output:  output,
			}
		}

		total, _ := strconv.ParseUint(parts[1], 10, 64)
		used, _ := strconv.ParseUint(parts[2], 10, 64)
		free, _ := strconv.ParseUint(parts[3], 10, 64)
		usedPercent := float64(used) / float64(total) * 100

		return &models.DiskInfo{
			Total:       total,
			Used:        used,
			Free:        free,
			UsedPercent: usedPercent,
			MountPoint:  "/",
		}, nil

	case models.SystemTypeWindows:
		cmd = `powershell.exe -Command "Get-PSDrive C | Select-Object @{Name='Used';Expression={$_.Used}}, @{Name='Free';Expression={$_.Free}}, @{Name='Total';Expression={$_.Used + $_.Free}} | ConvertTo-Json"`
		output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			return nil, c.handleSSHError(err, cmd, output)
		}

		var winDisk struct {
			Used  uint64
			Free  uint64
			Total uint64
		}

		if err := json.Unmarshal([]byte(output), &winDisk); err != nil {
			// Fallback to text parsing
			cmd = `powershell.exe -Command "$drive = Get-PSDrive C; Write-Output $drive.Used, $drive.Free"`
			output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
			if err != nil {
				return nil, c.handleSSHError(err, cmd, output)
			}

			lines := strings.Split(strings.TrimSpace(output), "\n")
			if len(lines) >= 2 {
				winDisk.Used, _ = strconv.ParseUint(strings.TrimSpace(lines[0]), 10, 64)
				winDisk.Free, _ = strconv.ParseUint(strings.TrimSpace(lines[1]), 10, 64)
				winDisk.Total = winDisk.Used + winDisk.Free
			}
		}

		usedPercent := float64(winDisk.Used) / float64(winDisk.Total) * 100

		return &models.DiskInfo{
			Total:       winDisk.Total,
			Used:        winDisk.Used,
			Free:        winDisk.Free,
			UsedPercent: usedPercent,
			MountPoint:  "C:",
		}, nil

	default:
		return nil, &CommandError{
			Type:    "UnsupportedError",
			Message: fmt.Sprintf("Disk usage not supported for system type: %s", systemType),
		}
	}
}

// CheckWakeOnLAN checks if Wake-on-LAN is supported
func (c *Commander) CheckWakeOnLAN(host string, port int, user string, keyPath string, systemType models.SystemType) (*models.WOLInfo, error) {
	c.logger.Debug("Checking Wake-on-LAN support")

	if err := c.validateSystemType(systemType, models.SystemTypeLinux, models.SystemTypeProxmox); err != nil {
		return nil, err
	}

	// Get network interfaces using ip command
	cmd := "ip link show | grep -E '^[0-9]+:' | grep -v 'lo:' | cut -d: -f2 | cut -d' ' -f2"
	output, err := c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
	if err != nil {
		c.logger.Warn("Failed to get network interfaces")
		return &models.WOLInfo{Supported: false}, nil
	}

	interfaces := strings.Fields(strings.TrimSpace(output))
	wolInfo := &models.WOLInfo{
		Interfaces: []string{},
	}

	// Check each interface for WoL support using ethtool
	for _, iface := range interfaces {
		// Skip virtual interfaces like docker, veth, etc.
		if strings.HasPrefix(iface, "docker") || strings.HasPrefix(iface, "veth") || 
		   strings.HasPrefix(iface, "br-") || strings.HasPrefix(iface, "virbr") {
			continue
		}
		
		// Check if ethtool can query this interface and if it supports WoL
		cmd := fmt.Sprintf("sudo ethtool %s 2>/dev/null | grep 'Supports Wake-on:'", iface)
		output, err := c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			continue // Interface doesn't support ethtool or WoL
		}
		
		// Check if the interface supports magic packet wake-on-lan
		if strings.Contains(output, "g") {
			wolInfo.Supported = true
			wolInfo.Interfaces = append(wolInfo.Interfaces, iface)
			c.logger.WithField("interface", iface).Debug("Found WoL-capable interface")
			
			// Check if WOL is currently armed
			cmd = fmt.Sprintf("sudo ethtool %s 2>/dev/null | grep 'Wake-on:'", iface)
			output, err = c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
			if err == nil && strings.Contains(output, "Wake-on: g") {
				wolInfo.Armed = true
				c.logger.WithField("interface", iface).Debug("WoL is armed on interface")
			}
		}
	}

	return wolInfo, nil
}

// ArmWakeOnLAN arms Wake-on-LAN on supported interfaces
func (c *Commander) ArmWakeOnLAN(host string, port int, user string, keyPath string, systemType models.SystemType) error {
	c.logger.Debug("Arming Wake-on-LAN")

	if err := c.validateSystemType(systemType, models.SystemTypeLinux, models.SystemTypeProxmox); err != nil {
		return err
	}

	wolInfo, err := c.CheckWakeOnLAN(host, port, user, keyPath, systemType)
	if err != nil {
		return err
	}

	if !wolInfo.Supported {
		return &CommandError{
			Type:    "UnsupportedError",
			Message: "Wake-on-LAN is not supported on this system",
		}
	}

	armedCount := 0
	failedInterfaces := []string{}

	for _, iface := range wolInfo.Interfaces {
		cmd := fmt.Sprintf("sudo ethtool -s %s wol g", iface)
		_, err := c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err != nil {
			c.logger.WithFields(logrus.Fields{
				"interface": iface,
				"error":     err.Error(),
			}).Warn("Failed to arm Wake-on-LAN")
			failedInterfaces = append(failedInterfaces, iface)
			continue
		}
		c.logger.WithField("interface", iface).Info("Wake-on-LAN armed")
		armedCount++
	}

	if armedCount == 0 {
		return &CommandError{
			Type:    "ArmError",
			Message: fmt.Sprintf("Failed to arm Wake-on-LAN on any interfaces. Failed interfaces: %s", strings.Join(failedInterfaces, ", ")),
		}
	}

	if len(failedInterfaces) > 0 {
		c.logger.WithFields(logrus.Fields{
			"armed":  armedCount,
			"failed": len(failedInterfaces),
		}).Warn("Wake-on-LAN armed partially")
	} else {
		c.logger.WithField("count", armedCount).Info("Wake-on-LAN armed on all interfaces")
	}

	return nil
}

// CreateProxmoxAPIKey creates a Proxmox API key and returns it
func (c *Commander) CreateProxmoxAPIKey(host string, port int, user string, keyPath string) (*models.ProxmoxAPIKey, error) {
	c.logger.Debug("Creating Proxmox API key")

	// Generate a unique token ID
	tokenID := fmt.Sprintf("homelab-%d", time.Now().Unix())
	
	// Create API token
	cmd := fmt.Sprintf("sudo pveum user token add %s@pam %s --privsep 0", user, tokenID)
	output, err := c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
	if err != nil {
		return nil, c.handleSSHError(err, cmd, output)
	}

	// Parse the output to extract the secret
	lines := strings.Split(output, "\n")
	var secret string
	for _, line := range lines {
		if strings.Contains(line, "value") {
			parts := strings.SplitN(line, "│", 3)
			if len(parts) >= 3 {
				secret = strings.TrimSpace(parts[2])
				break
			}
		}
	}

	if secret == "" {
		return nil, &CommandError{
			Type:    "ParseError",
			Message: "Failed to extract API token secret",
			Command: cmd,
			Output:  output,
		}
	}

	return &models.ProxmoxAPIKey{
		Username: user,
		Realm:    "pam",
		TokenID:  tokenID,
		Secret:   secret,
	}, nil
}

// CheckSuspendSupport checks if suspend is supported
func (c *Commander) CheckSuspendSupport(host string, port int, user string, keyPath string, systemType models.SystemType) (bool, error) {
	c.logger.Debug("Checking suspend support")

	if err := c.validateSystemType(systemType, models.SystemTypeLinux, models.SystemTypeProxmox); err != nil {
		return false, err
	}

	cmd := "sudo systemctl status sleep.target >/dev/null 2>&1 && echo supported"
	output, err := c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
	if err != nil {
		return false, nil
	}

	return strings.Contains(output, "supported"), nil
}

// Suspend suspends the system
func (c *Commander) Suspend(host string, port int, user string, keyPath string, systemType models.SystemType) error {
	c.logger.Info("Suspending system")

	if err := c.validateSystemType(systemType, models.SystemTypeLinux, models.SystemTypeProxmox); err != nil {
		return err
	}

	supported, err := c.CheckSuspendSupport(host, port, user, keyPath, systemType)
	if err != nil {
		return err
	}

	if !supported {
		return &CommandError{
			Type:    "UnsupportedError",
			Message: "Suspend is not supported on this system",
		}
	}

	cmd := "sudo systemctl suspend"
	return c.executor.ExecuteCommand(host, port, user, keyPath, cmd)
}

// CheckHibernateSupport checks if hibernate is supported
func (c *Commander) CheckHibernateSupport(host string, port int, user string, keyPath string, systemType models.SystemType) (bool, error) {
	c.logger.Debug("Checking hibernate support")

	if err := c.validateSystemType(systemType, models.SystemTypeLinux, models.SystemTypeProxmox); err != nil {
		return false, err
	}

	cmd := "sudo test -f /sys/power/disk && grep -q '\\[platform\\]' /sys/power/disk && echo supported"
	output, err := c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
	if err != nil {
		return false, nil
	}

	return strings.Contains(output, "supported"), nil
}

// Hibernate hibernates the system
func (c *Commander) Hibernate(host string, port int, user string, keyPath string, systemType models.SystemType) error {
	c.logger.Info("Hibernating system")

	if err := c.validateSystemType(systemType, models.SystemTypeLinux, models.SystemTypeProxmox); err != nil {
		return err
	}

	supported, err := c.CheckHibernateSupport(host, port, user, keyPath, systemType)
	if err != nil {
		return err
	}

	if !supported {
		return &CommandError{
			Type:    "UnsupportedError",
			Message: "Hibernate is not supported on this system",
		}
	}

	cmd := "sudo systemctl hibernate"
	return c.executor.ExecuteCommand(host, port, user, keyPath, cmd)
}

// Shutdown shuts down the system
func (c *Commander) Shutdown(host string, port int, user string, keyPath string, systemType models.SystemType) error {
	c.logger.Info("Shutting down system")

	var cmd string
	switch systemType {
	case models.SystemTypeLinux, models.SystemTypeProxmox:
		cmd = "sudo shutdown -h now"
	case models.SystemTypeWindows:
		cmd = "shutdown /s /t 0"
	default:
		return &CommandError{
			Type:    "UnsupportedError",
			Message: fmt.Sprintf("Shutdown not supported for system type: %s", systemType),
		}
	}

	return c.executor.ExecuteCommand(host, port, user, keyPath, cmd)
}

// Restart restarts the system
func (c *Commander) Restart(host string, port int, user string, keyPath string, systemType models.SystemType) error {
	c.logger.Info("Restarting system")

	var cmd string
	switch systemType {
	case models.SystemTypeLinux, models.SystemTypeProxmox:
		cmd = "sudo shutdown -r now"
	case models.SystemTypeWindows:
		cmd = "shutdown /r /t 0"
	default:
		return &CommandError{
			Type:    "UnsupportedError",
			Message: fmt.Sprintf("Restart not supported for system type: %s", systemType),
		}
	}

	return c.executor.ExecuteCommand(host, port, user, keyPath, cmd)
}

// GetSystemInfo gets comprehensive system information
func (c *Commander) GetSystemInfo(host string, port int, user string, keyPath string, systemType models.SystemType) (*models.SystemInfo, error) {
	c.logger.Info("Getting comprehensive system information")

	info := &models.SystemInfo{
		Type: systemType,
		LastUpdated: time.Now(),
	}

	// Get CPU usage
	if cpu, err := c.GetCPUUsage(host, port, user, keyPath, systemType); err == nil {
		info.CPUUsage = cpu
	} else {
		c.logger.WithError(err).Warn("Failed to get CPU usage")
	}

	// Get load average
	if load, err := c.GetLoadAverage(host, port, user, keyPath, systemType); err == nil {
		info.LoadAverage = load
	} else {
		c.logger.WithError(err).Warn("Failed to get load average")
	}

	// Get memory usage
	if mem, err := c.GetMemoryUsage(host, port, user, keyPath, systemType); err == nil {
		info.MemoryUsage = *mem
	} else {
		c.logger.WithError(err).Warn("Failed to get memory usage")
	}

	// Get network usage
	if net, err := c.GetNetworkUsage(host, port, user, keyPath, systemType); err == nil {
		info.NetworkUsage = *net
	} else {
		c.logger.WithError(err).Warn("Failed to get network usage")
	}

	// Get network interfaces
	if interfaces, err := c.GetNetworkInterfaces(host, port, user, keyPath, systemType); err == nil {
		info.IPAddresses = interfaces
	} else {
		c.logger.WithError(err).Warn("Failed to get network interfaces")
	}

	// Get system ID
	if id, err := c.GetSystemID(host, port, user, keyPath, systemType); err == nil {
		info.SystemID = id
	} else {
		c.logger.WithError(err).Warn("Failed to get system ID")
	}

	// Get OS version
	if version, err := c.GetOSVersion(host, port, user, keyPath, systemType); err == nil {
		info.OSVersion = version
	} else {
		c.logger.WithError(err).Warn("Failed to get OS version")
	}

	// Get disk usage
	if disk, err := c.GetDiskUsage(host, port, user, keyPath, systemType); err == nil {
		info.DiskUsage = *disk
	} else {
		c.logger.WithError(err).Warn("Failed to get disk usage")
	}

	// Check Wake-on-LAN
	if wol, err := c.CheckWakeOnLAN(host, port, user, keyPath, systemType); err == nil {
		info.WakeOnLAN = *wol
	} else {
		c.logger.WithError(err).Warn("Failed to check Wake-on-LAN")
	}

	// Check suspend support
	if suspend, err := c.CheckSuspendSupport(host, port, user, keyPath, systemType); err == nil {
		info.SuspendSupport = suspend
	} else {
		c.logger.WithError(err).Warn("Failed to check suspend support")
	}

	// Check hibernate support
	if hibernate, err := c.CheckHibernateSupport(host, port, user, keyPath, systemType); err == nil {
		info.HibernateSupport = hibernate
	} else {
		c.logger.WithError(err).Warn("Failed to check hibernate support")
	}

	return info, nil
}

// VerifyWakeOnLAN provides detailed information about WoL configuration
func (c *Commander) VerifyWakeOnLAN(host string, port int, user string, keyPath string, systemType models.SystemType) (*models.WOLInfo, error) {
	c.logger.Info("Verifying Wake-on-LAN configuration")

	if err := c.validateSystemType(systemType, models.SystemTypeLinux, models.SystemTypeProxmox); err != nil {
		return nil, err
	}

	// Get comprehensive WoL info
	wolInfo, err := c.CheckWakeOnLAN(host, port, user, keyPath, systemType)
	if err != nil {
		return nil, err
	}

	// Log detailed information for debugging
	c.logger.WithFields(logrus.Fields{
		"supported":       wolInfo.Supported,
		"armed":           wolInfo.Armed,
		"interfaces":      wolInfo.Interfaces,
		"interface_count": len(wolInfo.Interfaces),
	}).Info("Wake-on-LAN status summary")

	// Check detailed status for each interface
	for _, iface := range wolInfo.Interfaces {
		cmd := fmt.Sprintf("sudo ethtool %s 2>/dev/null | grep -A5 -B5 'Wake-on'", iface)
		output, err := c.executor.ExecuteCommandWithOutput(host, port, user, keyPath, cmd)
		if err == nil {
			c.logger.WithFields(logrus.Fields{
				"interface": iface,
				"details":   strings.TrimSpace(output),
			}).Debug("Interface WoL details")
		}
	}

	return wolInfo, nil
}

// Helper functions

func (c *Commander) validateSystemType(actual models.SystemType, expected ...models.SystemType) error {
	for _, exp := range expected {
		if actual == exp {
			return nil
		}
	}
	
	return &CommandError{
		Type:    "SystemTypeMismatch",
		Message: fmt.Sprintf("Operation requires system type %v, but got %s", expected, actual),
	}
}

func (c *Commander) handleSSHError(err error, cmd string, output string) error {
	errStr := err.Error()
	
	// Check for specific error types
	if strings.Contains(errStr, "permission denied") || strings.Contains(errStr, "publickey") {
		return &CommandError{
			Type:    "AuthenticationError",
			Message: "SSH authentication failed. Please check credentials and key permissions",
			Command: cmd,
			Output:  output,
			Err:     err,
		}
	}
	
	if strings.Contains(output, "sudo: a password is required") {
		return &CommandError{
			Type:    "SudoPasswordRequired",
			Message: "Sudo requires a password. Please configure passwordless sudo for the user",
			Command: cmd,
			Output:  output,
			Err:     err,
		}
	}
	
	if strings.Contains(errStr, "connection refused") {
		return &CommandError{
			Type:    "ConnectionError",
			Message: "Connection refused. Please check if SSH service is running and port is correct",
			Command: cmd,
			Output:  output,
			Err:     err,
		}
	}
	
	if strings.Contains(errStr, "no such host") || strings.Contains(errStr, "cannot resolve") {
		return &CommandError{
			Type:    "DNSError",
			Message: "Cannot resolve hostname. Please check the hostname",
			Command: cmd,
			Output:  output,
			Err:     err,
		}
	}
	
	if strings.Contains(errStr, "timeout") {
		return &CommandError{
			Type:    "TimeoutError",
			Message: "Connection timeout. Host may be unreachable or not responding",
			Command: cmd,
			Output:  output,
			Err:     err,
		}
	}
	
	// Generic error
	return &CommandError{
		Type:    "CommandExecutionError",
		Message: fmt.Sprintf("Failed to execute command: %s", err.Error()),
		Command: cmd,
		Output:  output,
		Err:     err,
	}
}

func (c *Commander) parseNetworkInterfacesJSON(jsonOutput string) ([]models.NetworkInterface, error) {
	var interfaces []models.NetworkInterface
	var jsonData []map[string]interface{}
	
	if err := json.Unmarshal([]byte(jsonOutput), &jsonData); err != nil {
		return nil, &CommandError{
			Type:    "ParseError",
			Message: "Failed to parse network interfaces JSON",
			Output:  jsonOutput,
			Err:     err,
		}
	}
	
	for _, iface := range jsonData {
		ifname, _ := iface["ifname"].(string)
		if ifname == "lo" {
			continue // Skip loopback
		}
		
		// Get MAC address
		var macAddr string
		if address, ok := iface["address"].(string); ok {
			macAddr = address
		}
		
		// Get IP addresses
		if addrInfo, ok := iface["addr_info"].([]interface{}); ok {
			for _, addr := range addrInfo {
				if addrMap, ok := addr.(map[string]interface{}); ok {
					if ip, ok := addrMap["local"].(string); ok {
						family, _ := addrMap["family"].(string)
						interfaces = append(interfaces, models.NetworkInterface{
							Name:       ifname,
							IPAddress:  ip,
							MACAddress: macAddr,
							IsIPv6:     family == "inet6",
						})
					}
				}
			}
		}
	}
	
	return interfaces, nil
}

func (c *Commander) parseNetworkInterfacesText(output string) []models.NetworkInterface {
	var interfaces []models.NetworkInterface
	lines := strings.Split(output, "\n")
	
	var currentMAC string
	var currentIface string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Extract interface name and MAC address
		if strings.Contains(line, "link/ether") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				currentMAC = parts[1]
			}
		}
		
		// Extract IPv4 addresses
		if strings.HasPrefix(line, "inet ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				ipParts := strings.Split(parts[1], "/")
				if len(ipParts) > 0 {
					// Try to extract interface name
					for i, part := range parts {
						if part == "dev" && i+1 < len(parts) {
							currentIface = parts[i+1]
							break
						}
					}
					
					interfaces = append(interfaces, models.NetworkInterface{
						Name:       currentIface,
						IPAddress:  ipParts[0],
						MACAddress: currentMAC,
						IsIPv6:     false,
					})
				}
			}
		}
		
		// Extract IPv6 addresses
		if strings.HasPrefix(line, "inet6 ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				ipParts := strings.Split(parts[1], "/")
				if len(ipParts) > 0 && !strings.HasPrefix(ipParts[0], "fe80") { // Skip link-local
					interfaces = append(interfaces, models.NetworkInterface{
						Name:       currentIface,
						IPAddress:  ipParts[0],
						MACAddress: currentMAC,
						IsIPv6:     true,
					})
				}
			}
		}
	}
	
	return interfaces
}

func (c *Commander) parseWindowsNetworkInterfaces(jsonOutput string) ([]models.NetworkInterface, error) {
	var interfaces []models.NetworkInterface
	
	// Handle both single object and array
	jsonOutput = strings.TrimSpace(jsonOutput)
	if !strings.HasPrefix(jsonOutput, "[") {
		jsonOutput = "[" + jsonOutput + "]"
	}
	
	var winInterfaces []struct {
		Name   string
		MAC    string
		IP     string
		Family int
	}
	
	if err := json.Unmarshal([]byte(jsonOutput), &winInterfaces); err != nil {
		return nil, &CommandError{
			Type:    "ParseError",
			Message: "Failed to parse Windows network interfaces",
			Output:  jsonOutput,
			Err:     err,
		}
	}
	
	for _, iface := range winInterfaces {
		// Skip loopback addresses
		if strings.HasPrefix(iface.IP, "127.") || iface.IP == "::1" {
			continue
		}
		
		// Format MAC address properly (add colons if needed)
		mac := iface.MAC
		if len(mac) == 12 && !strings.Contains(mac, ":") && !strings.Contains(mac, "-") {
			// Convert from format like "001122334455" to "00:11:22:33:44:55"
			formatted := ""
			for i := 0; i < len(mac); i += 2 {
				if i > 0 {
					formatted += ":"
				}
				formatted += mac[i:i+2]
			}
			mac = formatted
		} else if strings.Contains(mac, "-") {
			// Convert Windows format "00-11-22-33-44-55" to standard format
			mac = strings.ReplaceAll(mac, "-", ":")
		}
		
		interfaces = append(interfaces, models.NetworkInterface{
			Name:       iface.Name,
			IPAddress:  iface.IP,
			MACAddress: mac,
			IsIPv6:     iface.Family == 23, // AddressFamily.InterNetworkV6 = 23
		})
	}
	
	return interfaces, nil
}

func (c *Commander) parseWindowsNetworkInterfacesWMI(jsonOutput string) ([]models.NetworkInterface, error) {
	var interfaces []models.NetworkInterface
	
	// Handle both single object and array
	jsonOutput = strings.TrimSpace(jsonOutput)
	if !strings.HasPrefix(jsonOutput, "[") {
		jsonOutput = "[" + jsonOutput + "]"
	}
	
	var wmiInterfaces []struct {
		Description string
		MACAddress  string
		IPAddress   []string
	}
	
	if err := json.Unmarshal([]byte(jsonOutput), &wmiInterfaces); err != nil {
		return nil, &CommandError{
			Type:    "ParseError",
			Message: "Failed to parse Windows WMI network interfaces",
			Output:  jsonOutput,
			Err:     err,
		}
	}
	
	for _, iface := range wmiInterfaces {
		// Format MAC address
		mac := iface.MACAddress
		if strings.Contains(mac, "-") {
			mac = strings.ReplaceAll(mac, "-", ":")
		}
		
		for _, ip := range iface.IPAddress {
			// Skip empty IPs and loopback
			if ip == "" || strings.HasPrefix(ip, "127.") || ip == "::1" {
				continue
			}
			
			isIPv6 := strings.Contains(ip, ":")
			
			interfaces = append(interfaces, models.NetworkInterface{
				Name:       iface.Description,
				IPAddress:  ip,
				MACAddress: mac,
				IsIPv6:     isIPv6,
			})
		}
	}
	
	return interfaces, nil
}