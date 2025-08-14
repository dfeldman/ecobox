# EcoBox Frontend

Modern Vue.js 3 frontend for the EcoBox homelab management system.

## Features

- **Vue 3 + Composition API**: Modern reactive framework
- **Pinia**: State management for authentication, servers, and users
- **Vue Router**: Client-side routing with authentication guards
- **WebSocket Integration**: Real-time server status updates
- **Responsive Design**: Mobile-friendly interface
- **Chart.js Integration**: Reuses existing metrics.js library for graphing
- **Modern UI**: Clean, accessible interface with dark mode support

## Development Setup

### Prerequisites

- Node.js 16+ and npm
- Backend server running on port 8080

### Install Dependencies

```bash
npm install
```

### Development Server

```bash
npm run dev
```

This starts the Vite development server with hot reload on `http://localhost:5173`. The dev server proxies API requests to the backend on `http://localhost:8080`.

### Build for Production

```bash
npm run build
```

Built files are output to `../web/static-vue/` for serving by the Go backend.

## Architecture

### State Management

- **Auth Store** (`stores/auth.js`): User authentication and session management
- **Servers Store** (`stores/servers.js`): Server data and WebSocket integration
- **Users Store** (`stores/users.js`): User management (admin only)

### Views

- **Login** (`views/Login.vue`): Authentication page
- **Setup** (`views/Setup.vue`): Initial admin setup
- **Dashboard** (`views/Dashboard.vue`): Main server overview
- **ServerDetail** (`views/ServerDetail.vue`): Individual server details with metrics
- **Users** (`views/Users.vue`): User management (admin only)
- **Profile** (`views/Profile.vue`): User profile and password change

### Components

- **ServerCard** (`components/ServerCard.vue`): Server overview card
- **MetricsCharts** (`components/MetricsCharts.vue`): Integrates with existing metrics.js

### Services

- **API Service** (`services/api.js`): HTTP client with authentication handling
- **WebSocket Service** (`services/websocket.js`): Real-time connection management

## Integration with Backend

The frontend integrates seamlessly with the existing Go backend:

1. **Authentication**: Uses JWT cookies for session management
2. **REST API**: All data operations via `/api/*` endpoints
3. **WebSocket**: Real-time updates via `/ws` endpoint
4. **Static Files**: Production build served from `web/static-vue/`
5. **Metrics Library**: Reuses existing `metrics.js` and `metrics.css`

## Deployment

The production build integrates with the existing Go application:

1. Build frontend: `npm run build`
2. Files are placed in `../web/static-vue/`
3. Go server serves Vue SPA from this directory
4. Vue router handles client-side navigation
5. API requests are served by Go backend

## Development Workflow

### Frontend Only

```bash
# Terminal 1: Start backend
cd ../
make run

# Terminal 2: Start frontend dev server
cd frontend
npm run dev
```

### Full Development

```bash
# Terminal 1: Start backend with hot reload
make dev

# Terminal 2: Start frontend dev server
make dev-frontend
```

The frontend dev server proxies API requests to the backend, providing hot reload for frontend changes while the backend runs separately.
