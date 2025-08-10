package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ecobox-server/internal/auth"
	"ecobox-server/internal/config"
	"ecobox-server/internal/control"
	"ecobox-server/internal/models"
	"ecobox-server/internal/monitor"
	"ecobox-server/internal/storage"
	"ecobox-server/internal/web"
	"github.com/sirupsen/logrus"
)

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config.toml", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Setup logging
	logger := setupLogging(cfg.Dashboard.LogLevel)
	logger.Info("Starting Network Dashboard")
	logger.Infof("Configuration loaded from: %s", *configPath)

	// Initialize storage
	storage := storage.NewMemoryStorage()
	logger.Info("Initialized memory storage")

	// Load servers from configuration
	if err := loadServersFromConfig(cfg, storage, logger); err != nil {
		logger.Fatalf("Failed to load servers from configuration: %v", err)
	}

	// Create power manager
	powerManager := control.NewPowerManager(storage)
	powerManager.SetLogger(logger)
	logger.Info("Initialized power manager")

	// Initialize authentication
	authManager := auth.NewManager(cfg)
	authManager.SetLogger(logger)
	if err := authManager.Initialize(); err != nil {
		logger.Fatalf("Failed to initialize authentication: %v", err)
	}
	logger.Info("Initialized authentication system")

	// Create monitor
	monitor := monitor.NewMonitor(cfg, storage, powerManager)
	monitor.SetLogger(logger)
	logger.Info("Initialized server monitor")

	// Create web server
	webServer := web.NewWebServer(cfg, storage, monitor, powerManager, authManager)
	webServer.SetLogger(logger)
	logger.Info("Initialized web server")

	// Start monitor
	monitor.Start()
	logger.Info("Server monitor started")

	// Start web server in goroutine with error handling
	webServerErr := make(chan error, 1)
	go func() {
		logger.Infof("Starting web server on port %d", cfg.Dashboard.Port)
		if err := webServer.Start(); err != nil {
			if err != http.ErrServerClosed {
				webServerErr <- err
				return
			}
		}
		webServerErr <- nil
	}()

	// Wait for either interrupt signal or web server error
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	select {
	case sig := <-quit:
		logger.Infof("Received signal: %v", sig)
	case err := <-webServerErr:
		if err != nil {
			logger.Fatalf("Web server failed to start: %v", err)
		}
		logger.Info("Web server stopped")
		return
	}

	logger.Info("Shutting down Network Dashboard...")

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown web server
	if err := webServer.Stop(ctx); err != nil {
		logger.Errorf("Failed to shutdown web server: %v", err)
	}

	// Stop monitor
	monitor.Stop()

	logger.Info("Network Dashboard stopped")
}

// setupLogging configures the logger based on the specified log level
func setupLogging(logLevel string) *logrus.Logger {
	logger := logrus.New()

	// Set log format
	logger.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: time.RFC3339,
		FullTimestamp:   true,
	})

	// Set log level
	switch logLevel {
	case "debug":
		logger.SetLevel(logrus.DebugLevel)
	case "info":
		logger.SetLevel(logrus.InfoLevel)
	case "warn":
		logger.SetLevel(logrus.WarnLevel)
	case "error":
		logger.SetLevel(logrus.ErrorLevel)
	default:
		logger.SetLevel(logrus.InfoLevel)
		logger.Warnf("Unknown log level '%s', defaulting to 'info'", logLevel)
	}

	return logger
}

// loadServersFromConfig loads servers from configuration into storage
func loadServersFromConfig(cfg *config.Config, storage storage.Storage, logger *logrus.Logger) error {
	for _, serverConfig := range cfg.Servers {
		// Create services from configuration
		services := make([]models.Service, len(serverConfig.Services))
		for i, serviceConfig := range serverConfig.Services {
			serviceType := models.ServiceType(serviceConfig.Type)
			if serviceType == "" {
				serviceType = models.DetectServiceType(serviceConfig.Port)
			}

			services[i] = models.Service{
				ID:       fmt.Sprintf("%s-%s", serverConfig.ID, serviceConfig.Name),
				ServerID: serverConfig.ID,
				Name:     serviceConfig.Name,
				Port:     serviceConfig.Port,
				Type:     serviceType,
				Status:   models.ServiceStatusDown, // Will be updated by monitor
				Source:   models.SourceConfig,
			}
		}

		// Create server
		server := &models.Server{
			ID:             serverConfig.ID,
			Name:           serverConfig.Name,
			Hostname:       serverConfig.Hostname,
			MACAddress:     serverConfig.MACAddress,
			CurrentState:   models.PowerStateUnknown,
			DesiredState:   models.PowerStateUnknown,
			ParentServerID: serverConfig.ParentServerID,
			Services:       services,
			Source:         models.SourceConfig,
			SSHUser:        serverConfig.SSHUser,
			SSHPort:        serverConfig.SSHPort,
			SSHKeyPath:     serverConfig.SSHKeyPath,
			RecentActions:  make([]models.ServerAction, 0),
			LastStateChange: time.Now(),
		}

		// Add server to storage
		if err := storage.AddServer(server); err != nil {
			return fmt.Errorf("failed to add server %s: %w", server.ID, err)
		}

		logger.Infof("Loaded server: %s (%s)", server.Name, server.Hostname)
	}

	logger.Infof("Loaded %d servers from configuration", len(cfg.Servers))
	return nil
}
