// Network Dashboard JavaScript
class NetworkDashboard {
    constructor() {
        this.servers = new Map();
        this.websocket = null;
        this.connectionStatus = document.getElementById('connection-status');
        this.serversContainer = document.getElementById('servers-container');
        this.refreshBtn = document.getElementById('refresh-btn');
        
        this.init();
    }

    init() {
        this.setupEventListeners();
        this.loadServers();
        this.connectWebSocket();
    }

    setupEventListeners() {
        this.refreshBtn.addEventListener('click', () => {
            this.loadServers();
        });
    }

    async loadServers() {
        try {
            const response = await fetch('/api/servers');
            const data = await response.json();
            
            if (data.success) {
                this.updateServers(data.data);
                this.renderServers();
            } else {
                this.showError('Failed to load servers: ' + data.message);
            }
        } catch (error) {
            this.showError('Failed to load servers: ' + error.message);
        }
    }

    updateServers(serverData) {
        this.servers.clear();
        serverData.forEach(server => {
            this.servers.set(server.id, server);
        });
    }

    renderServers() {
        if (this.servers.size === 0) {
            this.serversContainer.innerHTML = '<div class="loading">No servers configured</div>';
            return;
        }

        const serversGrid = document.createElement('div');
        serversGrid.className = 'servers-grid';

        for (const server of this.servers.values()) {
            const serverCard = this.createServerCard(server);
            serversGrid.appendChild(serverCard);
        }

        this.serversContainer.innerHTML = '';
        this.serversContainer.appendChild(serversGrid);
    }

    createServerCard(server) {
        const card = document.createElement('div');
        card.className = 'server-card';
        card.id = `server-${server.id}`;

        card.innerHTML = `
            <div class="server-header">
                <div class="server-info">
                    <h3>${this.escapeHtml(server.name)}</h3>
                    <div class="server-hostname">${this.escapeHtml(server.hostname)}</div>
                    ${server.parent_server_id ? `<div class="parent-info">Child of: ${this.getParentName(server.parent_server_id)}</div>` : ''}
                </div>
                <div class="power-state ${server.current_state}">${server.current_state.toUpperCase()}</div>
            </div>
            
            <div class="server-actions">
                ${this.createPowerButtons(server)}
            </div>
            
            <div class="services-section">
                <h4>Services</h4>
                <div class="services-list">
                    ${this.createServicesHTML(server.services || [])}
                </div>
            </div>
            
            <div class="uptime-stats">
                <h4>Statistics</h4>
                <div class="stats-grid">
                    <div class="stat-item">
                        <div class="stat-value">${this.formatUptime(server.total_on_time + this.getCurrentUptime(server))}</div>
                        <div>Total Uptime</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value">${this.formatUptime(this.getCurrentUptime(server))}</div>
                        <div>Current Session</div>
                    </div>
                </div>
            </div>
            
            <div class="metrics-section" id="metrics-section-${server.id}">
                <h4>Current Metrics</h4>
                <div class="metrics-grid" id="metrics-grid-${server.id}">
                    <!-- Metrics will be populated by WebSocket updates -->
                </div>
            </div>
            
            ${server.recent_actions && server.recent_actions.length > 0 ? `
                <details class="recent-actions">
                    <summary>Recent Actions (${server.recent_actions.length})</summary>
                    <div class="actions-list">
                        ${this.createActionsHTML(server.recent_actions)}
                    </div>
                </details>
            ` : ''}
        `;

        return card;
    }

    createPowerButtons(server) {
        const canWake = server.current_state === 'off' || server.current_state === 'suspended';
        const canSuspend = server.current_state === 'on';

        return `
            <button class="btn btn-success" onclick="dashboard.wakeServer('${server.id}')" ${!canWake ? 'disabled' : ''}>
                Wake Up
            </button>
            <button class="btn btn-warning" onclick="dashboard.suspendServer('${server.id}')" ${!canSuspend ? 'disabled' : ''}>
                Suspend
            </button>
            <button class="btn btn-info" onclick="dashboard.showMetrics('${server.id}', '${this.escapeHtml(server.name)}')">
                Show Metrics
            </button>
        `;
    }

    createServicesHTML(services) {
        return services.map(service => `
            <div class="service-tag ${service.status}">
                <div class="service-status ${service.status}"></div>
                <span>${this.escapeHtml(service.name)} (${service.port})</span>
            </div>
        `).join('');
    }

    createActionsHTML(actions) {
        return actions.slice(-10).reverse().map(action => `
            <div class="action-item">
                <span class="${action.success ? 'action-success' : 'action-error'}">
                    ${action.action.toUpperCase()} ${action.success ? '✓' : '✗'}
                </span>
                <span>${this.formatDateTime(action.timestamp)}</span>
            </div>
        `).join('');
    }

    async wakeServer(serverId) {
        try {
            const response = await fetch(`/api/servers/${serverId}/wake`, {
                method: 'POST'
            });
            const data = await response.json();
            
            if (!data.success) {
                this.showError(data.message);
            }
        } catch (error) {
            this.showError('Failed to wake server: ' + error.message);
        }
    }

    async suspendServer(serverId) {
        if (!confirm('Are you sure you want to suspend this server?')) {
            return;
        }

        try {
            const response = await fetch(`/api/servers/${serverId}/suspend`, {
                method: 'POST'
            });
            const data = await response.json();
            
            if (!data.success) {
                this.showError(data.message);
            }
        } catch (error) {
            this.showError('Failed to suspend server: ' + error.message);
        }
    }

    showMetrics(serverId, serverName) {
        this.createMetricsModal(serverId, serverName);
    }

    createMetricsModal(serverId, serverName) {
        // Remove existing modal if present
        const existingModal = document.getElementById('metrics-modal');
        if (existingModal) {
            existingModal.remove();
        }

        // Create modal HTML
        const modal = document.createElement('div');
        modal.id = 'metrics-modal';
        modal.className = 'metrics-modal';
        modal.innerHTML = `
            <div class="metrics-modal-overlay" onclick="dashboard.closeMetricsModal()"></div>
            <div class="metrics-modal-content">
                <div class="metrics-modal-header">
                    <h2>Metrics for ${this.escapeHtml(serverName)}</h2>
                    <button class="close-btn" onclick="dashboard.closeMetricsModal()">✕</button>
                </div>
                <div class="metrics-dashboard-container">
                    <div class="dashboard">
                        <div class="header">
                            <div class="title">System Metrics</div>
                            <div class="controls">
                                <div class="nav-controls">
                                    <button id="prevButton" class="nav-button">
                                        ← Prev
                                    </button>
                                    <div id="currentRange" class="current-range">Last 1 Hour</div>
                                    <button id="nextButton" class="nav-button">
                                        Next →
                                    </button>
                                </div>
                                <div class="control-group">
                                    <label>Period:</label>
                                    <select id="period">
                                        <option value="1h">Last 1 Hour</option>
                                        <option value="6h">Last 6 Hours</option>
                                        <option value="24h">Last 24 Hours</option>
                                        <option value="7d">Last 7 Days</option>
                                        <option value="30d">Last 30 Days</option>
                                        <option value="custom">Custom Range</option>
                                    </select>
                                </div>
                                <div class="control-group custom-range" style="display: none;">
                                    <label>Start Time:</label>
                                    <input type="datetime-local" id="startTime">
                                </div>
                                <div class="control-group custom-range" style="display: none;">
                                    <label>End Time:</label>
                                    <input type="datetime-local" id="endTime">
                                </div>
                            </div>
                        </div>
                        <div id="metricsGrid" class="metrics-grid"></div>
                    </div>
                </div>
            </div>
        `;

        document.body.appendChild(modal);

        // Initialize metrics dashboard for this specific server
        const container = modal.querySelector('.metrics-dashboard-container');
        
        // Wait for modal to be added to DOM, then initialize
        setTimeout(() => {
            window.metricsInstance = new MetricsDashboard(container, {
                defaultServer: serverId
            });
        }, 100);

        // Show modal
        modal.style.display = 'flex';
    }

    closeMetricsModal() {
        const modal = document.getElementById('metrics-modal');
        if (modal) {
            modal.remove();
        }
        if (window.metricsInstance) {
            window.metricsInstance = null;
        }
    }

    connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/ws`;
        
        this.websocket = new WebSocket(wsUrl);
        
        this.websocket.onopen = () => {
            this.updateConnectionStatus('connected');
        };
        
        this.websocket.onmessage = (event) => {
            const update = JSON.parse(event.data);
            this.handleServerUpdate(update);
        };
        
        this.websocket.onclose = () => {
            this.updateConnectionStatus('disconnected');
            // Attempt to reconnect after 5 seconds
            setTimeout(() => this.connectWebSocket(), 5000);
        };
        
        this.websocket.onerror = (error) => {
            console.error('WebSocket error:', error);
            this.updateConnectionStatus('disconnected');
        };
    }

    handleServerUpdate(update) {
        if (this.servers.has(update.server_id)) {
            // Update the server data
            const server = this.servers.get(update.server_id);
            server.current_state = update.state;
            server.services = update.services;
            if (update.server) {
                this.servers.set(update.server_id, update.server);
            }
            
            // Update the UI for this specific server
            this.updateServerCard(update.server_id);
            
            // Update metrics if available
            if (update.metrics) {
                this.updateServerMetrics(update.server_id, update.metrics);
            }
        }
    }

    updateServerMetrics(serverId, metrics) {
        const metricsGrid = document.getElementById(`metrics-grid-${serverId}`);
        if (!metricsGrid) return;

        // Clear existing metrics
        metricsGrid.innerHTML = '';

        // Define metric display configurations - always show these in this order
        const metricConfigs = {
            'cpu': { label: 'CPU', unit: '%', color: '#3b82f6' },
            'memory': { label: 'Memory', unit: '%', color: '#10b981' },
            'network': { label: 'Network', unit: 'MB/s', color: '#f59e0b' },
            'wattage': { label: 'Power', unit: 'W', color: '#ef4444' }
        };

        // Always display all expected metrics, even if no data available
        Object.entries(metricConfigs).forEach(([key, config]) => {
            const value = metrics && metrics[key] !== undefined ? metrics[key] : null;
            
            const metricElement = document.createElement('div');
            metricElement.className = 'metric-item';
            metricElement.innerHTML = `
                <div class="metric-label">${config.label}</div>
                <div class="metric-value" style="color: ${value !== null ? config.color : '#9ca3af'}">
                    ${value !== null ? (typeof value === 'number' ? value.toFixed(1) : value) + config.unit : '--'}
                </div>
            `;
            metricsGrid.appendChild(metricElement);
        });
    }

    updateServerCard(serverId) {
        const server = this.servers.get(serverId);
        if (!server) return;
        
        const cardElement = document.getElementById(`server-${serverId}`);
        if (!cardElement) return;
        
        // Add updating animation
        cardElement.classList.add('updating');
        setTimeout(() => cardElement.classList.remove('updating'), 1000);
        
        // Replace the entire card content
        const newCard = this.createServerCard(server);
        cardElement.innerHTML = newCard.innerHTML;
    }

    updateConnectionStatus(status) {
        this.connectionStatus.textContent = status === 'connected' ? 'Connected' : 'Disconnected';
        this.connectionStatus.className = `status-indicator ${status}`;
    }

    getParentName(parentId) {
        const parent = this.servers.get(parentId);
        return parent ? parent.name : parentId;
    }

    getCurrentUptime(server) {
        if (server.current_state !== 'on' || !server.last_state_change) {
            return 0;
        }
        const lastChange = new Date(server.last_state_change);
        const now = new Date();
        return Math.floor((now - lastChange) / 1000);
    }

    formatUptime(seconds) {
        if (seconds < 60) return `${seconds}s`;
        if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
        if (seconds < 86400) return `${Math.floor(seconds / 3600)}h`;
        return `${Math.floor(seconds / 86400)}d`;
    }

    formatDateTime(timestamp) {
        const date = new Date(timestamp);
        return date.toLocaleString();
    }

    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    showError(message) {
        // Remove any existing error messages
        const existingError = document.querySelector('.error-message');
        if (existingError) {
            existingError.remove();
        }
        
        const errorDiv = document.createElement('div');
        errorDiv.className = 'error-message';
        errorDiv.textContent = message;
        
        this.serversContainer.insertBefore(errorDiv, this.serversContainer.firstChild);
        
        // Auto-remove error after 5 seconds
        setTimeout(() => {
            errorDiv.remove();
        }, 5000);
    }
}

// Initialize the dashboard when the page loads
let dashboard;
document.addEventListener('DOMContentLoaded', () => {
    dashboard = new NetworkDashboard();
});

// Expose globally for button clicks
window.dashboard = dashboard;
