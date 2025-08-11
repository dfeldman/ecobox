# Metrics API Documentation

## Overview

The EcoBox server provides a comprehensive metrics API that collects and serves system monitoring data. The API uses standardized metric names shared between frontend and backend components.

## Endpoints

### GET /api/metrics

Returns metrics data for a specific server and time range.

**Query Parameters:**
- `server` (required): Server name/identifier 
- `start` (required): ISO 8601 timestamp (e.g., "2025-08-10T14:00:00.000Z")
- `end` (required): ISO 8601 timestamp (e.g., "2025-08-10T15:00:00.000Z")

**Example Request:**
```
GET /api/metrics?server=web-01&start=2025-08-10T14:00:00.000Z&end=2025-08-10T15:00:00.000Z
```

**Response Format:**
```json
{
  "success": true,
  "data": {
    "memory": [
      { "timestamp": "2025-08-10T14:00:00.000Z", "value": 45.2 },
      { "timestamp": "2025-08-10T14:01:00.000Z", "value": 46.8 }
    ],
    "cpu": [
      { "timestamp": "2025-08-10T14:00:00.000Z", "value": 23.5 },
      { "timestamp": "2025-08-10T14:01:00.000Z", "value": 28.1 }
    ],
    "network": [
      { "timestamp": "2025-08-10T14:00:00.000Z", "value": 12.4 },
      { "timestamp": "2025-08-10T14:01:00.000Z", "value": 15.7 }
    ],
    "wattage": [
      { "timestamp": "2025-08-10T14:00:00.000Z", "value": 185.3 },
      { "timestamp": "2025-08-10T14:01:00.000Z", "value": 192.1 }
    ]
  }
}
```

### GET /api/metrics/{server}/available

Returns all available metrics for a specific server.

**Response Format:**
```json
{
  "success": true,
  "data": {
    "frontend_metrics": ["memory", "cpu", "network", "wattage"],
    "all_metrics": [
      "memory", "cpu", "network", "wattage",
      "power_state_change", "wake_attempt", "init_success", ...
    ],
    "server": "web-01"
  }
}
```

## Standard Metrics

### Frontend Display Metrics
These are the primary metrics displayed in the dashboard:
- `memory`: RAM usage percentage (0-100)
- `cpu`: CPU utilization percentage (0-100)  
- `network`: Network throughput in MB/s (0+)
- `wattage`: Power consumption in watts (0+)

### System Monitoring Metrics
- `power_state_change`: Number of power state transitions
- `power_state_on/off/suspended/init_failed`: State-specific counters
- `service_availability_percent`: Percentage of services online
- `system_check_*`: System check results and timing

### Power Management Metrics
- `wake_attempt/success/failure`: Wake operation results
- `wake_duration_seconds`: Time taken for wake operations
- `suspend_attempt/success/failure`: Suspend operation results  
- `suspend_duration_seconds`: Time taken for suspend operations

### Initialization Metrics
- `init_attempt/success/failure`: Initialization results
- `init_duration_seconds`: Time taken for initialization
- `init_retry_count`: Number of retry attempts
- `init_state_reset`: Number of state resets

## Data Aggregation

The API automatically aggregates data based on the requested time range:
- ≤ 2 hours: 1-minute intervals
- ≤ 12 hours: 5-minute intervals  
- ≤ 48 hours: 30-minute intervals
- ≤ 1 week: 1-hour intervals
- ≤ 1 month: 4-hour intervals
- > 1 month: 1-day intervals

## Error Handling

All endpoints return HTTP status codes:
- 200: Success
- 400: Bad request (missing/invalid parameters)
- 404: Server not found
- 503: Metrics system unavailable

Error responses include details:
```json
{
  "success": false,
  "message": "Missing required parameter: server"
}
```

## Authentication

All metrics API endpoints require authentication. Include session cookies or authentication headers as configured for the server.

## Performance Notes

- API calls are optimized for 100-200 data points per response
- Data is automatically cached and compressed  
- Metrics are buffered and flushed periodically to disk
- Historical data is stored in compressed CSV files by date
