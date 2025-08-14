import { defineStore } from 'pinia'
import { api } from '@/services/api'
import { websocketService } from '@/services/websocket'

export const useServersStore = defineStore('servers', {
  state: () => ({
    servers: [],
    currentServer: null,
    loading: false,
    error: null,
    websocketConnected: false
  }),

  getters: {
    getServerById: (state) => (id) => {
      return state.servers.find(server => server.id === id)
    },

    onlineServers: (state) => {
      return state.servers.filter(server => server.current_state === 'on')
    },

    offlineServers: (state) => {
      return state.servers.filter(server => server.current_state === 'off')
    },

    suspendedServers: (state) => {
      return state.servers.filter(server => server.current_state === 'suspended')
    },

    stoppedServers: (state) => {
      return state.servers.filter(server => server.current_state === 'stopped')
    },

    transitioningServers: (state) => {
      return state.servers.filter(server => 
        ['waking', 'suspending', 'stopping'].includes(server.current_state)
      )
    },

    serversByState: (state) => {
      return state.servers.reduce((acc, server) => {
        if (!acc[server.current_state]) {
          acc[server.current_state] = []
        }
        acc[server.current_state].push(server)
        return acc
      }, {})
    }
  },

  actions: {
    async fetchServers() {
      this.loading = true
      this.error = null
      
      try {
        const response = await api.get('/servers')
        this.servers = response.data.data
      } catch (error) {
        this.error = error.response?.data?.message || 'Failed to fetch servers'
        console.error('Error fetching servers:', error)
      } finally {
        this.loading = false
      }
    },

    async fetchServer(id) {
      this.loading = true
      this.error = null
      
      try {
        const response = await api.get(`/servers/${id}`)
        this.currentServer = response.data.data
        
        // Update the server in the list if it exists
        const index = this.servers.findIndex(s => s.id === id)
        if (index !== -1) {
          this.servers[index] = response.data.data
        }
      } catch (error) {
        this.error = error.response?.data?.message || 'Failed to fetch server'
        console.error('Error fetching server:', error)
      } finally {
        this.loading = false
      }
    },

    async wakeServer(id) {
      try {
        const response = await api.post(`/servers/${id}/wake`)
        return { success: true, message: response.data.message }
      } catch (error) {
        const message = error.response?.data?.message || 'Failed to wake server'
        console.error('Error waking server:', error)
        return { success: false, message }
      }
    },

    async suspendServer(id) {
      try {
        const response = await api.post(`/servers/${id}/suspend`)
        return { success: true, message: response.data.message }
      } catch (error) {
        const message = error.response?.data?.message || 'Failed to suspend server'
        console.error('Error suspending server:', error)
        return { success: false, message }
      }
    },

    async shutdownServer(id) {
      try {
        const response = await api.post(`/servers/${id}/shutdown`)
        return { success: true, message: response.data.message }
      } catch (error) {
        const message = error.response?.data?.message || 'Failed to shutdown server'
        console.error('Error shutting down server:', error)
        return { success: false, message }
      }
    },

    async stopServer(id) {
      try {
        const response = await api.post(`/servers/${id}/stop`)
        return { success: true, message: response.data.message }
      } catch (error) {
        const message = error.response?.data?.message || 'Failed to stop server'
        console.error('Error stopping server:', error)
        return { success: false, message }
      }
    },

    // WebSocket handlers
    initializeWebSocket() {
      websocketService.connect()
      
      websocketService.onOpen(() => {
        this.websocketConnected = true
      })
      
      websocketService.onClose(() => {
        this.websocketConnected = false
      })
      
      websocketService.onMessage((data) => {
        this.handleWebSocketUpdate(data)
      })
    },

    handleWebSocketUpdate(data) {
      if (data.server_id && data.server) {
        // Update server in the list
        const index = this.servers.findIndex(s => s.id === data.server_id)
        if (index !== -1) {
          this.servers[index] = { ...this.servers[index], ...data.server }
        }
        
        // Update current server if it's the same
        if (this.currentServer && this.currentServer.id === data.server_id) {
          this.currentServer = { ...this.currentServer, ...data.server }
        }
      }
    },

    disconnectWebSocket() {
      websocketService.disconnect()
      this.websocketConnected = false
    },

    clearError() {
      this.error = null
    }
  }
})
