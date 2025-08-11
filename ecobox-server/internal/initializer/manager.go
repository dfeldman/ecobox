package initializer

import (
	"ecobox-server/internal/control"
	"ecobox-server/internal/models"
	"ecobox-server/internal/storage"
	"github.com/sirupsen/logrus"
)

// Manager handles server initialization tasks
type Manager struct {
	storage       storage.Storage
	systemMonitor *control.SystemMonitor
	logger        *logrus.Logger
}

// NewManager creates a new initializer manager
func NewManager(storage storage.Storage) *Manager {
	logger := logrus.New()
	return &Manager{
		storage:       storage,
		systemMonitor: control.NewSystemMonitor(storage, logger),
		logger:        logger,
	}
}

// SetLogger sets a custom logger
func (m *Manager) SetLogger(logger *logrus.Logger) {
	m.logger = logger
}

// InitializeServer performs initial setup for a server
func (m *Manager) InitializeServer(server *models.Server) error {
	m.logger.Infof("Initializing server %s (%s)", server.Name, server.Hostname)
	
	// Use the SystemMonitor for comprehensive initialization
	return m.systemMonitor.PerformInitializationCheck(server)
}


