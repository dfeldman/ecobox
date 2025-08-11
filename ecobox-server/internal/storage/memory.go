package storage

import (
	"fmt"
	"sync"
	"time"

	"ecobox-server/internal/models"
)

// Storage interface defines the contract for data storage operations
type Storage interface {
	GetServer(id string) (*models.Server, error)
	GetAllServers() map[string]*models.Server
	UpdateServer(server *models.Server) error
	AddServer(server *models.Server) error
	DeleteServer(id string) error

	UpdateServerState(id string, state models.PowerState) error
	UpdateServerTimes(id string) error
	AddServerAction(id string, action models.ServerAction) error
	
	// SystemInfo operations
	UpdateServerSystemInfo(id string, systemInfo *models.SystemInfo) error
	GetServerSystemInfo(id string) (*models.SystemInfo, error)
}

// MemoryStorage provides in-memory storage implementation
type MemoryStorage struct {
	servers map[string]*models.Server
	mu      sync.RWMutex
}

// NewMemoryStorage creates a new in-memory storage instance
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		servers: make(map[string]*models.Server),
	}
}

// GetServer retrieves a server by ID
func (ms *MemoryStorage) GetServer(id string) (*models.Server, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	server, exists := ms.servers[id]
	if !exists {
		return nil, fmt.Errorf("server with ID '%s' not found", id)
	}

	// Return a copy to prevent external modifications
	serverCopy := *server
	return &serverCopy, nil
}

// GetAllServers returns a copy of all servers
func (ms *MemoryStorage) GetAllServers() map[string]*models.Server {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	result := make(map[string]*models.Server)
	for id, server := range ms.servers {
		serverCopy := *server
		result[id] = &serverCopy
	}
	return result
}

// UpdateServer updates an existing server
func (ms *MemoryStorage) UpdateServer(server *models.Server) error {
	if server == nil {
		return fmt.Errorf("server cannot be nil")
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, exists := ms.servers[server.ID]; !exists {
		return fmt.Errorf("server with ID '%s' not found", server.ID)
	}

	// Create a copy before storing
	serverCopy := *server
	ms.servers[server.ID] = &serverCopy
	return nil
}

// AddServer adds a new server
func (ms *MemoryStorage) AddServer(server *models.Server) error {
	if server == nil {
		return fmt.Errorf("server cannot be nil")
	}

	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, exists := ms.servers[server.ID]; exists {
		return fmt.Errorf("server with ID '%s' already exists", server.ID)
	}

	// Initialize time tracking fields if not set
	if server.LastStateChange.IsZero() {
		server.LastStateChange = time.Now()
	}
	if server.CurrentState == "" {
		server.CurrentState = models.PowerStateUnknown
	}
	if server.DesiredState == "" {
		server.DesiredState = models.PowerStateUnknown
	}
	if server.RecentActions == nil {
		server.RecentActions = make([]models.ServerAction, 0)
	}

	// Create a copy before storing
	serverCopy := *server
	ms.servers[server.ID] = &serverCopy
	return nil
}

// DeleteServer removes a server
func (ms *MemoryStorage) DeleteServer(id string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if _, exists := ms.servers[id]; !exists {
		return fmt.Errorf("server with ID '%s' not found", id)
	}

	delete(ms.servers, id)
	return nil
}

// UpdateServerState updates the power state of a server
func (ms *MemoryStorage) UpdateServerState(id string, state models.PowerState) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	server, exists := ms.servers[id]
	if !exists {
		return fmt.Errorf("server with ID '%s' not found", id)
	}

	// Update time tracking before changing state
	if err := ms.updateServerTimesLocked(server); err != nil {
		return fmt.Errorf("failed to update server times: %w", err)
	}

	// Update the state
	server.CurrentState = state
	server.LastStateChange = time.Now()

	return nil
}

// UpdateServerTimes updates time tracking based on state changes
func (ms *MemoryStorage) UpdateServerTimes(id string) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	server, exists := ms.servers[id]
	if !exists {
		return fmt.Errorf("server with ID '%s' not found", id)
	}

	return ms.updateServerTimesLocked(server)
}

// updateServerTimesLocked updates time tracking (requires lock to be held)
func (ms *MemoryStorage) updateServerTimesLocked(server *models.Server) error {
	if server.LastStateChange.IsZero() {
		server.LastStateChange = time.Now()
		return nil
	}

	duration := int64(time.Since(server.LastStateChange).Seconds())

	switch server.CurrentState {
	case models.PowerStateOn:
		server.TotalOnTime += duration
	case models.PowerStateSuspended:
		server.TotalSuspendedTime += duration
	case models.PowerStateOff:
		server.TotalOffTime += duration
	// INIT_FAILED is treated as off time since the server is effectively unavailable
	case models.PowerStateInitFailed:
		server.TotalOffTime += duration
	}

	return nil
}

// AddServerAction adds an action to the server's recent actions list
func (ms *MemoryStorage) AddServerAction(id string, action models.ServerAction) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	server, exists := ms.servers[id]
	if !exists {
		return fmt.Errorf("server with ID '%s' not found", id)
	}

	// Add the new action
	server.RecentActions = append(server.RecentActions, action)

	// Keep only the last 50 actions (FIFO)
	if len(server.RecentActions) > 50 {
		server.RecentActions = server.RecentActions[len(server.RecentActions)-50:]
	}

	return nil
}

// UpdateServerSystemInfo updates the system information for a server
func (ms *MemoryStorage) UpdateServerSystemInfo(id string, systemInfo *models.SystemInfo) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	server, exists := ms.servers[id]
	if !exists {
		return fmt.Errorf("server with ID '%s' not found", id)
	}

	// Update the system info with current timestamp
	if systemInfo != nil {
		systemInfo.LastUpdated = time.Now()
	}
	server.SystemInfo = systemInfo

	return nil
}

// GetServerSystemInfo retrieves the system information for a server
func (ms *MemoryStorage) GetServerSystemInfo(id string) (*models.SystemInfo, error) {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	server, exists := ms.servers[id]
	if !exists {
		return nil, fmt.Errorf("server with ID '%s' not found", id)
	}

	if server.SystemInfo == nil {
		return nil, nil // No system info available
	}

	// Return a copy to prevent external modifications
	systemInfoCopy := *server.SystemInfo
	return &systemInfoCopy, nil
}
