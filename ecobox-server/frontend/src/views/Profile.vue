<template>
  <div class="min-h-screen bg-gray-50">
    <!-- Header -->
    <header class="bg-white shadow">
      <div class="container mx-auto px-6 py-4">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-4">
            <router-link to="/" class="btn btn-secondary btn-sm">
              ← Dashboard
            </router-link>
            <h1 class="text-2xl font-bold text-gray-900">Profile</h1>
          </div>
        </div>
      </div>
    </header>

    <!-- Main Content -->
    <main class="container mx-auto px-6 py-8 max-w-2xl">
      <div class="space-y-6">
        <!-- User Information -->
        <div class="card">
          <div class="card-header">
            <h3 class="font-semibold">User Information</h3>
          </div>
          <div class="card-body">
            <div v-if="user" class="space-y-4">
              <div class="flex justify-between">
                <span class="text-gray-600">Username:</span>
                <span class="font-medium">{{ user.username }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-gray-600">Account Type:</span>
                <span class="font-medium">{{ user.is_admin ? 'Administrator' : 'User' }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-gray-600">Created:</span>
                <span class="font-medium">{{ formatDate(user.created_at) }}</span>
              </div>
              <div class="flex justify-between">
                <span class="text-gray-600">Last Login:</span>
                <span class="font-medium">{{ formatDate(user.last_login) }}</span>
              </div>
            </div>
          </div>
        </div>

        <!-- Change Password -->
        <div class="card">
          <div class="card-header">
            <h3 class="font-semibold">Change Password</h3>
          </div>
          <div class="card-body">
            <form @submit.prevent="handleChangePassword" class="space-y-4">
              <div class="form-group">
                <label class="form-label">Current Password</label>
                <input
                  v-model="passwordForm.currentPassword"
                  type="password"
                  required
                  class="form-input"
                  :disabled="loading"
                />
              </div>
              <div class="form-group">
                <label class="form-label">New Password</label>
                <input
                  v-model="passwordForm.newPassword"
                  type="password"
                  required
                  minlength="6"
                  class="form-input"
                  :disabled="loading"
                />
              </div>
              <div class="form-group">
                <label class="form-label">Confirm New Password</label>
                <input
                  v-model="passwordForm.confirmPassword"
                  type="password"
                  required
                  minlength="6"
                  class="form-input"
                  :disabled="loading"
                />
              </div>
              
              <div v-if="error" class="text-red text-sm">
                {{ error }}
              </div>
              
              <div v-if="passwordMismatch" class="text-red text-sm">
                New passwords do not match
              </div>
              
              <div v-if="successMessage" class="text-green text-sm">
                {{ successMessage }}
              </div>
              
              <div class="flex gap-2">
                <button
                  type="submit"
                  class="btn btn-primary"
                  :disabled="loading || passwordMismatch"
                >
                  <span v-if="loading" class="loading w-3 h-3 mr-1"></span>
                  Change Password
                </button>
                <button
                  type="button"
                  @click="resetForm"
                  class="btn btn-secondary"
                  :disabled="loading"
                >
                  Cancel
                </button>
              </div>
            </form>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<script>
import { reactive, computed, ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { format } from 'date-fns'

export default {
  name: 'Profile',
  setup() {
    const authStore = useAuthStore()
    const successMessage = ref('')
    
    const passwordForm = reactive({
      currentPassword: '',
      newPassword: '',
      confirmPassword: ''
    })
    
    const user = computed(() => authStore.user)
    const loading = computed(() => authStore.loading)
    const error = computed(() => authStore.error)
    
    const passwordMismatch = computed(() => {
      return passwordForm.newPassword && 
             passwordForm.confirmPassword && 
             passwordForm.newPassword !== passwordForm.confirmPassword
    })
    
    const formatDate = (dateString) => {
      if (!dateString || dateString === '0001-01-01T00:00:00Z') {
        return 'Never'
      }
      try {
        return format(new Date(dateString), 'MMM dd, yyyy HH:mm')
      } catch {
        return 'Invalid date'
      }
    }
    
    const resetForm = () => {
      passwordForm.currentPassword = ''
      passwordForm.newPassword = ''
      passwordForm.confirmPassword = ''
      authStore.clearError()
      successMessage.value = ''
    }
    
    const handleChangePassword = async () => {
      if (passwordMismatch.value) return
      
      authStore.clearError()
      successMessage.value = ''
      
      const result = await authStore.changePassword(
        passwordForm.currentPassword,
        passwordForm.newPassword,
        passwordForm.confirmPassword
      )
      
      if (result.success) {
        successMessage.value = result.message
        resetForm()
      }
    }
    
    return {
      user,
      loading,
      error,
      passwordForm,
      passwordMismatch,
      successMessage,
      formatDate,
      resetForm,
      handleChangePassword
    }
  }
}
</script>
