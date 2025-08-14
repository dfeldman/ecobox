<template>
  <div class="space-y-6">
    <div class="card">
      <div class="card-header">
        <div class="flex items-center justify-between">
          <h3 class="font-semibold">Metrics - {{ serverName }}</h3>
          <div class="flex items-center gap-4">
            <select
              v-model="selectedTimeRange"
              @change="updateTimeRange"
              class="form-input w-auto text-sm"
            >
              <option value="1h">Last Hour</option>
              <option value="6h">Last 6 Hours</option>
              <option value="24h">Last 24 Hours</option>
              <option value="7d">Last 7 Days</option>
              <option value="30d">Last 30 Days</option>
            </select>
            <button
              @click="refreshMetrics"
              class="btn btn-secondary btn-sm"
              :disabled="loading"
            >
              <span v-if="loading" class="loading w-3 h-3 mr-1"></span>
              Refresh
            </button>
          </div>
        </div>
      </div>
      <div class="card-body">
        <div v-if="loading && !hasData" class="text-center py-8">
          <div class="loading w-8 h-8 mx-auto"></div>
          <p class="mt-4 text-gray-600">Loading metrics...</p>
        </div>

        <div v-else-if="error" class="text-center py-8">
          <p class="text-red-500">{{ error }}</p>
          <button @click="refreshMetrics" class="btn btn-primary mt-4">
            Retry
          </button>
        </div>

        <div v-else-if="!hasData" class="text-center py-8">
          <p class="text-gray-600">No metrics data available</p>
        </div>

        <div v-else>
          <!-- Charts Container -->
          <div class="metrics-charts">
            <div class="grid grid-cols-1 md:grid-cols-2 gap-6">
              <!-- Memory Usage Chart -->
              <div class="metric-card">
                <div class="metric-header">
                  <div class="metric-title">Memory Usage</div>
                  <div class="metric-value">{{ getLatestValue('memory') }}%</div>
                </div>
                <div class="chart-container">
                  <Scatter
                    v-if="chartData.memory.datasets && chartData.memory.datasets[0].data.length > 0"
                    :data="chartData.memory"
                    :options="chartOptions.memory"
                  />
                  <div v-else class="no-data">No data available</div>
                </div>
              </div>

              <!-- CPU Usage Chart -->
              <div class="metric-card">
                <div class="metric-header">
                  <div class="metric-title">CPU Usage</div>
                  <div class="metric-value">{{ getLatestValue('cpu') }}%</div>
                </div>
                <div class="chart-container">
                  <Scatter
                    v-if="chartData.cpu.datasets && chartData.cpu.datasets[0].data.length > 0"
                    :data="chartData.cpu"
                    :options="chartOptions.cpu"
                  />
                  <div v-else class="no-data">No data available</div>
                </div>
              </div>

              <!-- Network Usage Chart -->
              <div class="metric-card">
                <div class="metric-header">
                  <div class="metric-title">Network Usage</div>
                  <div class="metric-value">{{ getLatestValue('network') }} MB/s</div>
                </div>
                <div class="chart-container">
                  <Scatter
                    v-if="chartData.network.datasets && chartData.network.datasets[0].data.length > 0"
                    :data="chartData.network"
                    :options="chartOptions.network"
                  />
                  <div v-else class="no-data">No data available</div>
                </div>
              </div>

              <!-- Power Consumption Chart -->
              <div class="metric-card">
                <div class="metric-header">
                  <div class="metric-title">Power Consumption</div>
                  <div class="metric-value">{{ getLatestValue('wattage') }} W</div>
                </div>
                <div class="chart-container">
                  <Scatter
                    v-if="chartData.wattage.datasets && chartData.wattage.datasets[0].data.length > 0"
                    :data="chartData.wattage"
                    :options="chartOptions.wattage"
                  />
                  <div v-else class="no-data">No data available</div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { metricsApi } from '@/services/api'
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  TimeScale,
  Filler
} from 'chart.js'
import { Scatter } from 'vue-chartjs'
import 'chartjs-adapter-date-fns'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  TimeScale,
  Filler
)

export default {
  name: 'MetricsCharts',
  components: {
    Scatter
  },
  props: {
    serverId: {
      type: String,
      required: true
    },
    serverName: {
      type: String,
      required: true
    }
  },
  setup(props) {
    const selectedTimeRange = ref('24h')
    const loading = ref(false)
    const error = ref(null)
    const metricsData = ref(null)
    
    const hasData = computed(() => {
      return metricsData.value && Object.keys(metricsData.value).length > 0
    })

    // Chart configuration
    const metricConfigs = {
      memory: { 
        title: 'Memory Usage', 
        unit: '%', 
        color: '#3b82f6',
        backgroundColor: 'rgba(59, 130, 246, 0.1)' 
      },
      cpu: { 
        title: 'CPU Usage', 
        unit: '%', 
        color: '#10b981',
        backgroundColor: 'rgba(16, 185, 129, 0.1)' 
      },
      network: { 
        title: 'Network Usage', 
        unit: 'MB/s', 
        color: '#f59e0b',
        backgroundColor: 'rgba(245, 158, 11, 0.1)' 
      },
      wattage: { 
        title: 'Power Consumption', 
        unit: 'W', 
        color: '#ef4444',
        backgroundColor: 'rgba(239, 68, 68, 0.1)' 
      }
    }

    // Chart data reactive refs
    const chartData = ref({
      memory: { datasets: [] },
      cpu: { datasets: [] },
      network: { datasets: [] },
      wattage: { datasets: [] }
    })

    // Chart options reactive refs
    const chartOptions = ref({})

    const getTimeRange = () => {
      const now = new Date()
      const ranges = {
        '1h': new Date(now.getTime() - 60 * 60 * 1000),
        '6h': new Date(now.getTime() - 6 * 60 * 60 * 1000),
        '24h': new Date(now.getTime() - 24 * 60 * 60 * 1000),
        '7d': new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000),
        '30d': new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000)
      }
      
      return {
        start: ranges[selectedTimeRange.value] || ranges['24h'],
        end: now
      }
    }

    const createChartOptions = (metricKey, metricConfig, timeRange) => {
      return {
        responsive: true,
        maintainAspectRatio: false,
        scales: {
          x: {
            type: 'time',
            min: timeRange.start,
            max: timeRange.end,
            time: {
              displayFormats: {
                hour: 'HH:mm',
                day: 'MM/dd HH:mm',
                week: 'MM/dd',
                month: 'MM/dd'
              }
            },
            title: {
              display: true,
              text: 'Time'
            },
            grid: {
              color: 'rgba(0, 0, 0, 0.1)'
            }
          },
          y: {
            beginAtZero: true,
            title: {
              display: true,
              text: `${metricConfig.title} (${metricConfig.unit})`
            },
            grid: {
              color: 'rgba(0, 0, 0, 0.1)'
            }
          }
        },
        elements: {
          point: {
            radius: 0, // Hide points by default
            hoverRadius: 5
          },
          line: {
            tension: 0.4, // Smooth lines
            borderWidth: 2
          }
        },
        interaction: {
          intersect: false,
          mode: 'index'
        },
        plugins: {
          legend: {
            display: false
          },
          tooltip: {
            mode: 'index',
            intersect: false,
            callbacks: {
              label: function(context) {
                return `${metricConfig.title}: ${context.parsed.y.toFixed(2)} ${metricConfig.unit}`
              }
            }
          }
        },
        showLine: true, // Enable line drawing for scatter chart
        fill: true // Enable area fill
      }
    }

    const processMetricsData = (rawData, timeRange) => {
      const processedData = {}
      
      Object.keys(metricConfigs).forEach(metricKey => {
        const metricConfig = metricConfigs[metricKey]
        const rawMetricData = rawData[metricKey] || []
        
        // Convert data to Chart.js scatter format with time on x-axis
        const scatterData = rawMetricData.map(point => ({
          x: new Date(point.timestamp),
          y: point.value
        }))

        // Sort by time to ensure proper line drawing
        scatterData.sort((a, b) => a.x - b.x)

        processedData[metricKey] = {
          datasets: [{
            label: metricConfig.title,
            data: scatterData,
            borderColor: metricConfig.color,
            backgroundColor: metricConfig.backgroundColor,
            pointBackgroundColor: metricConfig.color,
            pointBorderColor: metricConfig.color,
            tension: 0.4
          }]
        }
      })
      
      return processedData
    }

    const updateChartOptions = (timeRange) => {
      const newOptions = {}
      Object.keys(metricConfigs).forEach(metricKey => {
        newOptions[metricKey] = createChartOptions(metricKey, metricConfigs[metricKey], timeRange)
      })
      chartOptions.value = newOptions
    }

    const loadMetrics = async () => {
      if (!props.serverId) return
      
      loading.value = true
      error.value = null
      
      try {
        const timeRange = getTimeRange()
        console.log('Loading metrics for server:', props.serverId, 'from', timeRange.start.toISOString(), 'to', timeRange.end.toISOString())
        
        const data = await metricsApi.getMetrics(props.serverId, timeRange.start, timeRange.end)
        console.log('Received metrics data:', data)
        
        // Check if we have any data at all
        if (data && typeof data === 'object') {
          for (const [key, value] of Object.entries(data)) {
            console.log(`Metric ${key}:`, Array.isArray(value) ? `${value.length} data points` : 'not an array', value)
          }
        }
        
        metricsData.value = data
        
        if (!data || Object.keys(data).length === 0) {
          console.warn('No metrics data received for server:', props.serverId)
          error.value = 'No metrics data available for this server'
          // Set empty chart data
          chartData.value = {
            memory: { datasets: [] },
            cpu: { datasets: [] },
            network: { datasets: [] },
            wattage: { datasets: [] }
          }
        } else {
          // Process data for charts
          chartData.value = processMetricsData(data, timeRange)
        }
        
        // Update chart options with current time range
        updateChartOptions(timeRange)
        
      } catch (err) {
        error.value = err.response?.data?.message || 'Failed to load metrics'
        console.error('Error loading metrics:', err)
        console.error('Error details:', {
          status: err.response?.status,
          statusText: err.response?.statusText,
          data: err.response?.data,
          url: err.config?.url
        })
      } finally {
        loading.value = false
      }
    }

    const getLatestValue = (metricKey) => {
      const data = metricsData.value?.[metricKey]
      if (!data || !Array.isArray(data) || data.length === 0) {
        return '0.0'
      }
      const latestValue = data[data.length - 1].value
      return latestValue.toFixed(1)
    }
    
    const refreshMetrics = () => {
      loadMetrics()
    }
    
    const updateTimeRange = () => {
      loadMetrics()
    }
    
    watch(() => props.serverId, () => {
      loadMetrics()
    })
    
    onMounted(() => {
      loadMetrics()
    })
    
    return {
      selectedTimeRange,
      loading,
      error,
      hasData,
      chartData,
      chartOptions,
      getLatestValue,
      refreshMetrics,
      updateTimeRange
    }
  }
}
</script>

<style scoped>
.metrics-charts {
  min-height: 400px;
}

.metric-card {
  background: white;
  border-radius: 8px;
  padding: 1.5rem;
  box-shadow: 0 1px 3px 0 rgba(0, 0, 0, 0.1), 0 1px 2px 0 rgba(0, 0, 0, 0.06);
  border: 1px solid #e5e7eb;
}

.metric-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 1rem;
}

.metric-title {
  font-weight: 600;
  color: #374151;
  font-size: 0.875rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.metric-value {
  font-size: 1.5rem;
  font-weight: bold;
  color: #1f2937;
}

.chart-container {
  height: 300px;
  width: 100%;
  position: relative;
}

.no-data {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 100%;
  color: #6b7280;
  font-style: italic;
}

/* Dark mode support */
@media (prefers-color-scheme: dark) {
  .metric-card {
    background: #1f2937;
    border-color: #374151;
  }
  
  .metric-title {
    color: #d1d5db;
  }
  
  .metric-value {
    color: #f9fafb;
  }
  
  .no-data {
    color: #9ca3af;
  }
}
</style>
