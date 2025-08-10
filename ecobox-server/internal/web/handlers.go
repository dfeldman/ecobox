package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"text/template"
	"time"

	"ecobox-server/internal/auth"
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

// handleWebSocket upgrades connection to WebSocket and manages real-time updates
func (ws *WebServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		ws.logger.Errorf("Failed to upgrade WebSocket connection: %v", err)
		return
	}

	ws.logger.Info("New WebSocket connection established")

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
		ws.logger.Info("WebSocket connection closed")
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
	
	for _, server := range servers {
		update := monitor.ServerUpdate{
			ServerID: server.ID,
			State:    server.CurrentState,
			Services: server.Services,
			Server:   server,
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
