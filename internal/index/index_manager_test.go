package index

import (
	"fmt"
	"testing"
)

func TestIndexManager_HashIndex(t *testing.T) {
	// Test hash index through manager
	im, err := NewIndexManager(IndexTypeHash)
	if err != nil {
		t.Fatalf("Failed to create index manager: %v", err)
	}
	defer im.Close()

	if im.GetIndexType() != IndexTypeHash {
		t.Errorf("Expected index type %s, got %s", IndexTypeHash, im.GetIndexType())
	}

	if !im.IsEnabled() {
		t.Error("Expected index to be enabled")
	}

	// Test basic operations
	entry := IndexEntry{
		Key:       "test_key",
		Offset:    100,
		Size:      50,
		Timestamp: 1234567890,
	}

	err = im.Insert("test_key", entry)
	if err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	retrieved, err := im.Get("test_key")
	if err != nil {
		t.Fatalf("Failed to get: %v", err)
	}

	if retrieved.Key != entry.Key {
		t.Errorf("Expected key %s, got %s", entry.Key, retrieved.Key)
	}

	if !im.Exists("test_key") {
		t.Error("Expected key to exist")
	}

	if im.Size() != 1 {
		t.Errorf("Expected size 1, got %d", im.Size())
	}
}

func TestIndexManager_BTreeIndex(t *testing.T) {
	// Test B-tree index through manager
	im, err := NewIndexManager(IndexTypeBTree)
	if err != nil {
		t.Fatalf("Failed to create index manager: %v", err)
	}
	defer im.Close()

	if im.GetIndexType() != IndexTypeBTree {
		t.Errorf("Expected index type %s, got %s", IndexTypeBTree, im.GetIndexType())
	}

	// Test range query (B-tree's strength)
	testData := map[string]IndexEntry{
		"apple":  {Key: "apple", Offset: 100},
		"banana": {Key: "banana", Offset: 200},
		"cherry": {Key: "cherry", Offset: 300},
		"date":   {Key: "date", Offset: 400},
	}

	for key, entry := range testData {
		if err := im.Insert(key, entry); err != nil {
			t.Fatalf("Failed to insert key %s: %v", key, err)
		}
	}

	// Test range query
	rangeEntries, err := im.Range("banana", "cherry")
	if err != nil {
		t.Fatalf("Failed to perform range query: %v", err)
	}

	expectedKeys := []string{"banana", "cherry"}
	if len(rangeEntries) != len(expectedKeys) {
		t.Errorf("Expected %d entries in range, got %d", len(expectedKeys), len(rangeEntries))
	}

	for i, expected := range expectedKeys {
		if i >= len(rangeEntries) || rangeEntries[i].Key != expected {
			t.Errorf("Expected key %s at position %d, got %s", expected, i, rangeEntries[i].Key)
		}
	}

	// Test sorted keys
	keys := im.Keys()
	expectedSorted := []string{"apple", "banana", "cherry", "date"}
	for i, expected := range expectedSorted {
		if i >= len(keys) || keys[i] != expected {
			t.Errorf("Expected key %s at position %d, got %s", expected, i, keys[i])
		}
	}
}

func TestIndexManager_NoIndex(t *testing.T) {
	// Test disabled index
	im, err := NewIndexManager(IndexTypeNone)
	if err != nil {
		t.Fatalf("Failed to create index manager: %v", err)
	}
	defer im.Close()

	if im.GetIndexType() != IndexTypeNone {
		t.Errorf("Expected index type %s, got %s", IndexTypeNone, im.GetIndexType())
	}

	if im.IsEnabled() {
		t.Error("Expected index to be disabled")
	}

	// Operations should be no-ops or return errors
	entry := IndexEntry{Key: "test", Offset: 100}

	// Insert should succeed (no-op)
	if err := im.Insert("test", entry); err != nil {
		t.Errorf("Insert should succeed as no-op: %v", err)
	}

	// Get should return error
	_, err = im.Get("test")
	if err == nil {
		t.Error("Get should return error when index is disabled")
	}

	// Exists should return false
	if im.Exists("test") {
		t.Error("Exists should return false when index is disabled")
	}

	// Size should return 0
	if im.Size() != 0 {
		t.Errorf("Size should return 0 when index is disabled, got %d", im.Size())
	}

	// Range should return error
	_, err = im.Range("a", "z")
	if err == nil {
		t.Error("Range should return error when index is disabled")
	}

	// Prefix should return error
	_, err = im.Prefix("test")
	if err == nil {
		t.Error("Prefix should return error when index is disabled")
	}
}

func TestIndexManager_UnsupportedType(t *testing.T) {
	// Test creating manager with unsupported index type
	_, err := NewIndexManager(IndexType("unsupported"))
	if err == nil {
		t.Error("Expected error for unsupported index type")
	}
}

func TestIndexManager_ConcurrentAccess(t *testing.T) {
	im, err := NewIndexManager(IndexTypeHash)
	if err != nil {
		t.Fatalf("Failed to create index manager: %v", err)
	}
	defer im.Close()

	// Test concurrent operations
	done := make(chan bool, 2)

	// Goroutine 1: Insert operations
	go func() {
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key_%d", i)
			entry := IndexEntry{Key: key, Offset: int64(i * 100)}
			im.Insert(key, entry)
		}
		done <- true
	}()

	// Goroutine 2: Read operations
	go func() {
		for i := 0; i < 100; i++ {
			key := fmt.Sprintf("key_%d", i)
			im.Get(key) // May fail, that's OK
			im.Exists(key)
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify some entries exist
	if im.Size() < 50 { // Should have at least some entries
		t.Errorf("Expected at least 50 entries after concurrent access, got %d", im.Size())
	}
}

func TestIndexManager_MemoryUsage(t *testing.T) {
	im, err := NewIndexManager(IndexTypeHash)
	if err != nil {
		t.Fatalf("Failed to create index manager: %v", err)
	}
	defer im.Close()

	// Initially should have minimal memory usage
	initialMemory := im.MemoryUsage()
	if initialMemory < 0 {
		t.Error("Memory usage should not be negative")
	}

	// Add some entries
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key_%04d", i)
		entry := IndexEntry{Key: key, Offset: int64(i * 100), Size: 100}
		if err := im.Insert(key, entry); err != nil {
			t.Fatalf("Failed to insert key %s: %v", key, err)
		}
	}

	// Memory usage should increase
	finalMemory := im.MemoryUsage()
	if finalMemory <= initialMemory {
		t.Errorf("Memory usage should increase after adding entries: initial=%d, final=%d",
			initialMemory, finalMemory)
	}

	t.Logf("Memory usage: initial=%d bytes, final=%d bytes", initialMemory, finalMemory)
}

func TestIndexManager_Validation(t *testing.T) {
	im, err := NewIndexManager(IndexTypeBTree)
	if err != nil {
		t.Fatalf("Failed to create index manager: %v", err)
	}
	defer im.Close()

	// Add some test data
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("key_%03d", i)
		entry := IndexEntry{Key: key, Offset: int64(i * 100)}
		if err := im.Insert(key, entry); err != nil {
			t.Fatalf("Failed to insert key %s: %v", key, err)
		}
	}

	// Validation should pass
	if err := im.Validate(); err != nil {
		t.Errorf("Validation failed: %v", err)
	}
}

func TestIndexManager_Rebuild(t *testing.T) {
	im, err := NewIndexManager(IndexTypeHash)
	if err != nil {
		t.Fatalf("Failed to create index manager: %v", err)
	}
	defer im.Close()

	// Add initial data
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("old_%d", i)
		entry := IndexEntry{Key: key, Offset: int64(i * 100)}
		if err := im.Insert(key, entry); err != nil {
			t.Fatalf("Failed to insert key %s: %v", key, err)
		}
	}

	if im.Size() != 50 {
		t.Errorf("Expected size 50 before rebuild, got %d", im.Size())
	}

	// Rebuild with new data
	newEntries := make(map[string]IndexEntry)
	for i := 0; i < 25; i++ {
		key := fmt.Sprintf("new_%d", i)
		newEntries[key] = IndexEntry{Key: key, Offset: int64(i * 200)}
	}

	if err := im.Rebuild(newEntries); err != nil {
		t.Fatalf("Failed to rebuild: %v", err)
	}

	// Verify rebuild results
	if im.Size() != 25 {
		t.Errorf("Expected size 25 after rebuild, got %d", im.Size())
	}

	// Old entries should be gone
	if im.Exists("old_0") {
		t.Error("Expected old entries to be removed after rebuild")
	}

	// New entries should exist
	if !im.Exists("new_0") {
		t.Error("Expected new entries to exist after rebuild")
	}
}
