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
	dataDir string
	logFile string
	mu      sync.RWMutex
}

func New() *KVStore {
	dataDir := DefaultDataDir
	if envDir := os.Getenv("MOZ_DATA_DIR"); envDir != "" {
		dataDir = envDir
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		panic(fmt.Sprintf("Failed to create data directory: %v", err))
	}

	return &KVStore{
		dataDir: dataDir,
		logFile: filepath.Join(dataDir, LogFileName),
	}
}

func (kv *KVStore) Put(key, value string) error {
	// Validate key format (legacy compatibility)
	if err := ValidateKey(key); err != nil {
		return err
	}

	kv.mu.Lock()
	defer kv.mu.Unlock()

	file, err := os.OpenFile(kv.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Use TAB-delimited format for legacy compatibility
	logEntry := fmt.Sprintf("%s\t%s\n", key, value)
	if _, err := file.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write to log: %w", err)
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

	file, err := os.OpenFile(kv.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Use TAB-delimited format with __DELETED__ marker for legacy compatibility
	logEntry := fmt.Sprintf("%s\t__DELETED__\n", key)
	if _, err := file.WriteString(logEntry); err != nil {
		return fmt.Errorf("failed to write to log: %w", err)
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
	file, err := os.Create(tempFile)
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
			os.Remove(tempFile)
			return fmt.Errorf("failed to write to temp file: %w", err)
		}
	}

	if err := os.Rename(tempFile, kv.logFile); err != nil {
		os.Remove(tempFile)
		return fmt.Errorf("failed to replace log file: %w", err)
	}

	return nil
}

func (kv *KVStore) buildCurrentState() (map[string]string, error) {
	// Use the new LogReader for better format compatibility
	reader := NewLogReader(kv.logFile)
	return reader.ReadAll()
}
