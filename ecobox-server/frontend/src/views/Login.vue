<template>
  <div class="min-h-screen bg-gray-50 flex flex-col justify-center py-12 px-6 lg:px-8">
    <div class="sm:mx-auto sm:w-full sm:max-w-md">
      <div class="text-center">
        <h1 class="text-3xl font-bold text-gray-900 mb-2">EcoBox</h1>
        <p class="text-gray-600">Homelab Management System</p>
      </div>
    </div>

    <div class="mt-8 sm:mx-auto sm:w-full sm:max-w-md">
      <div class="card">
        <div class="card-body">
          <form @submit.prevent="handleLogin" class="space-y-6">
            <div class="form-group">
              <label for="username" class="form-label">Username</label>
              <input
                id="username"
                v-model="form.username"
                type="text"
                required
                class="form-input"
                :disabled="loading"
              />
            </div>

            <div class="form-group">
              <label for="password" class="form-label">Password</label>
              <input
                id="password"
                v-model="form.password"
                type="password"
                required
                class="form-input"
                :disabled="loading"
              />
            </div>

            <div v-if="error" class="text-red text-sm text-center">
              {{ error }}
            </div>

            <div>
              <button
                type="submit"
                class="btn btn-primary w-full"
                :disabled="loading"
              >
                <span v-if="loading" class="loading mr-2"></span>
                Sign In
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { reactive, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'

export default {
  name: 'Login',
  setup() {
    const router = useRouter()
    const authStore = useAuthStore()
    
    const form = reactive({
      username: '',
      password: ''
    })
    
    const loading = computed(() => authStore.loading)
    const error = computed(() => authStore.error)
    
    const handleLogin = async () => {
      authStore.clearError()
      
      const result = await authStore.login(form.username, form.password)
      
      if (result.success) {
        router.push('/')
      }
    }
    
    return {
      form,
      loading,
      error,
      handleLogin
    }
  }
}
</script>
