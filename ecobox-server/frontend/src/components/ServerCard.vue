<template>
  <div class="card hover:shadow-lg transition-shadow cursor-pointer" style="background-color: var(--bg-primary); border: 1px solid var(--border-color);">
    <div class="card-body">
      <!-- Header -->
      <div class="flex items-start justify-between mb-4">
        <div @click="navigateToDetail" class="flex-1 cursor-pointer">
          <h3 class="font-semibold text-lg" style="color: var(--text-primary);">{{ server.name }}</h3>
          <p class="text-sm" style="color: var(--text-secondary);">{{ server.hostname }}</p>
        </div>
        <div class="flex flex-col items-end gap-2">
          <span :class="statusClass">{{ statusText }}</span>
          <div v-if="powerUsage" class="text-sm" style="color: var(--text-secondary);">
            {{ powerUsage }}W
          </div>
        </div>
      </div>

      <!-- System Info -->
      <div v-if="server.system_info" class="mb-4 space-y-2">
        <div class="flex justify-between text-sm">
          <span style="color: var(--text-secondary);">CPU:</span>
          <span style="color: var(--text-primary);">{{ server.system_info.cpu_usage?.toFixed(1) || 0 }}%</span>
        </div>
        <div class="flex justify-between text-sm">
          <span style="color: var(--text-secondary);">Memory:</span>
          <span style="color: var(--text-primary);">{{ memoryUsage }}%</span>
        </div>
        <div class="flex justify-between text-sm">
          <span style="color: var(--text-secondary);">Network:</span>
          <span style="color: var(--text-primary);">{{ networkUsage }} Mbps</span>
        </div>
      </div>

      <!-- Services -->
      <div v-if="server.services && server.services.length > 0" class="mb-4">
        <div class="text-sm mb-2" style="color: var(--text-secondary);">
          Services ({{ server.services.length }}):
        </div>
        <div class="flex flex-wrap gap-2">
          <ServiceButton
            v-for="service in server.services.slice(0, 4)"
            :key="service.id"
            :service="service"
            @click="handleServiceClick"
          />
          <span
            v-if="server.services.length > 4"
            class="service-button"
            style="background-color: var(--bg-secondary); color: var(--text-secondary); border-color: var(--border-color);"
          >
            +{{ server.services.length - 4 }} more
          </span>
        </div>
      </div>

      <!-- VMs -->
      <div v-if="server.system_info?.vms && server.system_info.vms.length > 0" class="mb-4">
        <div class="text-sm mb-2" style="color: var(--text-secondary);">
          VMs ({{ server.system_info.vms.length }}):
        </div>
        <div class="space-y-2">
          <VMDetails
            v-for="vm in server.system_info.vms.slice(0, 3)"
            :key="vm.vm_id"
            :vm="vm"
            size="compact"
            :vm-system-info="getVMSystemInfo(vm)"
            :uptime-seconds="getVMUptime(vm)"
          />
          <div
            v-if="server.system_info.vms.length > 3"
            class="vm-details-card compact"
            style="background-color: var(--bg-secondary); color: var(--text-secondary); border-color: var(--border-color); text-align: center; padding: 0.5rem;"
          >
            +{{ server.system_info.vms.length - 3 }} more VMs
          </div>
        </div>
      </div>

      <!-- Uptime Display -->
      <div v-if="uptimeDisplay" class="mb-4">
        <div class="flex justify-between text-sm">
          <span style="color: var(--text-secondary);">Uptime:</span>
          <span style="color: var(--text-primary); font-weight: 500;">{{ uptimeDisplay }}</span>
        </div>
      </div>

      <!-- Power Controls -->
      <div class="flex gap-2 mt-4">
        <button
          v-if="showWakeButton"
          @click="$emit('wake', server.id)"
          class="btn btn-primary btn-sm flex-1"
          :disabled="actionLoading"
        >
          <span v-if="actionLoading" class="loading w-3 h-3 mr-1"></span>
          Wake
        </button>
        <button
          v-if="showSuspendButton"
          @click="$emit('suspend', server.id)"
          class="btn btn-secondary btn-sm flex-1"
          :disabled="actionLoading"
        >
          <span v-if="actionLoading" class="loading w-3 h-3 mr-1"></span>
          Suspend
        </button>
        <button
          v-if="showShutdownButton"
          @click="$emit('shutdown', server.id)"
          class="btn btn-warning btn-sm flex-1"
          :disabled="actionLoading"
        >
          <span v-if="actionLoading" class="loading w-3 h-3 mr-1"></span>
          Shutdown
        </button>
        <button
          v-if="showStopButton"
          @click="$emit('stop', server.id)"
          class="btn btn-error btn-sm flex-1"
          :disabled="actionLoading"
        >
          <span v-if="actionLoading" class="loading w-3 h-3 mr-1"></span>
          Stop
        </button>
        <button
          @click="navigateToDetail"
          class="btn btn-secondary btn-sm"
        >
          Details
        </button>
      </div>
    </div>
  </div>
</template>

<script>
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import ServiceButton from './ServiceButton.vue'
import VMDetails from './VMDetails.vue'

export default {
  name: 'ServerCard',
  components: {
    ServiceButton,
    VMDetails
  },
  props: {
    server: {
      type: Object,
      required: true
    }
  },
  emits: ['wake', 'suspend', 'shutdown', 'stop'],
  setup(props) {
    const router = useRouter()
    const actionLoading = ref(false)
    
    const statusText = computed(() => {
      switch (props.server.current_state) {
        case 'on': return 'Online'
        case 'off': return 'Offline'
        case 'suspended': return 'Suspended'
        case 'unknown': return 'Unknown'
        case 'init_failed': return 'Init Failed'
        default: return props.server.current_state
      }
    })
    
    const statusClass = computed(() => {
      const base = 'status-indicator'
      switch (props.server.current_state) {
        case 'on': return `${base} status-online`
        case 'off': return `${base} status-offline`
        case 'suspended': return `${base} status-suspended`
        default: return `${base} status-unknown`
      }
    })
    
    const powerUsage = computed(() => {
      const power = props.server.system_info?.power_meter_watts || 
                   props.server.system_info?.power_estimate_watts
      return power ? Math.round(power) : null
    })
    
    const memoryUsage = computed(() => {
      const memory = props.server.system_info?.memory_usage
      return memory ? memory.used_percent.toFixed(1) : 0
    })
    
    const networkUsage = computed(() => {
      const network = props.server.system_info?.network_usage
      if (!network) return '0'
      return ((network.mbps_recv || 0) + (network.mbps_sent || 0)).toFixed(1)
    })
    
    const uptimeDisplay = computed(() => {
      if (props.server.current_state !== 'on' || !props.server.last_state_change) {
        return null
      }
      
      const lastChange = new Date(props.server.last_state_change)
      const now = new Date()
      const diffMs = now - lastChange
      const diffSeconds = Math.floor(diffMs / 1000)
      const diffMinutes = Math.floor(diffSeconds / 60)
      const diffHours = Math.floor(diffMinutes / 60)
      const diffDays = Math.floor(diffHours / 24)
      
      if (diffDays > 0) {
        return `${diffDays}d ${diffHours % 24}h ${diffMinutes % 60}m`
      } else if (diffHours > 0) {
        return `${diffHours}h ${diffMinutes % 60}m`
      } else {
        return `${diffMinutes}m`
      }
    })
    
    const serviceStatusClass = (status) => {
      return status === 'up' ? 'service-online' : 'service-offline'
    }
    
    const vmStatusClass = (status) => {
      return status === 'running' ? 'vm-running' : 'vm-stopped'
    }
    
    const handleServiceClick = (service) => {
      // For now, just navigate to server detail
      // In the future, this could open service-specific pages
      console.log(`Clicked service: ${service.name}:${service.port}`)
      navigateToDetail()
    }
    
    const getVMSystemInfo = (vm) => {
      // Try to find system info if this VM is also configured as a server
      // This would require matching VM to servers by name or other identifier
      // For now, return null - could be enhanced with VM-to-server mapping
      return null
    }
    
    const getVMUptime = (vm) => {
      // VM uptime would need to be provided by the backend
      // For now, return 0
      return 0
    }
    
    const navigateToDetail = () => {
      router.push(`/server/${props.server.id}`)
    }
    
    const showWakeButton = computed(() => {
      const state = props.server.current_state
      return state === 'off' || state === 'suspended' || state === 'unknown' || state === 'init_failed' || state === 'stopped'
    })
    
    const showSuspendButton = computed(() => {
      const state = props.server.current_state
      // For regular servers: show if on and supports suspend
      // For VMs (identified by having a parent_server_id): always show when running
      const isVM = !!props.server.parent_server_id
      
      // Debug logging for VMs
      if (isVM) {
        console.log(`VM ${props.server.name}: state=${state}, parent_server_id=${props.server.parent_server_id}`)
      }
      
      if (isVM) {
        return state === 'on'
      } else {
        return (state === 'on' || state === 'init_failed') && props.server.system_info?.suspend_support
      }
    })
    
    const showShutdownButton = computed(() => {
      // Only show for VMs (identified by having a parent_server_id) when running
      const isVM = !!props.server.parent_server_id
      const state = props.server.current_state
      const shouldShow = isVM && state === 'on'
      
      if (isVM) {
        console.log(`VM ${props.server.name}: showShutdownButton=${shouldShow} (state=${state})`)
      }
      
      return shouldShow
    })
    
    const showStopButton = computed(() => {
      // Only show for VMs (identified by having a parent_server_id) when running
      const isVM = !!props.server.parent_server_id
      const state = props.server.current_state
      const shouldShow = isVM && state === 'on'
      
      if (isVM) {
        console.log(`VM ${props.server.name}: showStopButton=${shouldShow} (state=${state})`)
      }
      
      return shouldShow
    })
    
    return {
      actionLoading,
      statusText,
      statusClass,
      powerUsage,
      memoryUsage,
      networkUsage,
      uptimeDisplay,
      serviceStatusClass,
      vmStatusClass,
      handleServiceClick,
      getVMSystemInfo,
      getVMUptime,
      navigateToDetail,
      showWakeButton,
      showSuspendButton,
      showShutdownButton,
      showStopButton
    }
  }
}
</script>
