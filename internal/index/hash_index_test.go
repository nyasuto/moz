package index

import (
	"os"
	"testing"
)

func TestHashIndex_BasicOperations(t *testing.T) {
	hi, err := NewHashIndex(DefaultHashIndexConfig())
	if err != nil {
		t.Fatalf("Failed to create hash index: %v", err)
	}
	defer hi.Close()

	// Test Insert and Get
	entry := IndexEntry{
		Key:       "test_key",
		Offset:    100,
		Size:      50,
		Timestamp: 1234567890,
		Deleted:   false,
	}

	err = hi.Insert("test_key", entry)
	if err != nil {
		t.Fatalf("Failed to insert entry: %v", err)
	}

	retrieved, err := hi.Get("test_key")
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
	if !hi.Exists("test_key") {
		t.Error("Expected key to exist")
	}

	if hi.Exists("nonexistent_key") {
		t.Error("Expected key to not exist")
	}

	// Test Size
	if hi.Size() != 1 {
		t.Errorf("Expected size 1, got %d", hi.Size())
	}

	// Test Delete
	err = hi.Delete("test_key")
	if err != nil {
		t.Fatalf("Failed to delete entry: %v", err)
	}

	if hi.Exists("test_key") {
		t.Error("Expected key to be deleted")
	}

	if hi.Size() != 0 {
		t.Errorf("Expected size 0 after deletion, got %d", hi.Size())
	}
}

func TestHashIndex_MultipleEntries(t *testing.T) {
	hi, err := NewHashIndex(DefaultHashIndexConfig())
	if err != nil {
		t.Fatalf("Failed to create hash index: %v", err)
	}
	defer hi.Close()

	// Insert multiple entries
	entries := map[string]IndexEntry{
		"key1": {Key: "key1", Offset: 100, Size: 10, Timestamp: 1000},
		"key2": {Key: "key2", Offset: 200, Size: 20, Timestamp: 2000},
		"key3": {Key: "key3", Offset: 300, Size: 30, Timestamp: 3000},
	}

	for key, entry := range entries {
		if err := hi.Insert(key, entry); err != nil {
			t.Fatalf("Failed to insert key %s: %v", key, err)
		}
	}

	// Verify all entries
	if hi.Size() != 3 {
		t.Errorf("Expected size 3, got %d", hi.Size())
	}

	for key, expected := range entries {
		retrieved, err := hi.Get(key)
		if err != nil {
			t.Errorf("Failed to get key %s: %v", key, err)
			continue
		}

		if retrieved.Offset != expected.Offset {
			t.Errorf("Key %s: expected offset %d, got %d", key, expected.Offset, retrieved.Offset)
		}
	}

	// Test Keys method
	keys := hi.Keys()
	if len(keys) != 3 {
		t.Errorf("Expected 3 keys, got %d", len(keys))
	}

	// Keys should be sorted
	expectedKeys := []string{"key1", "key2", "key3"}
	for i, expected := range expectedKeys {
		if i >= len(keys) || keys[i] != expected {
			t.Errorf("Expected key %s at position %d, got %s", expected, i, keys[i])
		}
	}
}

func TestHashIndex_BatchOperations(t *testing.T) {
	hi, err := NewHashIndex(DefaultHashIndexConfig())
	if err != nil {
		t.Fatalf("Failed to create hash index: %v", err)
	}
	defer hi.Close()

	// Test BatchInsert
	entries := map[string]IndexEntry{
		"batch1": {Key: "batch1", Offset: 100, Size: 10},
		"batch2": {Key: "batch2", Offset: 200, Size: 20},
		"batch3": {Key: "batch3", Offset: 300, Size: 30},
	}

	err = hi.BatchInsert(entries)
	if err != nil {
		t.Fatalf("Failed to batch insert: %v", err)
	}

	if hi.Size() != 3 {
		t.Errorf("Expected size 3 after batch insert, got %d", hi.Size())
	}

	// Test BatchDelete
	keys := []string{"batch1", "batch3"}
	err = hi.BatchDelete(keys)
	if err != nil {
		t.Fatalf("Failed to batch delete: %v", err)
	}

	if hi.Size() != 1 {
		t.Errorf("Expected size 1 after batch delete, got %d", hi.Size())
	}

	if !hi.Exists("batch2") {
		t.Error("Expected batch2 to still exist")
	}

	if hi.Exists("batch1") || hi.Exists("batch3") {
		t.Error("Expected batch1 and batch3 to be deleted")
	}
}

func TestHashIndex_Prefix(t *testing.T) {
	hi, err := NewHashIndex(DefaultHashIndexConfig())
	if err != nil {
		t.Fatalf("Failed to create hash index: %v", err)
	}
	defer hi.Close()

	// Insert entries with various prefixes
	entries := map[string]IndexEntry{
		"user:1":  {Key: "user:1", Offset: 100},
		"user:2":  {Key: "user:2", Offset: 200},
		"admin:1": {Key: "admin:1", Offset: 300},
		"guest:1": {Key: "guest:1", Offset: 400},
	}

	for key, entry := range entries {
		if err := hi.Insert(key, entry); err != nil {
			t.Fatalf("Failed to insert key %s: %v", key, err)
		}
	}

	// Test prefix search
	userEntries, err := hi.Prefix("user:")
	if err != nil {
		t.Fatalf("Failed to search prefix: %v", err)
	}

	if len(userEntries) != 2 {
		t.Errorf("Expected 2 user entries, got %d", len(userEntries))
	}

	// Verify entries are sorted
	if len(userEntries) >= 2 {
		if userEntries[0].Key > userEntries[1].Key {
			t.Error("Expected prefix results to be sorted")
		}
	}
}

func TestHashIndex_Persistence(t *testing.T) {
	// Create temporary file
	tmpFile := "test_hash_index.gob"
	defer os.Remove(tmpFile)

	// Create index and add data
	hi1, err := NewHashIndex(DefaultHashIndexConfig())
	if err != nil {
		t.Fatalf("Failed to create hash index: %v", err)
	}

	entries := map[string]IndexEntry{
		"persist1": {Key: "persist1", Offset: 100, Size: 10},
		"persist2": {Key: "persist2", Offset: 200, Size: 20},
	}

	for key, entry := range entries {
		if err := hi1.Insert(key, entry); err != nil {
			t.Fatalf("Failed to insert key %s: %v", key, err)
		}
	}

	// Save to file
	if err := hi1.Save(tmpFile); err != nil {
		t.Fatalf("Failed to save index: %v", err)
	}
	hi1.Close()

	// Create new index and load from file
	hi2, err := NewHashIndex(DefaultHashIndexConfig())
	if err != nil {
		t.Fatalf("Failed to create new hash index: %v", err)
	}
	defer hi2.Close()

	if err := hi2.Load(tmpFile); err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	// Verify data was loaded correctly
	if hi2.Size() != 2 {
		t.Errorf("Expected size 2 after load, got %d", hi2.Size())
	}

	for key, expected := range entries {
		retrieved, err := hi2.Get(key)
		if err != nil {
			t.Errorf("Failed to get key %s after load: %v", key, err)
			continue
		}

		if retrieved.Offset != expected.Offset {
			t.Errorf("Key %s: expected offset %d after load, got %d", key, expected.Offset, retrieved.Offset)
		}
	}
}

func TestHashIndex_Validation(t *testing.T) {
	hi, err := NewHashIndex(DefaultHashIndexConfig())
	if err != nil {
		t.Fatalf("Failed to create hash index: %v", err)
	}
	defer hi.Close()

	// Add some entries
	entries := map[string]IndexEntry{
		"val1": {Key: "val1", Offset: 100},
		"val2": {Key: "val2", Offset: 200},
		"val3": {Key: "val3", Offset: 300},
	}

	for key, entry := range entries {
		if err := hi.Insert(key, entry); err != nil {
			t.Fatalf("Failed to insert key %s: %v", key, err)
		}
	}

	// Validate should pass
	if err := hi.Validate(); err != nil {
		t.Errorf("Validation failed: %v", err)
	}
}

func TestHashIndex_Rebuild(t *testing.T) {
	hi, err := NewHashIndex(DefaultHashIndexConfig())
	if err != nil {
		t.Fatalf("Failed to create hash index: %v", err)
	}
	defer hi.Close()

	// Add initial entries
	if err := hi.Insert("old1", IndexEntry{Key: "old1", Offset: 100}); err != nil {
		t.Fatalf("Failed to insert: %v", err)
	}

	// Rebuild with new data
	newEntries := map[string]IndexEntry{
		"new1": {Key: "new1", Offset: 200},
		"new2": {Key: "new2", Offset: 300},
	}

	if err := hi.Rebuild(newEntries); err != nil {
		t.Fatalf("Failed to rebuild: %v", err)
	}

	// Verify old data is gone and new data is present
	if hi.Exists("old1") {
		t.Error("Expected old1 to be removed after rebuild")
	}

	if !hi.Exists("new1") || !hi.Exists("new2") {
		t.Error("Expected new entries to exist after rebuild")
	}

	if hi.Size() != 2 {
		t.Errorf("Expected size 2 after rebuild, got %d", hi.Size())
	}
}
