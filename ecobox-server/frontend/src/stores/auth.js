import { defineStore } from 'pinia'
import axios from 'axios'
import { api } from '@/services/api'

export const useAuthStore = defineStore('auth', {
  state: () => ({
    user: null,
    initialized: false,
    loading: false,
    error: null
  }),

  getters: {
    isAuthenticated: (state) => !!state.user,
    isAdmin: (state) => state.user?.is_admin || false
  },

  actions: {
    async initializeAuth() {
      if (this.initialized) return
      
      this.loading = true
      this.error = null
      
      try {
        const response = await api.get('/auth/me')
        this.user = response.data.data
      } catch (error) {
        // User not authenticated, which is fine
        this.user = null
      } finally {
        this.initialized = true
        this.loading = false
      }
    },

    async login(username, password) {
      this.loading = true
      this.error = null
      
      try {
        // Use form data for login as per API spec
        const formData = new FormData()
        formData.append('username', username)
        formData.append('password', password)
        
        // Login endpoint is at root level, not under /api
        await axios.post('/login', formData, {
          headers: {
            'Content-Type': 'application/x-www-form-urlencoded'
          },
          withCredentials: true
        })
        
        // After successful login, get user data
        const response = await api.get('/auth/me')
        this.user = response.data.data
        
        return { success: true }
      } catch (error) {
        this.error = error.response?.data?.message || 'Login failed'
        return { success: false, message: this.error }
      } finally {
        this.loading = false
      }
    },

    async logout() {
      this.loading = true
      this.error = null
      
      try {
        await axios.post('/logout', null, {
          headers: {
            'Accept': 'application/json'
          },
          withCredentials: true
        })
      } catch (error) {
        console.error('Logout error:', error)
      } finally {
        this.user = null
        this.loading = false
      }
    },

    async changePassword(currentPassword, newPassword, confirmPassword) {
      this.loading = true
      this.error = null
      
      try {
        const response = await api.post('/auth/password', {
          current_password: currentPassword,
          new_password: newPassword,
          confirm_password: confirmPassword
        })
        
        return { success: true, message: response.data.message }
      } catch (error) {
        this.error = error.response?.data?.message || 'Password change failed'
        return { success: false, message: this.error }
      } finally {
        this.loading = false
      }
    },

    async setup(password, confirmPassword) {
      this.loading = true
      this.error = null
      
      try {
        const formData = new FormData()
        formData.append('password', password)
        formData.append('confirm_password', confirmPassword)
        
        await api.post('/setup', formData, {
          headers: {
            'Content-Type': 'application/x-www-form-urlencoded'
          }
        })
        
        return { success: true }
      } catch (error) {
        this.error = error.response?.data?.message || 'Setup failed'
        return { success: false, message: this.error }
      } finally {
        this.loading = false
      }
    },

    clearError() {
      this.error = null
    }
  }
})
