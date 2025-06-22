package index

import (
	"testing"
)

func TestBTreeIndex_BasicOperations(t *testing.T) {
	bt, err := NewBTreeIndex(DefaultBTreeIndexConfig())
	if err != nil {
		t.Fatalf("Failed to create B-tree index: %v", err)
	}
	defer bt.Close()

	// Test Insert and Get
	entry := IndexEntry{
		Key:       "test_key",
		Offset:    100,
		Size:      50,
		Timestamp: 1234567890,
		Deleted:   false,
	}

	err = bt.Insert("test_key", entry)
	if err != nil {
		t.Fatalf("Failed to insert entry: %v", err)
	}

	retrieved, err := bt.Get("test_key")
	if err != nil {
		t.Fatalf("Failed to get entry: %v", err)
	}

	if retrieved.Key != entry.Key {
		t.Errorf("Expected key %s, got %s", entry.Key, retrieved.Key)
	}

	if retrieved.Offset != entry.Offset {
		t.Errorf("Expected offset %d, got %d", entry.Offset, retrieved.Offset)
	}

	// Test Exists
	if !bt.Exists("test_key") {
		t.Error("Expected key to exist")
	}

	if bt.Exists("nonexistent_key") {
		t.Error("Expected key to not exist")
	}

	// Test Size
	if bt.Size() != 1 {
		t.Errorf("Expected size 1, got %d", bt.Size())
	}

	// Test Delete
	err = bt.Delete("test_key")
	if err != nil {
		t.Fatalf("Failed to delete entry: %v", err)
	}

	if bt.Exists("test_key") {
		t.Error("Expected key to be deleted")
	}

	if bt.Size() != 0 {
		t.Errorf("Expected size 0 after deletion, got %d", bt.Size())
	}
}

func TestBTreeIndex_SortedOperations(t *testing.T) {
	bt, err := NewBTreeIndex(DefaultBTreeIndexConfig())
	if err != nil {
		t.Fatalf("Failed to create B-tree index: %v", err)
	}
	defer bt.Close()

	// Insert entries in random order
	entries := map[string]IndexEntry{
		"charlie": {Key: "charlie", Offset: 300, Size: 30},
		"alice":   {Key: "alice", Offset: 100, Size: 10},
		"bob":     {Key: "bob", Offset: 200, Size: 20},
		"david":   {Key: "david", Offset: 400, Size: 40},
	}

	for key, entry := range entries {
		if err := bt.Insert(key, entry); err != nil {
			t.Fatalf("Failed to insert key %s: %v", key, err)
		}
	}

	// Test Keys method returns sorted keys
	keys := bt.Keys()
	expectedKeys := []string{"alice", "bob", "charlie", "david"}

	if len(keys) != len(expectedKeys) {
		t.Errorf("Expected %d keys, got %d", len(expectedKeys), len(keys))
	}

	for i, expected := range expectedKeys {
		if i >= len(keys) || keys[i] != expected {
			t.Errorf("Expected key %s at position %d, got %s", expected, i, keys[i])
		}
	}
}

func TestBTreeIndex_RangeQueries(t *testing.T) {
	bt, err := NewBTreeIndex(DefaultBTreeIndexConfig())
	if err != nil {
		t.Fatalf("Failed to create B-tree index: %v", err)
	}
	defer bt.Close()

	// Insert test data
	testData := map[string]int64{
		"apple":      100,
		"banana":     200,
		"cherry":     300,
		"date":       400,
		"elderberry": 500,
	}

	for key, offset := range testData {
		entry := IndexEntry{Key: key, Offset: offset}
		if err := bt.Insert(key, entry); err != nil {
			t.Fatalf("Failed to insert key %s: %v", key, err)
		}
	}

	// Test range query
	rangeEntries, err := bt.Range("banana", "date")
	if err != nil {
		t.Fatalf("Failed to perform range query: %v", err)
	}

	expectedKeys := []string{"banana", "cherry", "date"}
	if len(rangeEntries) != len(expectedKeys) {
		t.Errorf("Expected %d entries in range, got %d", len(expectedKeys), len(rangeEntries))
	}

	for i, expected := range expectedKeys {
		if i >= len(rangeEntries) || rangeEntries[i].Key != expected {
			t.Errorf("Expected key %s at position %d, got %s", expected, i, rangeEntries[i].Key)
		}
	}
}

func TestBTreeIndex_PrefixSearch(t *testing.T) {
	bt, err := NewBTreeIndex(DefaultBTreeIndexConfig())
	if err != nil {
		t.Fatalf("Failed to create B-tree index: %v", err)
	}
	defer bt.Close()

	// Insert test data with prefixes
	testData := map[string]int64{
		"user:alice":    100,
		"user:bob":      200,
		"admin:john":    300,
		"user:charlie":  400,
		"guest:visitor": 500,
	}

	for key, offset := range testData {
		entry := IndexEntry{Key: key, Offset: offset}
		if err := bt.Insert(key, entry); err != nil {
			t.Fatalf("Failed to insert key %s: %v", key, err)
		}
	}

	// Test prefix search
	userEntries, err := bt.Prefix("user:")
	if err != nil {
		t.Fatalf("Failed to perform prefix search: %v", err)
	}

	expectedKeys := []string{"user:alice", "user:bob", "user:charlie"}
	if len(userEntries) != len(expectedKeys) {
		t.Errorf("Expected %d entries with prefix 'user:', got %d", len(expectedKeys), len(userEntries))
	}

	// Verify entries are sorted
	for i := 1; i < len(userEntries); i++ {
		if userEntries[i-1].Key >= userEntries[i].Key {
			t.Error("Expected prefix results to be sorted")
		}
	}
}

func TestBTreeIndex_BatchOperations(t *testing.T) {
	bt, err := NewBTreeIndex(DefaultBTreeIndexConfig())
	if err != nil {
		t.Fatalf("Failed to create B-tree index: %v", err)
	}
	defer bt.Close()

	// Test BatchInsert
	entries := map[string]IndexEntry{
		"batch3": {Key: "batch3", Offset: 300, Size: 30},
		"batch1": {Key: "batch1", Offset: 100, Size: 10},
		"batch2": {Key: "batch2", Offset: 200, Size: 20},
	}

	err = bt.BatchInsert(entries)
	if err != nil {
		t.Fatalf("Failed to batch insert: %v", err)
	}

	if bt.Size() != 3 {
		t.Errorf("Expected size 3 after batch insert, got %d", bt.Size())
	}

	// Verify entries are stored in sorted order
	keys := bt.Keys()
	expectedKeys := []string{"batch1", "batch2", "batch3"}
	for i, expected := range expectedKeys {
		if i >= len(keys) || keys[i] != expected {
			t.Errorf("Expected key %s at position %d, got %s", expected, i, keys[i])
		}
	}

	// Test BatchDelete
	deleteKeys := []string{"batch1", "batch3"}
	err = bt.BatchDelete(deleteKeys)
	if err != nil {
		t.Fatalf("Failed to batch delete: %v", err)
	}

	if bt.Size() != 1 {
		t.Errorf("Expected size 1 after batch delete, got %d", bt.Size())
	}

	if !bt.Exists("batch2") {
		t.Error("Expected batch2 to still exist")
	}

	if bt.Exists("batch1") || bt.Exists("batch3") {
		t.Error("Expected batch1 and batch3 to be deleted")
	}
}

func TestBTreeIndex_Validation(t *testing.T) {
	bt, err := NewBTreeIndex(DefaultBTreeIndexConfig())
	if err != nil {
		t.Fatalf("Failed to create B-tree index: %v", err)
	}
	defer bt.Close()

	// Add some entries
	entries := map[string]IndexEntry{
		"val3": {Key: "val3", Offset: 300},
		"val1": {Key: "val1", Offset: 100},
		"val2": {Key: "val2", Offset: 200},
	}

	for key, entry := range entries {
		if err := bt.Insert(key, entry); err != nil {
			t.Fatalf("Failed to insert key %s: %v", key, err)
		}
	}

	// Validate should pass
	if err := bt.Validate(); err != nil {
		t.Errorf("Validation failed: %v", err)
	}
}

func TestBTreeIndex_Rebuild(t *testing.T) {
	bt, err := NewBTreeIndex(DefaultBTreeIndexConfig())
	if err != nil {
		t.Fatalf("Failed to create B-tree index: %v", err)
	}
	defer bt.Close()

	// Add initial entries
	if err := bt.Insert("old1", IndexEntry{Key: "old1", Offset: 100}); err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	// Rebuild with new data
	newEntries := map[string]IndexEntry{
		"new2": {Key: "new2", Offset: 300},
		"new1": {Key: "new1", Offset: 200},
	}

	if err := bt.Rebuild(newEntries); err != nil {
		t.Fatalf("Failed to rebuild: %v", err)
	}

	// Verify old data is gone and new data is present
	if bt.Exists("old1") {
		t.Error("Expected old1 to be removed after rebuild")
	}

	if !bt.Exists("new1") || !bt.Exists("new2") {
		t.Error("Expected new entries to exist after rebuild")
	}

	if bt.Size() != 2 {
		t.Errorf("Expected size 2 after rebuild, got %d", bt.Size())
	}

	// Verify entries are sorted
	keys := bt.Keys()
	expectedKeys := []string{"new1", "new2"}
	for i, expected := range expectedKeys {
		if i >= len(keys) || keys[i] != expected {
			t.Errorf("Expected key %s at position %d after rebuild, got %s", expected, i, keys[i])
		}
	}
}
