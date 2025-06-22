package kvstore

import (
	"fmt"
	"os"
	"testing"
)

func TestKVStore_IndexIntegration_Hash(t *testing.T) {
	// Create KVStore with hash index
	compactionConfig := CompactionConfig{
		Enabled:         false,
		MaxFileSize:     1024 * 1024,
		MaxOperations:   1000,
		CompactionRatio: 0.5,
	}

	storageConfig := StorageConfig{
		Format:     "text",
		TextFile:   "test_hash_index.log",
		BinaryFile: "test_hash_index.bin",
		IndexType:  "hash",
		IndexFile:  "test_hash_index.idx",
	}

	store := NewWithConfig(compactionConfig, storageConfig)
	defer func() {
		store.indexManager.Close()
		// Clean up test files
		removeTestFiles("test_hash_index.log", "test_hash_index.bin", "test_hash_index.idx")
	}()

	// Test basic operations with indexing
	testIndexedOperations(t, store, "hash")
}

func TestKVStore_IndexIntegration_BTree(t *testing.T) {
	// Create KVStore with B-tree index
	compactionConfig := CompactionConfig{
		Enabled:         false,
		MaxFileSize:     1024 * 1024,
		MaxOperations:   1000,
		CompactionRatio: 0.5,
	}

	storageConfig := StorageConfig{
		Format:     "text",
		TextFile:   "test_btree_index.log",
		BinaryFile: "test_btree_index.bin",
		IndexType:  "btree",
		IndexFile:  "test_btree_index.idx",
	}

	store := NewWithConfig(compactionConfig, storageConfig)
	defer func() {
		store.indexManager.Close()
		// Clean up test files
		removeTestFiles("test_btree_index.log", "test_btree_index.bin", "test_btree_index.idx")
	}()

	// Test basic operations with indexing
	testIndexedOperations(t, store, "btree")
}

func TestKVStore_IndexIntegration_None(t *testing.T) {
	// Create KVStore with no index (default behavior)
	compactionConfig := CompactionConfig{
		Enabled:         false,
		MaxFileSize:     1024 * 1024,
		MaxOperations:   1000,
		CompactionRatio: 0.5,
	}

	storageConfig := StorageConfig{
		Format:     "text",
		TextFile:   "test_no_index.log",
		BinaryFile: "test_no_index.bin",
		IndexType:  "none",
		IndexFile:  "test_no_index.idx",
	}

	store := NewWithConfig(compactionConfig, storageConfig)
	defer func() {
		store.indexManager.Close()
		// Clean up test files
		removeTestFiles("test_no_index.log", "test_no_index.bin", "test_no_index.idx")
	}()

	// Test basic operations without indexing
	testIndexedOperations(t, store, "none")
}

func testIndexedOperations(t *testing.T, store *KVStore, indexType string) {
	// Test PUT and GET
	testData := map[string]string{
		"user:alice":    "Alice Johnson",
		"user:bob":      "Bob Smith",
		"admin:john":    "John Doe",
		"user:charlie":  "Charlie Brown",
		"guest:visitor": "Guest User",
	}

	// Insert test data
	for key, value := range testData {
		if err := store.Put(key, value); err != nil {
			t.Fatalf("Failed to put %s: %v", key, err)
		}
	}

	// Verify all data can be retrieved
	for key, expected := range testData {
		value, err := store.Get(key)
		if err != nil {
			t.Errorf("Failed to get %s: %v", key, err)
			continue
		}
		if value != expected {
			t.Errorf("Key %s: expected %s, got %s", key, expected, value)
		}
	}

	// Test index statistics
	stats, err := store.GetIndexStats()
	if err != nil {
		t.Errorf("Failed to get index stats: %v", err)
	} else {
		expectedEnabled := indexType != "none"
		if stats["enabled"] != expectedEnabled {
			t.Errorf("Expected index enabled=%v, got %v", expectedEnabled, stats["enabled"])
		}
		if stats["type"] != indexType {
			t.Errorf("Expected index type %s, got %s", indexType, stats["type"])
		}

		if expectedEnabled {
			if stats["size"].(int64) != int64(len(testData)) {
				t.Errorf("Expected index size %d, got %d", len(testData), stats["size"])
			}
		}
	}

	// Test range queries
	if indexType != "none" {
		rangeResult, err := store.GetRange("user:a", "user:z")
		if err != nil {
			t.Errorf("Failed to perform range query: %v", err)
		} else {
			expectedUserKeys := []string{"user:alice", "user:bob", "user:charlie"}
			if len(rangeResult) != len(expectedUserKeys) {
				t.Errorf("Range query: expected %d results, got %d", len(expectedUserKeys), len(rangeResult))
			}
			for _, key := range expectedUserKeys {
				if _, exists := rangeResult[key]; !exists {
					t.Errorf("Range query: missing expected key %s", key)
				}
			}
		}
	}

	// Test prefix search
	prefixResult, err := store.PrefixSearch("user:")
	if err != nil {
		t.Errorf("Failed to perform prefix search: %v", err)
	} else {
		expectedUserKeys := []string{"user:alice", "user:bob", "user:charlie"}
		if len(prefixResult) != len(expectedUserKeys) {
			t.Errorf("Prefix search: expected %d results, got %d", len(expectedUserKeys), len(prefixResult))
		}
		for _, key := range expectedUserKeys {
			if _, exists := prefixResult[key]; !exists {
				t.Errorf("Prefix search: missing expected key %s", key)
			}
		}
	}

	// Test sorted listing
	sortedKeys, err := store.ListSorted()
	if err != nil {
		t.Errorf("Failed to get sorted keys: %v", err)
	} else {
		if len(sortedKeys) != len(testData) {
			t.Errorf("Sorted list: expected %d keys, got %d", len(testData), len(sortedKeys))
		}
		// Verify keys are sorted
		for i := 1; i < len(sortedKeys); i++ {
			if sortedKeys[i-1] >= sortedKeys[i] {
				t.Errorf("Sorted list: keys not properly sorted at position %d: %s >= %s",
					i, sortedKeys[i-1], sortedKeys[i])
				break
			}
		}
	}

	// Test DELETE
	if err := store.Delete("user:bob"); err != nil {
		t.Errorf("Failed to delete user:bob: %v", err)
	}

	// Verify deletion
	_, err = store.Get("user:bob")
	if err == nil {
		t.Error("Expected error when getting deleted key")
	}

	// Verify other keys still exist
	if _, err := store.Get("user:alice"); err != nil {
		t.Errorf("Key user:alice should still exist after deleting user:bob: %v", err)
	}

	// Test index validation
	if indexType != "none" {
		if err := store.ValidateIndex(); err != nil {
			t.Errorf("Index validation failed: %v", err)
		}
	}

	// Test index rebuild
	if indexType != "none" {
		if err := store.RebuildIndex(); err != nil {
			t.Errorf("Index rebuild failed: %v", err)
		}

		// Verify data is still accessible after rebuild
		if _, err := store.Get("user:alice"); err != nil {
			t.Errorf("Key user:alice should be accessible after index rebuild: %v", err)
		}
	}
}

func TestKVStore_RangeQueries_Performance(t *testing.T) {
	// Create stores with different index types for performance comparison
	testConfigs := []struct {
		name      string
		indexType string
	}{
		{"NoIndex", "none"},
		{"HashIndex", "hash"},
		{"BTreeIndex", "btree"},
	}

	for _, config := range testConfigs {
		t.Run(config.name, func(t *testing.T) {
			compactionConfig := CompactionConfig{
				Enabled:         false,
				MaxFileSize:     1024 * 1024,
				MaxOperations:   1000,
				CompactionRatio: 0.5,
			}

			storageConfig := StorageConfig{
				Format:     "text",
				TextFile:   "test_perf_" + config.indexType + ".log",
				BinaryFile: "test_perf_" + config.indexType + ".bin",
				IndexType:  config.indexType,
				IndexFile:  "test_perf_" + config.indexType + ".idx",
			}

			store := NewWithConfig(compactionConfig, storageConfig)
			defer func() {
				store.indexManager.Close()
				removeTestFiles(
					"test_perf_"+config.indexType+".log",
					"test_perf_"+config.indexType+".bin",
					"test_perf_"+config.indexType+".idx",
				)
			}()

			// Insert test data
			for i := 0; i < 1000; i++ {
				key := fmt.Sprintf("key_%04d", i)
				value := fmt.Sprintf("value_%04d", i)
				if err := store.Put(key, value); err != nil {
					t.Fatalf("Failed to put %s: %v", key, err)
				}
			}

			// Test range query performance
			results, err := store.GetRange("key_0100", "key_0200")
			if err != nil {
				t.Errorf("Range query failed: %v", err)
			} else {
				if len(results) != 101 { // 0100 to 0200 inclusive
					t.Errorf("Expected 101 results, got %d", len(results))
				}
			}

			// Log performance information
			stats, _ := store.GetIndexStats()
			if stats["enabled"].(bool) {
				t.Logf("%s: Index size=%d, memory=%d bytes",
					config.name, stats["size"], stats["memory_usage"])
			} else {
				t.Logf("%s: No indexing", config.name)
			}
		})
	}
}

// Helper function to remove test files
func removeTestFiles(files ...string) {
	for _, file := range files {
		os.Remove(file)
	}
}
