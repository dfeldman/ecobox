<template>
  <div class="min-h-screen bg-gray-50" style="background-color: var(--bg-secondary);">
    <!-- Header -->
    <header class="main-header">
      <div class="container mx-auto px-6 py-4">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-4">
            <h1 class="brand-title">EcoBox</h1>
            <p class="text-sm" style="color: var(--text-light); margin: 0;">Sustainable Server Management</p>
            <div 
              :class="['connection-indicator', websocketConnected ? 'connection-connected' : 'connection-disconnected']"
            >
              {{ websocketConnected ? 'Connected' : 'Disconnected' }}
            </div>
          </div>
          
          <nav class="flex items-center gap-3">
            <ThemeToggle />
            <router-link
              v-if="isAdmin"
              to="/users"
              class="btn btn-secondary btn-sm"
            >
              Users
            </router-link>
            <router-link
              to="/profile"
              class="btn btn-secondary btn-sm"
            >
              Profile
            </router-link>
            <button @click="handleLogout" class="btn btn-danger btn-sm">
              Logout
            </button>
          </nav>
        </div>
      </div>
    </header>

    <!-- Main Content -->
    <main class="container mx-auto px-6 py-6">
      <div v-if="loading" class="text-center py-8">
        <div class="loading w-8 h-8 mx-auto"></div>
        <p class="mt-4" style="color: var(--text-secondary);">Loading servers...</p>
      </div>

      <div v-else-if="error" class="text-center py-8">
        <p class="text-red-500">{{ error }}</p>
        <button @click="fetchServers" class="btn btn-primary mt-4">
          Retry
        </button>
      </div>

      <div v-else>
        <!-- Compact Stats Cards -->
        <div class="grid grid-cols-2 md:grid-cols-4 gap-4 mb-6">
          <div class="card stats-card">
            <h3 style="color: var(--primary-color);">{{ onlineCount }}</h3>
            <p>Online</p>
          </div>
          <div class="card stats-card">
            <h3 style="color: #ef4444;">{{ offlineCount }}</h3>
            <p>Offline</p>
          </div>
          <div class="card stats-card">
            <h3 style="color: #f59e0b;">{{ suspendedCount }}</h3>
            <p>Suspended</p>
          </div>
          <div class="card stats-card">
            <h3 style="color: var(--accent-color);">{{ totalPowerUsage }}W</h3>
            <p>Power Usage</p>
          </div>
        </div>

        <!-- Servers Grid -->
        <div v-if="servers.length === 0" class="text-center py-8">
          <p style="color: var(--text-secondary);">No servers configured</p>
        </div>

        <div v-else class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
          <ServerCard
            v-for="server in sortedServers"
            :key="server.id"
            :server="server"
            @wake="handleWake"
            @suspend="handleSuspend"
          />
        </div>
      </div>
    </main>
  </div>
</template>

<script>
import { computed, onMounted, onUnmounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useServersStore } from '@/stores/servers'
import ServerCard from '@/components/ServerCard.vue'
import ThemeToggle from '@/components/ThemeToggle.vue'

export default {
  name: 'Dashboard',
  components: {
    ServerCard,
    ThemeToggle
  },
  setup() {
    const router = useRouter()
    const authStore = useAuthStore()
    const serversStore = useServersStore()
    
    const servers = computed(() => serversStore.servers)
    const loading = computed(() => serversStore.loading)
    const error = computed(() => serversStore.error)
    const websocketConnected = computed(() => serversStore.websocketConnected)
    const isAdmin = computed(() => authStore.isAdmin)
    
    // Sort servers alphabetically, grouping VMs with their parent servers
    const sortedServers = computed(() => {
      const serversList = [...servers.value]
      
      // Helper function to check if a server is a VM based on naming convention
      const isVM = (serverName) => {
        return serverName.includes('-vm-') || serverName.toLowerCase().includes('vm')
      }
      
      // Helper function to get parent server name from VM name
      const getParentServerName = (vmName) => {
        if (vmName.includes('-vm-')) {
          return vmName.split('-vm-')[0]
        }
        // Fallback for other VM naming patterns
        return vmName.replace(/[-_]?vm[-_]?\d+/i, '')
      }
      
      // Separate physical servers and VMs
      const physicalServers = serversList.filter(s => !isVM(s.name))
      const vms = serversList.filter(s => isVM(s.name))
      
      // Sort physical servers alphabetically
      physicalServers.sort((a, b) => a.name.localeCompare(b.name))
      
      // Group VMs by their parent server and sort within groups
      const vmGroups = {}
      vms.forEach(vm => {
        const parentName = getParentServerName(vm.name)
        if (!vmGroups[parentName]) {
          vmGroups[parentName] = []
        }
        vmGroups[parentName].push(vm)
      })
      
      // Sort VMs within each group
      Object.keys(vmGroups).forEach(parentName => {
        vmGroups[parentName].sort((a, b) => a.name.localeCompare(b.name))
      })
      
      // Build final sorted list
      const result = []
      physicalServers.forEach(server => {
        result.push(server)
        // Add VMs for this server if they exist
        if (vmGroups[server.name]) {
          result.push(...vmGroups[server.name])
          delete vmGroups[server.name]
        }
      })
      
      // Add any remaining VMs that didn't match a parent server
      Object.values(vmGroups).forEach(vmGroup => {
        result.push(...vmGroup)
      })
      
      return result
    })
    
    const onlineCount = computed(() => {
      return servers.value.filter(s => s.current_state === 'on').length
    })
    
    const offlineCount = computed(() => {
      return servers.value.filter(s => s.current_state === 'off').length
    })
    
    const suspendedCount = computed(() => {
      return servers.value.filter(s => s.current_state === 'suspended').length
    })
    
    const totalPowerUsage = computed(() => {
      return servers.value.reduce((total, server) => {
        const power = server.system_info?.power_meter_watts || 
                     server.system_info?.power_estimate_watts || 0
        return total + power
      }, 0).toFixed(0)
    })
    
    const fetchServers = async () => {
      await serversStore.fetchServers()
    }
    
    const handleLogout = async () => {
      await authStore.logout()
      router.push('/login')
    }
    
    const handleWake = async (serverId) => {
      const result = await serversStore.wakeServer(serverId)
      if (result.success) {
        // Success feedback could be added here
        console.log(result.message)
      } else {
        // Error feedback could be added here
        console.error(result.message)
      }
    }
    
    const handleSuspend = async (serverId) => {
      const result = await serversStore.suspendServer(serverId)
      if (result.success) {
        // Success feedback could be added here
        console.log(result.message)
      } else {
        // Error feedback could be added here
        console.error(result.message)
      }
    }
    
    onMounted(async () => {
      await fetchServers()
      serversStore.initializeWebSocket()
    })
    
    onUnmounted(() => {
      serversStore.disconnectWebSocket()
    })
    
    return {
      servers,
      sortedServers,
      loading,
      error,
      websocketConnected,
      isAdmin,
      onlineCount,
      offlineCount,
      suspendedCount,
      totalPowerUsage,
      fetchServers,
      handleLogout,
      handleWake,
      handleSuspend
    }
  }
}
</script>
