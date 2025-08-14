import axios from 'axios'

// Create axios instance
export const api = axios.create({
  baseURL: '/api',
  withCredentials: true, // Important for JWT cookies
  headers: {
    'Content-Type': 'application/json'
  }
})

// Request interceptor
api.interceptors.request.use(
  (config) => {
    // Add any request modifications here
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// Response interceptor
api.interceptors.response.use(
  (response) => {
    return response
  },
  (error) => {
    // Handle authentication errors
    if (error.response?.status === 401) {
      // Redirect to login if not already there
      if (window.location.pathname !== '/login') {
        window.location.href = '/login'
      }
    }
    
    return Promise.reject(error)
  }
)

// Metrics API
export const metricsApi = {
  async getMetrics(serverId, start, end) {
    const params = new URLSearchParams({
      server: serverId,
      start: start.toISOString(),
      end: end.toISOString()
    })
    
    const response = await api.get(`/metrics?${params}`)
    return response.data.data
  },

  async getAvailableMetrics(serverId) {
    const response = await api.get(`/metrics/${serverId}/available`)
    return response.data.data
  }
}

export default api
