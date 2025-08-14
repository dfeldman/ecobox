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
            <h1 class="text-2xl font-bold text-gray-900">User Management</h1>
          </div>
        </div>
      </div>
    </header>

    <!-- Main Content -->
    <main class="container mx-auto px-6 py-8">
      <div class="card">
        <div class="card-header">
          <div class="flex items-center justify-between">
            <h3 class="font-semibold">Users</h3>
            <button @click="showCreateUser = true" class="btn btn-primary">
              Create User
            </button>
          </div>
        </div>
        <div class="card-body">
          <div v-if="loading" class="text-center py-8">
            <div class="loading w-8 h-8 mx-auto"></div>
            <p class="mt-4 text-gray-600">Loading users...</p>
          </div>

          <div v-else-if="error" class="text-center py-8">
            <p class="text-red-500">{{ error }}</p>
            <button @click="fetchUsers" class="btn btn-primary mt-4">
              Retry
            </button>
          </div>

          <div v-else-if="users.length === 0" class="text-center py-8">
            <p class="text-gray-600">No users found</p>
          </div>

          <div v-else class="overflow-x-auto">
            <table class="w-full">
              <thead>
                <tr class="border-b">
                  <th class="text-left py-3 px-4">Username</th>
                  <th class="text-left py-3 px-4">Admin</th>
                  <th class="text-left py-3 px-4">Created</th>
                  <th class="text-left py-3 px-4">Last Login</th>
                  <th class="text-left py-3 px-4">Actions</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="user in users" :key="user.username" class="border-b">
                  <td class="py-3 px-4 font-medium">{{ user.username }}</td>
                  <td class="py-3 px-4">
                    <span v-if="user.is_admin" class="text-green-600">Yes</span>
                    <span v-else class="text-gray-600">No</span>
                  </td>
                  <td class="py-3 px-4 text-sm text-gray-600">
                    {{ formatDate(user.created_at) }}
                  </td>
                  <td class="py-3 px-4 text-sm text-gray-600">
                    {{ formatDate(user.last_login) }}
                  </td>
                  <td class="py-3 px-4">
                    <button
                      v-if="user.username !== currentUser?.username"
                      @click="confirmDelete(user)"
                      class="btn btn-danger btn-sm"
                    >
                      Delete
                    </button>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </main>

    <!-- Create User Modal -->
    <div v-if="showCreateUser" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
      <div class="card w-full max-w-md">
        <div class="card-header">
          <h3 class="font-semibold">Create New User</h3>
        </div>
        <div class="card-body">
          <form @submit.prevent="handleCreateUser">
            <div class="form-group">
              <label class="form-label">Username</label>
              <input
                v-model="newUser.username"
                type="text"
                required
                class="form-input"
                :disabled="createLoading"
              />
            </div>
            <div class="form-group">
              <label class="flex items-center gap-2">
                <input
                  v-model="newUser.isAdmin"
                  type="checkbox"
                  :disabled="createLoading"
                />
                <span class="text-sm">Administrator</span>
              </label>
            </div>
            <div v-if="createError" class="text-red text-sm mb-4">
              {{ createError }}
            </div>
            <div class="flex gap-2">
              <button
                type="submit"
                class="btn btn-primary flex-1"
                :disabled="createLoading"
              >
                <span v-if="createLoading" class="loading w-3 h-3 mr-1"></span>
                Create
              </button>
              <button
                type="button"
                @click="showCreateUser = false"
                class="btn btn-secondary flex-1"
                :disabled="createLoading"
              >
                Cancel
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>

    <!-- Delete Confirmation Modal -->
    <div v-if="userToDelete" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
      <div class="card w-full max-w-md">
        <div class="card-header">
          <h3 class="font-semibold">Confirm Delete</h3>
        </div>
        <div class="card-body">
          <p class="mb-4">Are you sure you want to delete user "{{ userToDelete.username }}"? This action cannot be undone.</p>
          <div v-if="deleteError" class="text-red text-sm mb-4">
            {{ deleteError }}
          </div>
          <div class="flex gap-2">
            <button
              @click="handleDeleteUser"
              class="btn btn-danger flex-1"
              :disabled="deleteLoading"
            >
              <span v-if="deleteLoading" class="loading w-3 h-3 mr-1"></span>
              Delete
            </button>
            <button
              @click="userToDelete = null"
              class="btn btn-secondary flex-1"
              :disabled="deleteLoading"
            >
              Cancel
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Success Modal -->
    <div v-if="showSuccess" class="fixed inset-0 bg-black bg-opacity-50 flex items-center justify-center p-4 z-50">
      <div class="card w-full max-w-md">
        <div class="card-header">
          <h3 class="font-semibold">User Created</h3>
        </div>
        <div class="card-body">
          <p class="mb-4">User "{{ successData.username }}" has been created successfully.</p>
          <div class="bg-yellow-50 border border-yellow-200 rounded p-4 mb-4">
            <p class="text-sm font-medium text-yellow-800">Initial Password:</p>
            <p class="font-mono text-sm bg-yellow-100 p-2 rounded mt-2">{{ successData.initialPassword }}</p>
            <p class="text-xs text-yellow-700 mt-2">Please save this password. It will not be shown again.</p>
          </div>
          <button @click="showSuccess = false" class="btn btn-primary w-full">
            OK
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, reactive, computed, onMounted } from 'vue'
import { useUsersStore } from '@/stores/users'
import { useAuthStore } from '@/stores/auth'
import { format } from 'date-fns'

export default {
  name: 'Users',
  setup() {
    const usersStore = useUsersStore()
    const authStore = useAuthStore()
    
    const showCreateUser = ref(false)
    const userToDelete = ref(null)
    const showSuccess = ref(false)
    const successData = ref({})
    const createLoading = ref(false)
    const deleteLoading = ref(false)
    const createError = ref(null)
    const deleteError = ref(null)
    
    const newUser = reactive({
      username: '',
      isAdmin: false
    })
    
    const users = computed(() => usersStore.users)
    const loading = computed(() => usersStore.loading)
    const error = computed(() => usersStore.error)
    const currentUser = computed(() => authStore.user)
    
    const fetchUsers = async () => {
      await usersStore.fetchUsers()
    }
    
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
    
    const handleCreateUser = async () => {
      createLoading.value = true
      createError.value = null
      
      const result = await usersStore.createUser(newUser.username, newUser.isAdmin)
      
      if (result.success) {
        successData.value = {
          username: newUser.username,
          initialPassword: result.initialPassword
        }
        newUser.username = ''
        newUser.isAdmin = false
        showCreateUser.value = false
        showSuccess.value = true
      } else {
        createError.value = result.message
      }
      
      createLoading.value = false
    }
    
    const confirmDelete = (user) => {
      userToDelete.value = user
      deleteError.value = null
    }
    
    const handleDeleteUser = async () => {
      if (!userToDelete.value) return
      
      deleteLoading.value = true
      deleteError.value = null
      
      const result = await usersStore.deleteUser(userToDelete.value.username)
      
      if (result.success) {
        userToDelete.value = null
      } else {
        deleteError.value = result.message
      }
      
      deleteLoading.value = false
    }
    
    onMounted(() => {
      fetchUsers()
    })
    
    return {
      users,
      loading,
      error,
      currentUser,
      showCreateUser,
      userToDelete,
      showSuccess,
      successData,
      createLoading,
      deleteLoading,
      createError,
      deleteError,
      newUser,
      fetchUsers,
      formatDate,
      handleCreateUser,
      confirmDelete,
      handleDeleteUser
    }
  }
}
</script>
