<template>
  <div class="min-h-screen bg-gray-50 flex flex-col justify-center py-12 px-6 lg:px-8">
    <div class="sm:mx-auto sm:w-full sm:max-w-md">
      <div class="text-center">
        <h1 class="text-3xl font-bold text-gray-900 mb-2">EcoBox Setup</h1>
        <p class="text-gray-600">Create your admin account</p>
      </div>
    </div>

    <div class="mt-8 sm:mx-auto sm:w-full sm:max-w-md">
      <div class="card">
        <div class="card-body">
          <form @submit.prevent="handleSetup" class="space-y-6">
            <div class="form-group">
              <label for="password" class="form-label">Admin Password</label>
              <input
                id="password"
                v-model="form.password"
                type="password"
                required
                class="form-input"
                :disabled="loading"
                minlength="6"
              />
            </div>

            <div class="form-group">
              <label for="confirmPassword" class="form-label">Confirm Password</label>
              <input
                id="confirmPassword"
                v-model="form.confirmPassword"
                type="password"
                required
                class="form-input"
                :disabled="loading"
                minlength="6"
              />
            </div>

            <div v-if="error" class="text-red text-sm text-center">
              {{ error }}
            </div>

            <div v-if="passwordMismatch" class="text-red text-sm text-center">
              Passwords do not match
            </div>

            <div>
              <button
                type="submit"
                class="btn btn-primary w-full"
                :disabled="loading || passwordMismatch"
              >
                <span v-if="loading" class="loading mr-2"></span>
                Complete Setup
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
  name: 'Setup',
  setup() {
    const router = useRouter()
    const authStore = useAuthStore()
    
    const form = reactive({
      password: '',
      confirmPassword: ''
    })
    
    const loading = computed(() => authStore.loading)
    const error = computed(() => authStore.error)
    const passwordMismatch = computed(() => {
      return form.password && form.confirmPassword && form.password !== form.confirmPassword
    })
    
    const handleSetup = async () => {
      if (passwordMismatch.value) return
      
      authStore.clearError()
      
      const result = await authStore.setup(form.password, form.confirmPassword)
      
      if (result.success) {
        router.push('/login')
      }
    }
    
    return {
      form,
      loading,
      error,
      passwordMismatch,
      handleSetup
    }
  }
}
</script>
