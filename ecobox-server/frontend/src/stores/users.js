import { defineStore } from 'pinia'
import { api } from '@/services/api'

export const useUsersStore = defineStore('users', {
  state: () => ({
    users: [],
    loading: false,
    error: null
  }),

  actions: {
    async fetchUsers() {
      this.loading = true
      this.error = null
      
      try {
        const response = await api.get('/auth/users')
        this.users = response.data.data.users
      } catch (error) {
        this.error = error.response?.data?.message || 'Failed to fetch users'
        console.error('Error fetching users:', error)
      } finally {
        this.loading = false
      }
    },

    async createUser(username, isAdmin = false) {
      this.loading = true
      this.error = null
      
      try {
        const response = await api.post('/auth/users', {
          username,
          is_admin: isAdmin
        })
        
        // Add the new user to the list
        this.users.push(response.data.data.user)
        
        return { 
          success: true, 
          message: response.data.message,
          initialPassword: response.data.data.initial_password
        }
      } catch (error) {
        this.error = error.response?.data?.message || 'Failed to create user'
        return { success: false, message: this.error }
      } finally {
        this.loading = false
      }
    },

    async deleteUser(username) {
      this.loading = true
      this.error = null
      
      try {
        const response = await api.delete(`/auth/users/${username}`)
        
        // Remove the user from the list
        this.users = this.users.filter(user => user.username !== username)
        
        return { success: true, message: response.data.message }
      } catch (error) {
        this.error = error.response?.data?.message || 'Failed to delete user'
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
