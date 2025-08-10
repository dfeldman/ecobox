package metrics

import (
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Metric represents a single metric data point
type Metric struct {
	Timestamp time.Time
	Name      string
	Value     float64
}

// Store handles metrics storage and retrieval
type Store struct {
	mu           sync.RWMutex
	dataDir      string
	buffer       []Metric
	flushTicker  *time.Ticker
	stopChan     chan struct{}
	flushOnClose bool
}

// Config holds configuration for the metrics store
type Config struct {
	DataDir       string        // Directory to store metric files
	FlushInterval time.Duration // How often to flush buffer to disk
}

// ManagerConfig holds configuration for the metrics manager
type ManagerConfig struct {
	BaseDataDir   string        // Base directory for all server metrics
	FlushInterval time.Duration // How often to flush buffer to disk
}

// Manager handles metrics for multiple servers
type Manager struct {
	mu          sync.RWMutex
	baseDataDir string
	servers     map[string]*Store
	config      ManagerConfig
}

// NewManager creates a new metrics manager for multiple servers
func NewManager(config ManagerConfig) (*Manager, error) {
	if config.FlushInterval == 0 {
		config.FlushInterval = 5 * time.Minute // Default flush every 5 minutes
	}

	// Create base data directory if it doesn't exist
	if err := os.MkdirAll(config.BaseDataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base data directory: %w", err)
	}

	return &Manager{
		baseDataDir: config.BaseDataDir,
		servers:     make(map[string]*Store),
		config:      config,
	}, nil
}

// GetStore returns or creates a store for the specified server
func (m *Manager) GetStore(serverName string) (*Store, error) {
	m.mu.RLock()
	if store, exists := m.servers[serverName]; exists {
		m.mu.RUnlock()
		return store, nil
	}
	m.mu.RUnlock()

	// Need to create new store
	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check in case another goroutine created it
	if store, exists := m.servers[serverName]; exists {
		return store, nil
	}

	// Create server-specific data directory
	serverDataDir := filepath.Join(m.baseDataDir, serverName)
	storeConfig := Config{
		DataDir:       serverDataDir,
		FlushInterval: m.config.FlushInterval,
	}

	store, err := NewStore(storeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create store for server %s: %w", serverName, err)
	}

	m.servers[serverName] = store
	return store, nil
}

// Push adds a metric for the specified server
func (m *Manager) Push(serverName, metricName string, value float64) error {
	store, err := m.GetStore(serverName)
	if err != nil {
		return err
	}
	store.Push(metricName, value)
	return nil
}

// GetSummary gets summary for a specific server's metric
func (m *Manager) GetSummary(serverName, metricName string, startTime, endTime time.Time, timePeriodSec int) ([]Summary, error) {
	m.mu.RLock()
	store, exists := m.servers[serverName]
	m.mu.RUnlock()

	if !exists {
		return []Summary{}, nil // No data for this server
	}

	return store.GetSummary(metricName, startTime, endTime, timePeriodSec)
}

// GetServerNames returns list of all server names with data
func (m *Manager) GetServerNames() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ForceFlushAll flushes all server stores
func (m *Manager) ForceFlushAll() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for serverName, store := range m.servers {
		if err := store.ForceFlush(); err != nil {
			return fmt.Errorf("failed to flush server %s: %w", serverName, err)
		}
	}
	return nil
}

// ForceFlushServer flushes a specific server's store
func (m *Manager) ForceFlushServer(serverName string) error {
	m.mu.RLock()
	store, exists := m.servers[serverName]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("server %s not found", serverName)
	}

	return store.ForceFlush()
}

// GetTotalBufferSize returns total buffered metrics across all servers
func (m *Manager) GetTotalBufferSize() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	total := 0
	for _, store := range m.servers {
		total += store.GetBufferSize()
	}
	return total
}

// GetServerBufferSize returns buffered metrics for a specific server
func (m *Manager) GetServerBufferSize(serverName string) int {
	m.mu.RLock()
	store, exists := m.servers[serverName]
	m.mu.RUnlock()

	if !exists {
		return 0
	}

	return store.GetBufferSize()
}

// Close gracefully shuts down all stores
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastError error
	for serverName, store := range m.servers {
		if err := store.Close(); err != nil {
			lastError = fmt.Errorf("failed to close server %s: %w", serverName, err)
		}
	}

	return lastError
}

// Summary represents aggregated metrics over a time period
type Summary struct {
	MetricName    string
	StartTime     time.Time
	EndTime       time.Time
	Average       float64
	Count         int64
	TimePeriodSec int
}

// NewStore creates a new metrics store
func NewStore(config Config) (*Store, error) {
	if config.FlushInterval == 0 {
		config.FlushInterval = 5 * time.Minute // Default flush every 5 minutes
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	store := &Store{
		dataDir:      config.DataDir,
		buffer:       make([]Metric, 0, 1000), // Pre-allocate for efficiency
		stopChan:     make(chan struct{}),
		flushOnClose: true,
	}

	// Start background flushing
	store.flushTicker = time.NewTicker(config.FlushInterval)
	go store.flushLoop()

	return store, nil
}

// Push adds a metric to the store (rounded to 2 decimal places)
func (s *Store) Push(name string, value float64) {
	// Round to 2 decimal places
	roundedValue := math.Round(value*100) / 100

	metric := Metric{
		Timestamp: time.Now(),
		Name:      name,
		Value:     roundedValue,
	}

	s.mu.Lock()
	s.buffer = append(s.buffer, metric)
	s.mu.Unlock()
}

// flushLoop runs the background flush process
func (s *Store) flushLoop() {
	for {
		select {
		case <-s.flushTicker.C:
			s.flush()
		case <-s.stopChan:
			return
		}
	}
}

// flush writes buffered metrics to disk
func (s *Store) flush() error {
	s.mu.Lock()
	if len(s.buffer) == 0 {
		s.mu.Unlock()
		return nil
	}

	// Copy buffer and clear it
	toFlush := make([]Metric, len(s.buffer))
	copy(toFlush, s.buffer)
	s.buffer = s.buffer[:0] // Clear but keep capacity
	s.mu.Unlock()

	// Group metrics by date
	metricsByDate := make(map[string][]Metric)
	for _, metric := range toFlush {
		dateStr := metric.Timestamp.Format("2006-01-02")
		metricsByDate[dateStr] = append(metricsByDate[dateStr], metric)
	}

	// Write each date's metrics
	for dateStr, metrics := range metricsByDate {
		if err := s.writeMetricsToFile(dateStr, metrics); err != nil {
			return fmt.Errorf("failed to write metrics for date %s: %w", dateStr, err)
		}
	}

	return nil
}

// writeMetricsToFile writes metrics to a gzipped CSV file for a specific date
func (s *Store) writeMetricsToFile(dateStr string, metrics []Metric) error {
	filename := filepath.Join(s.dataDir, fmt.Sprintf("%s.csv.gz", dateStr))
	
	// Check if file exists to determine if we need to write headers
	fileExists := false
	if _, err := os.Stat(filename); err == nil {
		fileExists = true
	}

	// Open file for append
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	gzipWriter := gzip.NewWriter(file)
	defer gzipWriter.Close()

	csvWriter := csv.NewWriter(gzipWriter)
	defer csvWriter.Flush()

	// Write header if new file
	if !fileExists {
		if err := csvWriter.Write([]string{"timestamp", "metric_name", "value"}); err != nil {
			return fmt.Errorf("failed to write header: %w", err)
		}
	}

	// Sort metrics by timestamp for consistent ordering
	sort.Slice(metrics, func(i, j int) bool {
		return metrics[i].Timestamp.Before(metrics[j].Timestamp)
	})

	// Write metrics
	for _, metric := range metrics {
		record := []string{
			strconv.FormatInt(metric.Timestamp.Unix(), 10),
			metric.Name,
			fmt.Sprintf("%.2f", metric.Value),
		}
		if err := csvWriter.Write(record); err != nil {
			return fmt.Errorf("failed to write metric: %w", err)
		}
	}

	return nil
}

// GetSummary calculates averages for metrics over specified time periods
func (s *Store) GetSummary(metricName string, startTime, endTime time.Time, timePeriodSec int) ([]Summary, error) {
	// First flush any pending metrics
	if err := s.flush(); err != nil {
		return nil, fmt.Errorf("failed to flush pending metrics: %w", err)
	}

	// Determine which files we need to read
	filesToRead := s.getFilesInRange(startTime, endTime)
	
	// Read all metrics from relevant files
	allMetrics, err := s.readMetricsFromFiles(filesToRead, metricName, startTime, endTime)
	if err != nil {
		return nil, fmt.Errorf("failed to read metrics: %w", err)
	}

	if len(allMetrics) == 0 {
		return []Summary{}, nil
	}

	// Group metrics into time periods and calculate averages
	return s.calculateSummaries(allMetrics, metricName, startTime, endTime, timePeriodSec), nil
}

// getFilesInRange returns list of files that might contain data in the time range
func (s *Store) getFilesInRange(startTime, endTime time.Time) []string {
	var files []string
	
	// Iterate through each day in the range
	current := time.Date(startTime.Year(), startTime.Month(), startTime.Day(), 0, 0, 0, 0, startTime.Location())
	endDate := time.Date(endTime.Year(), endTime.Month(), endTime.Day(), 23, 59, 59, 999999999, endTime.Location())
	
	for current.Before(endDate) || current.Equal(endDate) {
		filename := filepath.Join(s.dataDir, fmt.Sprintf("%s.csv.gz", current.Format("2006-01-02")))
		if _, err := os.Stat(filename); err == nil {
			files = append(files, filename)
		}
		current = current.AddDate(0, 0, 1)
	}
	
	return files
}

// readMetricsFromFiles reads metrics from specified files within time range
func (s *Store) readMetricsFromFiles(files []string, metricName string, startTime, endTime time.Time) ([]Metric, error) {
	var allMetrics []Metric

	for _, filename := range files {
		metrics, err := s.readMetricsFromFile(filename, metricName, startTime, endTime)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %w", filename, err)
		}
		allMetrics = append(allMetrics, metrics...)
	}

	// Sort by timestamp
	sort.Slice(allMetrics, func(i, j int) bool {
		return allMetrics[i].Timestamp.Before(allMetrics[j].Timestamp)
	})

	return allMetrics, nil
}

// readMetricsFromFile reads and filters metrics from a single file
func (s *Store) readMetricsFromFile(filename, metricName string, startTime, endTime time.Time) ([]Metric, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gzipReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzipReader.Close()

	csvReader := csv.NewReader(gzipReader)
	
	var metrics []Metric
	isFirstRow := true

	for {
		record, err := csvReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV record: %w", err)
		}

		// Skip header row
		if isFirstRow {
			isFirstRow = false
			continue
		}

		// Parse record
		if len(record) != 3 {
			continue // Skip malformed records
		}

		timestamp, err := strconv.ParseInt(record[0], 10, 64)
		if err != nil {
			continue // Skip records with invalid timestamps
		}

		name := record[1]
		if name != metricName {
			continue // Skip metrics we're not interested in
		}

		value, err := strconv.ParseFloat(record[2], 64)
		if err != nil {
			continue // Skip records with invalid values
		}

		metricTime := time.Unix(timestamp, 0)
		
		// Filter by time range
		if metricTime.Before(startTime) || metricTime.After(endTime) {
			continue
		}

		metrics = append(metrics, Metric{
			Timestamp: metricTime,
			Name:      name,
			Value:     value,
		})
	}

	return metrics, nil
}

// calculateSummaries groups metrics into time periods and calculates averages
func (s *Store) calculateSummaries(metrics []Metric, metricName string, startTime, endTime time.Time, timePeriodSec int) []Summary {
	if len(metrics) == 0 {
		return []Summary{}
	}

	var summaries []Summary
	periodDuration := time.Duration(timePeriodSec) * time.Second

	// Align start time to period boundary
	alignedStart := time.Unix((startTime.Unix()/int64(timePeriodSec))*int64(timePeriodSec), 0)
	
	currentPeriodStart := alignedStart
	
	for currentPeriodStart.Before(endTime) {
		currentPeriodEnd := currentPeriodStart.Add(periodDuration)
		if currentPeriodEnd.After(endTime) {
			currentPeriodEnd = endTime
		}

		// Find metrics in this period
		var periodMetrics []Metric
		for _, metric := range metrics {
			if !metric.Timestamp.Before(currentPeriodStart) && metric.Timestamp.Before(currentPeriodEnd) {
				periodMetrics = append(periodMetrics, metric)
			}
		}

		// Calculate average if we have metrics
		if len(periodMetrics) > 0 {
			sum := 0.0
			for _, metric := range periodMetrics {
				sum += metric.Value
			}
			average := sum / float64(len(periodMetrics))

			summaries = append(summaries, Summary{
				MetricName:    metricName,
				StartTime:     currentPeriodStart,
				EndTime:       currentPeriodEnd,
				Average:       math.Round(average*100) / 100, // Round to 2 decimal places
				Count:         int64(len(periodMetrics)),
				TimePeriodSec: timePeriodSec,
			})
		}

		currentPeriodStart = currentPeriodEnd
	}

	return summaries
}

// Close gracefully shuts down the store
func (s *Store) Close() error {
	// Stop the flush loop
	s.flushTicker.Stop()
	close(s.stopChan)

	// Final flush if enabled
	if s.flushOnClose {
		return s.flush()
	}

	return nil
}

// ForceFlush immediately writes all buffered metrics to disk
func (s *Store) ForceFlush() error {
	return s.flush()
}

// GetBufferSize returns the current number of buffered metrics
func (s *Store) GetBufferSize() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.buffer)
}
