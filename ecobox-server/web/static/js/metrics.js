/**
 * METRICS API DOCUMENTATION
 * =========================
 * 
 * The dashboard expects a REST API endpoint that provides system metrics data.
 * Replace the MockMetricsAPI with actual HTTP calls to your server.
 * 
 * ENDPOINT: GET /api/metrics
 * 
 * QUERY PARAMETERS:
 * - server: Server identifier/name (e.g., "web-01", "db-primary", "cache-redis-01")
 * - start: ISO 8601 timestamp (e.g., "2025-08-10T14:00:00.000Z")
 * - end: ISO 8601 timestamp (e.g., "2025-08-10T15:00:00.000Z")
 * 
 * EXAMPLE REQUEST:
 * GET /api/metrics?server=web-01&start=2025-08-10T14:00:00.000Z&end=2025-08-10T15:00:00.000Z
 * 
 * EXPECTED RESPONSE FORMAT:
 * {
 *   "memory": [
 *     { "timestamp": "2025-08-10T14:00:00.000Z", "value": 45.2 },
 *     { "timestamp": "2025-08-10T14:01:00.000Z", "value": 46.8 },
 *     ...
 *   ],
 *   "cpu": [
 *     { "timestamp": "2025-08-10T14:00:00.000Z", "value": 23.5 },
 *     { "timestamp": "2025-08-10T14:01:00.000Z", "value": 28.1 },
 *     ...
 *   ],
 *   "network": [
 *     { "timestamp": "2025-08-10T14:00:00.000Z", "value": 12.4 },
 *     { "timestamp": "2025-08-10T14:01:00.000Z", "value": 15.7 },
 *     ...
 *   ],
 *   "wattage": [
 *     { "timestamp": "2025-08-10T14:00:00.000Z", "value": 185.3 },
 *     { "timestamp": "2025-08-10T14:01:00.000Z", "value": 192.1 },
 *     ...
 *   ]
 * }
 * 
 * DATA REQUIREMENTS:
 * - timestamp: ISO 8601 string, will be converted to Date object
 * - value: Number, representing the metric value at that timestamp
 * - Recommended: 100-200 data points per time range for optimal performance
 * - Data should be sorted by timestamp (ascending)
 * 
 * METRIC SPECIFICATIONS:
 * - memory: Percentage (0-100), represents RAM usage
 * - cpu: Percentage (0-100), represents CPU utilization
 * - network: MB/s (0+), represents network throughput
 * - wattage: Watts (0+), represents power consumption
 * 
 * ERROR HANDLING:
 * - Return HTTP 200 with data on success
 * - Return HTTP 4xx/5xx with error message on failure
 * - Dashboard will show retry button on errors
 * - Network timeouts are handled gracefully
 * 
 * PERFORMANCE CONSIDERATIONS:
 * - API calls are debounced during rapid navigation
 * - Implement server-side caching for recent data
 * - Consider data compression for large time ranges
 * - Aggregate data points for very long time ranges (>1 month)
 * 
 * ADDITIONAL ENDPOINTS:
 * 
 * GET /api/servers - Returns list of available servers
 * RESPONSE: 
 * {
 *   "servers": [
 *     { "id": "web-01", "name": "Web Server 01", "status": "online" },
 *     { "id": "web-02", "name": "Web Server 02", "status": "online" },
 *     { "id": "db-primary", "name": "Database Primary", "status": "online" },
 *     { "id": "db-replica", "name": "Database Replica", "status": "offline" },
 *     { "id": "cache-redis-01", "name": "Redis Cache 01", "status": "online" }
 *   ]
 * }
 * 
 * SERVER STATUS:
 * - "online": Server is active and metrics are available
 * - "offline": Server is down, metrics may be stale
 * - "maintenance": Server is in maintenance mode
 * 
 * IMPLEMENTATION EXAMPLE:
 * 
 * // Replace MockMetricsAPI.fetchMetrics with:
 * static async fetchMetrics(serverName, startTime, endTime) {
 *   const params = new URLSearchParams({
 *     server: serverName,
 *     start: startTime.toISOString(),
 *     end: endTime.toISOString()
 *   });
 *   
 *   const response = await fetch(`/api/metrics?${params}`);
 *   if (!response.ok) {
 *     throw new Error(`HTTP ${response.status}: ${response.statusText}`);
 *   }
 *   
 *   const data = await response.json();
 *   
 *   // Convert timestamp strings to Date objects
 *   Object.keys(data).forEach(metric => {
 *     data[metric].forEach(point => {
 *       point.timestamp = new Date(point.timestamp);
 *     });
 *   });
 *   
 *   return data;
 * }
 * 
 * // Add server list fetching:
 * static async fetchServers() {
 *   const response = await fetch('/api/servers');
 *   if (!response.ok) {
 *     throw new Error(`HTTP ${response.status}: ${response.statusText}`);
 *   }
 *   return response.json();
 * }
 */

// Mock API for metrics data
class MockMetricsAPI {
    static mockServers = [
        { id: 'web-01', name: 'Web Server 01', status: 'online' },
        { id: 'web-02', name: 'Web Server 02', status: 'online' },
        { id: 'db-primary', name: 'Database Primary', status: 'online' },
        { id: 'db-replica', name: 'Database Replica', status: 'offline' },
        { id: 'cache-redis-01', name: 'Redis Cache 01', status: 'online' },
        { id: 'api-gateway', name: 'API Gateway', status: 'maintenance' },
        { id: 'worker-01', name: 'Background Worker 01', status: 'online' }
    ];

    static async fetchServers() {
        // Simulate API delay
        await new Promise(resolve => setTimeout(resolve, 200));
        return { servers: this.mockServers };
    }
    
    static generateDataPoints(start, end, interval) {
        const points = [];
        const current = new Date(start);
        const endTime = new Date(end);
        
        while (current <= endTime) {
            points.push(new Date(current));
            current.setTime(current.getTime() + interval);
        }
        
        return points;
    }
    
    static generateMetricData(metricType, timePoints, serverType) {
        const baseValues = {
            'memory': { base: 40, variance: 30, max: 100 },
            'cpu': { base: 25, variance: 40, max: 100 },
            'network': { base: 50, variance: 45, max: 1000 },
            'wattage': { base: 150, variance: 100, max: 500 }
        };
        
        // Adjust base values based on server type
        const serverModifiers = {
            'db': { memory: 1.5, cpu: 1.3, network: 0.7, wattage: 1.2 },
            'web': { memory: 0.8, cpu: 1.1, network: 1.4, wattage: 0.9 },
            'cache': { memory: 2.0, cpu: 0.6, network: 1.8, wattage: 0.8 },
            'api': { memory: 1.2, cpu: 1.5, network: 1.2, wattage: 1.0 },
            'worker': { memory: 1.1, cpu: 2.0, network: 0.5, wattage: 1.1 }
        };
        
        const modifier = serverModifiers[serverType] || { memory: 1, cpu: 1, network: 1, wattage: 1 };
        const config = baseValues[metricType];
        const adjustedBase = config.base * modifier[metricType];
        
        const data = [];
        let trend = 0;
        
        timePoints.forEach((time, index) => {
            // Add some trending behavior
            trend += (Math.random() - 0.5) * 2;
            trend = Math.max(-10, Math.min(10, trend));
            
            // Generate value with trend and noise
            let value = adjustedBase + trend + 
                        (Math.sin(index * 0.1) * config.variance * 0.3) +
                        (Math.random() - 0.5) * config.variance;
            
            value = Math.max(0, Math.min(config.max, value));
            
            data.push({
                timestamp: time,
                value: Math.round(value * 100) / 100
            });
        });
        
        return data;
    }
    
    static async fetchMetrics(serverName, startTime, endTime) {
        // Simulate API delay
        await new Promise(resolve => setTimeout(resolve, 300));
        
        const duration = endTime - startTime;
        const interval = Math.max(60000, duration / 200); // At most 200 points
        
        const timePoints = this.generateDataPoints(startTime, endTime, interval);
        
        return {
            memory: this.generateMetricData('memory', timePoints),
            cpu: this.generateMetricData('cpu', timePoints),
            network: this.generateMetricData('network', timePoints),
            wattage: this.generateMetricData('wattage', timePoints)
        };
    }
}

// Metrics Dashboard Library
class MetricsDashboard {
    constructor(container, options = {}) {
        this.container = container;
        this.options = {
            defaultServer: null,
            serverListUrl: '/api/servers',
            metricsUrl: '/api/metrics',
            ...options
        };
        this.metrics = [
            { key: 'memory', title: 'Memory Usage', unit: '%', color: '#3b82f6' },
            { key: 'cpu', title: 'CPU Usage', unit: '%', color: '#10b981' },
            { key: 'network', title: 'Network Usage', unit: 'MB/s', color: '#f59e0b' },
            { key: 'wattage', title: 'Power Consumption', unit: 'W', color: '#ef4444' }
        ];
        this.currentData = {};
        this.charts = {};
        this.currentOffset = 0; // Offset from current time for navigation
        this.resizeTimeout = null;
        this.isLoading = false;
        this.servers = [];
        this.currentServer = null;
        
        this.init();
        this.setupEventListeners();
        this.setupResizeHandler();
        this.loadServers();
    }
    
    init() {
        this.createMetricCards();
    }
    
    createMetricCards() {
        const grid = document.getElementById('metricsGrid');
        grid.innerHTML = '';
        
        this.metrics.forEach(metric => {
            const card = document.createElement('div');
            card.className = 'metric-card';
            card.style.setProperty('--metric-color', metric.color);
            
            card.innerHTML = `
                <div class="metric-header">
                    <div class="metric-title">${metric.title}</div>
                    <div class="metric-value" id="${metric.key}-value">--</div>
                </div>
                <div class="chart-container" id="${metric.key}-chart">
                    <div class="loading">
                        <div class="spinner"></div>
                        Loading data...
                    </div>
                </div>
                <div class="tooltip" id="${metric.key}-tooltip">
                    <div class="tooltip-time"></div>
                    <div class="tooltip-value"></div>
                </div>
            `;
            
            grid.appendChild(card);
        });
    }
    
    async loadServers() {
        try {
            const response = await fetch('/api/servers');
            const data = await response.json();
            
            if (!data.success) {
                throw new Error(data.message || 'Failed to load servers');
            }
            
            // Convert server data to metrics format
            this.servers = data.data.map(server => ({
                id: server.id,
                name: server.name,
                status: server.current_state === 'on' ? 'online' : 'offline'
            }));
            
            // Auto-select default server
            const defaultServer = this.options.defaultServer;
            if (defaultServer) {
                this.currentServer = defaultServer;
                this.updateTitle();
                this.loadData();
            } else {
                // If no default server provided, show error
                this.showServerError();
            }
        } catch (error) {
            console.error('Failed to load servers:', error);
            this.showServerError();
        }
    }
    
    updateTitle() {
        const titleElement = document.querySelector('.title');
        const server = this.servers.find(s => s.id === this.currentServer);
        if (server) {
            titleElement.textContent = `${server.name} - System Metrics`;
        } else {
            titleElement.textContent = 'System Metrics Dashboard';
        }
    }
    
    showServerError() {
        // Show error in the title or create an error message
        const titleElement = document.querySelector('.title');
        if (titleElement) {
            titleElement.textContent = 'System Metrics - Error Loading Server Data';
            titleElement.style.color = '#ef4444';
        }
        console.error('Failed to load server data for metrics dashboard');
    }
    
    setupEventListeners() {
        const periodSelect = document.getElementById('period');
        const startTimeInput = document.getElementById('startTime');
        const endTimeInput = document.getElementById('endTime');
        const customRangeElements = document.querySelectorAll('.custom-range');
        const prevButton = document.getElementById('prevButton');
        const nextButton = document.getElementById('nextButton');
        const navControls = document.querySelector('.nav-controls');
        
        periodSelect.addEventListener('change', () => {
            const isCustom = periodSelect.value === 'custom';
            customRangeElements.forEach(el => {
                el.style.display = isCustom ? 'flex' : 'none';
            });
            navControls.style.display = isCustom ? 'none' : 'flex';
            
            this.currentOffset = 0; // Reset offset when changing period
            
            if (!isCustom && this.currentServer) {
                this.loadData();
            }
        });
        
        startTimeInput.addEventListener('change', () => {
            if (this.currentServer) this.loadData();
        });
        endTimeInput.addEventListener('change', () => {
            if (this.currentServer) this.loadData();
        });
        
        prevButton.addEventListener('click', () => {
            if (this.isLoading || !this.currentServer) return;
            this.currentOffset++;
            this.loadData();
        });
        
        nextButton.addEventListener('click', () => {
            if (this.isLoading || !this.currentServer) return;
            this.currentOffset--;
            this.loadData();
        });
        
        // Set default custom range
        const now = new Date();
        const oneHourAgo = new Date(now.getTime() - 60 * 60 * 1000);
        
        // Use datetime-local format (YYYY-MM-DDTHH:mm) for input[type="datetime-local"]
        endTimeInput.value = now.toISOString().slice(0, 16);
        startTimeInput.value = oneHourAgo.toISOString().slice(0, 16);
    }
    
    setupResizeHandler() {
        window.addEventListener('resize', () => {
            // Debounce resize events to avoid excessive redraws
            clearTimeout(this.resizeTimeout);
            this.resizeTimeout = setTimeout(() => {
                if (Object.keys(this.currentData).length > 0) {
                    this.renderCharts();
                }
            }, 150);
        });
    }
    
    getTimeRange() {
        const period = document.getElementById('period').value;
        const now = new Date();
        
        if (period === 'custom') {
            const startInput = document.getElementById('startTime');
            const endInput = document.getElementById('endTime');
            
            // Check if inputs exist and have values
            if (!startInput || !endInput || !startInput.value || !endInput.value) {
                console.warn('Custom time inputs not available, falling back to 1h period');
                const duration = 60 * 60 * 1000; // 1 hour
                return {
                    start: new Date(now.getTime() - duration),
                    end: new Date(now.getTime())
                };
            }
            
            const start = new Date(startInput.value);
            const end = new Date(endInput.value);
            
            // Validate the dates
            if (isNaN(start.getTime()) || isNaN(end.getTime())) {
                console.warn('Invalid custom time values, falling back to 1h period');
                const duration = 60 * 60 * 1000; // 1 hour
                return {
                    start: new Date(now.getTime() - duration),
                    end: new Date(now.getTime())
                };
            }
            
            return { start, end };
        }
        
        const periods = {
            '1h': 60 * 60 * 1000,
            '6h': 6 * 60 * 60 * 1000,
            '24h': 24 * 60 * 60 * 1000,
            '7d': 7 * 24 * 60 * 60 * 1000,
            '30d': 30 * 24 * 60 * 60 * 1000
        };
        
        const duration = periods[period] || periods['1h']; // Default to 1h if period not found
        const offsetDuration = this.currentOffset * duration;
        
        const start = new Date(now.getTime() - duration - offsetDuration);
        const end = new Date(now.getTime() - offsetDuration);
        
        // Validate the computed dates
        if (isNaN(start.getTime()) || isNaN(end.getTime())) {
            console.warn('Invalid computed time range, using defaults');
            return {
                start: new Date(now.getTime() - periods['1h']),
                end: new Date(now.getTime())
            };
        }
        
        return { start, end };
    }
    
    async loadData() {
        if (this.isLoading || !this.currentServer) return;
        
        this.isLoading = true;
        this.setLoadingState(true);
        
        const { start, end } = this.getTimeRange();
        
        // Update current range display and navigation buttons
        this.updateNavigationUI(start, end);
        
        try {
            // Call real API endpoint
            const params = new URLSearchParams({
                server: this.currentServer,
                start: start.toISOString(),
                end: end.toISOString()
            });
            
            const response = await fetch(`/api/metrics?${params}`);
            if (!response.ok) {
                if (response.status === 404) {
                    throw new Error(`Server "${this.currentServer}" not found or metrics not available`);
                } else if (response.status >= 500) {
                    throw new Error(`Server error: ${response.status} ${response.statusText}`);
                } else {
                    throw new Error(`HTTP ${response.status}: ${response.statusText}`);
                }
            }
            
            const data = await response.json();
            if (!data.success) {
                throw new Error(data.message || 'API returned error');
            }
            
            // Convert timestamp strings to Date objects
            const metricsData = data.data;
            Object.keys(metricsData).forEach(metric => {
                metricsData[metric].forEach(point => {
                    point.timestamp = new Date(point.timestamp);
                });
            });
            
            this.currentData = metricsData;
            this.renderCharts();
            this.clearErrorStates();
        } catch (error) {
            console.error('Failed to load metrics data:', error);
            this.showErrorStates(error.message);
        } finally {
            this.isLoading = false;
            this.setLoadingState(false);
        }
    }
    
    setLoadingState(loading) {
        const prevButton = document.getElementById('prevButton');
        const nextButton = document.getElementById('nextButton');
        
        if (loading) {
            prevButton.classList.add('loading');
            nextButton.classList.add('loading');
            
            // Add loading overlays to existing charts
            this.metrics.forEach(metric => {
                const container = document.getElementById(`${metric.key}-chart`);
                if (container && !container.querySelector('.loading') && !container.querySelector('.chart-loading-overlay')) {
                    const overlay = document.createElement('div');
                    overlay.className = 'chart-loading-overlay';
                    overlay.innerHTML = '<div class="spinner"></div>Loading data...';
                    container.appendChild(overlay);
                }
            });
        } else {
            prevButton.classList.remove('loading');
            nextButton.classList.remove('loading');
            
            // Remove loading overlays
            document.querySelectorAll('.chart-loading-overlay').forEach(overlay => {
                overlay.remove();
            });
        }
    }
    
    showErrorStates(errorMessage = 'Failed to load data') {
        this.metrics.forEach(metric => {
            const container = document.getElementById(`${metric.key}-chart`);
            const valueElement = document.getElementById(`${metric.key}-value`);
            
            container.innerHTML = `
                <div class="error-state">
                    <div>${errorMessage}</div>
                    <button onclick="window.dashboard.loadData()">Retry</button>
                </div>
            `;
            valueElement.textContent = '--';
        });
    }
    
    clearErrorStates() {
        // Error states are cleared when charts are re-rendered successfully
    }
    
    updateNavigationUI(start, end) {
        const period = document.getElementById('period').value;
        const currentRangeElement = document.getElementById('currentRange');
        const nextButton = document.getElementById('nextButton');
        
        if (period === 'custom') return;
        
        // Update current range display
        const formatOptions = {
            month: 'short',
            day: 'numeric',
            hour: '2-digit',
            minute: '2-digit'
        };
        
        let rangeText;
        if (this.currentOffset === 0) {
            const periodLabels = {
                '1h': 'Last 1 Hour',
                '6h': 'Last 6 Hours', 
                '24h': 'Last 24 Hours',
                '7d': 'Last 7 Days',
                '30d': 'Last 30 Days'
            };
            rangeText = periodLabels[period];
        } else {
            const duration = end - start;
            if (duration >= 24 * 60 * 60 * 1000) {
                // For day+ ranges, just show dates
                rangeText = `${start.toLocaleDateString()} - ${end.toLocaleDateString()}`;
            } else {
                // For hour ranges, show date + time
                rangeText = `${start.toLocaleDateString()} ${start.toLocaleTimeString('en-US', {hour: '2-digit', minute: '2-digit'})} - ${end.toLocaleTimeString('en-US', {hour: '2-digit', minute: '2-digit'})}`;
            }
        }
        currentRangeElement.textContent = rangeText;
        
        // Disable next button if we're at current time
        nextButton.disabled = this.currentOffset <= 0;
    }
    
    renderCharts() {
        this.metrics.forEach(metric => {
            this.renderChart(metric);
        });
    }
    
    renderChart(metric) {
        const data = this.currentData[metric.key] || [];
        const container = document.getElementById(`${metric.key}-chart`);
        const valueElement = document.getElementById(`${metric.key}-value`);
        
        // Update current value
        if (data.length > 0) {
            const latestValue = data[data.length - 1].value;
            valueElement.textContent = `${latestValue.toFixed(1)} ${metric.unit}`;
        } else {
            valueElement.textContent = `0.0 ${metric.unit}`;
        }
        
        container.innerHTML = '';
        
        const margin = { top: 20, right: 20, bottom: 40, left: 50 };
        const width = container.offsetWidth - margin.left - margin.right;
        const height = 300 - margin.top - margin.bottom;
        
        const svg = d3.select(container)
            .append('svg')
            .attr('class', 'chart-svg')
            .attr('width', width + margin.left + margin.right)
            .attr('height', height + margin.top + margin.bottom);
        
        const g = svg.append('g')
            .attr('transform', `translate(${margin.left},${margin.top})`);
        
        // Always use the full requested time range, regardless of actual data coverage
        const { start, end } = this.getTimeRange();
        let chartData, yMax;
        
        if (data.length === 0) {
            // Create a baseline chart with zero values for empty data
            const midTime = new Date((start.getTime() + end.getTime()) / 2);
            
            // Create three data points at 0 for start, middle, and end of time range
            chartData = [
                { timestamp: start, value: 0 },
                { timestamp: midTime, value: 0 },
                { timestamp: end, value: 0 }
            ];
            yMax = 10; // Small default range for better visual
        } else {
            // Use actual data but ensure we cover the full time range
            chartData = [...data];
            
            // Add boundary points at 0 if data doesn't cover the full range
            const dataStart = d3.min(data, d => d.timestamp);
            const dataEnd = d3.max(data, d => d.timestamp);
            
            if (dataStart > start) {
                chartData.unshift({ timestamp: start, value: 0 });
            }
            if (dataEnd < end) {
                chartData.push({ timestamp: end, value: 0 });
            }
            
            // Sort by timestamp to ensure proper ordering
            chartData.sort((a, b) => a.timestamp - b.timestamp);
            
            yMax = d3.max(data, d => d.value) * 1.1 || 10;
        }
        
        // Always use the requested time domain, not just the data extent
        const xDomain = [start, end];
        
        // Scales
        const xScale = d3.scaleTime()
            .domain(xDomain)
            .range([0, width]);
        
        const yScale = d3.scaleLinear()
            .domain([0, yMax])
            .range([height, 0]);
        
        // Grid lines
        const yTicks = yScale.ticks(5);
        g.selectAll('.grid-line-y')
            .data(yTicks)
            .enter()
            .append('line')
            .attr('class', 'grid-line')
            .attr('x1', 0)
            .attr('x2', width)
            .attr('y1', d => yScale(d))
            .attr('y2', d => yScale(d));
        
        // Area
        const area = d3.area()
            .x(d => xScale(d.timestamp))
            .y0(height)
            .y1(d => yScale(d.value))
            .curve(d3.curveMonotoneX);
        
        g.append('path')
            .datum(chartData)
            .attr('class', 'area')
            .attr('d', area)
            .style('fill', data.length === 0 ? 'rgba(128,128,128,0.1)' : metric.color);
        
        // Line
        const line = d3.line()
            .x(d => xScale(d.timestamp))
            .y(d => yScale(d.value))
            .curve(d3.curveMonotoneX);
        
        g.append('path')
            .datum(chartData)
            .attr('class', 'line')
            .attr('d', line)
            .style('stroke', data.length === 0 ? '#888' : metric.color);
        
        // Axes with dynamic time formatting
        const timeRange = xScale.domain();
        const duration = timeRange[1] - timeRange[0];
        
        let timeFormat, tickCount;
        if (duration <= 2 * 60 * 60 * 1000) { // <= 2 hours
            timeFormat = d3.timeFormat('%H:%M');
            tickCount = 6;
        } else if (duration <= 24 * 60 * 60 * 1000) { // <= 1 day
            timeFormat = d3.timeFormat('%H:%M');
            tickCount = 8;
        } else if (duration <= 7 * 24 * 60 * 60 * 1000) { // <= 1 week
            timeFormat = (d) => {
                const hours = d.getHours();
                const minutes = d.getMinutes();
                if (hours === 0 && minutes === 0) {
                    return d3.timeFormat('%m/%d')(d);
                }
                return d3.timeFormat('%m/%d %H:%M')(d);
            };
            tickCount = 7;
        } else if (duration <= 30 * 24 * 60 * 60 * 1000) { // <= 1 month
            timeFormat = d3.timeFormat('%m/%d');
            tickCount = 8;
        } else { // > 1 month
            timeFormat = d3.timeFormat('%m/%d/%y');
            tickCount = 6;
        }
        
        const xAxis = d3.axisBottom(xScale)
            .tickFormat(timeFormat)
            .ticks(tickCount);
        
        g.append('g')
            .attr('class', 'axis')
            .attr('transform', `translate(0,${height})`)
            .call(xAxis);
        
        const yAxis = d3.axisLeft(yScale)
            .tickFormat(d => `${d}${metric.unit}`)
            .ticks(5);
        
        g.append('g')
            .attr('class', 'axis')
            .call(yAxis);
        
        // Interactive elements (only if we have actual data)
        if (data.length > 0) {
            this.addInteractivity(g, chartData, xScale, yScale, metric, width, height);
        }
    }
    
    addInteractivity(g, data, xScale, yScale, metric, width, height) {
        const tooltip = document.getElementById(`${metric.key}-tooltip`);
        
        // Hover line and dot
        const hoverLine = g.append('line')
            .attr('class', 'hover-line')
            .attr('y1', 0)
            .attr('y2', height);
        
        const hoverDot = g.append('circle')
            .attr('class', 'hover-dot')
            .style('fill', metric.color);
        
        // Invisible overlay for mouse events
        const overlay = g.append('rect')
            .attr('width', width)
            .attr('height', height)
            .style('fill', 'none')
            .style('pointer-events', 'all');
        
        const bisect = d3.bisector(d => d.timestamp).left;
        
        overlay.on('mousemove', (event) => {
            const [mouseX] = d3.pointer(event);
            const x0 = xScale.invert(mouseX);
            const i = bisect(data, x0, 1);
            const d0 = data[i - 1];
            const d1 = data[i];
            
            if (!d0 && !d1) return;
            
            const d = !d1 || (d0 && (x0 - d0.timestamp < d1.timestamp - x0)) ? d0 : d1;
            
            const x = xScale(d.timestamp);
            const y = yScale(d.value);
            
            hoverLine
                .attr('x1', x)
                .attr('x2', x)
                .style('opacity', 1);
            
            hoverDot
                .attr('cx', x)
                .attr('cy', y)
                .style('opacity', 1);
            
            // Position tooltip
            const containerRect = g.node().getBoundingClientRect();
            const tooltipX = mouseX + 60;
            const tooltipY = y - 10;
            
            tooltip.style.left = `${tooltipX}px`;
            tooltip.style.top = `${tooltipY}px`;
            tooltip.querySelector('.tooltip-time').textContent = 
                d.timestamp.toLocaleString();
            tooltip.querySelector('.tooltip-value').textContent = 
                `${d.value.toFixed(2)} ${metric.unit}`;
            tooltip.classList.add('show');
            tooltip.style.setProperty('--metric-color', metric.color);
        })
        .on('mouseout', () => {
            hoverLine.style('opacity', 0);
            hoverDot.style('opacity', 0);
            tooltip.classList.remove('show');
        });
    }

    navigateTime(direction) {
        if (this.isLoading || !this.currentServer) return;
        this.currentOffset += direction;
        this.loadData();
    }

    timeRangeChanged() {
        const periodSelect = document.getElementById('period');
        const isCustom = periodSelect.value === 'custom';
        const customRangeElements = document.querySelectorAll('.custom-range');
        const navControls = document.querySelector('.nav-controls');
        
        customRangeElements.forEach(el => {
            el.style.display = isCustom ? 'flex' : 'none';
        });
        
        if (navControls) {
            navControls.style.display = isCustom ? 'none' : 'flex';
        }
        
        this.currentOffset = 0; // Reset offset when changing period
        
        if (!isCustom && this.currentServer) {
            this.loadData();
        }
    }
}

// Initialize dashboard
// document.addEventListener('DOMContentLoaded', () => {
//     // Example initialization with options:
//     window.dashboard = new MetricsDashboard(document.body, {
//         defaultServer: 'web-01', // Optional: auto-select this server
//         // serverListUrl: '/api/servers', // Custom server list endpoint
//         // metricsUrl: '/api/metrics' // Custom metrics endpoint
//     });
// });
