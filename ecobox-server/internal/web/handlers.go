package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"ecobox-server/internal/auth"
	"ecobox-server/internal/metrics"
	"ecobox-server/internal/models"
	"ecobox-server/internal/monitor"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// handleIndex renders the main dashboard page
func (ws *WebServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Get current user from context
	user := auth.GetUserFromContext(r.Context())
	
	tmplContent := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Network Dashboard</title>
    <link rel="stylesheet" href="/static/css/dashboard.css">
    <link rel="stylesheet" href="/static/css/metrics.css">
	    <script src="https://cdnjs.cloudflare.com/ajax/libs/d3/7.8.5/d3.min.js"></script>

</head>
<body>
    <div class="container">
        <header>
            <h1>Network Dashboard</h1>
            <div class="status-bar">
                <span id="connection-status" class="status-indicator">Connecting...</span>
                <div class="header-actions">
                    <button id="refresh-btn" class="btn btn-secondary">Refresh</button>
                    {{if .User}}
                    <div class="user-menu">
                        <span class="username">{{.User.Username}}</span>
                        {{if eq .User.Username "admin"}}
                        <a href="/users" class="btn btn-sm">Manage Users</a>
                        {{end}}
                        <a href="/change-password" class="btn btn-sm">Change Password</a>
                        <button onclick="logout()" class="btn btn-sm btn-danger">Logout</button>
                    </div>
                    {{end}}
                </div>
            </div>
        </header>
        
        <main id="servers-container">
            <div class="loading">Loading servers...</div>
        </main>
    </div>

    <script src="/static/js/dashboard.js"></script>
    <script src="/static/js/metrics.js"></script>
    <script>
        function logout() {
            if (confirm('Are you sure you want to logout?')) {
                fetch('/logout', {
                    method: 'POST',
                    headers: {
                        'Accept': 'application/json'
                    }
                }).then(() => {
                    window.location.href = '/login';
                }).catch(error => {
                    console.error('Logout error:', error);
                    window.location.href = '/login';
                });
            }
        }
    </script>
</body>
</html>`

	tmpl, err := template.New("index").Parse(tmplContent)
	if err != nil {
		ws.logger.Errorf("Failed to parse template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := struct {
		User *auth.User
	}{
		User: user,
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		ws.logger.Errorf("Failed to execute template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// handleGetServers returns JSON list of all servers
func (ws *WebServer) handleGetServers(w http.ResponseWriter, r *http.Request) {
	servers := ws.storage.GetAllServers()
	
	// Convert map to slice for JSON response
	serverList := make([]*models.Server, 0, len(servers))
	for _, server := range servers {
		serverList = append(serverList, server)
	}

	// Debug logging to see what servers we're returning
	ws.logger.WithField("server_count", len(serverList)).Debug("Returning servers to frontend")
	for i, server := range serverList {
		ws.logger.WithFields(map[string]interface{}{
			"index":           i,
			"name":            server.Name,
			"hostname":        server.Hostname,
			"current_state":   server.CurrentState,
			"initialized":     server.Initialized,
			"is_proxmox_vm":   server.IsProxmoxVM,
			"parent_server":   server.ParentServerID,
		}).Debug("Server details")
	}

	response := APIResponse{
		Success: true,
		Data:    serverList,
	}

	ws.writeJSONResponse(w, http.StatusOK, response)
}

// handleGetServer returns a specific server by ID
func (ws *WebServer) handleGetServer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["id"]

	server, err := ws.storage.GetServer(serverID)
	if err != nil {
		response := APIResponse{
			Success: false,
			Message: fmt.Sprintf("Server not found: %s", serverID),
		}
		ws.writeJSONResponse(w, http.StatusNotFound, response)
		return
	}

	response := APIResponse{
		Success: true,
		Data:    server,
	}

	ws.writeJSONResponse(w, http.StatusOK, response)
}

// handleWakeServer handles wake requests for a server
func (ws *WebServer) handleWakeServer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["id"]

	server, err := ws.storage.GetServer(serverID)
	if err != nil {
		response := APIResponse{
			Success: false,
			Message: fmt.Sprintf("Server not found: %s", serverID),
		}
		ws.writeJSONResponse(w, http.StatusNotFound, response)
		return
	}

	// Set desired state to "on"
	server.DesiredState = models.PowerStateOn
	if err := ws.storage.UpdateServer(server); err != nil {
		ws.logger.Errorf("Failed to update server desired state: %v", err)
	}

	// Attempt to wake the server
	if err := ws.powerManager.WakeServer(server); err != nil {
		response := APIResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to wake server: %v", err),
		}
		ws.writeJSONResponse(w, http.StatusInternalServerError, response)
		return
	}

	response := APIResponse{
		Success: true,
		Message: fmt.Sprintf("Wake signal sent to %s", server.Name),
	}

	ws.writeJSONResponse(w, http.StatusOK, response)
}

// handleSuspendServer handles suspend requests for a server
func (ws *WebServer) handleSuspendServer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["id"]

	server, err := ws.storage.GetServer(serverID)
	if err != nil {
		response := APIResponse{
			Success: false,
			Message: fmt.Sprintf("Server not found: %s", serverID),
		}
		ws.writeJSONResponse(w, http.StatusNotFound, response)
		return
	}

	// Set desired state to "suspended"
	server.DesiredState = models.PowerStateSuspended
	if err := ws.storage.UpdateServer(server); err != nil {
		ws.logger.Errorf("Failed to update server desired state: %v", err)
	}

	// Attempt to suspend the server
	if err := ws.powerManager.SuspendServer(server); err != nil {
		response := APIResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to suspend server: %v", err),
		}
		ws.writeJSONResponse(w, http.StatusInternalServerError, response)
		return
	}

	response := APIResponse{
		Success: true,
		Message: fmt.Sprintf("Suspend command sent to %s", server.Name),
	}

	ws.writeJSONResponse(w, http.StatusOK, response)
}

// handleShutdownServer handles clean shutdown requests for a server
func (ws *WebServer) handleShutdownServer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["id"]

	server, err := ws.storage.GetServer(serverID)
	if err != nil {
		response := APIResponse{
			Success: false,
			Message: fmt.Sprintf("Server not found: %v", err),
		}
		ws.writeJSONResponse(w, http.StatusNotFound, response)
		return
	}

	// Set desired state to "stopped"
	server.DesiredState = models.PowerStateStopped

	if err := ws.storage.UpdateServer(server); err != nil {
		response := APIResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update server state: %v", err),
		}
		ws.writeJSONResponse(w, http.StatusInternalServerError, response)
		return
	}

	// Attempt to shutdown the server
	if err := ws.powerManager.ShutdownServer(server); err != nil {
		response := APIResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to shutdown server: %v", err),
		}
		ws.writeJSONResponse(w, http.StatusInternalServerError, response)
		return
	}

	response := APIResponse{
		Success: true,
		Message: fmt.Sprintf("Shutdown command sent to %s", server.Name),
	}

	ws.writeJSONResponse(w, http.StatusOK, response)
}

// handleStopServer handles force stop requests for a server (Proxmox VMs only)
func (ws *WebServer) handleStopServer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverID := vars["id"]

	server, err := ws.storage.GetServer(serverID)
	if err != nil {
		response := APIResponse{
			Success: false,
			Message: fmt.Sprintf("Server not found: %v", err),
		}
		ws.writeJSONResponse(w, http.StatusNotFound, response)
		return
	}

	// Check if this is a Proxmox VM
	if !server.IsProxmoxVM {
		response := APIResponse{
			Success: false,
			Message: "Force stop is only supported for Proxmox VMs",
		}
		ws.writeJSONResponse(w, http.StatusBadRequest, response)
		return
	}

	// Set desired state to "stopped"
	server.DesiredState = models.PowerStateStopped

	if err := ws.storage.UpdateServer(server); err != nil {
		response := APIResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to update server state: %v", err),
		}
		ws.writeJSONResponse(w, http.StatusInternalServerError, response)
		return
	}

	// Attempt to force stop the server
	if err := ws.powerManager.StopServer(server); err != nil {
		response := APIResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to stop server: %v", err),
		}
		ws.writeJSONResponse(w, http.StatusInternalServerError, response)
		return
	}

	response := APIResponse{
		Success: true,
		Message: fmt.Sprintf("Stop command sent to %s", server.Name),
	}

	ws.writeJSONResponse(w, http.StatusOK, response)
}

// handleWebSocket upgrades connection to WebSocket and manages real-time updates
func (ws *WebServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Authenticate the WebSocket connection
	user, err := ws.authManager.AuthenticateRequest(r)
	if err != nil {
		ws.logger.Errorf("WebSocket authentication failed: %v", err)
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	
	conn, err := ws.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		ws.logger.Errorf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	ws.logger.WithField("user", user.Username).Info("New WebSocket connection established")

	// Add client to the list
	ws.mu.Lock()
	ws.wsClients[conn] = true
	ws.mu.Unlock()

	// Send current server states immediately
	go ws.sendInitialData(conn)

	// Handle connection cleanup when client disconnects
	defer func() {
		ws.mu.Lock()
		delete(ws.wsClients, conn)
		ws.mu.Unlock()
		conn.Close()
		ws.logger.WithField("user", user.Username).Info("WebSocket connection closed")
	}()

	// Keep connection alive and handle ping/pong
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	// Read messages from client (mainly for ping/pong)
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				ws.logger.Errorf("WebSocket error: %v", err)
			}
			break
		}
	}
}

// sendInitialData sends current server states to a new WebSocket client
func (ws *WebServer) sendInitialData(conn *websocket.Conn) {
	servers := ws.storage.GetAllServers()
	metricsManager := ws.monitor.GetMetricsManager()
	
	for _, server := range servers {
		// Get current metrics for this server
		var metrics map[string]float64
		if metricsManager != nil {
			if currentMetrics, err := metricsManager.GetLatestValues(server.ID); err != nil {
				ws.logger.Errorf("Failed to get latest metrics for server %s: %v", server.ID, err)
				metrics = make(map[string]float64)
			} else {
				metrics = currentMetrics
			}
		} else {
			metrics = make(map[string]float64)
		}

		update := monitor.ServerUpdate{
			ServerID: server.ID,
			State:    server.CurrentState,
			Services: server.Services,
			Server:   server,
			Metrics:  metrics,
		}

		message, err := json.Marshal(update)
		if err != nil {
			ws.logger.Errorf("Failed to marshal initial data: %v", err)
			continue
		}

		if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
			ws.logger.Errorf("Failed to send initial data: %v", err)
			return
		}
	}
}

// writeJSONResponse writes a JSON response with proper headers
func (ws *WebServer) writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		ws.logger.Errorf("Failed to encode JSON response: %v", err)
	}
}

// MetricDataPoint represents a single metric data point for the frontend
type MetricDataPoint struct {
	Timestamp string  `json:"timestamp"` // ISO 8601 format
	Value     float64 `json:"value"`
}

// MetricsResponse represents the complete metrics response for the frontend
type MetricsResponse struct {
	Memory  []MetricDataPoint `json:"memory"`
	CPU     []MetricDataPoint `json:"cpu"`
	Network []MetricDataPoint `json:"network"`
	Wattage []MetricDataPoint `json:"wattage"`
}

// handleGetMetrics returns metrics data for a specific server and time range
func (ws *WebServer) handleGetMetrics(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	serverID := r.URL.Query().Get("server")
	startTimeStr := r.URL.Query().Get("start")
	endTimeStr := r.URL.Query().Get("end")
	
	if serverID == "" {
		response := APIResponse{
			Success: false,
			Message: "Missing required parameter: server",
		}
		ws.writeJSONResponse(w, http.StatusBadRequest, response)
		return
	}
	
	if startTimeStr == "" || endTimeStr == "" {
		response := APIResponse{
			Success: false,
			Message: "Missing required parameters: start and end time",
		}
		ws.writeJSONResponse(w, http.StatusBadRequest, response)
		return
	}
	
	// Parse time parameters
	startTime, err := time.Parse(time.RFC3339, startTimeStr)
	if err != nil {
		response := APIResponse{
			Success: false,
			Message: "Invalid start time format. Use ISO 8601 format.",
		}
		ws.writeJSONResponse(w, http.StatusBadRequest, response)
		return
	}
	
	endTime, err := time.Parse(time.RFC3339, endTimeStr)
	if err != nil {
		response := APIResponse{
			Success: false,
			Message: "Invalid end time format. Use ISO 8601 format.",
		}
		ws.writeJSONResponse(w, http.StatusBadRequest, response)
		return
	}
	
	// Validate time range
	if endTime.Before(startTime) {
		response := APIResponse{
			Success: false,
			Message: "End time must be after start time",
		}
		ws.writeJSONResponse(w, http.StatusBadRequest, response)
		return
	}
	
	// Get metrics manager from monitor
	metricsManager := ws.monitor.GetMetricsManager()
	if metricsManager == nil {
		response := APIResponse{
			Success: false,
			Message: "Metrics system not available",
		}
		ws.writeJSONResponse(w, http.StatusServiceUnavailable, response)
		return
	}
	
	// Calculate appropriate time period for aggregation
	timeRange := endTime.Sub(startTime)
	timePeriodSec := ws.calculateTimePeriod(timeRange)
	
	// Fetch metrics data for each metric type
	metricsResponse := MetricsResponse{
		Memory:  []MetricDataPoint{},
		CPU:     []MetricDataPoint{},
		Network: []MetricDataPoint{},
		Wattage: []MetricDataPoint{},
	}
	
	// Get memory metrics
	if memoryData, err := metricsManager.GetSummary(serverID, metrics.StandardMetrics.Memory, startTime, endTime, timePeriodSec); err == nil {
		metricsResponse.Memory = ws.convertSummaryToDataPoints(memoryData)
	}
	
	// Get CPU metrics  
	if cpuData, err := metricsManager.GetSummary(serverID, metrics.StandardMetrics.CPU, startTime, endTime, timePeriodSec); err == nil {
		metricsResponse.CPU = ws.convertSummaryToDataPoints(cpuData)
	}
	
	// Get network metrics
	if networkData, err := metricsManager.GetSummary(serverID, metrics.StandardMetrics.Network, startTime, endTime, timePeriodSec); err == nil {
		metricsResponse.Network = ws.convertSummaryToDataPoints(networkData)
	}
	
	// Get wattage metrics
	if wattageData, err := metricsManager.GetSummary(serverID, metrics.StandardMetrics.Wattage, startTime, endTime, timePeriodSec); err == nil {
		metricsResponse.Wattage = ws.convertSummaryToDataPoints(wattageData)
	}
	
	response := APIResponse{
		Success: true,
		Data:    metricsResponse,
	}
	
	ws.writeJSONResponse(w, http.StatusOK, response)
}

// handleGetAvailableMetrics returns all available metrics for a server
func (ws *WebServer) handleGetAvailableMetrics(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serverName := vars["server"]
	
	if serverName == "" {
		response := APIResponse{
			Success: false,
			Message: "Missing server name",
		}
		ws.writeJSONResponse(w, http.StatusBadRequest, response)
		return
	}
	
	// Return all available metrics
	availableMetrics := map[string]interface{}{
		"frontend_metrics": metrics.FrontendMetrics(),
		"all_metrics":     metrics.AllMetrics(),
		"server":          serverName,
	}
	
	response := APIResponse{
		Success: true,
		Data:    availableMetrics,
	}
	
	ws.writeJSONResponse(w, http.StatusOK, response)
}

// calculateTimePeriod determines appropriate aggregation period based on time range
func (ws *WebServer) calculateTimePeriod(timeRange time.Duration) int {
	// For optimal performance, aim for 100-200 data points
	hours := int(timeRange.Hours())
	
	if hours <= 2 {
		return 60 // 1 minute intervals
	} else if hours <= 12 {
		return 300 // 5 minute intervals
	} else if hours <= 48 {
		return 1800 // 30 minute intervals
	} else if hours <= 168 { // 1 week
		return 3600 // 1 hour intervals
	} else if hours <= 720 { // 1 month
		return 14400 // 4 hour intervals
	} else {
		return 86400 // 1 day intervals
	}
}

// convertSummaryToDataPoints converts metrics.Summary to MetricDataPoint format
func (ws *WebServer) convertSummaryToDataPoints(summaries []metrics.Summary) []MetricDataPoint {
	dataPoints := make([]MetricDataPoint, len(summaries))
	
	for i, summary := range summaries {
		dataPoints[i] = MetricDataPoint{
			Timestamp: summary.StartTime.Format(time.RFC3339),
			Value:     summary.Average,
		}
	}
	
	return dataPoints
}
