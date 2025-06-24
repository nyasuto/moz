package lsm

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/nyasuto/moz/internal/kvstore"
)

func TestLSMTree_BasicOperations(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	config := DefaultLSMConfig()
	config.DataDir = tempDir
	config.MemTableConfig.MaxSize = 1024 // Small size for testing

	lsm, err := NewLSMTree(config)
	if err != nil {
		t.Fatalf("Failed to create LSM-Tree: %v", err)
	}
	defer lsm.Close()

	// Test basic put and get
	testData := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}

	// Put operations
	for key, value := range testData {
		if err := lsm.Put(key, value); err != nil {
			t.Errorf("Put failed for %s: %v", key, err)
		}
	}

	// Get operations
	for key, expectedValue := range testData {
		value, err := lsm.Get(key)
		if err != nil {
			t.Errorf("Get failed for %s: %v", key, err)
			continue
		}
		if value != expectedValue {
			t.Errorf("Expected %s, got %s for key %s", expectedValue, value, key)
		}
	}

	// Test delete
	if err := lsm.Delete("key2"); err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	// Verify deletion
	_, err = lsm.Get("key2")
	if err == nil {
		t.Error("Expected error when getting deleted key")
	}

	// Verify other keys still exist
	if value, err := lsm.Get("key1"); err != nil || value != "value1" {
		t.Errorf("key1 should still exist, got: %v, %v", value, err)
	}
}

func TestLSMTree_MemTableFlush(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultLSMConfig()
	config.DataDir = tempDir
	config.MemTableConfig.MaxSize = 200  // Very small to trigger flush
	config.MemTableConfig.MaxEntries = 3 // Small entry count

	lsm, err := NewLSMTree(config)
	if err != nil {
		t.Fatalf("Failed to create LSM-Tree: %v", err)
	}
	defer lsm.Close()

	// Add enough data to trigger MemTable flush
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("flush_key_%d", i)
		value := fmt.Sprintf("flush_value_with_extra_data_%d", i)

		if err := lsm.Put(key, value); err != nil {
			t.Errorf("Put failed for %s: %v", key, err)
		}
	}

	// Wait for background flush
	time.Sleep(500 * time.Millisecond)

	// Check that L0 has SSTables
	if len(lsm.levels[0].SSTables) == 0 {
		t.Error("Expected SSTables in L0 after flush")
	}

	// Verify data is still accessible
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("flush_key_%d", i)
		expectedValue := fmt.Sprintf("flush_value_with_extra_data_%d", i)

		value, err := lsm.Get(key)
		if err != nil {
			t.Errorf("Get failed after flush for %s: %v", key, err)
			continue
		}
		if value != expectedValue {
			t.Errorf("Expected %s, got %s for key %s after flush", expectedValue, value, key)
		}
	}
}

func TestSSTable_BasicOperations(t *testing.T) {
	tempDir := t.TempDir()
	sstableID := "test_sstable"

	// Create SSTable
	sstable, err := NewSSTable(sstableID, tempDir, 1)
	if err != nil {
		t.Fatalf("Failed to create SSTable: %v", err)
	}

	// Add test data
	testData := []struct {
		key     string
		value   string
		deleted bool
	}{
		{"apple", "red", false},
		{"banana", "yellow", false},
		{"cherry", "red", false},
		{"date", "", true}, // Deleted entry
	}

	for _, item := range testData {
		if err := sstable.Put(item.key, item.value, item.deleted); err != nil {
			t.Errorf("Put failed for %s: %v", item.key, err)
		}
	}

	// Finalize SSTable
	if err := sstable.Finalize(); err != nil {
		t.Fatalf("Failed to finalize SSTable: %v", err)
	}

	// Test reads
	for _, item := range testData {
		value, found, err := sstable.Get(item.key)
		if err != nil {
			t.Errorf("Get failed for %s: %v", item.key, err)
			continue
		}

		if item.deleted {
			if found {
				t.Errorf("Expected deleted key %s to not be found", item.key)
			}
		} else {
			if !found {
				t.Errorf("Expected key %s to be found", item.key)
				continue
			}
			if value != item.value {
				t.Errorf("Expected %s, got %s for key %s", item.value, value, item.key)
			}
		}
	}

	// Test key range
	if !sstable.ContainsKey("banana") {
		t.Error("SSTable should contain banana")
	}
	if sstable.ContainsKey("zebra") {
		t.Error("SSTable should not contain zebra")
	}

	// Clean up
	sstable.Close()
}

func TestSSTable_Persistence(t *testing.T) {
	tempDir := t.TempDir()
	sstableID := "persistence_test"

	// Create and populate SSTable
	{
		sstable, err := NewSSTable(sstableID, tempDir, 1)
		if err != nil {
			t.Fatalf("Failed to create SSTable: %v", err)
		}

		testData := map[string]string{
			"persistent_key1": "persistent_value1",
			"persistent_key2": "persistent_value2",
			"persistent_key3": "persistent_value3",
		}

		for key, value := range testData {
			if err := sstable.Put(key, value, false); err != nil {
				t.Errorf("Put failed for %s: %v", key, err)
			}
		}

		if err := sstable.Finalize(); err != nil {
			t.Fatalf("Failed to finalize SSTable: %v", err)
		}

		sstable.Close()
	}

	// Reopen SSTable and verify data
	{
		sstable, err := OpenSSTable(sstableID, tempDir)
		if err != nil {
			t.Fatalf("Failed to open SSTable: %v", err)
		}
		defer sstable.Close()

		expectedData := map[string]string{
			"persistent_key1": "persistent_value1",
			"persistent_key2": "persistent_value2",
			"persistent_key3": "persistent_value3",
		}

		for key, expectedValue := range expectedData {
			value, found, err := sstable.Get(key)
			if err != nil {
				t.Errorf("Get failed for %s: %v", key, err)
				continue
			}
			if !found {
				t.Errorf("Key %s not found after reopening", key)
				continue
			}
			if value != expectedValue {
				t.Errorf("Expected %s, got %s for key %s after reopening", expectedValue, value, key)
			}
		}

		// Verify metadata
		if sstable.NumEntries != 3 {
			t.Errorf("Expected 3 entries, got %d", sstable.NumEntries)
		}
	}
}

func TestBloomFilter_BasicOperations(t *testing.T) {
	expectedItems := uint64(1000)
	falsePositiveRate := 0.01

	bf := NewBloomFilter(expectedItems, falsePositiveRate)

	// Test data
	testKeys := []string{
		"bloom_test_key1",
		"bloom_test_key2",
		"bloom_test_key3",
		"bloom_test_key4",
		"bloom_test_key5",
	}

	// Add keys to bloom filter
	for _, key := range testKeys {
		bf.Add([]byte(key))
	}

	// Test positive cases (should all return true)
	for _, key := range testKeys {
		if !bf.MightContain([]byte(key)) {
			t.Errorf("Bloom filter should contain %s", key)
		}
	}

	// Test negative cases (should mostly return false)
	negativeKeys := []string{
		"definitely_not_present1",
		"definitely_not_present2",
		"definitely_not_present3",
	}

	falsePositives := 0
	for _, key := range negativeKeys {
		if bf.MightContain([]byte(key)) {
			falsePositives++
		}
	}

	// Allow some false positives but not too many
	if falsePositives > len(negativeKeys)/2 {
		t.Errorf("Too many false positives: %d out of %d", falsePositives, len(negativeKeys))
	}

	// Test statistics
	stats := bf.GetStats()
	if stats.NumItems != uint64(len(testKeys)) {
		t.Errorf("Expected %d items, got %d", len(testKeys), stats.NumItems)
	}
	if stats.EstimatedFPR > falsePositiveRate*2 {
		t.Errorf("Estimated FPR too high: %f", stats.EstimatedFPR)
	}
}

func TestBloomFilter_Serialization(t *testing.T) {
	// Create and populate bloom filter
	bf1 := NewBloomFilter(100, 0.01)
	testData := []string{"serialize_test1", "serialize_test2", "serialize_test3"}

	for _, key := range testData {
		bf1.Add([]byte(key))
	}

	// Serialize
	data := bf1.Serialize()

	// Deserialize
	bf2, err := DeserializeBloomFilter(data, 0.01)
	if err != nil {
		t.Fatalf("Failed to deserialize bloom filter: %v", err)
	}

	// Verify deserialized filter works correctly
	for _, key := range testData {
		if !bf2.MightContain([]byte(key)) {
			t.Errorf("Deserialized bloom filter should contain %s", key)
		}
	}

	// Verify statistics match
	stats1 := bf1.GetStats()
	stats2 := bf2.GetStats()

	if stats1.NumItems != stats2.NumItems {
		t.Errorf("NumItems mismatch: %d vs %d", stats1.NumItems, stats2.NumItems)
	}
	if stats1.Size != stats2.Size {
		t.Errorf("Size mismatch: %d vs %d", stats1.Size, stats2.Size)
	}
}

func TestLSMKVStore_Integration(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultLSMKVStoreConfig()
	config.DataDir = tempDir
	config.EnableMigration = false // Disable migration for this test

	store, err := NewLSMKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create LSM KVStore: %v", err)
	}
	defer store.Close()

	// Test KVStore interface
	testData := map[string]string{
		"integration_key1": "integration_value1",
		"integration_key2": "integration_value2",
		"integration_key3": "integration_value3",
	}

	// Test Put/Get
	for key, value := range testData {
		if err := store.Put(key, value); err != nil {
			t.Errorf("Put failed for %s: %v", key, err)
		}

		retrieved, err := store.Get(key)
		if err != nil {
			t.Errorf("Get failed for %s: %v", key, err)
			continue
		}
		if retrieved != value {
			t.Errorf("Expected %s, got %s for key %s", value, retrieved, key)
		}
	}

	// Test Delete
	if err := store.Delete("integration_key2"); err != nil {
		t.Errorf("Delete failed: %v", err)
	}

	_, err = store.Get("integration_key2")
	if err == nil {
		t.Error("Expected error when getting deleted key")
	}

	// Test List
	keys, err := store.List()
	if err != nil {
		t.Errorf("List failed: %v", err)
	}

	expectedKeys := 2 // integration_key1 and integration_key3
	if len(keys) != expectedKeys {
		t.Errorf("Expected %d keys, got %d", expectedKeys, len(keys))
	}

	// Test Stats
	stats := store.Stats()
	if stats == nil {
		t.Error("Stats should not be nil")
	}

	lsmStats, exists := stats["lsm"]
	if !exists {
		t.Error("LSM stats should be present")
	}
	if lsmStats == nil {
		t.Error("LSM stats should not be nil")
	}
}

func TestLSMKVStore_Migration(t *testing.T) {
	tempDir := t.TempDir()

	// Create legacy data first
	legacyStore := kvstore.New()
	legacyData := map[string]string{
		"legacy_key1": "legacy_value1",
		"legacy_key2": "legacy_value2",
	}

	for key, value := range legacyData {
		if err := legacyStore.Put(key, value); err != nil {
			t.Errorf("Failed to put legacy data: %v", err)
		}
	}

	// Create LSM store with migration enabled
	config := DefaultLSMKVStoreConfig()
	config.DataDir = tempDir
	config.EnableMigration = true

	store, err := NewLSMKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create LSM KVStore: %v", err)
	}
	defer store.Close()

	// Verify migration mode
	if !store.IsInMigrationMode() {
		t.Error("Store should be in migration mode")
	}

	// Test that we can read legacy data
	for key, expectedValue := range legacyData {
		value, err := store.Get(key)
		if err != nil {
			t.Errorf("Get failed for legacy key %s: %v", key, err)
			continue
		}
		if value != expectedValue {
			t.Errorf("Expected %s, got %s for legacy key %s", expectedValue, value, key)
		}
	}

	// Add new data
	if err := store.Put("new_key", "new_value"); err != nil {
		t.Errorf("Put failed for new key: %v", err)
	}

	// Complete migration
	if err := store.CompleteMigration(); err != nil {
		t.Errorf("Failed to complete migration: %v", err)
	}

	// Verify migration is complete
	if store.IsInMigrationMode() {
		t.Error("Store should not be in migration mode after completion")
	}

	// Verify all data is still accessible
	allData := make(map[string]string)
	for k, v := range legacyData {
		allData[k] = v
	}
	allData["new_key"] = "new_value"

	for key, expectedValue := range allData {
		value, err := store.Get(key)
		if err != nil {
			t.Errorf("Get failed after migration for key %s: %v", key, err)
			continue
		}
		if value != expectedValue {
			t.Errorf("Expected %s, got %s after migration for key %s", expectedValue, value, key)
		}
	}
}

func TestLSMTree_Compaction(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultLSMConfig()
	config.DataDir = tempDir
	config.MemTableConfig.MaxSize = 100  // Very small to trigger flushes
	config.MemTableConfig.MaxEntries = 2 // Very small
	config.L0MaxSSTables = 2             // Trigger compaction quickly

	lsm, err := NewLSMTree(config)
	if err != nil {
		t.Fatalf("Failed to create LSM-Tree: %v", err)
	}
	defer lsm.Close()

	// Add enough data to trigger multiple flushes and compaction
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("compaction_key_%03d", i)
		value := fmt.Sprintf("compaction_value_with_long_data_%03d", i)

		if err := lsm.Put(key, value); err != nil {
			t.Errorf("Put failed for %s: %v", key, err)
		}

		// Small delay to allow background processes
		if i%5 == 0 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Wait for compaction to complete
	time.Sleep(2 * time.Second)

	// Verify all data is still accessible
	for i := 0; i < 20; i++ {
		key := fmt.Sprintf("compaction_key_%03d", i)
		expectedValue := fmt.Sprintf("compaction_value_with_long_data_%03d", i)

		value, err := lsm.Get(key)
		if err != nil {
			t.Errorf("Get failed after compaction for %s: %v", key, err)
			continue
		}
		if value != expectedValue {
			t.Errorf("Expected %s, got %s after compaction for key %s", expectedValue, value, key)
		}
	}

	// Check that compaction occurred
	stats := lsm.GetStats()
	if stats.CompactionCount == 0 {
		t.Log("Warning: No compactions occurred during test")
	}
}

// Helper function to verify file exists
func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// Benchmark tests
func BenchmarkLSMTree_Put(b *testing.B) {
	tempDir := b.TempDir()
	config := DefaultLSMConfig()
	config.DataDir = tempDir

	lsm, err := NewLSMTree(config)
	if err != nil {
		b.Fatalf("Failed to create LSM-Tree: %v", err)
	}
	defer lsm.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_key_%d", i)
		value := fmt.Sprintf("bench_value_%d", i)
		if err := lsm.Put(key, value); err != nil {
			b.Errorf("Put failed: %v", err)
		}
	}
}

func BenchmarkLSMTree_Get(b *testing.B) {
	tempDir := b.TempDir()
	config := DefaultLSMConfig()
	config.DataDir = tempDir

	lsm, err := NewLSMTree(config)
	if err != nil {
		b.Fatalf("Failed to create LSM-Tree: %v", err)
	}
	defer lsm.Close()

	// Populate with test data
	numKeys := 10000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("bench_key_%d", i)
		value := fmt.Sprintf("bench_value_%d", i)
		if err := lsm.Put(key, value); err != nil {
			b.Fatalf("Setup Put failed: %v", err)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_key_%d", i%numKeys)
		_, err := lsm.Get(key)
		if err != nil {
			b.Errorf("Get failed: %v", err)
		}
	}
}

func BenchmarkBloomFilter_Add(b *testing.B) {
	bf := NewBloomFilter(uint64(b.N), 0.01)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_bloom_key_%d", i)
		bf.Add([]byte(key))
	}
}

func BenchmarkBloomFilter_MightContain(b *testing.B) {
	bf := NewBloomFilter(10000, 0.01)

	// Populate bloom filter
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("bench_bloom_key_%d", i)
		bf.Add([]byte(key))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("bench_bloom_key_%d", i%10000)
		bf.MightContain([]byte(key))
	}
}
