<template>
  <div class="vm-details-card" :class="[size === 'compact' ? 'compact' : 'expanded']">
    <!-- VM Header -->
    <div class="vm-header">
      <div class="vm-info">
        <h4 class="vm-name">{{ vm.name }}</h4>
        <div class="vm-meta">
          <span v-if="vm.vm_id" class="vm-id">ID: {{ vm.vm_id }}</span>
          <span v-if="vm.primary_ip" class="vm-ip">{{ vm.primary_ip }}</span>
        </div>
      </div>
      <div class="vm-status-container">
        <span :class="['vm-status', vmStatusClass]">{{ vm.status }}</span>
      </div>
    </div>

    <!-- Expanded Details -->
    <div v-if="size === 'expanded' && vmSystemInfo" class="vm-system-details">
      <div class="vm-metrics">
        <div class="metric-item">
          <span class="metric-label">OS:</span>
          <span class="metric-value">{{ vmSystemInfo.os_version || 'Unknown' }}</span>
        </div>
        <div v-if="vmSystemInfo.cpu_usage" class="metric-item">
          <span class="metric-label">CPU:</span>
          <span class="metric-value">{{ vmSystemInfo.cpu_usage.toFixed(1) }}%</span>
        </div>
        <div v-if="vmSystemInfo.memory_usage" class="metric-item">
          <span class="metric-label">Memory:</span>
          <span class="metric-value">{{ vmSystemInfo.memory_usage.used_percent.toFixed(1) }}%</span>
        </div>
        <div v-if="vmPowerUsage" class="metric-item">
          <span class="metric-label">Power:</span>
          <span class="metric-value">{{ vmPowerUsage }}W</span>
        </div>
      </div>
      
      <div v-if="vmSystemInfo.ip_addresses && vmSystemInfo.ip_addresses.length > 0" class="vm-network">
        <h5 class="section-title">Network Interfaces</h5>
        <div class="interface-list">
          <div 
            v-for="iface in vmSystemInfo.ip_addresses" 
            :key="iface.name"
            class="interface-item"
          >
            <span class="interface-name">{{ iface.name }}</span>
            <span class="interface-ip">{{ iface.ip_address }}</span>
            <span class="interface-mac">{{ iface.mac_address }}</span>
          </div>
        </div>
      </div>

      <div v-if="uptime" class="vm-uptime">
        <span class="uptime-label">Uptime:</span>
        <span class="uptime-value">{{ uptime }}</span>
      </div>
    </div>

    <!-- Action Buttons for expanded view -->
    <div v-if="size === 'expanded' && showActions" class="vm-actions">
      <button
        v-if="canWake"
        @click="$emit('wake', vm.vm_id)"
        class="btn btn-primary btn-sm"
        :disabled="actionLoading"
        title="Start VM"
      >
        <span v-if="actionLoading" class="loading w-3 h-3 mr-1"></span>
        <svg v-else class="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14.828 14.828a4 4 0 01-5.656 0M9 10h1m4 0h1m-6 4h1m4 0h1m-6-8h1m4 0h1m-6 4h1m4 0h1"></path>
        </svg>
        Start
      </button>
      
      <button
        v-if="canSuspend"
        @click="$emit('suspend', vm.vm_id)"
        class="btn btn-warning btn-sm"
        :disabled="actionLoading"
        title="Suspend VM (preserves RAM state)"
      >
        <span v-if="actionLoading" class="loading w-3 h-3 mr-1"></span>
        <svg v-else class="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 9v6m4-6v6"></path>
        </svg>
        Suspend
      </button>
      
      <button
        v-if="canShutdown"
        @click="$emit('shutdown', vm.vm_id)"
        class="btn btn-secondary btn-sm"
        :disabled="actionLoading"
        title="Graceful shutdown"
      >
        <span v-if="actionLoading" class="loading w-3 h-3 mr-1"></span>
        <svg v-else class="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728L5.636 5.636m12.728 12.728L18 12M6 12l12.728 6.364"></path>
        </svg>
        Shutdown
      </button>
      
      <button
        v-if="canStop"
        @click="$emit('stop', vm.vm_id)"
        class="btn btn-error btn-sm"
        :disabled="actionLoading"
        title="Force stop (like pulling power cord)"
      >
        <span v-if="actionLoading" class="loading w-3 h-3 mr-1"></span>
        <svg v-else class="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 10h6v4H9z"></path>
        </svg>
        Stop
      </button>
      
      <button
        @click="navigateToDetail"
        class="btn btn-ghost btn-sm"
      >
        Details
      </button>
    </div>
  </div>
</template>

<script>
import { computed } from 'vue'
import { useRouter } from 'vue-router'

export default {
  name: 'VMDetails',
  props: {
    vm: {
      type: Object,
      required: true
    },
    size: {
      type: String,
      default: 'compact', // 'compact' or 'expanded'
      validator: value => ['compact', 'expanded'].includes(value)
    },
    showActions: {
      type: Boolean,
      default: false
    },
    actionLoading: {
      type: Boolean,
      default: false
    },
    // Optional: system info if VM is also a managed server
    vmSystemInfo: {
      type: Object,
      default: null
    },
    // Optional: uptime data
    uptimeSeconds: {
      type: Number,
      default: 0
    },
    // Optional: servers list to find matching VM server
    servers: {
      type: Array,
      default: () => []
    }
  },
  emits: ['wake', 'suspend', 'shutdown', 'stop'],
  setup(props) {
    const router = useRouter()
    
    const vmStatusClass = computed(() => {
      switch (props.vm.status?.toLowerCase()) {
        case 'running':
        case 'active':
        case 'on':
          return 'status-online'
        case 'stopped':
        case 'off':
        case 'inactive':
          return 'status-offline'
        case 'suspended':
        case 'paused':
          return 'status-suspended'
        case 'stopping':
        case 'suspending':
        case 'waking':
          return 'status-transitioning'
        default:
          return 'status-unknown'
      }
    })
    
    const vmPowerUsage = computed(() => {
      if (!props.vmSystemInfo) return null
      const power = props.vmSystemInfo.power_meter_watts || props.vmSystemInfo.power_estimate_watts
      return power ? Math.round(power) : null
    })
    
    const canWake = computed(() => {
      const status = props.vm.status?.toLowerCase()
      return status === 'stopped' || status === 'off' || status === 'inactive' || status === 'suspended' || status === 'paused'
    })
    
    const canSuspend = computed(() => {
      const status = props.vm.status?.toLowerCase()
      return status === 'running' || status === 'active' || status === 'on'
    })
    
    const canShutdown = computed(() => {
      const status = props.vm.status?.toLowerCase()
      return status === 'running' || status === 'active' || status === 'on'
    })
    
    const canStop = computed(() => {
      const status = props.vm.status?.toLowerCase()
      return status === 'running' || status === 'active' || status === 'on' || status === 'suspended' || status === 'paused'
    })
    
    const uptime = computed(() => {
      if (!props.uptimeSeconds) return null
      
      const seconds = props.uptimeSeconds
      const days = Math.floor(seconds / (24 * 3600))
      const hours = Math.floor((seconds % (24 * 3600)) / 3600)
      const minutes = Math.floor((seconds % 3600) / 60)
      
      if (days > 0) {
        return `${days}d ${hours}h ${minutes}m`
      } else if (hours > 0) {
        return `${hours}h ${minutes}m`
      } else {
        return `${minutes}m`
      }
    })
    
    const navigateToDetail = () => {
      // Try to find VM as a managed server by name
      const matchingServer = props.servers.find(server => 
        server.name === props.vm.name || 
        server.hostname === props.vm.name ||
        (server.system_info?.vm_id && server.system_info.vm_id === props.vm.vm_id)
      )
      
      if (matchingServer) {
        console.log(`Navigating to VM server details: ${matchingServer.name} (${matchingServer.id})`)
        router.push(`/server/${matchingServer.id}`)
      } else {
        console.log(`VM ${props.vm.name} not found as managed server`)
        // Could show a notification or modal here
      }
    }
    
    return {
      vmStatusClass,
      vmPowerUsage,
      canWake,
      canSuspend,
      canShutdown,
      canStop,
      uptime,
      navigateToDetail
    }
  }
}
</script>

<style scoped>
.vm-details-card {
  border: 1px solid var(--border-color);
  border-radius: var(--radius);
  background-color: var(--bg-primary);
  transition: var(--transition);
}

.vm-details-card.compact {
  padding: 0.75rem;
}

.vm-details-card.expanded {
  padding: 1rem;
  margin-bottom: 1rem;
}

.vm-details-card:hover {
  border-color: var(--primary-color);
  box-shadow: var(--shadow-sm);
}

.vm-header {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  margin-bottom: 0.5rem;
}

.vm-info {
  flex: 1;
}

.vm-name {
  font-weight: 600;
  font-size: 0.875rem;
  color: var(--text-primary);
  margin: 0 0 0.25rem 0;
}

.vm-meta {
  display: flex;
  gap: 0.75rem;
  font-size: 0.75rem;
  color: var(--text-secondary);
}

.vm-id {
  font-family: monospace;
}

.vm-ip {
  color: var(--primary-color);
}

.vm-status-container {
  display: flex;
  align-items: center;
}

.vm-status {
  font-size: 0.75rem;
  font-weight: 500;
  padding: 0.25rem 0.5rem;
  border-radius: var(--radius);
  display: flex;
  align-items: center;
  gap: 0.375rem;
}

.vm-status::before {
  content: '';
  width: 0.375rem;
  height: 0.375rem;
  border-radius: 50%;
  flex-shrink: 0;
}

.vm-status.status-online {
  background-color: rgb(34 197 94 / 0.1);
  color: var(--primary-dark);
}

.vm-status.status-online::before {
  background-color: var(--primary-color);
}

.vm-status.status-offline {
  background-color: rgb(107 114 128 / 0.1);
  color: #6b7280;
}

.vm-status.status-offline::before {
  background-color: #9ca3af;
}

.vm-status.status-suspended {
  background-color: rgb(245 158 11 / 0.1);
  color: #d97706;
}

.vm-status.status-suspended::before {
  background-color: #f59e0b;
}

.vm-status.status-unknown {
  background-color: rgb(107 114 128 / 0.1);
  color: #6b7280;
}

.vm-status.status-unknown::before {
  background-color: #9ca3af;
}

/* Expanded view styles */
.vm-system-details {
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border-color);
}

.vm-metrics {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(120px, 1fr));
  gap: 0.5rem;
  margin-bottom: 1rem;
}

.metric-item {
  display: flex;
  justify-content: space-between;
  font-size: 0.75rem;
}

.metric-label {
  color: var(--text-secondary);
}

.metric-value {
  color: var(--text-primary);
  font-weight: 500;
}

.vm-network {
  margin: 1rem 0;
}

.section-title {
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--text-secondary);
  margin-bottom: 0.5rem;
  text-transform: uppercase;
  letter-spacing: 0.025em;
}

.interface-list {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.interface-item {
  display: grid;
  grid-template-columns: auto 1fr auto;
  gap: 0.5rem;
  font-size: 0.75rem;
  padding: 0.25rem 0.5rem;
  background-color: var(--bg-secondary);
  border-radius: var(--radius);
}

.interface-name {
  font-weight: 500;
  color: var(--text-primary);
}

.interface-ip {
  color: var(--primary-color);
  font-family: monospace;
}

.interface-mac {
  color: var(--text-light);
  font-family: monospace;
  font-size: 0.625rem;
}

.vm-uptime {
  display: flex;
  justify-content: space-between;
  font-size: 0.75rem;
  padding: 0.5rem 0;
  border-top: 1px solid var(--border-color);
  margin-top: 1rem;
}

.uptime-label {
  color: var(--text-secondary);
}

.uptime-value {
  color: var(--text-primary);
  font-weight: 500;
}

.vm-actions {
  display: flex;
  gap: 0.5rem;
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border-color);
}

/* Compact view adjustments */
.vm-details-card.compact .vm-header {
  margin-bottom: 0;
}

.vm-details-card.compact .vm-name {
  font-size: 0.75rem;
}

.vm-details-card.compact .vm-meta {
  font-size: 0.625rem;
}

.vm-details-card.compact .vm-status {
  font-size: 0.625rem;
  padding: 0.125rem 0.375rem;
}
</style>
