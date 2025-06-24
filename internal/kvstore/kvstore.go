package kvstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/nyasuto/moz/internal/index"
)

const (
	DefaultDataDir = "." // Current directory for shell compatibility
	LogFileName    = "moz.log"

	// Auto-compaction thresholds
	DefaultMaxFileSize     = 1024 * 1024 // 1MB
	DefaultMaxOperations   = 1000        // Max operations before compaction
	DefaultCompactionRatio = 0.5         // Compact when deleted entries > 50%
)

// CompactionConfig holds auto-compaction settings
type CompactionConfig struct {
	Enabled         bool    // Enable auto-compaction
	MaxFileSize     int64   // Max file size before compaction (bytes)
	MaxOperations   int     // Max operations before compaction
	CompactionRatio float64 // Compact when deleted ratio exceeds this
}

// StorageConfig holds storage format settings
type StorageConfig struct {
	Format     string // "text" or "binary"
	TextFile   string // Text format log file
	BinaryFile string // Binary format log file
	IndexType  string // "hash", "btree", or "none"
	IndexFile  string // Index persistence file
}

type KVStore struct {
	dataDir   string
	logFile   string
	mu        sync.RWMutex
	memoryMap map[string]string
	isLoaded  bool
	mapMu     sync.RWMutex

	// Auto-compaction fields
	compactionConfig CompactionConfig
	operationCount   int
	lastCompaction   int64      // Unix timestamp
	compactionMu     sync.Mutex // Prevents concurrent compactions
	isCompacting     bool

	// Storage format fields
	storageConfig StorageConfig

	// Index fields
	indexManager *index.IndexManager

	// Memory optimization fields
	memoryOptimizer *MemoryOptimizer
}

func New() *KVStore {
	return NewWithConfig(CompactionConfig{
		Enabled:         true,
		MaxFileSize:     DefaultMaxFileSize,
		MaxOperations:   DefaultMaxOperations,
		CompactionRatio: DefaultCompactionRatio,
	}, StorageConfig{
		Format:     "text", // Default to text for compatibility
		TextFile:   LogFileName,
		BinaryFile: "moz.bin",
		IndexType:  "none", // Default to no indexing for compatibility
		IndexFile:  "moz.idx",
	})
}

func NewWithConfig(compactionConfig CompactionConfig, storageConfig StorageConfig) *KVStore {
	dataDir := DefaultDataDir
	if envDir := os.Getenv("MOZ_DATA_DIR"); envDir != "" {
		dataDir = envDir
	}

	// Only create directory if it's not the current directory
	if dataDir != "." {
		if err := os.MkdirAll(dataDir, 0750); err != nil {
			panic(fmt.Sprintf("Failed to create data directory: %v", err))
		}
	}

	var logFile string
	if storageConfig.Format == "binary" {
		logFile = filepath.Join(dataDir, storageConfig.BinaryFile)
	} else {
		logFile = filepath.Join(dataDir, storageConfig.TextFile)
	}

	// Initialize index manager
	var indexType index.IndexType
	switch storageConfig.IndexType {
	case "hash":
		indexType = index.IndexTypeHash
	case "btree":
		indexType = index.IndexTypeBTree
	default:
		indexType = index.IndexTypeNone
	}

	// Initialize memory optimizer with default config first
	memoryOptimizer := NewMemoryOptimizer(DefaultMemoryPoolConfig())

	indexManager, err := index.NewIndexManagerWithPool(indexType, memoryOptimizer.GetPools())
	if err != nil {
		panic(fmt.Sprintf("Failed to create index manager: %v", err))
	}

	return &KVStore{
		dataDir:          dataDir,
		logFile:          logFile,
		memoryMap:        make(map[string]string),
		isLoaded:         false,
		compactionConfig: compactionConfig,
		storageConfig:    storageConfig,
		operationCount:   0,
		lastCompaction:   0,
		isCompacting:     false,
		indexManager:     indexManager,
		memoryOptimizer:  memoryOptimizer,
	}
}

func (kv *KVStore) Put(key, value string) error {
	// Validate key format (legacy compatibility)
	if err := ValidateKey(key); err != nil {
		return err
	}

	kv.mu.Lock()
	defer kv.mu.Unlock()

	file, err := os.OpenFile(kv.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log close error but don't override main error
			fmt.Printf("Warning: failed to close file: %v\n", closeErr)
		}
	}()

	// Use TAB-delimited format for legacy compatibility
	logEntry := fmt.Sprintf("%s\t%s\n", key, value)
	if _, err := file.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write to log: %w", err)
	}

	// Update memory map after successful write
	if err := kv.updateMemoryMap(key, value); err != nil {
		return fmt.Errorf("failed to update memory map: %w", err)
	}

	// Update index if enabled
	if kv.indexManager.IsEnabled() {
		// Get file offset for the entry (approximate)
		fileInfo, _ := file.Stat()
		offset := fileInfo.Size() - int64(len(logEntry))

		indexEntry := index.IndexEntry{
			Key:       key,
			Offset:    offset,
			Size:      int32(len(logEntry)),
			Timestamp: time.Now().UnixNano(),
			Deleted:   false,
		}

		if err := kv.indexManager.Insert(key, indexEntry); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to update index: %v\n", err)
		}
	}

	// Increment operation count and check for auto-compaction
	kv.operationCount++
	kv.triggerAutoCompactionIfNeeded()

	return nil
}

func (kv *KVStore) Get(key string) (string, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	// If index is enabled and available, try index lookup first
	if kv.indexManager.IsEnabled() && kv.indexManager.Exists(key) {
		// For now, we still need to fall back to memory map
		// since the index doesn't store the actual values
		// TODO: Implement direct file reading using index offset
		_ = key // Prevent staticcheck SA9003 warning about empty branch
	}

	data, err := kv.buildCurrentState()
	if err != nil {
		return "", err
	}

	value, exists := data[key]
	if !exists {
		return "", fmt.Errorf("key not found: %s", key)
	}

	return value, nil
}

func (kv *KVStore) Delete(key string) error {
	// Validate key format (legacy compatibility)
	if err := ValidateKey(key); err != nil {
		return err
	}

	kv.mu.Lock()
	defer kv.mu.Unlock()

	data, err := kv.buildCurrentState()
	if err != nil {
		return err
	}

	if _, exists := data[key]; !exists {
		return fmt.Errorf("key not found: %s", key)
	}

	file, err := os.OpenFile(kv.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log close error but don't override main error
			fmt.Printf("Warning: failed to close file: %v\n", closeErr)
		}
	}()

	// Use TAB-delimited format with __DELETED__ marker for legacy compatibility
	logEntry := fmt.Sprintf("%s\t__DELETED__\n", key)
	if _, err := file.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write to log: %w", err)
	}

	// Update memory map after successful write
	if err := kv.updateMemoryMap(key, "__DELETED__"); err != nil {
		return fmt.Errorf("failed to update memory map: %w", err)
	}

	// Update index if enabled
	if kv.indexManager.IsEnabled() {
		if err := kv.indexManager.Delete(key); err != nil {
			// Log error but don't fail the operation
			fmt.Printf("Warning: failed to update index for deletion: %v\n", err)
		}
	}

	// Increment operation count and check for auto-compaction
	kv.operationCount++
	kv.triggerAutoCompactionIfNeeded()

	return nil
}

func (kv *KVStore) List() ([]string, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	data, err := kv.buildCurrentState()
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys, nil
}

func (kv *KVStore) Compact() error {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	data, err := kv.buildCurrentState()
	if err != nil {
		return err
	}

	tempFile := kv.logFile + ".tmp"
	file, err := os.Create(tempFile) // #nosec G304 - safe controlled temp file creation
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log close error but don't override main error
			fmt.Printf("Warning: failed to close temp file: %v\n", closeErr)
		}
	}()

	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		// Use TAB-delimited format for legacy compatibility
		logEntry := fmt.Sprintf("%s\t%s\n", key, data[key])
		if _, err := file.WriteString(logEntry); err != nil {
			_ = os.Remove(tempFile) // Best effort cleanup
			return fmt.Errorf("failed to write to temp file: %w", err)
		}
	}

	if err := os.Rename(tempFile, kv.logFile); err != nil {
		_ = os.Remove(tempFile) // Best effort cleanup
		return fmt.Errorf("failed to replace log file: %w", err)
	}

	// Reload memory map after compaction
	kv.mapMu.Lock()
	kv.isLoaded = false
	kv.mapMu.Unlock()

	return nil
}

// loadMemoryMap loads the current state from disk into memory
func (kv *KVStore) loadMemoryMap() error {
	kv.mapMu.Lock()
	defer kv.mapMu.Unlock()

	if kv.isLoaded {
		return nil
	}

	// Use shared memory pools if available
	var reader *LogReader
	if kv.memoryOptimizer != nil {
		reader = NewLogReaderWithPools(kv.logFile, kv.memoryOptimizer.GetPools())
	} else {
		reader = NewLogReader(kv.logFile)
	}

	data, err := reader.ReadAll()
	if err != nil {
		return err
	}

	kv.memoryMap = data
	kv.isLoaded = true
	return nil
}

// getMemoryMap returns a copy of the current memory map
func (kv *KVStore) getMemoryMap() (map[string]string, error) {
	if err := kv.loadMemoryMap(); err != nil {
		return nil, err
	}

	kv.mapMu.RLock()
	defer kv.mapMu.RUnlock()

	// Return a copy to prevent external modifications
	result := make(map[string]string, len(kv.memoryMap))
	for k, v := range kv.memoryMap {
		result[k] = v
	}
	return result, nil
}

// updateMemoryMap updates the in-memory map with a key-value pair
func (kv *KVStore) updateMemoryMap(key, value string) error {
	if err := kv.loadMemoryMap(); err != nil {
		return err
	}

	kv.mapMu.Lock()
	defer kv.mapMu.Unlock()

	if value == "__DELETED__" {
		delete(kv.memoryMap, key)
	} else {
		kv.memoryMap[key] = value
	}
	return nil
}

// Stats returns statistics about the current memory map
type Stats struct {
	MemoryMapSize int  `json:"memory_map_size"`
	IsLoaded      bool `json:"is_loaded"`
}

// GetStats returns current statistics about the KVStore
func (kv *KVStore) GetStats() (Stats, error) {
	if err := kv.loadMemoryMap(); err != nil {
		return Stats{}, err
	}

	kv.mapMu.RLock()
	defer kv.mapMu.RUnlock()

	return Stats{
		MemoryMapSize: len(kv.memoryMap),
		IsLoaded:      kv.isLoaded,
	}, nil
}

func (kv *KVStore) buildCurrentState() (map[string]string, error) {
	return kv.getMemoryMap()
}

// shouldTriggerCompaction checks if auto-compaction should be triggered (thread-safe)
func (kv *KVStore) shouldTriggerCompaction() bool {
	if !kv.compactionConfig.Enabled {
		return false
	}

	// Check operation count threshold
	if kv.operationCount >= kv.compactionConfig.MaxOperations {
		return true
	}

	// Check file size threshold
	if fileInfo, err := os.Stat(kv.logFile); err == nil {
		if fileInfo.Size() >= kv.compactionConfig.MaxFileSize {
			return true
		}
	}

	// Check compaction ratio (deleted entries ratio)
	if ratio, err := kv.calculateDeletedRatio(); err == nil {
		if ratio >= kv.compactionConfig.CompactionRatio {
			return true
		}
	}

	return false
}

// triggerAutoCompactionIfNeeded triggers auto-compaction if conditions are met
func (kv *KVStore) triggerAutoCompactionIfNeeded() {
	// Check if auto-compaction should be triggered (but don't perform it yet)
	shouldCompact := kv.shouldTriggerCompaction()

	if shouldCompact {
		kv.compactionMu.Lock()
		if !kv.isCompacting {
			kv.isCompacting = true
			kv.compactionMu.Unlock()

			// Perform compaction asynchronously to avoid deadlock
			go func() {
				defer func() {
					kv.compactionMu.Lock()
					kv.isCompacting = false
					kv.compactionMu.Unlock()
				}()

				fmt.Printf("🗜️ Auto-compaction triggered (ops: %d, ratio: %.2f)\n",
					kv.operationCount, kv.getCurrentDeletedRatio())
				if err := kv.performCompactionAsync(); err != nil {
					fmt.Printf("Warning: auto-compaction failed: %v\n", err)
				}
			}()
		} else {
			kv.compactionMu.Unlock()
		}
	}
}

// calculateDeletedRatio calculates the ratio of deleted entries in the log
func (kv *KVStore) calculateDeletedRatio() (float64, error) {
	// Use shared memory pools if available
	var reader *LogReader
	if kv.memoryOptimizer != nil {
		reader = NewLogReaderWithPools(kv.logFile, kv.memoryOptimizer.GetPools())
	} else {
		reader = NewLogReader(kv.logFile)
	}

	entries, err := reader.ReadAllEntries()
	if err != nil {
		return 0, err
	}

	if len(entries) == 0 {
		return 0, nil
	}

	deletedCount := 0
	for _, entry := range entries {
		if entry.Value == "__DELETED__" {
			deletedCount++
		}
	}

	return float64(deletedCount) / float64(len(entries)), nil
}

// getCurrentDeletedRatio gets current deleted ratio for logging
func (kv *KVStore) getCurrentDeletedRatio() float64 {
	ratio, _ := kv.calculateDeletedRatio()
	return ratio
}

// performCompactionAsync performs the actual compaction operation asynchronously
func (kv *KVStore) performCompactionAsync() error {
	// Use the existing Compact method which is already thread-safe
	if err := kv.Compact(); err != nil {
		return err
	}

	// Reset operation counter and update last compaction time
	kv.mu.Lock()
	kv.operationCount = 0
	kv.lastCompaction = getCurrentTimestamp()
	kv.mu.Unlock()

	return nil
}

// GetCompactionStats returns statistics about compaction
type CompactionStats struct {
	Enabled          bool    `json:"enabled"`
	OperationCount   int     `json:"operation_count"`
	LastCompaction   int64   `json:"last_compaction"`
	DeletedRatio     float64 `json:"deleted_ratio"`
	FileSize         int64   `json:"file_size"`
	NextCompactionAt int     `json:"next_compaction_at"`
}

// MemoryStats holds memory optimization statistics
type MemoryStats struct {
	PoolStats           MemoryPoolStats `json:"pool_stats"`
	Efficiency          PoolEfficiency  `json:"efficiency"`
	GCStats             interface{}     `json:"gc_stats"`
	MemoryUsage         interface{}     `json:"memory_usage"`
	OptimizationEnabled bool            `json:"optimization_enabled"`
}

func (kv *KVStore) GetCompactionStats() (CompactionStats, error) {
	ratio, _ := kv.calculateDeletedRatio()

	var fileSize int64
	if fileInfo, err := os.Stat(kv.logFile); err == nil {
		fileSize = fileInfo.Size()
	}

	return CompactionStats{
		Enabled:          kv.compactionConfig.Enabled,
		OperationCount:   kv.operationCount,
		LastCompaction:   kv.lastCompaction,
		DeletedRatio:     ratio,
		FileSize:         fileSize,
		NextCompactionAt: kv.compactionConfig.MaxOperations - kv.operationCount,
	}, nil
}

// SetCompactionConfig updates the compaction configuration
func (kv *KVStore) SetCompactionConfig(config CompactionConfig) {
	kv.compactionConfig = config
}

// GetMemoryStats returns memory optimization statistics
func (kv *KVStore) GetMemoryStats() MemoryStats {
	if kv.memoryOptimizer == nil {
		return MemoryStats{OptimizationEnabled: false}
	}

	pools := kv.memoryOptimizer.GetPools()
	poolStats := pools.GetStats()
	efficiency := pools.GetEfficiency()

	// Get additional GC stats
	gcStats := poolStats.LastGCStats
	memStats := poolStats.MemoryUsage

	return MemoryStats{
		PoolStats:           poolStats,
		Efficiency:          efficiency,
		GCStats:             gcStats,
		MemoryUsage:         memStats,
		OptimizationEnabled: true,
	}
}

// OptimizeMemory performs manual memory optimization
func (kv *KVStore) OptimizeMemory() {
	if kv.memoryOptimizer != nil {
		kv.memoryOptimizer.GetPools().OptimizeGC()
	}
}

// ForceGC forces garbage collection and returns memory stats
func (kv *KVStore) ForceGC() (before, after interface{}) {
	if kv.memoryOptimizer != nil {
		beforeStats, afterStats := kv.memoryOptimizer.GetPools().ForceGC()
		return beforeStats, afterStats
	}
	return nil, nil
}

// GetDetailedMemoryStats returns comprehensive memory statistics including GC metrics
func (kv *KVStore) GetDetailedMemoryStats() map[string]interface{} {
	stats := make(map[string]interface{})

	if kv.memoryOptimizer == nil {
		stats["optimization_enabled"] = false
		return stats
	}

	pools := kv.memoryOptimizer.GetPools()
	poolStats := pools.GetStats()
	efficiency := pools.GetEfficiency()

	// Pool statistics
	stats["pool_stats"] = map[string]interface{}{
		"log_entry_gets":      poolStats.LogEntryGets,
		"log_entry_puts":      poolStats.LogEntryPuts,
		"buffer_gets":         poolStats.BufferGets,
		"buffer_puts":         poolStats.BufferPuts,
		"index_entry_gets":    poolStats.IndexEntryGets,
		"index_entry_puts":    poolStats.IndexEntryPuts,
		"total_allocations":   poolStats.TotalAllocations,
		"total_deallocations": poolStats.TotalDeallocations,
		"last_update":         poolStats.LastUpdate,
	}

	// Efficiency metrics
	stats["efficiency"] = map[string]interface{}{
		"log_entry_efficiency":   efficiency.LogEntryEfficiency,
		"buffer_efficiency":      efficiency.BufferEfficiency,
		"index_entry_efficiency": efficiency.IndexEntryEfficiency,
		"overall_efficiency":     efficiency.OverallEfficiency,
		"pool_hit_rate":          efficiency.PoolHitRate,
	}

	// Memory usage stats
	memStats := poolStats.MemoryUsage
	stats["memory_usage"] = map[string]interface{}{
		"alloc":           memStats.Alloc,
		"total_alloc":     memStats.TotalAlloc,
		"sys":             memStats.Sys,
		"lookups":         memStats.Lookups,
		"mallocs":         memStats.Mallocs,
		"frees":           memStats.Frees,
		"heap_alloc":      memStats.HeapAlloc,
		"heap_sys":        memStats.HeapSys,
		"heap_idle":       memStats.HeapIdle,
		"heap_inuse":      memStats.HeapInuse,
		"heap_released":   memStats.HeapReleased,
		"heap_objects":    memStats.HeapObjects,
		"stack_inuse":     memStats.StackInuse,
		"stack_sys":       memStats.StackSys,
		"gc_cpu_fraction": memStats.GCCPUFraction,
	}

	// GC statistics
	gcStats := poolStats.LastGCStats
	stats["gc_stats"] = map[string]interface{}{
		"last_gc":         gcStats.LastGC,
		"num_gc":          gcStats.NumGC,
		"pause_total":     gcStats.PauseTotal,
		"pause":           gcStats.Pause,
		"pause_end":       gcStats.PauseEnd,
		"pause_quantiles": gcStats.PauseQuantiles,
	}

	// KVStore specific stats
	kvStats, _ := kv.GetStats()
	compactionStats, _ := kv.GetCompactionStats()
	indexStats, _ := kv.GetIndexStats()

	stats["kvstore_stats"] = map[string]interface{}{
		"memory_map_size": kvStats.MemoryMapSize,
		"is_loaded":       kvStats.IsLoaded,
	}

	stats["compaction_stats"] = map[string]interface{}{
		"enabled":            compactionStats.Enabled,
		"operation_count":    compactionStats.OperationCount,
		"last_compaction":    compactionStats.LastCompaction,
		"deleted_ratio":      compactionStats.DeletedRatio,
		"file_size":          compactionStats.FileSize,
		"next_compaction_at": compactionStats.NextCompactionAt,
	}

	stats["index_stats"] = indexStats
	stats["optimization_enabled"] = true

	return stats
}

// GetRange returns entries within the specified key range [start, end]
// This method leverages indexing when available for efficient range queries
func (kv *KVStore) GetRange(start, end string) (map[string]string, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	result := make(map[string]string)

	// If index is enabled, use it for efficient range queries
	if kv.indexManager.IsEnabled() {
		indexEntries, err := kv.indexManager.Range(start, end)
		if err != nil {
			return nil, fmt.Errorf("index range query failed: %w", err)
		}

		// Get current state to resolve actual values
		data, err := kv.buildCurrentState()
		if err != nil {
			return nil, err
		}

		// Map index results to actual values
		for _, entry := range indexEntries {
			if value, exists := data[entry.Key]; exists {
				result[entry.Key] = value
			}
		}
		return result, nil
	}

	// Fall back to full scan when index is not available
	data, err := kv.buildCurrentState()
	if err != nil {
		return nil, err
	}

	for key, value := range data {
		if key >= start && key <= end {
			result[key] = value
		}
	}

	return result, nil
}

// ListSorted returns all keys in sorted order
// This method leverages indexing when available for efficient sorted access
func (kv *KVStore) ListSorted() ([]string, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	// If index is enabled, use it for efficient sorted access
	if kv.indexManager.IsEnabled() {
		keys := kv.indexManager.Keys()

		// Filter out deleted keys
		data, err := kv.buildCurrentState()
		if err != nil {
			return nil, err
		}

		var validKeys []string
		for _, key := range keys {
			if _, exists := data[key]; exists {
				validKeys = append(validKeys, key)
			}
		}
		return validKeys, nil
	}

	// Fall back to regular List method
	return kv.List()
}

// PrefixSearch returns all keys and values with the specified prefix
func (kv *KVStore) PrefixSearch(prefix string) (map[string]string, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

	result := make(map[string]string)

	// If index is enabled, use it for efficient prefix search
	if kv.indexManager.IsEnabled() {
		indexEntries, err := kv.indexManager.Prefix(prefix)
		if err != nil {
			return nil, fmt.Errorf("index prefix search failed: %w", err)
		}

		// Get current state to resolve actual values
		data, err := kv.buildCurrentState()
		if err != nil {
			return nil, err
		}

		// Map index results to actual values
		for _, entry := range indexEntries {
			if value, exists := data[entry.Key]; exists {
				result[entry.Key] = value
			}
		}
		return result, nil
	}

	// Fall back to full scan when index is not available
	data, err := kv.buildCurrentState()
	if err != nil {
		return nil, err
	}

	for key, value := range data {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			result[key] = value
		}
	}

	return result, nil
}

// GetIndexStats returns statistics about the index
func (kv *KVStore) GetIndexStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	stats["enabled"] = kv.indexManager.IsEnabled()
	stats["type"] = string(kv.indexManager.GetIndexType())

	if kv.indexManager.IsEnabled() {
		stats["size"] = kv.indexManager.Size()
		stats["memory_usage"] = kv.indexManager.MemoryUsage()
	} else {
		stats["size"] = 0
		stats["memory_usage"] = 0
	}

	return stats, nil
}

// RebuildIndex rebuilds the index from the current data
func (kv *KVStore) RebuildIndex() error {
	if !kv.indexManager.IsEnabled() {
		return fmt.Errorf("index is not enabled")
	}

	kv.mu.Lock()
	defer kv.mu.Unlock()

	// Get current state
	data, err := kv.buildCurrentState()
	if err != nil {
		return fmt.Errorf("failed to build current state: %w", err)
	}

	// Convert to index entries
	indexEntries := make(map[string]index.IndexEntry)
	for key, value := range data {
		// For rebuilt index, we don't have exact offset/size info
		// These will be approximated
		indexEntries[key] = index.IndexEntry{
			Key:       key,
			Offset:    0,                                // Will be recalculated if needed
			Size:      int32(len(key) + len(value) + 2), // Approximate
			Timestamp: time.Now().UnixNano(),
			Deleted:   false,
		}
	}

	// Rebuild the index
	return kv.indexManager.Rebuild(indexEntries)
}

// ValidateIndex validates the integrity of the index
func (kv *KVStore) ValidateIndex() error {
	if !kv.indexManager.IsEnabled() {
		return nil // No validation needed if index is disabled
	}

	return kv.indexManager.Validate()
}

// getCurrentTimestamp returns current Unix timestamp
func getCurrentTimestamp() int64 {
	return getCurrentTime().Unix()
}

// getCurrentTime returns current time (for easier testing)
var getCurrentTime = func() time.Time {
	return time.Now()
}
