package kvstore

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
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
}

func New() *KVStore {
	return NewWithConfig(CompactionConfig{
		Enabled:         true,
		MaxFileSize:     DefaultMaxFileSize,
		MaxOperations:   DefaultMaxOperations,
		CompactionRatio: DefaultCompactionRatio,
	})
}

func NewWithConfig(config CompactionConfig) *KVStore {
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

	return &KVStore{
		dataDir:          dataDir,
		logFile:          filepath.Join(dataDir, LogFileName),
		memoryMap:        make(map[string]string),
		isLoaded:         false,
		compactionConfig: config,
		operationCount:   0,
		lastCompaction:   0,
		isCompacting:     false,
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

	// Increment operation count and check for auto-compaction
	kv.operationCount++
	kv.triggerAutoCompactionIfNeeded()

	return nil
}

func (kv *KVStore) Get(key string) (string, error) {
	kv.mu.RLock()
	defer kv.mu.RUnlock()

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

	reader := NewLogReader(kv.logFile)
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
	maps.Copy(result, kv.memoryMap)
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
	reader := NewLogReader(kv.logFile)
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

// getCurrentTimestamp returns current Unix timestamp
func getCurrentTimestamp() int64 {
	return getCurrentTime().Unix()
}

// getCurrentTime returns current time (for easier testing)
var getCurrentTime = func() time.Time {
	return time.Now()
}
