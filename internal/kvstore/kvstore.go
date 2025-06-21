package kvstore

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	kv.mu.Lock()
	defer kv.mu.Unlock()

	file, err := os.OpenFile(kv.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	logEntry := fmt.Sprintf("PUT %s %s\n", key, value)
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

	logEntry := fmt.Sprintf("DEL %s\n", key)
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
		logEntry := fmt.Sprintf("PUT %s %s\n", key, data[key])
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
	data := make(map[string]string)

	file, err := os.Open(kv.logFile)
	if os.IsNotExist(err) {
		return data, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 {
			continue
		}

		operation := parts[0]
		key := parts[1]

		switch operation {
		case "PUT":
			if len(parts) == 3 {
				data[key] = parts[2]
			}
		case "DEL":
			delete(data, key)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return data, nil
}