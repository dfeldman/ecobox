<template>
  <div class="system-capabilities">
    <h4 class="capabilities-title">System Capabilities</h4>
    <div class="capabilities-grid">
      <div class="capability-item" :class="{ supported: systemInfo.suspend_support }">
        <div class="capability-icon">
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8 7a1 1 0 012 0v3a1 1 0 11-2 0V7zM8 13a1 1 0 011-1h2a1 1 0 110 2H9a1 1 0 01-1-1z" clip-rule="evenodd" />
          </svg>
        </div>
        <div class="capability-info">
          <span class="capability-name">Suspend</span>
          <span class="capability-status">{{ systemInfo.suspend_support ? 'Supported' : 'Not Supported' }}</span>
        </div>
      </div>

      <div class="capability-item" :class="{ supported: systemInfo.hibernate_support }">
        <div class="capability-icon">
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M10 2C5.58 2 2 5.58 2 10s3.58 8 8 8 8-3.58 8-8-3.58-8-8-8zM9 9a1 1 0 012 0v2a1 1 0 01-2 0V9z" clip-rule="evenodd" />
          </svg>
        </div>
        <div class="capability-info">
          <span class="capability-name">Hibernate</span>
          <span class="capability-status">{{ systemInfo.hibernate_support ? 'Supported' : 'Not Supported' }}</span>
        </div>
      </div>

      <div class="capability-item" :class="{ supported: systemInfo.wake_on_lan_support }">
        <div class="capability-icon">
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path d="M2.003 5.884L10 9.882l7.997-3.998A2 2 0 0016 4H4a2 2 0 00-1.997 1.884z" />
            <path d="M18 8.118l-8 4-8-4V14a2 2 0 002 2h12a2 2 0 002-2V8.118z" />
          </svg>
        </div>
        <div class="capability-info">
          <span class="capability-name">Wake-on-LAN</span>
          <span class="capability-status">{{ systemInfo.wake_on_lan_support ? 'Supported' : 'Not Supported' }}</span>
        </div>
      </div>

      <div class="capability-item" :class="{ supported: systemInfo.power_switch_support }">
        <div class="capability-icon">
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M10 2a1 1 0 011 1v6a1 1 0 11-2 0V3a1 1 0 011-1zm4.293 1.293a1 1 0 011.414 1.414l-3 3a1 1 0 01-1.414-1.414l3-3zm-8.586 0l3 3a1 1 0 11-1.414 1.414l-3-3a1 1 0 111.414-1.414z" clip-rule="evenodd" />
          </svg>
        </div>
        <div class="capability-info">
          <span class="capability-name">Power Switch</span>
          <span class="capability-status">{{ systemInfo.power_switch_support ? 'Supported' : 'Not Supported' }}</span>
        </div>
      </div>

      <div class="capability-item" :class="{ supported: systemInfo.power_meter_support }">
        <div class="capability-icon">
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
        <div class="capability-info">
          <span class="capability-name">Power Meter</span>
          <span class="capability-status">{{ systemInfo.power_meter_support ? 'Hardware' : 'Software Est.' }}</span>
        </div>
      </div>

      <div v-if="systemInfo.wake_on_lan && systemInfo.wake_on_lan.supported" class="capability-item wol-details">
        <div class="capability-icon">
          <svg class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M12.395 2.553a1 1 0 00-1.45-.385c-.345.23-.614.558-.822.88-.214.33-.403.713-.57 1.116-.334.804-.614 1.768-.84 2.734a31.365 31.365 0 00-.613 3.58 2.64 2.64 0 01-.945-1.067c-.328-.68-.398-1.534-.398-2.654A1 1 0 005.05 6.05 6.981 6.981 0 003 11a7 7 0 1011.95-4.95c-.592-.591-.98-.985-1.348-1.467-.363-.476-.724-1.063-1.207-2.03zM12.12 15.12A3 3 0 017 13s.879.5 2.5.5c0-1 .5-4 1.25-4.5.5 1 .786 1.293 1.371 1.879A2.99 2.99 0 0113 13a2.99 2.99 0 01-.879 2.121z" clip-rule="evenodd" />
          </svg>
        </div>
        <div class="capability-info">
          <span class="capability-name">WoL Status</span>
          <span class="capability-status">{{ systemInfo.wake_on_lan.armed ? 'Armed' : 'Disarmed' }}</span>
        </div>
      </div>
    </div>

    <!-- WoL Interface Details -->
    <div v-if="systemInfo.wake_on_lan && systemInfo.wake_on_lan.interfaces && systemInfo.wake_on_lan.interfaces.length > 0" 
         class="wol-interfaces">
      <h5 class="interfaces-title">WoL Interfaces</h5>
      <div class="interfaces-list">
        <span 
          v-for="interfaceName in systemInfo.wake_on_lan.interfaces" 
          :key="interfaceName"
          class="interface-badge"
        >
          {{ interfaceName }}
        </span>
      </div>
    </div>
  </div>
</template>

<script>
export default {
  name: 'SystemCapabilities',
  props: {
    systemInfo: {
      type: Object,
      required: true
    }
  }
}
</script>

<style scoped>
.system-capabilities {
  background-color: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius);
  padding: 1rem;
}

.capabilities-title {
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid var(--border-color);
}

.capabilities-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(180px, 1fr));
  gap: 0.75rem;
}

.capability-item {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding: 0.75rem;
  border-radius: var(--radius);
  border: 1px solid var(--border-color);
  transition: var(--transition);
}

.capability-item.supported {
  background-color: rgb(34 197 94 / 0.05);
  border-color: rgb(34 197 94 / 0.2);
}

.capability-item:not(.supported) {
  background-color: rgb(107 114 128 / 0.05);
  border-color: rgb(107 114 128 / 0.2);
  opacity: 0.7;
}

.capability-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border-radius: 50%;
  flex-shrink: 0;
}

.capability-item.supported .capability-icon {
  background-color: rgb(34 197 94 / 0.1);
  color: var(--primary-color);
}

.capability-item:not(.supported) .capability-icon {
  background-color: rgb(107 114 128 / 0.1);
  color: #6b7280;
}

.capability-info {
  display: flex;
  flex-direction: column;
  gap: 0.125rem;
  flex: 1;
}

.capability-name {
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--text-primary);
}

.capability-status {
  font-size: 0.625rem;
  color: var(--text-secondary);
  font-weight: 500;
}

.capability-item.supported .capability-status {
  color: var(--primary-dark);
}

.wol-details {
  grid-column: 1 / -1;
}

/* WoL Interfaces */
.wol-interfaces {
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border-color);
}

.interfaces-title {
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--text-secondary);
  margin-bottom: 0.5rem;
  text-transform: uppercase;
  letter-spacing: 0.025em;
}

.interfaces-list {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.interface-badge {
  display: inline-flex;
  align-items: center;
  padding: 0.25rem 0.5rem;
  background-color: var(--bg-secondary);
  color: var(--text-primary);
  border-radius: var(--radius);
  font-size: 0.75rem;
  font-weight: 500;
  font-family: monospace;
  border: 1px solid var(--border-color);
}

/* Responsive adjustments */
@media (max-width: 768px) {
  .capabilities-grid {
    grid-template-columns: 1fr;
  }
  
  .capability-item {
    padding: 0.5rem;
  }
  
  .capability-icon {
    width: 1.5rem;
    height: 1.5rem;
  }
}
</style>
