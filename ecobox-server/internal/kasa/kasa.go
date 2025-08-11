package kasa

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// SmartPlug represents a TP-Link smart plug device
type SmartPlug struct {
	IPAddress             string `json:"ip_address"`
	Nickname              string `json:"nickname"`
	MACAddress            string `json:"mac_address"`
	DeviceType            string `json:"device_type"`
	Model                 string `json:"model"`
	PowerProtectionStatus string `json:"power_protection_status"`
	OvercurrentStatus     string `json:"overcurrent_status"`
	ChargingStatus        string `json:"charging_status"`
	HasPowerMonitoring    bool   `json:"has_power_monitoring"`
	IsOnline              bool   `json:"is_online"`
	LastSeen              time.Time `json:"last_seen"`
}

// PowerReading represents current power consumption
type PowerReading struct {
	CurrentPowerMW int     `json:"current_power_mw"` // Power in milliwatts
	CurrentPowerW  float64 `json:"current_power_w"`  // Power in watts
	Timestamp      time.Time `json:"timestamp"`
}

// Manager handles all smart plug operations
type Manager struct {
	plugs   map[string]*SmartPlug // keyed by nickname
	mutex   sync.RWMutex
	kasaCmd string
}

// CommandResult represents the result of an asynchronous operation
type CommandResult struct {
	Success   bool
	Error     error
	Message   string
	Timestamp time.Time
}

// NewManager creates a new Kasa manager instance
func NewManager() (*Manager, error) {
	kasaPath, err := exec.LookPath("kasa")
	if err != nil {
		return nil, fmt.Errorf("kasa command not found in PATH: %w", err)
	}

	return &Manager{
		plugs:   make(map[string]*SmartPlug),
		kasaCmd: kasaPath,
	}, nil
}

// isValidPlugType checks if the device type is a supported smart plug
func isValidPlugType(deviceType string) bool {
	validTypes := []string{"SMART.TAPOPLUG", "SMART.KASAPLUG"}
	for _, validType := range validTypes {
		if deviceType == validType {
			return true
		}
	}
	return false
}

// decodeNickname decodes a base64 encoded nickname
func decodeNickname(encodedNickname string) string {
	decoded, err := base64.StdEncoding.DecodeString(encodedNickname)
	if err != nil {
		// If decoding fails, return the original string
		return encodedNickname
	}
	return string(decoded)
}

// DiscoverDevices discovers all smart plugs on the network
func (m *Manager) DiscoverDevices(ctx context.Context) (*CommandResult, <-chan *CommandResult) {
	resultChan := make(chan *CommandResult, 1)
	
	// Check if context is already cancelled
	if err := ctx.Err(); err != nil {
		result := &CommandResult{
			Success:   false,
			Error:     err,
			Message:   "Context cancelled before starting discovery",
			Timestamp: time.Now(),
		}
		go func() {
			resultChan <- result
			close(resultChan)
		}()
		return result, resultChan
	}

	go func() {
		defer close(resultChan)
		
		cmd := exec.CommandContext(ctx, m.kasaCmd, "--json", "discover")
		output, err := cmd.Output()
		
		result := &CommandResult{
			Timestamp: time.Now(),
		}

		if err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to run kasa discover: %w", err)
			result.Message = "Discovery command failed"
			resultChan <- result
			return
		}

		// Parse JSON output
		var discoveryData map[string]interface{}
		if err := json.Unmarshal(output, &discoveryData); err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to parse discovery JSON: %w", err)
			result.Message = "Failed to parse discovery response"
			resultChan <- result
			return
		}

		// Process discovered devices
		newPlugs := make(map[string]*SmartPlug)
		plugCount := 0

		for ipAddress, deviceData := range discoveryData {
			deviceMap, ok := deviceData.(map[string]interface{})
			if !ok {
				continue
			}

			plug := m.parseDeviceInfo(ipAddress, deviceMap)
			if plug != nil {
				newPlugs[plug.Nickname] = plug
				plugCount++
			}
		}

		// Update the manager's plug list
		m.mutex.Lock()
		m.plugs = newPlugs
		m.mutex.Unlock()

		result.Success = true
		result.Message = fmt.Sprintf("Discovered %d smart plugs", plugCount)
		resultChan <- result
	}()

	// Return immediate result for synchronous callers
	return &CommandResult{
		Success:   true,
		Message:   "Discovery started",
		Timestamp: time.Now(),
	}, resultChan
}

// parseDeviceInfo parses device information from discovery JSON
func (m *Manager) parseDeviceInfo(ipAddress string, deviceData map[string]interface{}) *SmartPlug {
	deviceInfo, ok := deviceData["get_device_info"].(map[string]interface{})
	if !ok {
		return nil
	}

	// Check if it's a valid smart plug type
	deviceType, _ := deviceInfo["type"].(string)
	if !isValidPlugType(deviceType) {
		return nil
	}

	// Extract basic device information
	plug := &SmartPlug{
		IPAddress:  ipAddress,
		DeviceType: deviceType,
		IsOnline:   true,
		LastSeen:   time.Now(),
	}

	// Extract nickname (decode from base64 if needed)
	if nickname, ok := deviceInfo["nickname"].(string); ok {
		plug.Nickname = decodeNickname(nickname)
	}

	// Extract MAC address
	if mac, ok := deviceInfo["mac"].(string); ok {
		plug.MACAddress = mac
	}

	// Extract model
	if model, ok := deviceInfo["model"].(string); ok {
		plug.Model = model
	}

	// Extract status information
	if status, ok := deviceInfo["power_protection_status"].(string); ok {
		plug.PowerProtectionStatus = status
	}
	if status, ok := deviceInfo["overcurrent_status"].(string); ok {
		plug.OvercurrentStatus = status
	}
	if status, ok := deviceInfo["charging_status"].(string); ok {
		plug.ChargingStatus = status
	}

	// Check if device has power monitoring
	if _, hasCurrentPower := deviceData["get_current_power"]; hasCurrentPower {
		plug.HasPowerMonitoring = true
	}

	return plug
}

// GetPlugs returns a copy of all discovered smart plugs
func (m *Manager) GetPlugs() map[string]*SmartPlug {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	plugsCopy := make(map[string]*SmartPlug)
	for nickname, plug := range m.plugs {
		plugCopy := *plug // Create a copy
		plugsCopy[nickname] = &plugCopy
	}
	return plugsCopy
}

// DeviceExists checks if a device with the given nickname exists
func (m *Manager) DeviceExists(nickname string) bool {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	_, exists := m.plugs[nickname]
	return exists
}

// GetDevice returns a copy of the device with the given nickname
func (m *Manager) GetDevice(nickname string) (*SmartPlug, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	plug, exists := m.plugs[nickname]
	if !exists {
		return nil, fmt.Errorf("device with nickname '%s' not found", nickname)
	}
	
	plugCopy := *plug // Create a copy
	return &plugCopy, nil
}

// TurnOn turns on a smart plug by nickname
func (m *Manager) TurnOn(ctx context.Context, nickname string) (*CommandResult, <-chan *CommandResult) {
	return m.executePlugCommand(ctx, nickname, "on")
}

// TurnOff turns off a smart plug by nickname
func (m *Manager) TurnOff(ctx context.Context, nickname string) (*CommandResult, <-chan *CommandResult) {
	return m.executePlugCommand(ctx, nickname, "off")
}

// executePlugCommand executes a command on a specific plug
func (m *Manager) executePlugCommand(ctx context.Context, nickname, command string) (*CommandResult, <-chan *CommandResult) {
	resultChan := make(chan *CommandResult, 1)

	// Check if device exists
	if !m.DeviceExists(nickname) {
		result := &CommandResult{
			Success:   false,
			Error:     fmt.Errorf("device with nickname '%s' not found", nickname),
			Message:   fmt.Sprintf("Device '%s' not found", nickname),
			Timestamp: time.Now(),
		}
		go func() {
			resultChan <- result
			close(resultChan)
		}()
		return result, resultChan
	}

	go func() {
		defer close(resultChan)

		cmd := exec.CommandContext(ctx, m.kasaCmd, "--alias", nickname, command)
		output, err := cmd.Output()

		result := &CommandResult{
			Timestamp: time.Now(),
		}

		if err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to execute command '%s' on device '%s': %w", command, nickname, err)
			result.Message = fmt.Sprintf("Failed to %s device '%s'", command, nickname)
		} else {
			result.Success = true
			result.Message = fmt.Sprintf("Successfully turned %s device '%s'", command, nickname)
			if len(output) > 0 {
				result.Message += fmt.Sprintf(" (output: %s)", strings.TrimSpace(string(output)))
			}
		}

		resultChan <- result
	}()

	return &CommandResult{
		Success:   true,
		Message:   fmt.Sprintf("Command '%s' started for device '%s'", command, nickname),
		Timestamp: time.Now(),
	}, resultChan
}

// GetCurrentPower gets the current power consumption of a device
func (m *Manager) GetCurrentPower(ctx context.Context, nickname string) (*CommandResult, <-chan *PowerReading) {
	resultChan := make(chan *CommandResult, 1)
	powerChan := make(chan *PowerReading, 1)

	// Check if device exists and has power monitoring
	plug, err := m.GetDevice(nickname)
	if err != nil {
		result := &CommandResult{
			Success:   false,
			Error:     err,
			Message:   fmt.Sprintf("Device '%s' not found", nickname),
			Timestamp: time.Now(),
		}
		go func() {
			resultChan <- result
			close(resultChan)
			close(powerChan)
		}()
		return result, powerChan
	}

	if !plug.HasPowerMonitoring {
		result := &CommandResult{
			Success:   false,
			Error:     errors.New("device does not support power monitoring"),
			Message:   fmt.Sprintf("Device '%s' does not support power monitoring", nickname),
			Timestamp: time.Now(),
		}
		go func() {
			resultChan <- result
			close(resultChan)
			close(powerChan)
		}()
		return result, powerChan
	}

	go func() {
		defer close(resultChan)
		defer close(powerChan)

		cmd := exec.CommandContext(ctx, m.kasaCmd, "--json", "--alias", nickname, "emeter")
		output, err := cmd.Output()

		result := &CommandResult{
			Timestamp: time.Now(),
		}

		if err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to get power reading from device '%s': %w", nickname, err)
			result.Message = fmt.Sprintf("Failed to get power reading from device '%s'", nickname)
			resultChan <- result
			return
		}

		// Parse the JSON output to extract current power
		var emeterData map[string]interface{}
		if err := json.Unmarshal(output, &emeterData); err != nil {
			result.Success = false
			result.Error = fmt.Errorf("failed to parse power reading JSON: %w", err)
			result.Message = "Failed to parse power reading response"
			resultChan <- result
			return
		}

		// Extract current power (assuming it's in the emeter data)
		currentPowerMW := 0
		if powerData, ok := emeterData["get_current_power"]; ok {
			if powerMap, ok := powerData.(map[string]interface{}); ok {
				if power, ok := powerMap["current_power"].(float64); ok {
					currentPowerMW = int(power)
				}
			}
		}

		powerReading := &PowerReading{
			CurrentPowerMW: currentPowerMW,
			CurrentPowerW:  float64(currentPowerMW) / 1000.0,
			Timestamp:      time.Now(),
		}

		result.Success = true
		result.Message = fmt.Sprintf("Power reading obtained: %.3f W", powerReading.CurrentPowerW)
		
		resultChan <- result
		powerChan <- powerReading
	}()

	return &CommandResult{
		Success:   true,
		Message:   fmt.Sprintf("Power reading started for device '%s'", nickname),
		Timestamp: time.Now(),
	}, powerChan
}

// GetPlugCount returns the number of discovered smart plugs
func (m *Manager) GetPlugCount() int {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return len(m.plugs)
}

// WaitForResult is a helper function to wait for a command result with timeout
func WaitForResult(resultChan <-chan *CommandResult, timeout time.Duration) *CommandResult {
	select {
	case result := <-resultChan:
		return result
	case <-time.After(timeout):
		return &CommandResult{
			Success:   false,
			Error:     errors.New("operation timed out"),
			Message:   "Operation timed out",
			Timestamp: time.Now(),
		}
	}
}
