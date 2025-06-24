package kvstore

import (
	"fmt"
	"sync"
	"time"
)

// AsyncKVStore extends KVStore with asynchronous I/O and WAL capabilities
type AsyncKVStore struct {
	*KVStore // Embed existing KVStore for compatibility

	// Async components
	wal      *WAL
	memTable *MemTable

	// Background workers
	flushWorker   *BackgroundWorker
	compactWorker *BackgroundWorker

	// Configuration
	asyncConfig AsyncConfig

	// Synchronization
	shutdownCh   chan struct{}
	shutdownOnce sync.Once
	wg           sync.WaitGroup
}

// AsyncConfig holds configuration for async operations
type AsyncConfig struct {
	WALConfig       WALConfig
	MemTableConfig  MemTableConfig
	FlushInterval   time.Duration
	CompactInterval time.Duration
	EnableAsync     bool // If false, falls back to sync behavior
}

// DefaultAsyncConfig returns default async configuration
func DefaultAsyncConfig() AsyncConfig {
	return AsyncConfig{
		WALConfig:       DefaultWALConfig(),
		MemTableConfig:  DefaultMemTableConfig(),
		FlushInterval:   5 * time.Second,
		CompactInterval: 30 * time.Second,
		EnableAsync:     true,
	}
}

// AsyncResult represents the result of an async operation
type AsyncResult struct {
	LSN   uint64
	Error error
	Done  chan struct{}
}

// NewAsyncResult creates a new AsyncResult
func NewAsyncResult() *AsyncResult {
	return &AsyncResult{
		Done: make(chan struct{}),
	}
}

// Wait waits for the async operation to complete
func (ar *AsyncResult) Wait() error {
	<-ar.Done
	return ar.Error
}

// Complete marks the async operation as complete
func (ar *AsyncResult) Complete(lsn uint64, err error) {
	ar.LSN = lsn
	ar.Error = err
	close(ar.Done)
}

// NewAsyncKVStore creates a new AsyncKVStore
func NewAsyncKVStore(config AsyncConfig) (*AsyncKVStore, error) {
	// Create base KVStore
	baseStore := NewWithConfig(
		CompactionConfig{
			Enabled:         false, // Disable auto-compaction, we'll handle it
			MaxFileSize:     DefaultMaxFileSize,
			MaxOperations:   DefaultMaxOperations,
			CompactionRatio: DefaultCompactionRatio,
		},
		StorageConfig{
			Format:     "text",
			TextFile:   LogFileName,
			BinaryFile: "moz.bin",
			IndexType:  "none",
			IndexFile:  "moz.idx",
		},
	)

	// Create WAL
	wal, err := NewWAL(config.WALConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create WAL: %w", err)
	}

	// Create MemTable
	memTable := NewMemTable(config.MemTableConfig)

	asyncStore := &AsyncKVStore{
		KVStore:     baseStore,
		wal:         wal,
		memTable:    memTable,
		asyncConfig: config,
		shutdownCh:  make(chan struct{}),
	}

	// Start background workers if async is enabled
	if config.EnableAsync {
		asyncStore.flushWorker = NewBackgroundWorker("flush", config.FlushInterval, asyncStore.flushTask)
		asyncStore.compactWorker = NewBackgroundWorker("compact", config.CompactInterval, asyncStore.compactTask)

		asyncStore.flushWorker.Start()
		asyncStore.compactWorker.Start()
	}

	return asyncStore, nil
}

// AsyncPut performs an asynchronous put operation
func (as *AsyncKVStore) AsyncPut(key, value string) *AsyncResult {
	result := NewAsyncResult()

	// If async is disabled, fall back to sync
	if !as.asyncConfig.EnableAsync {
		err := as.Put(key, value)
		result.Complete(0, err)
		return result
	}

	// Validate key
	if err := ValidateKey(key); err != nil {
		result.Complete(0, err)
		return result
	}

	go func() {
		// 1. Write to WAL first for durability
		lsn, err := as.wal.Append(OpTypePut, []byte(key), []byte(value))
		if err != nil {
			result.Complete(0, fmt.Errorf("WAL append failed: %w", err))
			return
		}

		// 2. Update MemTable
		as.memTable.Put(key, value, lsn)

		// 3. Check if MemTable needs flushing
		if as.memTable.ShouldFlush(as.asyncConfig.MemTableConfig) {
			as.triggerFlush()
		}

		result.Complete(lsn, nil)
	}()

	return result
}

// AsyncDelete performs an asynchronous delete operation
func (as *AsyncKVStore) AsyncDelete(key string) *AsyncResult {
	result := NewAsyncResult()

	// If async is disabled, fall back to sync
	if !as.asyncConfig.EnableAsync {
		err := as.Delete(key)
		result.Complete(0, err)
		return result
	}

	// Validate key
	if err := ValidateKey(key); err != nil {
		result.Complete(0, err)
		return result
	}

	go func() {
		// 1. Write to WAL first
		lsn, err := as.wal.Append(OpTypeDelete, []byte(key), nil)
		if err != nil {
			result.Complete(0, fmt.Errorf("WAL append failed: %w", err))
			return
		}

		// 2. Update MemTable with deletion marker
		as.memTable.Delete(key, lsn)

		// 3. Check if MemTable needs flushing
		if as.memTable.ShouldFlush(as.asyncConfig.MemTableConfig) {
			as.triggerFlush()
		}

		result.Complete(lsn, nil)
	}()

	return result
}

// Get reads from MemTable first, then falls back to disk
func (as *AsyncKVStore) Get(key string) (string, error) {
	// 1. Check MemTable first (most recent data)
	if value, found := as.memTable.Get(key); found {
		return value, nil
	}

	// 2. Fall back to base KVStore (disk)
	return as.KVStore.Get(key)
}

// triggerFlush signals the flush worker to flush MemTable
func (as *AsyncKVStore) triggerFlush() {
	if as.flushWorker != nil {
		as.flushWorker.TriggerNow()
	}
}

// flushTask is the background task that flushes MemTable to disk
func (as *AsyncKVStore) flushTask() error {
	// Skip if MemTable is empty
	if as.memTable.IsEmpty() {
		return nil
	}

	// Get all entries from MemTable
	entries := as.memTable.GetAll()
	if len(entries) == 0 {
		return nil
	}

	// Flush entries to base KVStore
	for _, entry := range entries {
		if entry.Deleted {
			// Apply deletion to base store
			if err := as.Delete(entry.Key); err != nil {
				// Log error but continue (deletion of non-existent key is OK)
				fmt.Printf("Warning: delete during flush failed: %v\n", err)
			}
		} else {
			// Apply put to base store
			if err := as.Put(entry.Key, entry.Value); err != nil {
				return fmt.Errorf("put during flush failed: %w", err)
			}
		}
	}

	// Clear MemTable after successful flush
	as.memTable.Clear()

	return nil
}

// compactTask is the background task that performs compaction
func (as *AsyncKVStore) compactTask() error {
	// Use the base KVStore's compaction
	return as.Compact()
}

// FlushMemTable manually flushes the MemTable to disk
func (as *AsyncKVStore) FlushMemTable() error {
	return as.flushTask()
}

// ForceFlush forces both WAL and MemTable to flush
func (as *AsyncKVStore) ForceFlush() error {
	// Flush WAL
	if err := as.wal.Flush(); err != nil {
		return fmt.Errorf("WAL flush failed: %w", err)
	}

	// Flush MemTable
	if err := as.FlushMemTable(); err != nil {
		return fmt.Errorf("MemTable flush failed: %w", err)
	}

	return nil
}

// GetAsyncStats returns statistics about async operations
func (as *AsyncKVStore) GetAsyncStats() map[string]interface{} {
	stats := make(map[string]interface{})

	// WAL stats
	walStats := as.wal.GetStats()
	stats["wal"] = map[string]interface{}{
		"total_entries":   walStats.TotalEntries,
		"bytes_written":   walStats.BytesWritten,
		"flush_count":     walStats.FlushCount,
		"error_count":     walStats.ErrorCount,
		"last_flush_time": walStats.LastFlushTime,
		"average_latency": walStats.AverageLatency,
	}

	// MemTable stats
	memStats := as.memTable.GetStats()
	stats["memtable"] = map[string]interface{}{
		"entries":         memStats.Entries,
		"memory_usage":    memStats.MemoryUsage,
		"put_count":       memStats.PutCount,
		"get_count":       memStats.GetCount,
		"delete_count":    memStats.DeleteCount,
		"flush_count":     memStats.FlushCount,
		"last_flush_time": memStats.LastFlushTime,
	}

	// Worker stats
	if as.flushWorker != nil {
		stats["flush_worker"] = as.flushWorker.GetStats()
	}
	if as.compactWorker != nil {
		stats["compact_worker"] = as.compactWorker.GetStats()
	}

	// Configuration
	stats["config"] = map[string]interface{}{
		"async_enabled":    as.asyncConfig.EnableAsync,
		"flush_interval":   as.asyncConfig.FlushInterval,
		"compact_interval": as.asyncConfig.CompactInterval,
	}

	return stats
}

// Close shuts down the AsyncKVStore gracefully
func (as *AsyncKVStore) Close() error {
	as.shutdownOnce.Do(func() {
		close(as.shutdownCh)

		// Stop background workers
		if as.flushWorker != nil {
			as.flushWorker.Stop()
		}
		if as.compactWorker != nil {
			as.compactWorker.Stop()
		}

		// Final flush
		if err := as.ForceFlush(); err != nil {
			fmt.Printf("Warning: flush during shutdown failed: %v\n", err)
		}

		// Close WAL
		if as.wal != nil {
			if err := as.wal.Close(); err != nil {
				fmt.Printf("Warning: WAL close during shutdown failed: %v\n", err)
			}
		}

		// Wait for all operations to complete
		as.wg.Wait()
	})

	return nil
}

// List returns all keys (from both MemTable and disk)
func (as *AsyncKVStore) List() ([]string, error) {
	// Get keys from MemTable
	memKeys := as.memTable.List()

	// Get keys from disk
	diskKeys, err := as.KVStore.List()
	if err != nil {
		return nil, err
	}

	// Merge and deduplicate
	keySet := make(map[string]bool)
	for _, key := range memKeys {
		keySet[key] = true
	}
	for _, key := range diskKeys {
		keySet[key] = true
	}

	// Convert back to slice
	var allKeys []string
	for key := range keySet {
		// Double-check that key still exists (not deleted)
		if _, err := as.Get(key); err == nil {
			allKeys = append(allKeys, key)
		}
	}

	return allKeys, nil
}
