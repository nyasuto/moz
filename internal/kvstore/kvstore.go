package kvstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

const (
	DefaultDataDir = "/tmp/moz_data"
	LogFileName    = "moz.log"
)

type KVStore struct {
	dataDir   string
	logFile   string
	mu        sync.RWMutex
	memoryMap map[string]string
	isLoaded  bool
	mapMu     sync.RWMutex
}

func New() *KVStore {
	dataDir := DefaultDataDir
	if envDir := os.Getenv("MOZ_DATA_DIR"); envDir != "" {
		dataDir = envDir
	}

	if err := os.MkdirAll(dataDir, 0750); err != nil {
		panic(fmt.Sprintf("Failed to create data directory: %v", err))
	}

	return &KVStore{
		dataDir:   dataDir,
		logFile:   filepath.Join(dataDir, LogFileName),
		memoryMap: make(map[string]string),
		isLoaded:  false,
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
	defer file.Close()

	// Use TAB-delimited format for legacy compatibility
	logEntry := fmt.Sprintf("%s\t%s\n", key, value)
	if _, err := file.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write to log: %w", err)
	}

	// Update memory map after successful write
	if err := kv.updateMemoryMap(key, value); err != nil {
		return fmt.Errorf("failed to update memory map: %w", err)
	}

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
	defer file.Close()

	// Use TAB-delimited format with __DELETED__ marker for legacy compatibility
	logEntry := fmt.Sprintf("%s\t__DELETED__\n", key)
	if _, err := file.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write to log: %w", err)
	}

	// Update memory map after successful write
	if err := kv.updateMemoryMap(key, "__DELETED__"); err != nil {
		return fmt.Errorf("failed to update memory map: %w", err)
	}

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
	defer file.Close()

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
