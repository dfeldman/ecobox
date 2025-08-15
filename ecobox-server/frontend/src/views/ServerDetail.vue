<template>
  <div class="min-h-screen" style="background-color: var(--bg-secondary);">
    <!-- Header -->
    <header class="main-header">
      <div class="container mx-auto px-6 py-4">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-4">
            <button @click="$router.back()" class="btn btn-secondary btn-sm">
              <svg class="w-4 h-4 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
              </svg>
              Back
            </button>
            <div class="flex items-center gap-3">
              <h1 class="brand-title text-xl">{{ server?.name || serverId }}</h1>
              <div v-if="server" :class="['connection-indicator', websocketConnected ? 'connection-connected' : 'connection-disconnected']">
                {{ websocketConnected ? 'Live Data' : 'Offline' }}
              </div>
            </div>
            <div v-if="server">
              <p class="text-sm" style="color: var(--text-light); margin: 0;">{{ server.hostname }}</p>
              <p v-if="server.system_info?.os_version" class="text-xs" style="color: var(--text-light); margin: 0;">
                {{ server.system_info.os_version }}
              </p>
              <div v-if="parentServer" class="mt-2">
                <router-link 
                  :to="`/server/${parentServer.id}`" 
                  class="text-xs flex items-center gap-1 hover:underline"
                  style="color: var(--text-secondary);"
                >
                  <svg class="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0l-4 4m4-4l-4-4" />
                  </svg>
                  VM Host: {{ parentServer.name }}
                </router-link>
              </div>
            </div>
          </div>
          
          <nav class="flex items-center gap-3">
            <ThemeToggle />
            <div v-if="server" class="flex items-center gap-3">
              <span :class="statusClass">{{ statusText }}</span>
              <div class="flex gap-2">
                <button
                  v-if="showWakeButton"
                  @click="handleWake"
                  class="btn btn-primary btn-sm"
                  :disabled="actionLoading"
                >
                  <span v-if="actionLoading" class="loading w-3 h-3 mr-1"></span>
                  Wake
                </button>
                <button
                  v-if="showSuspendButton"
                  @click="handleSuspend"
                  class="btn btn-secondary btn-sm"
                  :disabled="actionLoading"
                >
                  <span v-if="actionLoading" class="loading w-3 h-3 mr-1"></span>
                  Suspend
                </button>
              </div>
            </div>
          </nav>
        </div>
      </div>
    </header>

    <!-- Main Content -->
    <main class="container mx-auto px-6 py-6">
      <div v-if="loading && !server" class="text-center py-8">
        <div class="loading w-8 h-8 mx-auto"></div>
        <p class="mt-4" style="color: var(--text-secondary);">Loading server details...</p>
      </div>

      <div v-else-if="error" class="text-center py-8">
        <p class="text-red-500">{{ error }}</p>
        <button @click="fetchServer" class="btn btn-primary mt-4">
          Retry
        </button>
      </div>

      <div v-else-if="server">
        <!-- Quick Stats Row -->
        <div class="grid grid-cols-2 md:grid-cols-5 gap-4 mb-6">
          <div class="card stats-card">
            <h4 style="color: var(--primary-color); margin-bottom: 0.25rem;">{{ server.system_info?.cpu_usage?.toFixed(1) || 0 }}%</h4>
            <p>CPU Usage</p>
          </div>
          <div class="card stats-card">
            <h4 style="color: var(--accent-color); margin-bottom: 0.25rem;">{{ memoryUsage }}%</h4>
            <p>Memory</p>
          </div>
          <div class="card stats-card">
            <h4 style="color: #f59e0b; margin-bottom: 0.25rem;">{{ diskUsage }}%</h4>
            <p>Disk</p>
          </div>
          <div class="card stats-card">
            <h4 style="color: #8b5cf6; margin-bottom: 0.25rem;">{{ networkUsage }}</h4>
            <p>Network</p>
          </div>
          <div class="card stats-card">
            <h4 style="color: #ef4444; margin-bottom: 0.25rem;">{{ powerUsage }}W</h4>
            <p>Power</p>
          </div>
        </div>

        <div class="grid grid-cols-1 xl:grid-cols-3 gap-6">
          <!-- Left Column - System Info & Services -->
          <div class="xl:col-span-1 space-y-6">
            <!-- System Information -->
            <div class="card">
              <div class="card-header">
                <h3 class="font-semibold">System Information</h3>
              </div>
              <div class="card-body">
                <div v-if="server.system_info" class="space-y-3">
                  <div class="system-info-grid">
                    <div class="info-item">
                      <span class="info-label">System Type:</span>
                      <span class="info-value">{{ server.system_info.type }}</span>
                    </div>
                    <div class="info-item">
                      <span class="info-label">System ID:</span>
                      <span class="info-value">{{ server.system_info.system_id || 'Unknown' }}</span>
                    </div>
                    <div class="info-item">
                      <span class="info-label">Hostname:</span>
                      <span class="info-value">{{ server.system_info.hostname }}</span>
                    </div>
                    <div v-if="uptimeDisplay" class="info-item">
                      <span class="info-label">Uptime:</span>
                      <span class="info-value uptime-value">{{ uptimeDisplay }}</span>
                    </div>
                    <div v-if="server.system_info.load_average" class="info-item">
                      <span class="info-label">Load Average:</span>
                      <span class="info-value">{{ server.system_info.load_average.join(', ') }}</span>
                    </div>
                    <div class="info-item">
                      <span class="info-label">Last Updated:</span>
                      <span class="info-value">{{ formatLastUpdated }}</span>
                    </div>
                  </div>
                </div>
                <div v-else class="no-data">
                  No system information available
                </div>
              </div>
            </div>

            <!-- Network Interfaces -->
            <div v-if="server.system_info?.ip_addresses && server.system_info.ip_addresses.length > 0" class="card">
              <div class="card-header">
                <h3 class="font-semibold">Network Interfaces</h3>
              </div>
              <div class="card-body">
                <div class="interface-list">
                  <div
                    v-for="iface in server.system_info.ip_addresses"
                    :key="iface.name"
                    class="interface-item"
                  >
                    <div class="interface-header">
                      <span class="interface-name">{{ iface.name }}</span>
                      <span v-if="iface.is_ipv6" class="interface-type">IPv6</span>
                    </div>
                    <div class="interface-details">
                      <span class="interface-ip">{{ iface.ip_address }}</span>
                      <span class="interface-mac">{{ iface.mac_address }}</span>
                    </div>
                  </div>
                </div>
              </div>
            </div>

            <!-- Services -->
            <div v-if="server.services && server.services.length > 0" class="card">
              <div class="card-header">
                <h3 class="font-semibold">Services ({{ server.services.length }})</h3>
              </div>
              <div class="card-body">
                <div class="services-grid">
                  <ServiceButton
                    v-for="service in server.services"
                    :key="service.id"
                    :service="service"
                    @click="handleServiceClick"
                  />
                </div>
              </div>
            </div>

            <!-- System Capabilities -->
            <SystemCapabilities 
              v-if="server.system_info"
              :system-info="server.system_info"
            />

            <!-- Recent Actions -->
            <RecentActions 
              v-if="server.recent_actions"
              :actions="server.recent_actions"
              :max-visible="5"
            />
          </div>

          <!-- Middle & Right Columns - VMs & Charts -->
          <div class="xl:col-span-2 space-y-6">
            <!-- Virtual Machines -->
            <div v-if="server.system_info?.vms && server.system_info.vms.length > 0" class="card">
              <div class="card-header">
                <h3 class="font-semibold">Virtual Machines ({{ server.system_info.vms.length }})</h3>
              </div>
              <div class="card-body">
                <div class="vms-list">
                  <VMDetails
                    v-for="vm in server.system_info.vms"
                    :key="vm.vm_id"
                    :vm="vm"
                    size="expanded"
                    :show-actions="server.system_info.type === 'proxmox'"
                    :vm-system-info="getVMSystemInfo(vm)"
                    :uptime-seconds="getVMUptime(vm)"
                    :servers="servers"
                    @wake="handleVMWake"
                    @suspend="handleVMSuspend"
                    @shutdown="handleVMShutdown"
                    @stop="handleVMStop"
                  />
                </div>
              </div>
            </div>

            <!-- Metrics Charts -->
            <div class="card">
              <div class="card-header">
                <h3 class="font-semibold">Performance Metrics</h3>
              </div>
              <div class="card-body">
                <MetricsCharts
                  v-if="server"
                  :server-id="server.id"
                  :server-name="server.name"
                />
              </div>
            </div>
          </div>
        </div>
      </div>
    </main>
  </div>
</template>

<script>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useServersStore } from '@/stores/servers'
import ThemeToggle from '@/components/ThemeToggle.vue'
import ServiceButton from '@/components/ServiceButton.vue'
import VMDetails from '@/components/VMDetails.vue'
import SystemCapabilities from '@/components/SystemCapabilities.vue'
import RecentActions from '@/components/RecentActions.vue'
import MetricsCharts from '@/components/MetricsCharts.vue'

export default {
  name: 'ServerDetail',
  components: {
    ThemeToggle,
    ServiceButton,
    VMDetails,
    SystemCapabilities,
    RecentActions,
    MetricsCharts
  },
  setup() {
    const route = useRoute()
    const serversStore = useServersStore()
    const actionLoading = ref(false)
    
    const serverId = computed(() => route.params.id)
    const server = computed(() => serversStore.currentServer)
    const loading = computed(() => serversStore.loading)
    const error = computed(() => serversStore.error)
    const websocketConnected = computed(() => serversStore.websocketConnected)
    
    const statusText = computed(() => {
      if (!server.value) return 'Unknown'
      switch (server.value.current_state) {
        case 'on': return 'Online'
        case 'off': return 'Offline'
        case 'suspended': return 'Suspended'
        case 'unknown': return 'Unknown'
        case 'init_failed': return 'Init Failed'
        default: return server.value.current_state
      }
    })
    
    const statusClass = computed(() => {
      if (!server.value) return 'status-indicator status-unknown'
      const base = 'status-indicator'
      switch (server.value.current_state) {
        case 'on': return `${base} status-online`
        case 'off': return `${base} status-offline`
        case 'suspended': return `${base} status-suspended`
        default: return `${base} status-unknown`
      }
    })
    
    const powerUsage = computed(() => {
      if (!server.value?.system_info) return 0
      return Math.round(
        server.value.system_info.power_meter_watts || 
        server.value.system_info.power_estimate_watts || 0
      )
    })
    
    const memoryUsage = computed(() => {
      if (!server.value?.system_info?.memory_usage) return 0
      return server.value.system_info.memory_usage.used_percent.toFixed(1)
    })

    const diskUsage = computed(() => {
      if (!server.value?.system_info?.disk_usage) return 0
      return server.value.system_info.disk_usage.used_percent?.toFixed(1) || 0
    })

    const networkUsage = computed(() => {
      if (!server.value?.system_info?.network_usage) return '0 B/s'
      const usage = server.value.system_info.network_usage
      const total = (usage.bytes_sent || 0) + (usage.bytes_recv || 0)
      return formatBytes(total) + '/s'
    })

    const uptimeDisplay = computed(() => {
      if (!server.value?.system_info?.uptime_seconds) return null
      return formatUptime(server.value.system_info.uptime_seconds)
    })

    const formatLastUpdated = computed(() => {
      if (!server.value?.system_info?.last_updated) return 'Never'
      return new Date(server.value.system_info.last_updated).toLocaleString()
    })

    const parentServer = computed(() => {
      if (!server.value?.parent_server_id || !serversStore.servers) return null
      return serversStore.servers.find(s => s.id === server.value.parent_server_id)
    })
    
    const servers = computed(() => serversStore.servers || [])
    
    const formatBytes = (bytes) => {
      if (!bytes) return '0 B'
      const k = 1024
      const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
      const i = Math.floor(Math.log(bytes) / Math.log(k))
      return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
    }

    const formatUptime = (seconds) => {
      if (!seconds) return null
      const days = Math.floor(seconds / 86400)
      const hours = Math.floor((seconds % 86400) / 3600)
      const minutes = Math.floor((seconds % 3600) / 60)
      
      if (days > 0) {
        return `${days}d ${hours}h ${minutes}m`
      } else if (hours > 0) {
        return `${hours}h ${minutes}m`
      } else {
        return `${minutes}m`
      }
    }
    
    const showWakeButton = computed(() => {
      if (!server.value) return false
      const state = server.value.current_state
      return state === 'off' || state === 'suspended' || state === 'unknown' || state === 'init_failed'
    })
    
    const showSuspendButton = computed(() => {
      if (!server.value) return false
      const state = server.value.current_state
      // Show suspend for systems that are on, or in init_failed state (since we don't know the actual state)
      return (state === 'on' || state === 'init_failed') && server.value.system_info?.suspend_support
    })

    // VM-related methods
    const getVMSystemInfo = (vm) => {
      // Try to find corresponding server data for this VM
      const vmServer = serversStore.servers.find(s => s.id === vm.vm_id || s.name === vm.name)
      return vmServer?.system_info || null
    }

    const getVMUptime = (vm) => {
      const vmSystemInfo = getVMSystemInfo(vm)
      return vmSystemInfo?.uptime_seconds || 0
    }
    
    const fetchServer = async () => {
      await serversStore.fetchServer(serverId.value)
    }
    
    const handleWake = async () => {
      actionLoading.value = true
      try {
        const result = await serversStore.wakeServer(serverId.value)
        if (result.success) {
          console.log(result.message)
        } else {
          console.error(result.message)
        }
      } finally {
        actionLoading.value = false
      }
    }
    
    const handleSuspend = async () => {
      actionLoading.value = true
      try {
        const result = await serversStore.suspendServer(serverId.value)
        if (result.success) {
          console.log(result.message)
        } else {
          console.error(result.message)
        }
      } finally {
        actionLoading.value = false
      }
    }

    const handleVMWake = async (vmId) => {
      actionLoading.value = true
      try {
        const result = await serversStore.wakeServer(vmId)
        if (result.success) {
          console.log(result.message)
        } else {
          console.error(result.message)
        }
      } finally {
        actionLoading.value = false
      }
    }

    const handleVMSuspend = async (vmId) => {
      actionLoading.value = true
      try {
        const result = await serversStore.suspendServer(vmId)
        if (result.success) {
          console.log(result.message)
        } else {
          console.error(result.message)
        }
      } finally {
        actionLoading.value = false
      }
    }

    const handleVMShutdown = async (vmId) => {
      actionLoading.value = true
      try {
        const result = await serversStore.shutdownServer(vmId)
        if (result.success) {
          console.log(result.message)
        } else {
          console.error(result.message)
        }
      } finally {
        actionLoading.value = false
      }
    }

    const handleVMStop = async (vmId) => {
      actionLoading.value = true
      try {
        const result = await serversStore.stopServer(vmId)
        if (result.success) {
          console.log(result.message)
        } else {
          console.error(result.message)
        }
      } finally {
        actionLoading.value = false
      }
    }

    const handleServiceClick = (service) => {
      console.log('Service clicked:', service)
      // Could open service URL in new tab or show service details
      if (service.url) {
        window.open(service.url, '_blank')
      }
    }
    
    onMounted(async () => {
      await fetchServer()
      serversStore.initializeWebSocket()
    })
    
    // Watch for route parameter changes to refetch server data
    watch(
      () => route.params.id,
      async (newId, oldId) => {
        if (newId !== oldId) {
          await fetchServer()
        }
      }
    )
    
    onUnmounted(() => {
      // Don't disconnect websocket here as it's shared
      serversStore.currentServer = null
    })
    
    return {
      serverId,
      server,
      loading,
      error,
      websocketConnected,
      actionLoading,
      statusText,
      statusClass,
      powerUsage,
      memoryUsage,
      diskUsage,
      networkUsage,
      uptimeDisplay,
      formatLastUpdated,
      parentServer,
      servers,
      formatBytes,
      showWakeButton,
      showSuspendButton,
      getVMSystemInfo,
      getVMUptime,
      fetchServer,
      handleWake,
      handleSuspend,
      handleVMWake,
      handleVMSuspend,
      handleVMShutdown,
      handleVMStop,
      handleServiceClick
    }
  }
}
</script>
