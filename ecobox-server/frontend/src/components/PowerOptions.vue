<template>
  <div class="card">
    <div class="card-header">
      <h3 class="font-semibold">Power Management Options</h3>
      <button 
        @click="toggleEditing" 
        class="btn btn-sm"
        :class="editing ? 'btn-secondary' : 'btn-primary'"
      >
        {{ editing ? 'Cancel' : 'Configure' }}
      </button>
    </div>
    <div class="card-body">
      <div v-if="!editing" class="space-y-4">
        <!-- Read-only view -->
        <div class="power-options-summary">
          <div class="info-item">
            <span class="info-label">Suspend Method:</span>
            <span class="info-value">
              <span v-if="displayOptions.suspend_method === 'none'" class="status-indicator status-off">
                Disabled
              </span>
              <span v-else class="status-indicator status-on">
                {{ displayOptions.suspend_method }}
              </span>
            </span>
          </div>
          
          <div class="info-item">
            <span class="info-label">Manual Controls:</span>
            <span class="info-value">
              <span v-if="displayOptions.allow_shutdown || displayOptions.allow_restart">
                <span v-if="displayOptions.allow_shutdown">Shutdown</span>
                <span v-if="displayOptions.allow_shutdown && displayOptions.allow_restart">, </span>
                <span v-if="displayOptions.allow_restart">Restart</span>
              </span>
              <span v-else class="text-muted">None</span>
            </span>
          </div>

          <div v-if="displayOptions.wake_times && displayOptions.wake_times.length > 0" class="info-item">
            <span class="info-label">Wake Schedule:</span>
            <div class="info-value">
              <div v-for="(wakeTime, index) in displayOptions.wake_times" :key="index" class="wake-time-item">
                {{ formatWakeTime(wakeTime) }}
              </div>
            </div>
          </div>

          <div v-if="hasAutoSuspendConditions" class="info-item">
            <span class="info-label">Auto-Suspend Conditions:</span>
            <div class="info-value">
              <ul class="auto-suspend-list">
                <li v-if="displayOptions.use_cpu_suspend">
                  CPU below {{ displayOptions.cpu_suspend }}%
                </li>
                <li v-if="displayOptions.use_mem_suspend">
                  Memory below {{ displayOptions.mem_suspend }}%
                </li>
                <li v-if="displayOptions.use_load_suspend">
                  Load below {{ displayOptions.load_suspend }}
                </li>
                <li v-if="displayOptions.use_net_suspend">
                  Network below {{ displayOptions.net_suspend }} kbps
                </li>
              </ul>
              <p class="text-sm text-muted mt-1">All conditions must be met</p>
            </div>
          </div>
        </div>
      </div>

      <div v-else class="space-y-6">
        <!-- Editing form -->
        <form @submit.prevent="handleSave">
          <!-- Basic Settings -->
          <div class="form-section">
            <h4 class="form-section-title">Basic Settings</h4>
            
            <div class="form-group">
              <label class="form-label">Suspend Method</label>
              <select v-model="editingOptions.suspend_method" class="form-control">
                <option value="none">None - Never suspend</option>
                <option value="suspend">Suspend to RAM</option>
              </select>
            </div>

            <div class="form-group">
              <div class="checkbox-group">
                <label class="checkbox-label">
                  <input 
                    type="checkbox" 
                    v-model="editingOptions.allow_shutdown"
                    class="form-checkbox"
                  />
                  <span>Allow manual shutdown</span>
                </label>
              </div>
              <div class="checkbox-group">
                <label class="checkbox-label">
                  <input 
                    type="checkbox" 
                    v-model="editingOptions.allow_restart"
                    class="form-checkbox"
                  />
                  <span>Allow manual restart</span>
                </label>
              </div>
            </div>
          </div>

          <!-- Wake Schedule -->
          <div class="form-section">
            <h4 class="form-section-title">Wake Schedule</h4>
            
            <div class="wake-times-list">
              <div 
                v-for="(wakeTime, index) in editingOptions.wake_times" 
                :key="index" 
                class="wake-time-editor"
              >
                <div class="form-group">
                  <label class="form-label">Time</label>
                  <input 
                    type="time" 
                    v-model="wakeTime.time"
                    class="form-control"
                  />
                </div>
                
                <div class="form-group">
                  <label class="form-label">Days of Week</label>
                  <div class="days-selector">
                    <label 
                      v-for="(day, dayIndex) in daysOfWeek" 
                      :key="dayIndex"
                      class="day-checkbox"
                    >
                      <input 
                        type="checkbox" 
                        :checked="wakeTime.days_of_week.includes(dayIndex)"
                        @change="toggleDay(index, dayIndex)"
                        class="form-checkbox"
                      />
                      <span>{{ day.short }}</span>
                    </label>
                  </div>
                </div>
                
                <button 
                  type="button" 
                  @click="removeWakeTime(index)"
                  class="btn btn-danger btn-sm"
                >
                  Remove
                </button>
              </div>
              
              <button 
                type="button" 
                @click="addWakeTime"
                class="btn btn-secondary btn-sm"
              >
                Add Wake Time
              </button>
            </div>
          </div>

          <!-- Auto-Suspend Conditions -->
          <div v-if="editingOptions.suspend_method !== 'none'" class="form-section">
            <h4 class="form-section-title">Auto-Suspend Conditions</h4>
            <p class="text-sm text-muted mb-4">Server will suspend when ALL enabled conditions are met</p>
            
            <div class="form-group">
              <div class="checkbox-group">
                <label class="checkbox-label">
                  <input 
                    type="checkbox" 
                    v-model="editingOptions.use_cpu_suspend"
                    class="form-checkbox"
                  />
                  <span>Suspend when CPU usage below</span>
                </label>
                <div v-if="editingOptions.use_cpu_suspend" class="threshold-input">
                  <input 
                    type="number" 
                    v-model.number="editingOptions.cpu_suspend"
                    min="0" 
                    max="100"
                    class="form-control form-control-sm"
                  />
                  <span>%</span>
                </div>
              </div>
            </div>

            <div class="form-group">
              <div class="checkbox-group">
                <label class="checkbox-label">
                  <input 
                    type="checkbox" 
                    v-model="editingOptions.use_mem_suspend"
                    class="form-checkbox"
                  />
                  <span>Suspend when memory usage below</span>
                </label>
                <div v-if="editingOptions.use_mem_suspend" class="threshold-input">
                  <input 
                    type="number" 
                    v-model.number="editingOptions.mem_suspend"
                    min="0" 
                    max="100"
                    class="form-control form-control-sm"
                  />
                  <span>%</span>
                </div>
              </div>
            </div>

            <div class="form-group">
              <div class="checkbox-group">
                <label class="checkbox-label">
                  <input 
                    type="checkbox" 
                    v-model="editingOptions.use_load_suspend"
                    class="form-checkbox"
                  />
                  <span>Suspend when load average below</span>
                </label>
                <div v-if="editingOptions.use_load_suspend" class="threshold-input">
                  <input 
                    type="number" 
                    v-model.number="editingOptions.load_suspend"
                    min="0" 
                    step="0.1"
                    class="form-control form-control-sm"
                  />
                </div>
              </div>
            </div>

            <div class="form-group">
              <div class="checkbox-group">
                <label class="checkbox-label">
                  <input 
                    type="checkbox" 
                    v-model="editingOptions.use_net_suspend"
                    class="form-checkbox"
                  />
                  <span>Suspend when network usage below</span>
                </label>
                <div v-if="editingOptions.use_net_suspend" class="threshold-input">
                  <input 
                    type="number" 
                    v-model.number="editingOptions.net_suspend"
                    min="0"
                    class="form-control form-control-sm"
                  />
                  <span>kbps</span>
                </div>
              </div>
            </div>
          </div>

          <!-- Form Actions -->
          <div class="form-actions">
            <button type="submit" class="btn btn-primary" :disabled="saving">
              {{ saving ? 'Saving...' : 'Save Changes' }}
            </button>
            <button type="button" @click="toggleEditing" class="btn btn-secondary">
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script>
import { computed, ref, watch } from 'vue'
import { powerOptionsApi } from '@/services/api'

export default {
  name: 'PowerOptions',
  props: {
    serverId: {
      type: String,
      required: true
    },
    powerOptions: {
      type: Object,
      default: () => null
    }
  },
  emits: ['updated'],
  setup(props, { emit }) {
    const editing = ref(false)
    const saving = ref(false)
    const editingOptions = ref({})
    
    const daysOfWeek = [
      { name: 'Sunday', short: 'Sun' },
      { name: 'Monday', short: 'Mon' },
      { name: 'Tuesday', short: 'Tue' },
      { name: 'Wednesday', short: 'Wed' },
      { name: 'Thursday', short: 'Thu' },
      { name: 'Friday', short: 'Fri' },
      { name: 'Saturday', short: 'Sat' }
    ]

    const displayOptions = computed(() => {
      return props.powerOptions || {
        suspend_method: 'none',
        allow_shutdown: false,
        allow_restart: false,
        wake_times: [],
        use_cpu_suspend: false,
        cpu_suspend: 5,
        use_mem_suspend: false,
        mem_suspend: 20,
        use_load_suspend: false,
        load_suspend: 1,
        use_net_suspend: false,
        net_suspend: 100
      }
    })

    const hasAutoSuspendConditions = computed(() => {
      return displayOptions.value.use_cpu_suspend || 
             displayOptions.value.use_mem_suspend || 
             displayOptions.value.use_load_suspend || 
             displayOptions.value.use_net_suspend
    })

    const initializeEditingOptions = () => {
      editingOptions.value = {
        suspend_method: displayOptions.value.suspend_method,
        allow_shutdown: displayOptions.value.allow_shutdown,
        allow_restart: displayOptions.value.allow_restart,
        wake_times: (displayOptions.value.wake_times || []).map(wt => ({
          time: extractTime(wt.time),
          days_of_week: [...(wt.days_of_week || [])]
        })),
        use_cpu_suspend: displayOptions.value.use_cpu_suspend,
        cpu_suspend: displayOptions.value.cpu_suspend,
        use_mem_suspend: displayOptions.value.use_mem_suspend,
        mem_suspend: displayOptions.value.mem_suspend,
        use_load_suspend: displayOptions.value.use_load_suspend,
        load_suspend: displayOptions.value.load_suspend,
        use_net_suspend: displayOptions.value.use_net_suspend,
        net_suspend: displayOptions.value.net_suspend
      }
    }

    const extractTime = (timeString) => {
      if (!timeString) return '07:00'
      // Extract HH:MM from datetime string or return as-is
      const match = timeString.match(/(\d{2}):(\d{2})/)
      return match ? `${match[1]}:${match[2]}` : '07:00'
    }

    const formatWakeTime = (wakeTime) => {
      if (!wakeTime) return ''
      
      const time = extractTime(wakeTime.time)
      const days = (wakeTime.days_of_week || [])
        .map(dayIndex => daysOfWeek[dayIndex]?.short || '')
        .filter(day => day)
        .join(', ')
      
      return `${time} on ${days || 'No days selected'}`
    }

    const toggleEditing = () => {
      editing.value = !editing.value
      if (editing.value) {
        initializeEditingOptions()
      }
    }

    const addWakeTime = () => {
      editingOptions.value.wake_times.push({
        time: '07:00',
        days_of_week: [1, 2, 3, 4, 5] // Monday to Friday by default
      })
    }

    const removeWakeTime = (index) => {
      editingOptions.value.wake_times.splice(index, 1)
    }

    const toggleDay = (wakeTimeIndex, dayIndex) => {
      const wakeTime = editingOptions.value.wake_times[wakeTimeIndex]
      const dayIdx = wakeTime.days_of_week.indexOf(dayIndex)
      
      if (dayIdx > -1) {
        wakeTime.days_of_week.splice(dayIdx, 1)
      } else {
        wakeTime.days_of_week.push(dayIndex)
      }
    }

    const handleSave = async () => {
      saving.value = true
      
      try {
        const response = await powerOptionsApi.setPowerOptions(props.serverId, editingOptions.value)
        
        if (response.success) {
          editing.value = false
          emit('updated', response.data)
        } else {
          console.error('Failed to save power options:', response.message)
          // TODO: Show error toast
        }
      } catch (error) {
        console.error('Error saving power options:', error)
        // TODO: Show error toast
      } finally {
        saving.value = false
      }
    }

    // Watch for changes to power options prop
    watch(() => props.powerOptions, () => {
      if (!editing.value) {
        initializeEditingOptions()
      }
    }, { deep: true })

    return {
      editing,
      saving,
      editingOptions,
      displayOptions,
      hasAutoSuspendConditions,
      daysOfWeek,
      toggleEditing,
      addWakeTime,
      removeWakeTime,
      toggleDay,
      handleSave,
      formatWakeTime
    }
  }
}
</script>

<style scoped>
.power-options-summary .info-item {
  display: flex;
  justify-content: space-between;
  padding: 0.5rem 0;
  border-bottom: 1px solid var(--border-color);
}

.power-options-summary .info-item:last-child {
  border-bottom: none;
}

.info-label {
  font-weight: 500;
  color: var(--text-primary);
}

.info-value {
  color: var(--text-secondary);
}

.wake-time-item {
  font-size: 0.9rem;
  margin-bottom: 0.25rem;
}

.wake-time-item:last-child {
  margin-bottom: 0;
}

.auto-suspend-list {
  margin: 0;
  padding-left: 1.5rem;
  list-style-type: disc;
}

.auto-suspend-list li {
  margin-bottom: 0.25rem;
  font-size: 0.9rem;
}

.form-section {
  margin-bottom: 2rem;
  padding-bottom: 1.5rem;
  border-bottom: 1px solid var(--border-color);
}

.form-section:last-of-type {
  border-bottom: none;
}

.form-section-title {
  font-size: 1.1rem;
  font-weight: 600;
  margin-bottom: 1rem;
  color: var(--text-primary);
}

.form-group {
  margin-bottom: 1.5rem;
}

.form-label {
  display: block;
  font-weight: 500;
  margin-bottom: 0.5rem;
  color: var(--text-primary);
}

.form-control {
  width: 100%;
  padding: 0.5rem;
  border: 1px solid var(--border-color);
  border-radius: 0.375rem;
  background-color: var(--bg-primary);
  color: var(--text-primary);
}

.form-control-sm {
  padding: 0.25rem 0.5rem;
  font-size: 0.875rem;
  width: auto;
}

.checkbox-group {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 0.75rem;
}

.checkbox-label {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-weight: normal;
  margin-bottom: 0;
  cursor: pointer;
}

.form-checkbox {
  width: auto;
}

.threshold-input {
  display: flex;
  align-items: center;
  gap: 0.5rem;
}

.wake-times-list {
  margin-bottom: 1rem;
}

.wake-time-editor {
  padding: 1rem;
  border: 1px solid var(--border-color);
  border-radius: 0.5rem;
  margin-bottom: 1rem;
  background-color: var(--bg-secondary);
}

.days-selector {
  display: flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.day-checkbox {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.25rem;
  padding: 0.5rem;
  border: 1px solid var(--border-color);
  border-radius: 0.25rem;
  cursor: pointer;
  font-size: 0.875rem;
  min-width: 3rem;
  text-align: center;
}

.day-checkbox:hover {
  background-color: var(--bg-hover);
}

.day-checkbox input:checked + span {
  color: var(--primary-color);
  font-weight: 500;
}

.form-actions {
  display: flex;
  gap: 1rem;
  padding-top: 1.5rem;
  border-top: 1px solid var(--border-color);
}

.status-indicator {
  padding: 0.25rem 0.5rem;
  border-radius: 0.25rem;
  font-size: 0.875rem;
  font-weight: 500;
}

.status-on {
  background-color: #10b981;
  color: white;
}

.status-off {
  background-color: #6b7280;
  color: white;
}

.text-muted {
  color: var(--text-light);
}
</style>
