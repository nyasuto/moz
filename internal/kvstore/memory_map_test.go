package kvstore

import (
	"os"
	"testing"
)

func TestMemoryMapConstruction(t *testing.T) {
	// Create temporary directory for testing
	tempDir := t.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)

	kv := New()

	// Test initial state
	stats, err := kv.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.MemoryMapSize != 0 {
		t.Errorf("Expected empty memory map, got size %d", stats.MemoryMapSize)
	}

	if !stats.IsLoaded {
		t.Errorf("Expected memory map to be loaded after GetStats call")
	}

	// Test adding data
	testData := map[string]string{
		"name":    "Alice",
		"city":    "Tokyo",
		"country": "Japan",
	}

	for key, value := range testData {
		if err := kv.Put(key, value); err != nil {
			t.Fatalf("Failed to put %s=%s: %v", key, value, err)
		}
	}

	// Verify memory map is updated
	stats, err = kv.GetStats()
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	if stats.MemoryMapSize != len(testData) {
		t.Errorf("Expected memory map size %d, got %d", len(testData), stats.MemoryMapSize)
	}

	// Test retrieving data
	for key, expectedValue := range testData {
		value, err := kv.Get(key)
		if err != nil {
			t.Fatalf("Failed to get %s: %v", key, err)
		}
		if value != expectedValue {
			t.Errorf("Expected %s=%s, got %s", key, expectedValue, value)
		}
	}
}

func TestMemoryMapDeletion(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)

	kv := New()

	// Add some data
	kv.Put("key1", "value1")
	kv.Put("key2", "value2")
	kv.Put("key3", "value3")

	// Verify initial state
	stats, _ := kv.GetStats()
	if stats.MemoryMapSize != 3 {
		t.Errorf("Expected 3 items, got %d", stats.MemoryMapSize)
	}

	// Delete one key
	if err := kv.Delete("key2"); err != nil {
		t.Fatalf("Failed to delete key2: %v", err)
	}

	// Verify memory map is updated
	stats, _ = kv.GetStats()
	if stats.MemoryMapSize != 2 {
		t.Errorf("Expected 2 items after deletion, got %d", stats.MemoryMapSize)
	}

	// Verify key is actually deleted
	_, err := kv.Get("key2")
	if err == nil {
		t.Error("Expected error when getting deleted key")
	}

	// Verify other keys still exist
	if value, err := kv.Get("key1"); err != nil || value != "value1" {
		t.Errorf("Expected key1=value1, got err=%v, value=%s", err, value)
	}
}

func TestMemoryMapPersistence(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)

	// Create first KVStore instance
	kv1 := New()
	kv1.Put("persistent_key", "persistent_value")
	kv1.Put("temp_key", "temp_value")
	kv1.Delete("temp_key")

	// Create second KVStore instance (should load from disk)
	kv2 := New()

	// Verify data persistence
	value, err := kv2.Get("persistent_key")
	if err != nil {
		t.Fatalf("Failed to get persistent key: %v", err)
	}
	if value != "persistent_value" {
		t.Errorf("Expected persistent_value, got %s", value)
	}

	// Verify deleted key doesn't exist
	_, err = kv2.Get("temp_key")
	if err == nil {
		t.Error("Expected error when getting deleted key")
	}

	// Verify stats
	stats, _ := kv2.GetStats()
	if stats.MemoryMapSize != 1 {
		t.Errorf("Expected 1 item in memory map, got %d", stats.MemoryMapSize)
	}
}

func TestMemoryMapUpdate(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)

	kv := New()

	// Test key updates
	kv.Put("update_key", "initial_value")

	value, _ := kv.Get("update_key")
	if value != "initial_value" {
		t.Errorf("Expected initial_value, got %s", value)
	}

	// Update the same key
	kv.Put("update_key", "updated_value")

	value, _ = kv.Get("update_key")
	if value != "updated_value" {
		t.Errorf("Expected updated_value, got %s", value)
	}

	// Verify memory map size didn't increase
	stats, _ := kv.GetStats()
	if stats.MemoryMapSize != 1 {
		t.Errorf("Expected 1 item in memory map after update, got %d", stats.MemoryMapSize)
	}
}

func TestMemoryMapConcurrency(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)

	kv := New()

	// Test concurrent read/write operations
	done := make(chan bool, 2)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			kv.Put("concurrent_key", "value")
		}
		done <- true
	}()

	// Reader goroutine
	go func() {
		for i := 0; i < 100; i++ {
			kv.Get("concurrent_key")
			kv.GetStats()
		}
		done <- true
	}()

	// Wait for both goroutines to complete
	<-done
	<-done

	// Verify final state
	value, err := kv.Get("concurrent_key")
	if err != nil {
		t.Fatalf("Failed to get concurrent key: %v", err)
	}
	if value != "value" {
		t.Errorf("Expected 'value', got %s", value)
	}
}

func TestMemoryMapCompaction(t *testing.T) {
	tempDir := t.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)

	kv := New()

	// Add some data with updates and deletions
	kv.Put("key1", "value1")
	kv.Put("key2", "value2")
	kv.Put("key3", "value3")
	kv.Put("key1", "updated_value1") // Update
	kv.Delete("key2")                // Delete

	// Compact
	if err := kv.Compact(); err != nil {
		t.Fatalf("Failed to compact: %v", err)
	}

	// Verify memory map state after compaction
	stats, _ := kv.GetStats()
	if stats.MemoryMapSize != 2 {
		t.Errorf("Expected 2 items after compaction, got %d", stats.MemoryMapSize)
	}

	// Verify data integrity
	value, _ := kv.Get("key1")
	if value != "updated_value1" {
		t.Errorf("Expected updated_value1, got %s", value)
	}

	value, _ = kv.Get("key3")
	if value != "value3" {
		t.Errorf("Expected value3, got %s", value)
	}

	// Verify deleted key doesn't exist
	_, err := kv.Get("key2")
	if err == nil {
		t.Error("Expected error when getting deleted key after compaction")
	}
}
