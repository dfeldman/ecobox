<template>
  <div class="service-button-wrapper">
    <button
      :class="['service-button', statusClass]"
      @click="handleClick"
      @mouseenter="showTooltip = true"
      @mouseleave="showTooltip = false"
      :title="tooltipText"
    >
      <span class="service-name">{{ service.name }}</span>
      <span class="service-port">:{{ service.port }}</span>
    </button>
    
    <!-- Tooltip -->
    <div v-if="showTooltip && showTooltips" class="service-tooltip" :class="{ 'tooltip-visible': showTooltip }">
      <div class="tooltip-content">
        <div class="tooltip-header">
          <strong>{{ service.name }}:{{ service.port }}</strong>
          <span :class="['tooltip-status', statusClass]">{{ service.status }}</span>
        </div>
        <div class="tooltip-details">
          <div class="tooltip-row">
            <span>Type:</span>
            <span class="tooltip-value">{{ serviceTypeDisplay }}</span>
          </div>
          <div class="tooltip-row">
            <span>Source:</span>
            <span class="tooltip-value">{{ service.source || 'Unknown' }}</span>
          </div>
          <div v-if="service.last_check" class="tooltip-row">
            <span>Last Check:</span>
            <span class="tooltip-value">{{ formatLastCheck }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { computed, ref } from 'vue'

export default {
  name: 'ServiceButton',
  props: {
    service: {
      type: Object,
      required: true
    },
    showTooltips: {
      type: Boolean,
      default: true
    }
  },
  emits: ['click'],
  setup(props, { emit }) {
    const showTooltip = ref(false)
    
    const statusClass = computed(() => {
      return props.service.status === 'up' ? 'service-online' : 'service-offline'
    })
    
    const serviceTypeDisplay = computed(() => {
      const type = props.service.type || 'custom'
      return type.charAt(0).toUpperCase() + type.slice(1)
    })
    
    const tooltipText = computed(() => {
      const status = props.service.status === 'up' ? 'Online' : 'Offline'
      const type = serviceTypeDisplay.value
      return `${props.service.name}:${props.service.port} - ${status} (${type})`
    })
    
    const formatLastCheck = computed(() => {
      if (!props.service.last_check) return 'Never'
      
      const lastCheck = new Date(props.service.last_check)
      const now = new Date()
      const diffMs = now - lastCheck
      const diffSeconds = Math.floor(diffMs / 1000)
      const diffMinutes = Math.floor(diffSeconds / 60)
      const diffHours = Math.floor(diffMinutes / 60)
      const diffDays = Math.floor(diffHours / 24)
      
      if (diffDays > 0) {
        return `${diffDays}d ago`
      } else if (diffHours > 0) {
        return `${diffHours}h ago`
      } else if (diffMinutes > 0) {
        return `${diffMinutes}m ago`
      } else {
        return `${diffSeconds}s ago`
      }
    })
    
    const handleClick = () => {
      emit('click', props.service)
    }
    
    return {
      showTooltip,
      statusClass,
      serviceTypeDisplay,
      tooltipText,
      formatLastCheck,
      handleClick
    }
  }
}
</script>

<style scoped>
.service-button-wrapper {
  position: relative;
  display: inline-block;
}

.service-button {
  display: inline-flex;
  align-items: center;
  gap: 0.125rem;
  padding: 0.375rem 0.75rem;
  border-radius: var(--radius);
  font-size: 0.75rem;
  font-weight: 500;
  text-decoration: none;
  cursor: pointer;
  transition: var(--transition);
  border: 1px solid transparent;
  position: relative;
}

.service-button::before {
  content: '';
  width: 0.375rem;
  height: 0.375rem;
  border-radius: 50%;
  flex-shrink: 0;
  margin-right: 0.25rem;
}

.service-online {
  background-color: rgb(34 197 94 / 0.1);
  color: var(--primary-dark);
  border-color: rgb(34 197 94 / 0.2);
}

.service-online::before {
  background-color: var(--primary-color);
}

.service-online:hover {
  background-color: rgb(34 197 94 / 0.15);
  border-color: var(--primary-color);
  transform: translateY(-1px);
}

.service-offline {
  background-color: rgb(239 68 68 / 0.1);
  color: #dc2626;
  border-color: rgb(239 68 68 / 0.2);
}

.service-offline::before {
  background-color: #ef4444;
}

.service-offline:hover {
  background-color: rgb(239 68 68 / 0.15);
  border-color: #ef4444;
  transform: translateY(-1px);
}

.service-name {
  font-weight: 600;
}

.service-port {
  font-family: monospace;
  font-size: 0.625rem;
  opacity: 0.8;
}

/* Tooltip styles */
.service-tooltip {
  position: absolute;
  bottom: 100%;
  left: 50%;
  transform: translateX(-50%);
  margin-bottom: 0.5rem;
  background-color: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius);
  padding: 0.75rem;
  box-shadow: var(--shadow-lg);
  z-index: 1000;
  min-width: 200px;
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.2s ease;
}

.service-tooltip::after {
  content: '';
  position: absolute;
  top: 100%;
  left: 50%;
  transform: translateX(-50%);
  border: 6px solid transparent;
  border-top-color: var(--bg-primary);
}

.service-tooltip.tooltip-visible {
  opacity: 1;
}

.tooltip-content {
  font-size: 0.75rem;
}

.tooltip-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.5rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid var(--border-color);
}

.tooltip-header strong {
  color: var(--text-primary);
  font-family: monospace;
}

.tooltip-status {
  font-size: 0.625rem;
  padding: 0.125rem 0.375rem;
  border-radius: var(--radius);
  font-weight: 500;
}

.tooltip-status.service-online {
  background-color: rgb(34 197 94 / 0.2);
  color: var(--primary-dark);
}

.tooltip-status.service-offline {
  background-color: rgb(239 68 68 / 0.2);
  color: #dc2626;
}

.tooltip-details {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.tooltip-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.tooltip-row span:first-child {
  color: var(--text-secondary);
}

.tooltip-value {
  color: var(--text-primary);
  font-weight: 500;
}

/* Responsive tooltip positioning */
@media (max-width: 768px) {
  .service-tooltip {
    position: fixed;
    bottom: auto;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    margin: 0;
    max-width: 90vw;
  }
  
  .service-tooltip::after {
    display: none;
  }
}
</style>
