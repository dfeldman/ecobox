package control

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"ecobox-server/internal/command"
	"ecobox-server/internal/models"
	"ecobox-server/internal/storage"
	"github.com/sirupsen/logrus"
)

// SystemMonitor handles SSH-based system information gathering and monitoring
type SystemMonitor struct {
	storage   storage.Storage
	commander *command.Commander
	sshClient *SSHClient
	logger    *logrus.Logger
	mu        sync.RWMutex
}

// NewSystemMonitor creates a new SystemMonitor instance
func NewSystemMonitor(storage storage.Storage, logger *logrus.Logger) *SystemMonitor {
	sshClient := NewSSHClient()
	commander := command.NewCommander(sshClient, logger)
	
	return &SystemMonitor{
		storage:   storage,
		commander: commander,
		sshClient: sshClient,
		logger:    logger,
	}
}

// PerformInitializationCheck performs comprehensive initialization check on a server
// This includes detecting OS, system capabilities, network interfaces, and WoL setup
func (sm *SystemMonitor) PerformInitializationCheck(server *models.Server) error {
	if server == nil {
		return fmt.Errorf("server is nil")
	}

	sm.logger.WithFields(logrus.Fields{
		"server": server.Name,
		"host":   server.Hostname,
	}).Info("Starting initialization check")

	// Skip if server is known to be offline or in failed init state
	if server.CurrentState == models.PowerStateOff || server.CurrentState == models.PowerStateInitFailed {
		sm.logger.WithField("server", server.Name).Debug("Skipping initialization check - server is offline or init failed")
		return fmt.Errorf("server is offline or init failed")
	}

	action := models.ServerAction{
		Timestamp:   time.Now(),
		Action:      models.ActionTypeInitialize,
		Success:     false,
		InitiatedBy: "system_monitor",
	}

	var errors []string
	success := true

	// Initialize SystemInfo if it doesn't exist
	if server.SystemInfo == nil {
		server.SystemInfo = &models.SystemInfo{}
	}

	// 1. Test SSH connection first
	if err := sm.commander.TestConnection(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath); err != nil {
		errorMsg := fmt.Sprintf("SSH connection test failed: %v", err)
		errors = append(errors, errorMsg)
		success = false
		sm.logger.WithField("server", server.Name).Warn(errorMsg)
	} else {
		sm.logger.WithField("server", server.Name).Debug("SSH connection test successful")

		// 2. Detect system type
		if systemType, err := sm.commander.DetectSystemType(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath); err != nil {
			errorMsg := fmt.Sprintf("System type detection failed: %v", err)
			errors = append(errors, errorMsg)
			success = false
			sm.logger.WithField("server", server.Name).Warn(errorMsg)
		} else {
			server.SystemInfo.Type = systemType
			server.SystemInfo.Hostname = server.Hostname
			sm.logger.WithFields(logrus.Fields{
				"server": server.Name,
				"type":   systemType,
			}).Info("Detected system type")

			// 3. Get OS version
			if osVersion, err := sm.commander.GetOSVersion(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath, systemType); err != nil {
				errorMsg := fmt.Sprintf("OS version detection failed: %v", err)
				errors = append(errors, errorMsg)
				sm.logger.WithField("server", server.Name).Warn(errorMsg)
			} else {
				server.SystemInfo.OSVersion = osVersion
				sm.logger.WithFields(logrus.Fields{
					"server":  server.Name,
					"version": osVersion,
				}).Info("Detected OS version")
			}

			// 4. Get system ID
			if systemID, err := sm.commander.GetSystemID(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath, systemType); err != nil {
				errorMsg := fmt.Sprintf("System ID detection failed: %v", err)
				errors = append(errors, errorMsg)
				sm.logger.WithField("server", server.Name).Warn(errorMsg)
			} else {
				server.SystemInfo.SystemID = systemID
				sm.logger.WithFields(logrus.Fields{
					"server":    server.Name,
					"system_id": systemID,
				}).Debug("Retrieved system ID")
			}

			// 5. Get network interfaces
			if interfaces, err := sm.commander.GetNetworkInterfaces(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath, systemType); err != nil {
				errorMsg := fmt.Sprintf("Network interface detection failed: %v", err)
				errors = append(errors, errorMsg)
				sm.logger.WithField("server", server.Name).Warn(errorMsg)
			} else {
				server.SystemInfo.IPAddresses = interfaces
				
				// Extract primary MAC address for Wake-on-LAN
				sm.extractPrimaryMACAddress(server, interfaces)
				
				sm.logger.WithFields(logrus.Fields{
					"server":     server.Name,
					"interfaces": len(interfaces),
					"mac":        server.MACAddress,
				}).Info("Retrieved network interfaces and MAC address")
			}

			// 6. Check system capabilities (power management, WoL, etc.)
			sm.checkSystemCapabilities(server, systemType)
		}
	}

	// Update timestamps and finalize
	server.SystemInfo.LastUpdated = time.Now()
	server.Initialized = success

	// Record the action
	action.Success = success
	if !success {
		action.ErrorMsg = fmt.Sprintf("Initialization errors: %v", errors)
	}

	// Store the action and update server
	if err := sm.storage.AddServerAction(server.ID, action); err != nil {
		sm.logger.WithField("server", server.Name).Error("Failed to record initialization action")
	}

	if err := sm.storage.UpdateServer(server); err != nil {
		return fmt.Errorf("failed to update server after initialization: %w", err)
	}

	if success {
		sm.logger.WithField("server", server.Name).Info("Initialization check completed successfully")
	} else {
		sm.logger.WithField("server", server.Name).Warn("Initialization check completed with errors")
	}

	return nil
}

// PerformSystemCheck performs periodic system monitoring (CPU, memory, network usage, etc.)
func (sm *SystemMonitor) PerformSystemCheck(server *models.Server) (*SystemMetrics, error) {
	if server == nil {
		return nil, fmt.Errorf("server is nil")
	}

	// Skip if server is offline or not initialized
	if server.CurrentState != models.PowerStateOn || !server.Initialized {
		sm.logger.WithField("server", server.Name).Debug("Skipping system check - server offline or not initialized")
		return nil, nil
	}

	if server.SystemInfo == nil {
		return nil, fmt.Errorf("server system info is nil - needs initialization")
	}

	sm.logger.WithField("server", server.Name).Debug("Starting system check")

	systemType := server.SystemInfo.Type
	var errors []string
	updated := false
	
	// Initialize metrics struct to capture what was collected
	metrics := &SystemMetrics{}
	
	sm.logger.WithFields(logrus.Fields{
		"server":     server.Name,
		"systemType": systemType,
		"hostname":   server.Hostname,
	}).Debug("System check details")

	// 1. Get CPU usage
	sm.logger.WithField("server", server.Name).Debug("Attempting to get CPU usage")
	if cpuUsage, err := sm.commander.GetCPUUsage(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath, systemType); err != nil {
		errors = append(errors, fmt.Sprintf("CPU usage: %v", err))
		sm.logger.WithFields(logrus.Fields{
			"server": server.Name,
			"error":  err,
		}).Warn("Failed to get CPU usage")
	} else {
		server.SystemInfo.CPUUsage = cpuUsage
		metrics.CPUUsage = &cpuUsage
		updated = true
		sm.logger.WithFields(logrus.Fields{
			"server": server.Name,
			"cpu":    fmt.Sprintf("%.1f%%", cpuUsage),
		}).Info("Updated CPU usage")
	}

	// 2. Get load average
	if loadAvg, err := sm.commander.GetLoadAverage(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath, systemType); err != nil {
		errors = append(errors, fmt.Sprintf("Load average: %v", err))
		sm.logger.WithField("server", server.Name).Debug("Failed to get load average")
	} else {
		server.SystemInfo.LoadAverage = loadAvg
		if len(loadAvg) > 0 {
			metrics.LoadAverage1m = &loadAvg[0]
		}
		updated = true
		sm.logger.WithFields(logrus.Fields{
			"server": server.Name,
			"load":   fmt.Sprintf("%.2f %.2f %.2f", loadAvg[0], loadAvg[1], loadAvg[2]),
		}).Debug("Updated load average")
	}

	// 3. Get memory usage
	sm.logger.WithField("server", server.Name).Debug("Attempting to get memory usage")
	if memUsage, err := sm.commander.GetMemoryUsage(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath, systemType); err != nil {
		errors = append(errors, fmt.Sprintf("Memory usage: %v", err))
		sm.logger.WithFields(logrus.Fields{
			"server": server.Name,
			"error":  err,
		}).Warn("Failed to get memory usage")
	} else {
		server.SystemInfo.MemoryUsage = *memUsage
		metrics.MemoryPercent = &memUsage.UsedPercent
		updated = true
		sm.logger.WithFields(logrus.Fields{
			"server": server.Name,
			"memory": fmt.Sprintf("%.1f%%", memUsage.UsedPercent),
		}).Info("Updated memory usage")
	}

	// 4. Get network usage
	if netUsage, err := sm.commander.GetNetworkUsage(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath, systemType); err != nil {
		errors = append(errors, fmt.Sprintf("Network usage: %v", err))
		sm.logger.WithField("server", server.Name).Debug("Failed to get network usage")
	} else {
		server.SystemInfo.NetworkUsage = *netUsage
		metrics.NetworkRxMbps = &netUsage.MBpsRecv
		metrics.NetworkTxMbps = &netUsage.MBpsSent
		updated = true
		sm.logger.WithFields(logrus.Fields{
			"server": server.Name,
			"net_rx": fmt.Sprintf("%.2f MB/s", netUsage.MBpsRecv),
			"net_tx": fmt.Sprintf("%.2f MB/s", netUsage.MBpsSent),
		}).Debug("Updated network usage")
	}

	// 5. Get disk usage
	if diskUsage, err := sm.commander.GetDiskUsage(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath, systemType); err != nil {
		errors = append(errors, fmt.Sprintf("Disk usage: %v", err))
		sm.logger.WithField("server", server.Name).Debug("Failed to get disk usage")
	} else {
		server.SystemInfo.DiskUsage = *diskUsage
		metrics.DiskPercent = &diskUsage.UsedPercent
		updated = true
		sm.logger.WithFields(logrus.Fields{
			"server": server.Name,
			"disk":   fmt.Sprintf("%.1f%%", diskUsage.UsedPercent),
		}).Debug("Updated disk usage")
	}

	// Update last updated time if any data was collected
	if updated {
		server.SystemInfo.LastUpdated = time.Now()
		
		// Save updated server info
		if err := sm.storage.UpdateServer(server); err != nil {
			return metrics, fmt.Errorf("failed to update server after system check: %w", err)
		}

		sm.logger.WithField("server", server.Name).Debug("System check completed successfully")
	} else {
		sm.logger.WithFields(logrus.Fields{
			"server": server.Name,
			"errors": errors,
		}).Warn("System check completed with all operations failed")
	}

	// TODO: Future integration points:
	// - Evaluate suspend decision based on CPU/network activity
	
	return metrics, nil
}

// SystemMetrics represents system monitoring metrics that were successfully collected
type SystemMetrics struct {
	CPUUsage       *float64 `json:"cpu_usage,omitempty"`
	MemoryPercent  *float64 `json:"memory_percent,omitempty"`
	NetworkRxMbps  *float64 `json:"network_rx_mbps,omitempty"`
	NetworkTxMbps  *float64 `json:"network_tx_mbps,omitempty"`
	DiskPercent    *float64 `json:"disk_percent,omitempty"`
	LoadAverage1m  *float64 `json:"load_average_1m,omitempty"`
	// PowerWatts will be added later when power meter API integration is implemented
}

// checkSystemCapabilities determines what power management features are supported
func (sm *SystemMonitor) checkSystemCapabilities(server *models.Server, systemType models.SystemType) {
	// First set reasonable defaults based on system type
	switch systemType {
	case models.SystemTypeLinux, models.SystemTypeProxmox:
		// Linux systems typically support most power management features
		server.SystemInfo.SuspendSupport = true
		server.SystemInfo.HibernateSupport = true
		server.SystemInfo.WakeOnLANSupport = true
		server.SystemInfo.PowerEstimateSupport = true
		// Physical power control requires additional hardware
		server.SystemInfo.PowerSwitchSupport = false
		server.SystemInfo.PowerMeterSupport = false
		
	case models.SystemTypeWindows:
		// Windows systems support suspend and hibernate
		server.SystemInfo.SuspendSupport = true
		server.SystemInfo.HibernateSupport = true
		server.SystemInfo.WakeOnLANSupport = true
		server.SystemInfo.PowerEstimateSupport = true
		server.SystemInfo.PowerSwitchSupport = false
		server.SystemInfo.PowerMeterSupport = false
		
	default:
		// Unknown systems - assume minimal capabilities
		server.SystemInfo.SuspendSupport = false
		server.SystemInfo.HibernateSupport = false
		server.SystemInfo.WakeOnLANSupport = false
		server.SystemInfo.PowerEstimateSupport = false
		server.SystemInfo.PowerSwitchSupport = false
		server.SystemInfo.PowerMeterSupport = false
	}

	// Now actually check and configure Wake-on-LAN for supported systems
	if systemType == models.SystemTypeLinux || systemType == models.SystemTypeProxmox {
		// Check actual WoL support on the system
		if wolInfo, err := sm.commander.CheckWakeOnLAN(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath, systemType); err != nil {
			sm.logger.WithFields(logrus.Fields{
				"server": server.Name,
				"error":  err.Error(),
			}).Warn("Failed to check Wake-on-LAN support")
			server.SystemInfo.WakeOnLANSupport = false
		} else {
			server.SystemInfo.WakeOnLANSupport = wolInfo.Supported
			server.SystemInfo.WakeOnLAN = *wolInfo
			
			sm.logger.WithFields(logrus.Fields{
				"server":     server.Name,
				"supported":  wolInfo.Supported,
				"interfaces": wolInfo.Interfaces,
				"armed":      wolInfo.Armed,
			}).Info("Checked Wake-on-LAN support")
			
			// If WoL is supported but not armed, try to arm it
			if wolInfo.Supported && !wolInfo.Armed {
				sm.logger.WithField("server", server.Name).Info("Arming Wake-on-LAN on all supported interfaces")
				if err := sm.commander.ArmWakeOnLAN(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath, systemType); err != nil {
					sm.logger.WithFields(logrus.Fields{
						"server": server.Name,
						"error":  err.Error(),
					}).Warn("Failed to arm Wake-on-LAN")
				} else {
					// Check again to verify it's armed
					if wolInfo, err := sm.commander.CheckWakeOnLAN(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath, systemType); err == nil {
						server.SystemInfo.WakeOnLAN = *wolInfo
						sm.logger.WithFields(logrus.Fields{
							"server": server.Name,
							"armed":  wolInfo.Armed,
						}).Info("Wake-on-LAN arming completed")
					}
				}
			}
		}
	}
	
	sm.logger.WithFields(logrus.Fields{
		"server":      server.Name,
		"suspend":     server.SystemInfo.SuspendSupport,
		"hibernate":   server.SystemInfo.HibernateSupport,
		"wol":         server.SystemInfo.WakeOnLANSupport,
		"wol_armed":   server.SystemInfo.WakeOnLAN.Armed,
		"power_est":   server.SystemInfo.PowerEstimateSupport,
	}).Info("System capabilities configured")
}

// IsServerReachable checks if a server is reachable via SSH without performing full checks
func (sm *SystemMonitor) IsServerReachable(server *models.Server) bool {
	err := sm.commander.TestConnection(server.Hostname, server.SSHPort, server.SSHUser, server.SSHKeyPath)
	return err == nil
}

// extractPrimaryMACAddress sets the server's primary MAC address from network interfaces
func (sm *SystemMonitor) extractPrimaryMACAddress(server *models.Server, interfaces []models.NetworkInterface) {
	// If MAC address is already set, don't override it
	if server.MACAddress != "" {
		return
	}
	
	// Prioritize physical ethernet interfaces over wireless/virtual ones
	priorities := []string{"eth0", "eno1", "enp", "ens"}
	
	// First, try to find a preferred physical interface
	for _, priority := range priorities {
		for _, iface := range interfaces {
			if strings.HasPrefix(iface.Name, priority) && iface.MACAddress != "" && !iface.IsIPv6 {
				server.MACAddress = iface.MACAddress
				sm.logger.WithFields(logrus.Fields{
					"server":    server.Name,
					"interface": iface.Name,
					"mac":       iface.MACAddress,
				}).Info("Selected primary MAC address from preferred interface")
				return
			}
		}
	}
	
	// If no preferred interface found, use the first non-loopback interface with a MAC address
	for _, iface := range interfaces {
		if iface.MACAddress != "" && iface.Name != "lo" && !iface.IsIPv6 {
			// Skip virtual/bridge interfaces
			if strings.HasPrefix(iface.Name, "docker") || 
			   strings.HasPrefix(iface.Name, "veth") || 
			   strings.HasPrefix(iface.Name, "br-") || 
			   strings.HasPrefix(iface.Name, "virbr") {
				continue
			}
			
			server.MACAddress = iface.MACAddress
			sm.logger.WithFields(logrus.Fields{
				"server":    server.Name,
				"interface": iface.Name,
				"mac":       iface.MACAddress,
			}).Info("Selected primary MAC address from first available interface")
			return
		}
	}
	
	sm.logger.WithField("server", server.Name).Warn("No suitable MAC address found for Wake-on-LAN")
}
