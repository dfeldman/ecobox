<template>
  <div class="recent-actions">
    <h4 class="actions-title">Recent Actions</h4>
    
    <div v-if="!actions || actions.length === 0" class="no-actions">
      <div class="no-actions-icon">
        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      </div>
      <p class="no-actions-text">No recent actions</p>
    </div>

    <div v-else class="actions-list">
      <div
        v-for="(action, index) in sortedActions"
        :key="index"
        class="action-item"
        :class="{
          'action-success': action.success,
          'action-failed': !action.success
        }"
      >
        <div class="action-icon">
          <svg v-if="action.success" class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd" />
          </svg>
          <svg v-else class="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
            <path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" />
          </svg>
        </div>

        <div class="action-content">
          <div class="action-header">
            <span class="action-type">{{ formatActionType(action.action) }}</span>
            <span class="action-time">{{ formatTimestamp(action.timestamp) }}</span>
          </div>
          
          <div class="action-details">
            <div class="action-status">
              <span class="action-result">{{ action.success ? 'Success' : 'Failed' }}</span>
              <span v-if="action.initiated_by" class="action-initiator">by {{ action.initiated_by }}</span>
            </div>
            
            <div v-if="action.error_msg" class="action-error">
              {{ action.error_msg }}
            </div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="showMoreButton" class="actions-footer">
      <button @click="showAll = !showAll" class="btn btn-secondary btn-sm">
        {{ showAll ? 'Show Less' : `Show All (${actions.length})` }}
      </button>
    </div>
  </div>
</template>

<script>
import { computed, ref } from 'vue'

export default {
  name: 'RecentActions',
  props: {
    actions: {
      type: Array,
      default: () => []
    },
    maxVisible: {
      type: Number,
      default: 5
    }
  },
  setup(props) {
    const showAll = ref(false)
    
    const sortedActions = computed(() => {
      if (!props.actions) return []
      
      // Sort actions by timestamp (newest first)
      const sorted = [...props.actions].sort((a, b) => {
        return new Date(b.timestamp) - new Date(a.timestamp)
      })
      
      return showAll.value ? sorted : sorted.slice(0, props.maxVisible)
    })
    
    const showMoreButton = computed(() => {
      return props.actions && props.actions.length > props.maxVisible
    })
    
    const formatActionType = (actionType) => {
      switch (actionType) {
        case 'wake': return 'Wake Up'
        case 'suspend': return 'Suspend'
        case 'initialize': return 'Initialize'
        case 'reconcile': return 'Reconcile'
        default: return actionType.charAt(0).toUpperCase() + actionType.slice(1)
      }
    }
    
    const formatTimestamp = (timestamp) => {
      if (!timestamp) return 'Unknown'
      
      const date = new Date(timestamp)
      const now = new Date()
      const diffMs = now - date
      const diffSeconds = Math.floor(diffMs / 1000)
      const diffMinutes = Math.floor(diffSeconds / 60)
      const diffHours = Math.floor(diffMinutes / 60)
      const diffDays = Math.floor(diffHours / 24)
      
      if (diffDays > 7) {
        return date.toLocaleDateString()
      } else if (diffDays > 0) {
        return `${diffDays}d ago`
      } else if (diffHours > 0) {
        return `${diffHours}h ago`
      } else if (diffMinutes > 0) {
        return `${diffMinutes}m ago`
      } else if (diffSeconds > 0) {
        return `${diffSeconds}s ago`
      } else {
        return 'Just now'
      }
    }
    
    return {
      showAll,
      sortedActions,
      showMoreButton,
      formatActionType,
      formatTimestamp
    }
  }
}
</script>

<style scoped>
.recent-actions {
  background-color: var(--bg-primary);
  border: 1px solid var(--border-color);
  border-radius: var(--radius);
  padding: 1rem;
}

.actions-title {
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text-primary);
  margin-bottom: 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid var(--border-color);
}

/* No actions state */
.no-actions {
  text-align: center;
  padding: 2rem 1rem;
  color: var(--text-secondary);
}

.no-actions-icon {
  margin-bottom: 0.5rem;
  opacity: 0.5;
}

.no-actions-text {
  font-size: 0.875rem;
  margin: 0;
}

/* Actions list */
.actions-list {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.action-item {
  display: flex;
  align-items: flex-start;
  gap: 0.75rem;
  padding: 0.75rem;
  border-radius: var(--radius);
  border: 1px solid var(--border-color);
  transition: var(--transition);
}

.action-item:hover {
  background-color: var(--bg-secondary);
}

.action-success {
  border-left: 3px solid var(--primary-color);
}

.action-failed {
  border-left: 3px solid #ef4444;
}

.action-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.5rem;
  height: 1.5rem;
  border-radius: 50%;
  flex-shrink: 0;
}

.action-success .action-icon {
  background-color: rgb(34 197 94 / 0.1);
  color: var(--primary-color);
}

.action-failed .action-icon {
  background-color: rgb(239 68 68 / 0.1);
  color: #ef4444;
}

.action-content {
  flex: 1;
  min-width: 0;
}

.action-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 0.25rem;
}

.action-type {
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text-primary);
}

.action-time {
  font-size: 0.75rem;
  color: var(--text-secondary);
  flex-shrink: 0;
  margin-left: 1rem;
}

.action-details {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
}

.action-status {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.75rem;
}

.action-result {
  font-weight: 500;
}

.action-success .action-result {
  color: var(--primary-color);
}

.action-failed .action-result {
  color: #ef4444;
}

.action-initiator {
  color: var(--text-secondary);
}

.action-error {
  font-size: 0.75rem;
  color: #ef4444;
  background-color: rgb(239 68 68 / 0.05);
  padding: 0.375rem 0.5rem;
  border-radius: var(--radius);
  border: 1px solid rgb(239 68 68 / 0.2);
  margin-top: 0.25rem;
}

/* Footer */
.actions-footer {
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border-color);
  text-align: center;
}

/* Responsive adjustments */
@media (max-width: 768px) {
  .action-header {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.25rem;
  }
  
  .action-time {
    margin-left: 0;
  }
  
  .action-status {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.25rem;
  }
}
</style>
