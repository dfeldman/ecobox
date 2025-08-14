class WebSocketService {
  constructor() {
    this.ws = null
    this.reconnectAttempts = 0
    this.maxReconnectAttempts = 5
    this.reconnectDelay = 1000 // Start with 1 second
    this.maxReconnectDelay = 30000 // Max 30 seconds
    this.isManualClose = false
    this.listeners = {
      open: [],
      close: [],
      error: [],
      message: []
    }
  }

  connect() {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      return
    }

    this.isManualClose = false
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${protocol}//${window.location.host}/ws`

    try {
      this.ws = new WebSocket(wsUrl)
      
      this.ws.onopen = (event) => {
        console.log('WebSocket connected')
        this.reconnectAttempts = 0
        this.reconnectDelay = 1000
        this.listeners.open.forEach(callback => callback(event))
      }

      this.ws.onclose = (event) => {
        console.log('WebSocket disconnected', event.code, event.reason)
        this.listeners.close.forEach(callback => callback(event))
        
        // Only reconnect if it's not a manual close and not an auth error
        if (!this.isManualClose && this.reconnectAttempts < this.maxReconnectAttempts) {
          // If close code is 1006 (abnormal closure) or 1001 (going away), try to reconnect
          // Don't reconnect on 1000 (normal closure) or 1002 (protocol error) or auth errors
          if (event.code !== 1000 && event.code !== 1002) {
            this.scheduleReconnect()
          }
        }
      }

      this.ws.onerror = (error) => {
        console.error('WebSocket error:', error)
        this.listeners.error.forEach(callback => callback(error))
      }

      this.ws.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data)
          this.listeners.message.forEach(callback => callback(data))
        } catch (error) {
          console.error('Error parsing WebSocket message:', error)
        }
      }

    } catch (error) {
      console.error('Error creating WebSocket:', error)
      this.scheduleReconnect()
    }
  }

  scheduleReconnect() {
    this.reconnectAttempts++
    console.log(`Scheduling reconnect attempt ${this.reconnectAttempts} in ${this.reconnectDelay}ms`)
    
    setTimeout(() => {
      if (!this.isManualClose) {
        this.connect()
      }
    }, this.reconnectDelay)
    
    // Exponential backoff with max delay
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay)
  }

  disconnect() {
    this.isManualClose = true
    if (this.ws) {
      this.ws.close()
      this.ws = null
    }
  }

  send(data) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data))
    } else {
      console.warn('WebSocket is not open, cannot send message')
    }
  }

  // Event listeners
  onOpen(callback) {
    this.listeners.open.push(callback)
  }

  onClose(callback) {
    this.listeners.close.push(callback)
  }

  onError(callback) {
    this.listeners.error.push(callback)
  }

  onMessage(callback) {
    this.listeners.message.push(callback)
  }

  // Remove listeners
  removeListener(event, callback) {
    const index = this.listeners[event].indexOf(callback)
    if (index > -1) {
      this.listeners[event].splice(index, 1)
    }
  }

  get isConnected() {
    return this.ws && this.ws.readyState === WebSocket.OPEN
  }
}

export const websocketService = new WebSocketService()
export default websocketService
