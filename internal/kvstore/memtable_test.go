package kvstore

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestMemTable_BasicOperations(t *testing.T) {
	config := DefaultMemTableConfig()
	memTable := NewMemTable(config)

	// Test Put and Get
	memTable.Put("key1", "value1", 1)
	value, found := memTable.Get("key1")
	if !found {
		t.Error("Expected to find key1")
	}
	if value != "value1" {
		t.Errorf("Expected value1, got %s", value)
	}

	// Test overwrite
	memTable.Put("key1", "new_value1", 2)
	value, found = memTable.Get("key1")
	if !found {
		t.Error("Expected to find key1 after overwrite")
	}
	if value != "new_value1" {
		t.Errorf("Expected new_value1, got %s", value)
	}

	// Test non-existent key
	_, found = memTable.Get("nonexistent")
	if found {
		t.Error("Should not find nonexistent key")
	}

	// Test Delete
	memTable.Delete("key1", 3)
	_, found = memTable.Get("key1")
	if found {
		t.Error("Should not find deleted key")
	}
}

func TestMemTable_MultipleEntries(t *testing.T) {
	config := DefaultMemTableConfig()
	memTable := NewMemTable(config)

	// Add multiple entries
	entries := map[string]string{
		"user1": "alice",
		"user2": "bob",
		"user3": "charlie",
		"user4": "diana",
	}

	lsn := uint64(1)
	for key, value := range entries {
		memTable.Put(key, value, lsn)
		lsn++
	}

	// Verify all entries
	for key, expectedValue := range entries {
		value, found := memTable.Get(key)
		if !found {
			t.Errorf("Expected to find key %s", key)
			continue
		}
		if value != expectedValue {
			t.Errorf("For key %s, expected %s, got %s", key, expectedValue, value)
		}
	}

	// Test List
	keys := memTable.List()
	if len(keys) != len(entries) {
		t.Errorf("Expected %d keys in list, got %d", len(entries), len(keys))
	}

	// Verify keys are sorted
	for i := 1; i < len(keys); i++ {
		if keys[i-1] >= keys[i] {
			t.Errorf("Keys should be sorted: %s >= %s", keys[i-1], keys[i])
		}
	}

	// Test Count
	if memTable.Count() != len(entries) {
		t.Errorf("Expected count %d, got %d", len(entries), memTable.Count())
	}
}

func TestMemTable_DeleteOperations(t *testing.T) {
	config := DefaultMemTableConfig()
	memTable := NewMemTable(config)

	// Add and then delete entries
	memTable.Put("key1", "value1", 1)
	memTable.Put("key2", "value2", 2)
	memTable.Put("key3", "value3", 3)

	// Delete one key
	memTable.Delete("key2", 4)

	// Verify remaining keys
	if _, found := memTable.Get("key1"); !found {
		t.Error("key1 should still exist")
	}
	if _, found := memTable.Get("key2"); found {
		t.Error("key2 should be deleted")
	}
	if _, found := memTable.Get("key3"); !found {
		t.Error("key3 should still exist")
	}

	// List should not include deleted key
	keys := memTable.List()
	for _, key := range keys {
		if key == "key2" {
			t.Error("Deleted key should not appear in list")
		}
	}

	// GetAll should include deletion marker
	allEntries := memTable.GetAll()
	deletionFound := false
	for _, entry := range allEntries {
		if entry.Key == "key2" && entry.Deleted {
			deletionFound = true
			break
		}
	}
	if !deletionFound {
		t.Error("GetAll should include deletion marker for key2")
	}
}

func TestMemTable_SizeTracking(t *testing.T) {
	config := DefaultMemTableConfig()
	memTable := NewMemTable(config)

	initialSize := memTable.Size()
	if initialSize != 0 {
		t.Errorf("Initial size should be 0, got %d", initialSize)
	}

	// Add entries and check size increases
	memTable.Put("short", "val", 1)
	size1 := memTable.Size()
	if size1 <= initialSize {
		t.Error("Size should increase after adding entry")
	}

	memTable.Put("much_longer_key", "much_longer_value_with_more_data", 2)
	size2 := memTable.Size()
	if size2 <= size1 {
		t.Error("Size should increase with larger entry")
	}

	// Delete and check size changes
	memTable.Delete("short", 3)
	_ = memTable.Size() // Size might not decrease much due to deletion marker

	// Clear and check size resets
	memTable.Clear()
	finalSize := memTable.Size()
	if finalSize != 0 {
		t.Errorf("Size should be 0 after clear, got %d", finalSize)
	}
}

func TestMemTable_ShouldFlush(t *testing.T) {
	// Test size-based flush
	config := MemTableConfig{
		MaxSize:      100, // Very small max size
		MaxEntries:   1000,
		FlushTimeout: 1 * time.Hour, // Long timeout
	}

	memTable := NewMemTable(config)

	// Should not need flush initially
	if memTable.ShouldFlush(config) {
		t.Error("Empty MemTable should not need flush")
	}

	// Add entries until size limit is reached
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("size_test_key_%d", i)
		value := fmt.Sprintf("size_test_value_with_extra_data_%d", i)
		memTable.Put(key, value, uint64(i+1))

		if memTable.ShouldFlush(config) {
			// Size limit reached
			break
		}
	}

	// Should need flush due to size
	if !memTable.ShouldFlush(config) {
		t.Error("MemTable should need flush due to size limit")
	}
}

func TestMemTable_ShouldFlushByCount(t *testing.T) {
	// Test count-based flush
	config := MemTableConfig{
		MaxSize:      1024 * 1024, // Large size limit
		MaxEntries:   3,           // Small entry limit
		FlushTimeout: 1 * time.Hour,
	}

	memTable := NewMemTable(config)

	// Add entries up to limit
	for i := 0; i < 3; i++ {
		memTable.Put(fmt.Sprintf("count_key_%d", i), fmt.Sprintf("value_%d", i), uint64(i+1))
	}

	// Should need flush due to entry count
	if !memTable.ShouldFlush(config) {
		t.Error("MemTable should need flush due to entry count limit")
	}
}

func TestMemTable_ShouldFlushByTime(t *testing.T) {
	// Test time-based flush
	config := MemTableConfig{
		MaxSize:      1024 * 1024,
		MaxEntries:   1000,
		FlushTimeout: 50 * time.Millisecond, // Short timeout
	}

	memTable := NewMemTable(config)
	memTable.Put("time_key", "time_value", 1)

	// Should not need flush immediately
	if memTable.ShouldFlush(config) {
		t.Error("MemTable should not need immediate flush")
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Should need flush due to timeout
	if !memTable.ShouldFlush(config) {
		t.Error("MemTable should need flush due to timeout")
	}
}

func TestMemTable_Concurrency(t *testing.T) {
	config := DefaultMemTableConfig()
	memTable := NewMemTable(config)

	numGoroutines := 10
	entriesPerGoroutine := 20

	var wg sync.WaitGroup

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < entriesPerGoroutine; j++ {
				key := fmt.Sprintf("worker%d_key%d", workerID, j)
				value := fmt.Sprintf("worker%d_value%d", workerID, j)
				lsn := uint64(workerID*entriesPerGoroutine + j + 1)

				memTable.Put(key, value, lsn)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < entriesPerGoroutine; j++ {
				key := fmt.Sprintf("worker%d_key%d", workerID, j)

				// Try to read (may or may not find due to timing)
				memTable.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// Verify final state
	expectedEntries := numGoroutines * entriesPerGoroutine
	if memTable.Count() != expectedEntries {
		t.Errorf("Expected %d entries, got %d", expectedEntries, memTable.Count())
	}
}

func TestMemTable_RangeQueries(t *testing.T) {
	config := DefaultMemTableConfig()
	memTable := NewMemTable(config)

	// Add entries with keys in alphabetical order
	entries := []struct {
		key   string
		value string
	}{
		{"apple", "fruit1"},
		{"banana", "fruit2"},
		{"cherry", "fruit3"},
		{"date", "fruit4"},
		{"elderberry", "fruit5"},
	}

	for i, entry := range entries {
		memTable.Put(entry.key, entry.value, uint64(i+1))
	}

	// Test range query
	results := memTable.Range("banana", "date")

	expectedKeys := []string{"banana", "cherry", "date"}
	if len(results) != len(expectedKeys) {
		t.Errorf("Expected %d results, got %d", len(expectedKeys), len(results))
	}

	for i, result := range results {
		if i >= len(expectedKeys) {
			break
		}
		if result.Key != expectedKeys[i] {
			t.Errorf("Expected key %s at position %d, got %s", expectedKeys[i], i, result.Key)
		}
	}
}

func TestMemTable_PrefixSearch(t *testing.T) {
	config := DefaultMemTableConfig()
	memTable := NewMemTable(config)

	// Add entries with various prefixes
	entries := []struct {
		key   string
		value string
	}{
		{"user:1", "alice"},
		{"user:2", "bob"},
		{"user:3", "charlie"},
		{"session:1", "sess1"},
		{"session:2", "sess2"},
		{"config:timeout", "30s"},
	}

	for i, entry := range entries {
		memTable.Put(entry.key, entry.value, uint64(i+1))
	}

	// Test prefix search for "user:"
	userResults := memTable.PrefixSearch("user:")
	if len(userResults) != 3 {
		t.Errorf("Expected 3 user results, got %d", len(userResults))
	}

	for _, result := range userResults {
		if len(result.Key) < 5 || result.Key[:5] != "user:" {
			t.Errorf("Result key %s should start with 'user:'", result.Key)
		}
	}

	// Test prefix search for "session:"
	sessionResults := memTable.PrefixSearch("session:")
	if len(sessionResults) != 2 {
		t.Errorf("Expected 2 session results, got %d", len(sessionResults))
	}

	// Test prefix search for non-existent prefix
	noResults := memTable.PrefixSearch("nonexistent:")
	if len(noResults) != 0 {
		t.Errorf("Expected 0 results for non-existent prefix, got %d", len(noResults))
	}
}

func TestMemTable_Statistics(t *testing.T) {
	config := DefaultMemTableConfig()
	memTable := NewMemTable(config)

	// Initial stats
	stats := memTable.GetStats()
	if stats.Entries != 0 {
		t.Errorf("Initial entries should be 0, got %d", stats.Entries)
	}
	if stats.PutCount != 0 {
		t.Errorf("Initial put count should be 0, got %d", stats.PutCount)
	}

	// Add entries and check stats
	memTable.Put("key1", "value1", 1)
	memTable.Put("key2", "value2", 2)

	stats = memTable.GetStats()
	if stats.Entries != 2 {
		t.Errorf("Expected 2 entries, got %d", stats.Entries)
	}
	if stats.PutCount != 2 {
		t.Errorf("Expected 2 puts, got %d", stats.PutCount)
	}

	// Get operations
	memTable.Get("key1")
	memTable.Get("key2")
	memTable.Get("nonexistent")

	stats = memTable.GetStats()
	if stats.GetCount != 3 {
		t.Errorf("Expected 3 gets, got %d", stats.GetCount)
	}

	// Delete operations
	memTable.Delete("key1", 3)

	stats = memTable.GetStats()
	if stats.DeleteCount != 1 {
		t.Errorf("Expected 1 delete, got %d", stats.DeleteCount)
	}

	// Clear and check flush stats
	memTable.Clear()

	stats = memTable.GetStats()
	if stats.FlushCount != 1 {
		t.Errorf("Expected 1 flush, got %d", stats.FlushCount)
	}
	if stats.Entries != 0 {
		t.Errorf("Entries should be 0 after clear, got %d", stats.Entries)
	}
}
