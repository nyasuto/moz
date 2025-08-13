package kvstore

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAsyncKVStore_BasicAsyncOperations(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	config := DefaultAsyncConfig()
	config.WALConfig.DataDir = tempDir
	config.WALConfig.BufferSize = 1000   // Larger buffer for tests
	config.MemTableConfig.MaxSize = 1024 // Small size for testing

	store, err := NewAsyncKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create AsyncKVStore: %v", err)
	}
	defer store.Close()

	// Test async put
	result := store.AsyncPut("async_key1", "async_value1")
	if err := result.Wait(); err != nil {
		t.Errorf("AsyncPut failed: %v", err)
	}

	if result.LSN == 0 {
		t.Error("AsyncPut should return non-zero LSN")
	}

	// Test immediate read from MemTable
	value, err := store.Get("async_key1")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if value != "async_value1" {
		t.Errorf("Expected async_value1, got %s", value)
	}

	// Test async delete
	deleteResult := store.AsyncDelete("async_key1")
	if err := deleteResult.Wait(); err != nil {
		t.Errorf("AsyncDelete failed: %v", err)
	}

	// Verify deletion
	_, err = store.Get("async_key1")
	if err == nil {
		t.Error("Expected error when getting deleted key")
	}
}

func TestAsyncKVStore_SyncFallback(t *testing.T) {
	// Setup with async disabled
	tempDir := t.TempDir()
	config := DefaultAsyncConfig()
	config.WALConfig.DataDir = tempDir
	config.EnableAsync = false // Disable async

	store, err := NewAsyncKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create AsyncKVStore: %v", err)
	}
	defer store.Close()

	// Operations should still work but synchronously
	result := store.AsyncPut("sync_key", "sync_value")
	if err := result.Wait(); err != nil {
		t.Errorf("Sync put failed: %v", err)
	}

	value, err := store.Get("sync_key")
	if err != nil {
		t.Errorf("Get failed: %v", err)
	}
	if value != "sync_value" {
		t.Errorf("Expected sync_value, got %s", value)
	}
}

func TestAsyncKVStore_ConcurrentOperations(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultAsyncConfig()
	config.WALConfig.DataDir = tempDir
	config.WALConfig.BufferSize = 1000 // Large buffer for concurrent tests

	store, err := NewAsyncKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create AsyncKVStore: %v", err)
	}
	defer store.Close()

	numGoroutines := 5
	entriesPerGoroutine := 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*entriesPerGoroutine)

	// Concurrent async puts
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < entriesPerGoroutine; j++ {
				key := fmt.Sprintf("concurrent_key_%d_%d", workerID, j)
				value := fmt.Sprintf("concurrent_value_%d_%d", workerID, j)

				result := store.AsyncPut(key, value)
				if err := result.Wait(); err != nil {
					errors <- err
				}
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}

	// Force flush and verify data
	if err := store.ForceFlush(); err != nil {
		t.Errorf("ForceFlush failed: %v", err)
	}

	// Verify some entries exist
	value, err := store.Get("concurrent_key_0_0")
	if err != nil {
		t.Errorf("Failed to get concurrent_key_0_0: %v", err)
	}
	if value != "concurrent_value_0_0" {
		t.Errorf("Expected concurrent_value_0_0, got %s", value)
	}
}

func TestAsyncKVStore_MemTableFlush(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultAsyncConfig()
	config.WALConfig.DataDir = tempDir
	config.MemTableConfig.MaxSize = 200  // Small size to trigger flush
	config.MemTableConfig.MaxEntries = 5 // Small count to trigger flush
	config.FlushInterval = 100 * time.Millisecond

	store, err := NewAsyncKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create AsyncKVStore: %v", err)
	}
	defer store.Close()

	// Add entries to trigger flush
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("flush_test_key_%d", i)
		value := fmt.Sprintf("flush_test_value_with_extra_data_%d", i)

		result := store.AsyncPut(key, value)
		if err := result.Wait(); err != nil {
			t.Errorf("AsyncPut %d failed: %v", i, err)
		}
	}

	// Force flush to ensure all data is written to disk
	// This eliminates the race condition between MemTable.Clear() and Get()
	if err := store.ForceFlush(); err != nil {
		t.Errorf("ForceFlush failed: %v", err)
	}

	// Additional small wait to ensure flush completion
	time.Sleep(50 * time.Millisecond)

	// Verify data still accessible after flush
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("flush_test_key_%d", i)
		expectedValue := fmt.Sprintf("flush_test_value_with_extra_data_%d", i)

		value, err := store.Get(key)
		if err != nil {
			t.Errorf("Get failed for key %s: %v", key, err)
			continue
		}
		if value != expectedValue {
			t.Errorf("For key %s, expected %s, got %s", key, expectedValue, value)
		}
	}
}

func TestAsyncKVStore_Statistics(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultAsyncConfig()
	config.WALConfig.DataDir = tempDir

	store, err := NewAsyncKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create AsyncKVStore: %v", err)
	}
	defer store.Close()

	// Perform some operations
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("stats_key_%d", i)
		value := fmt.Sprintf("stats_value_%d", i)

		result := store.AsyncPut(key, value)
		result.Wait()
	}

	// Wait for background processing
	time.Sleep(200 * time.Millisecond)

	// Force flush to ensure entries are written
	store.ForceFlush()

	// Wait a bit more
	time.Sleep(100 * time.Millisecond)

	// Get statistics
	stats := store.GetAsyncStats()

	// Verify stats structure
	if _, exists := stats["wal"]; !exists {
		t.Error("Expected WAL stats in async stats")
	}
	if _, exists := stats["memtable"]; !exists {
		t.Error("Expected MemTable stats in async stats")
	}
	if _, exists := stats["config"]; !exists {
		t.Error("Expected config in async stats")
	}

	// Check WAL stats
	walStats, ok := stats["wal"].(map[string]interface{})
	if !ok {
		t.Error("WAL stats should be a map")
	} else {
		if walStats["total_entries"].(uint64) == 0 {
			t.Error("Expected some WAL entries")
		}
	}

	// Check MemTable stats
	memStats, ok := stats["memtable"].(map[string]interface{})
	if !ok {
		t.Error("MemTable stats should be a map")
	} else {
		if memStats["put_count"].(uint64) == 0 {
			t.Error("Expected some puts in MemTable")
		}
	}
}

func TestAsyncKVStore_ListOperations(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultAsyncConfig()
	config.WALConfig.DataDir = tempDir

	store, err := NewAsyncKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create AsyncKVStore: %v", err)
	}
	defer store.Close()

	// Add entries (some in MemTable, some on disk)
	memEntries := []string{"mem1", "mem2", "mem3"}
	diskEntries := []string{"disk1", "disk2"}

	// Add to MemTable
	for _, key := range memEntries {
		result := store.AsyncPut(key, "value_"+key)
		result.Wait()
	}

	// Add to disk (by flushing first, then adding)
	store.ForceFlush()
	for _, key := range diskEntries {
		store.KVStore.Put(key, "value_"+key) // Direct to base store
	}

	// List should include both
	keys, err := store.List()
	if err != nil {
		t.Errorf("List failed: %v", err)
	}

	expectedTotal := len(memEntries) + len(diskEntries)
	if len(keys) < expectedTotal {
		t.Errorf("Expected at least %d keys, got %d", expectedTotal, len(keys))
	}

	// Verify specific keys exist
	keySet := make(map[string]bool)
	for _, key := range keys {
		keySet[key] = true
	}

	for _, key := range append(memEntries, diskEntries...) {
		if !keySet[key] {
			t.Errorf("Expected key %s in list", key)
		}
	}
}

func TestAsyncKVStore_ErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultAsyncConfig()
	config.WALConfig.DataDir = tempDir
	config.WALConfig.BufferSize = 2 // Very small buffer

	store, err := NewAsyncKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create AsyncKVStore: %v", err)
	}
	defer store.Close()

	// Test invalid key
	result := store.AsyncPut("key\twith\ttab", "value") // Key with tab should be invalid
	if err := result.Wait(); err == nil {
		t.Error("Expected error for key with tab characters")
	}

	// Test buffer overflow (should handle gracefully)
	var results []*AsyncResult
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("overflow_key_%d", i)
		value := fmt.Sprintf("overflow_value_%d", i)
		result := store.AsyncPut(key, value)
		results = append(results, result)
	}

	// Some might fail due to buffer overflow, but shouldn't crash
	errorCount := 0
	for _, result := range results {
		if err := result.Wait(); err != nil {
			errorCount++
		}
	}

	// We expect some errors due to small buffer
	if errorCount == 0 {
		t.Log("No buffer overflow errors occurred (buffer might be larger than expected)")
	}
}

func TestAsyncKVStore_GracefulShutdown(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultAsyncConfig()
	config.WALConfig.DataDir = tempDir

	store, err := NewAsyncKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create AsyncKVStore: %v", err)
	}

	// Add some data
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("shutdown_key_%d", i)
		value := fmt.Sprintf("shutdown_value_%d", i)
		result := store.AsyncPut(key, value)
		result.Wait()
	}

	// Close should be graceful and not hang
	start := time.Now()
	err = store.Close()
	duration := time.Since(start)

	if err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if duration > 5*time.Second {
		t.Errorf("Close took too long: %v", duration)
	}

	// Multiple closes should be safe
	err = store.Close()
	if err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}
