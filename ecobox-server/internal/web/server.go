package web

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"ecobox-server/internal/auth"
	"ecobox-server/internal/config"
	"ecobox-server/internal/control"
	"ecobox-server/internal/monitor"
	"ecobox-server/internal/storage"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// WebServer handles HTTP requests and WebSocket connections
type WebServer struct {
	config        *config.Config
	storage       storage.Storage
	monitor       *monitor.Monitor
	powerManager  *control.PowerManager
	authManager   *auth.Manager
	authMiddleware *auth.Middleware
	router        *mux.Router
	server        *http.Server
	wsUpgrader    websocket.Upgrader
	wsClients     map[*websocket.Conn]bool
	logger        *logrus.Logger
	mu            sync.RWMutex
}

// NewWebServer creates a new web server instance
func NewWebServer(cfg *config.Config, storage storage.Storage, monitor *monitor.Monitor, pm *control.PowerManager, am *auth.Manager) *WebServer {
	ws := &WebServer{
		config:        cfg,
		storage:       storage,
		monitor:       monitor,
		powerManager:  pm,
		authManager:   am,
		authMiddleware: auth.NewMiddleware(am),
		router:        mux.NewRouter(),
		wsUpgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for simplicity
			},
		},
		wsClients: make(map[*websocket.Conn]bool),
		logger:    logrus.New(),
	}

	ws.setupRoutes()
	return ws
}

// Start starts the web server and begins handling requests
func (ws *WebServer) Start() error {
	ws.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", ws.config.Dashboard.Port),
		Handler:      ws.router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start goroutine to forward monitor updates to WebSocket clients
	go ws.handleMonitorUpdates()

	ws.logger.Infof("Starting web server on port %d", ws.config.Dashboard.Port)
	return ws.server.ListenAndServe()
}

// Stop gracefully shuts down the web server
func (ws *WebServer) Stop(ctx context.Context) error {
	ws.logger.Info("Shutting down web server")
	
	// Close all WebSocket connections
	ws.mu.Lock()
	for conn := range ws.wsClients {
		conn.Close()
	}
	ws.wsClients = make(map[*websocket.Conn]bool)
	ws.mu.Unlock()

	return ws.server.Shutdown(ctx)
}

// setupRoutes configures all HTTP routes
func (ws *WebServer) setupRoutes() {
	// Authentication routes (public)
	ws.router.HandleFunc("/login", ws.handleLogin).Methods("GET", "POST")
	ws.router.HandleFunc("/logout", ws.handleLogout).Methods("POST")
	ws.router.HandleFunc("/setup", ws.handleSetup).Methods("GET", "POST")

	// Static files (public)
	ws.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static/"))))

	// API routes (protected)
	api := ws.router.PathPrefix("/api").Subrouter()
	api.HandleFunc("/servers", ws.handleGetServers).Methods("GET")
	api.HandleFunc("/servers/{id}/wake", ws.handleWakeServer).Methods("POST")
	api.HandleFunc("/servers/{id}/suspend", ws.handleSuspendServer).Methods("POST")
	api.HandleFunc("/servers/{id}", ws.handleGetServer).Methods("GET")
	
	// Metrics API routes (protected)
	api.HandleFunc("/metrics", ws.handleGetMetrics).Methods("GET")
	api.HandleFunc("/metrics/{server}/available", ws.handleGetAvailableMetrics).Methods("GET")
	
	// Debug: Add a simple test route to verify API subrouter works
	api.HandleFunc("/debug-test", func(w http.ResponseWriter, r *http.Request) {
		ws.writeJSONResponse(w, http.StatusOK, APIResponse{
			Success: true,
			Message: "Debug API route is working",
		})
	}).Methods("GET")
	
	// Authentication API routes (protected)
	auth := api.PathPrefix("/auth").Subrouter()
	auth.HandleFunc("/me", ws.handleGetCurrentUser).Methods("GET")
	auth.HandleFunc("/password", ws.handleChangePassword).Methods("POST")
	auth.HandleFunc("/users", ws.handleGetUsers).Methods("GET")
	auth.HandleFunc("/users", ws.handleCreateUser).Methods("POST") 
	auth.HandleFunc("/users/{username}", ws.handleDeleteUser).Methods("DELETE")

	// WebSocket endpoint (protected)
	ws.router.HandleFunc("/ws", ws.handleWebSocket)

	// User management page (protected)
	ws.router.HandleFunc("/users", ws.handleUsersPage).Methods("GET")
	ws.router.HandleFunc("/change-password", ws.handleChangePasswordPage).Methods("GET")

	// Main page (protected)
	ws.router.HandleFunc("/", ws.handleIndex).Methods("GET")

	// Add middleware
	ws.router.Use(ws.loggingMiddleware)
	ws.router.Use(ws.corsMiddleware)
	ws.router.Use(ws.authMiddleware.RequireAuth)
}

// handleMonitorUpdates forwards monitor updates to WebSocket clients
func (ws *WebServer) handleMonitorUpdates() {
	for update := range ws.monitor.GetUpdates() {
		ws.broadcastUpdate(update)
	}
}

// broadcastUpdate sends updates to all connected WebSocket clients
func (ws *WebServer) broadcastUpdate(update monitor.ServerUpdate) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	message, err := json.Marshal(update)
	if err != nil {
		ws.logger.Errorf("Failed to marshal update: %v", err)
		return
	}

	// Send to all connected clients
	for conn := range ws.wsClients {
		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			ws.logger.Warnf("Failed to send WebSocket message: %v", err)
			conn.Close()
			delete(ws.wsClients, conn)
		}
	}
}

// SetLogger sets a custom logger
func (ws *WebServer) SetLogger(logger *logrus.Logger) {
	ws.logger = logger
}

// loggingMiddleware logs HTTP requests
func (ws *WebServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Skip wrapping for WebSocket upgrades to avoid hijacker issues
		if r.URL.Path == "/ws" {
			next.ServeHTTP(w, r)
			ws.logger.Infof("%s %s WebSocket %v", r.Method, r.URL.Path, time.Since(start))
			return
		}
		
		// Create a response recorder to capture status code for non-WebSocket requests
		recorder := &responseRecorder{ResponseWriter: w, statusCode: 200}
		
		next.ServeHTTP(recorder, r)
		
		ws.logger.Infof("%s %s %d %v", r.Method, r.URL.Path, recorder.statusCode, time.Since(start))
	})
}

// corsMiddleware adds CORS headers
func (ws *WebServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// responseRecorder wraps http.ResponseWriter to capture status code
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (rec *responseRecorder) WriteHeader(code int) {
	rec.statusCode = code
	rec.ResponseWriter.WriteHeader(code)
}

// Hijack implements http.Hijacker if the underlying ResponseWriter supports it
func (rec *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if hijacker, ok := rec.ResponseWriter.(http.Hijacker); ok {
		return hijacker.Hijack()
	}
	return nil, nil, fmt.Errorf("hijacking not supported")
}
