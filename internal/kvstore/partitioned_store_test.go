package kvstore

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestPartitionedKVStore_Basic(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	config := PartitionConfig{
		NumPartitions: 4,
		DataDir:       tempDir,
		BatchSize:     10,
		FlushInterval: 50 * time.Millisecond,
	}

	store, err := NewPartitionedKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create partitioned store: %v", err)
	}
	defer store.Close()

	// Test basic operations
	tests := []struct {
		key   string
		value string
	}{
		{"key1", "value1"},
		{"key2", "value2"},
		{"key3", "value3"},
		{"key4", "value4"},
	}

	// Test Put operations
	for _, test := range tests {
		if err := store.Put(test.key, test.value); err != nil {
			t.Fatalf("Put failed for %s: %v", test.key, err)
		}
	}

	// Ensure flush
	if err := store.FlushAll(); err != nil {
		t.Fatalf("FlushAll failed: %v", err)
	}

	// Test Get operations
	for _, test := range tests {
		value, err := store.Get(test.key)
		if err != nil {
			t.Errorf("Get failed for %s: %v", test.key, err)
			continue
		}
		if value != test.value {
			t.Errorf("Get returned wrong value for %s: got %s, want %s",
				test.key, value, test.value)
		}
	}

	// Test List operation
	keys, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(keys) != len(tests) {
		t.Errorf("List returned wrong number of keys: got %d, want %d",
			len(keys), len(tests))
	}

	// Test Delete operation
	if err := store.Delete("key1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Ensure flush after delete
	if err := store.FlushAll(); err != nil {
		t.Fatalf("FlushAll after delete failed: %v", err)
	}

	// Verify deletion
	_, err = store.Get("key1")
	if err == nil {
		t.Error("Expected error when getting deleted key")
	}

	// Verify other keys still exist
	for _, test := range tests[1:] {
		value, err := store.Get(test.key)
		if err != nil {
			t.Errorf("Get failed for %s after delete: %v", test.key, err)
			continue
		}
		if value != test.value {
			t.Errorf("Get returned wrong value for %s after delete: got %s, want %s",
				test.key, value, test.value)
		}
	}
}

func TestPartitionedKVStore_Concurrency(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	config := PartitionConfig{
		NumPartitions: 8,
		DataDir:       tempDir,
		BatchSize:     50,
		FlushInterval: 100 * time.Millisecond,
	}

	store, err := NewPartitionedKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create partitioned store: %v", err)
	}
	defer store.Close()

	// Concurrent write test
	numGoroutines := 10
	entriesPerGoroutine := 100
	var wg sync.WaitGroup

	// Start concurrent writers
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < entriesPerGoroutine; j++ {
				key := fmt.Sprintf("worker%d_key%d", workerID, j)
				value := fmt.Sprintf("worker%d_value%d", workerID, j)
				if err := store.Put(key, value); err != nil {
					t.Errorf("Put failed for %s: %v", key, err)
				}
			}
		}(i)
	}

	wg.Wait()

	// Ensure all data is flushed
	if err := store.FlushAll(); err != nil {
		t.Fatalf("FlushAll failed: %v", err)
	}

	// Verify all entries
	expectedCount := numGoroutines * entriesPerGoroutine
	keys, err := store.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(keys) != expectedCount {
		t.Errorf("Expected %d keys, got %d", expectedCount, len(keys))
	}

	// Verify data integrity
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < entriesPerGoroutine; j++ {
			key := fmt.Sprintf("worker%d_key%d", i, j)
			expectedValue := fmt.Sprintf("worker%d_value%d", i, j)

			value, err := store.Get(key)
			if err != nil {
				t.Errorf("Get failed for %s: %v", key, err)
				continue
			}
			if value != expectedValue {
				t.Errorf("Get returned wrong value for %s: got %s, want %s",
					key, value, expectedValue)
			}
		}
	}
}

func TestPartitionedKVStore_BatchFlush(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	config := PartitionConfig{
		NumPartitions: 2,
		DataDir:       tempDir,
		BatchSize:     5,               // Small batch size for testing
		FlushInterval: 1 * time.Second, // Long interval to test manual flush
	}

	store, err := NewPartitionedKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create partitioned store: %v", err)
	}
	defer store.Close()

	// Add entries below batch size threshold
	for i := 0; i < 3; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		if err := store.Put(key, value); err != nil {
			t.Fatalf("Put failed for %s: %v", key, err)
		}
	}

	// Data should be in buffer, not yet on disk
	// This is harder to test directly, but we can verify the buffer is working

	// Add more entries to trigger batch flush
	for i := 3; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		if err := store.Put(key, value); err != nil {
			t.Fatalf("Put failed for %s: %v", key, err)
		}
	}

	// Some batches should have flushed automatically
	// Manually flush to ensure all data is persisted
	if err := store.FlushAll(); err != nil {
		t.Fatalf("FlushAll failed: %v", err)
	}

	// Verify all data is accessible
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		expectedValue := fmt.Sprintf("value%d", i)

		value, err := store.Get(key)
		if err != nil {
			t.Errorf("Get failed for %s: %v", key, err)
			continue
		}
		if value != expectedValue {
			t.Errorf("Get returned wrong value for %s: got %s, want %s",
				key, value, expectedValue)
		}
	}
}

func TestPartitionedKVStore_Statistics(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	config := PartitionConfig{
		NumPartitions: 3,
		DataDir:       tempDir,
		BatchSize:     100,
		FlushInterval: 100 * time.Millisecond,
	}

	store, err := NewPartitionedKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create partitioned store: %v", err)
	}
	defer store.Close()

	// Add some test data
	for i := 0; i < 15; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		if err := store.Put(key, value); err != nil {
			t.Fatalf("Put failed for %s: %v", key, err)
		}
	}

	if err := store.FlushAll(); err != nil {
		t.Fatalf("FlushAll failed: %v", err)
	}

	// Get statistics
	stats, err := store.GetPartitionedStats()
	if err != nil {
		t.Fatalf("GetPartitionedStats failed: %v", err)
	}

	// Verify basic stats
	if stats["store_type"] != "partitioned" {
		t.Errorf("Expected store_type 'partitioned', got %v", stats["store_type"])
	}
	if stats["num_partitions"] != 3 {
		t.Errorf("Expected num_partitions 3, got %v", stats["num_partitions"])
	}
	if stats["total_entries"] != 15 {
		t.Errorf("Expected total_entries 15, got %v", stats["total_entries"])
	}

	// Verify partition stats
	partitionStats, ok := stats["partitions"].([]map[string]interface{})
	if !ok {
		t.Fatal("Expected partitions to be []map[string]interface{}")
	}
	if len(partitionStats) != 3 {
		t.Errorf("Expected 3 partition stats, got %d", len(partitionStats))
	}

	// Verify total entries match sum of partition entries
	totalFromPartitions := 0
	for _, pStats := range partitionStats {
		entries, ok := pStats["entries"].(int)
		if !ok {
			t.Errorf("Expected entries to be int, got %T", pStats["entries"])
			continue
		}
		totalFromPartitions += entries
	}
	if totalFromPartitions != 15 {
		t.Errorf("Sum of partition entries (%d) doesn't match total (%d)",
			totalFromPartitions, 15)
	}
}

func TestPartitionedKVStore_Compaction(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	config := PartitionConfig{
		NumPartitions: 2,
		DataDir:       tempDir,
		BatchSize:     10,
		FlushInterval: 50 * time.Millisecond,
	}

	store, err := NewPartitionedKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create partitioned store: %v", err)
	}
	defer store.Close()

	// Add initial data
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		if err := store.Put(key, value); err != nil {
			t.Fatalf("Put failed for %s: %v", key, err)
		}
	}

	// Update some entries (creates duplicates)
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("updated_value%d", i)
		if err := store.Put(key, value); err != nil {
			t.Fatalf("Put update failed for %s: %v", key, err)
		}
	}

	// Delete some entries
	for i := 5; i < 8; i++ {
		key := fmt.Sprintf("key%d", i)
		if err := store.Delete(key); err != nil {
			t.Fatalf("Delete failed for %s: %v", key, err)
		}
	}

	if err := store.FlushAll(); err != nil {
		t.Fatalf("FlushAll failed: %v", err)
	}

	// Perform compaction
	if err := store.Compact(); err != nil {
		t.Fatalf("Compact failed: %v", err)
	}

	// Verify final state
	expectedKeys := []string{"key0", "key1", "key2", "key3", "key4", "key8", "key9"}
	keys, err := store.List()
	if err != nil {
		t.Fatalf("List failed after compaction: %v", err)
	}

	if len(keys) != len(expectedKeys) {
		t.Errorf("Expected %d keys after compaction, got %d",
			len(expectedKeys), len(keys))
	}

	// Verify updated values are correct
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("key%d", i)
		expectedValue := fmt.Sprintf("updated_value%d", i)

		value, err := store.Get(key)
		if err != nil {
			t.Errorf("Get failed for %s after compaction: %v", key, err)
			continue
		}
		if value != expectedValue {
			t.Errorf("Get returned wrong updated value for %s: got %s, want %s",
				key, value, expectedValue)
		}
	}

	// Verify deleted keys are gone
	for i := 5; i < 8; i++ {
		key := fmt.Sprintf("key%d", i)
		_, err := store.Get(key)
		if err == nil {
			t.Errorf("Expected error when getting deleted key %s", key)
		}
	}
}

func TestPartitionedKVStore_PartitionDistribution(t *testing.T) {
	// Setup
	tempDir := t.TempDir()
	config := PartitionConfig{
		NumPartitions: 4,
		DataDir:       tempDir,
		BatchSize:     1000, // Large batch to avoid auto-flush
		FlushInterval: 10 * time.Second,
	}

	store, err := NewPartitionedKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create partitioned store: %v", err)
	}
	defer store.Close()

	// Add many entries to test distribution
	numEntries := 1000
	for i := 0; i < numEntries; i++ {
		key := fmt.Sprintf("key_%d", i)
		value := fmt.Sprintf("value_%d", i)
		if err := store.Put(key, value); err != nil {
			t.Fatalf("Put failed for %s: %v", key, err)
		}
	}

	if err := store.FlushAll(); err != nil {
		t.Fatalf("FlushAll failed: %v", err)
	}

	// Check that all partitions have some data (rough distribution test)
	stats, err := store.GetPartitionedStats()
	if err != nil {
		t.Fatalf("GetPartitionedStats failed: %v", err)
	}

	partitionStats, ok := stats["partitions"].([]map[string]interface{})
	if !ok {
		t.Fatal("Expected partitions to be []map[string]interface{}")
	}

	emptyPartitions := 0
	for _, pStats := range partitionStats {
		entries, ok := pStats["entries"].(int)
		if !ok {
			t.Errorf("Expected entries to be int, got %T", pStats["entries"])
			continue
		}
		if entries == 0 {
			emptyPartitions++
		}
	}

	// With 1000 entries and 4 partitions, it's very unlikely any partition would be empty
	// if the hash distribution is working properly
	if emptyPartitions > 0 {
		t.Errorf("Found %d empty partitions out of %d - distribution may be poor",
			emptyPartitions, config.NumPartitions)
	}

	// Verify partition directories were created
	for i := 0; i < config.NumPartitions; i++ {
		partitionDir := filepath.Join(tempDir, fmt.Sprintf("partition_%d", i))
		if _, err := os.Stat(partitionDir); os.IsNotExist(err) {
			t.Errorf("Partition directory %s was not created", partitionDir)
		}
	}
}

func TestPartitionedKVStore_Configuration(t *testing.T) {
	tempDir := t.TempDir()

	// Test invalid partition counts
	invalidConfigs := []PartitionConfig{
		{NumPartitions: 0, DataDir: tempDir},
		{NumPartitions: -1, DataDir: tempDir},
		{NumPartitions: 17, DataDir: tempDir}, // Over maximum
	}

	for _, config := range invalidConfigs {
		_, err := NewPartitionedKVStore(config)
		if err == nil {
			t.Errorf("Expected error for invalid config with %d partitions",
				config.NumPartitions)
		}
	}

	// Test valid configuration
	validConfig := PartitionConfig{
		NumPartitions: 8,
		DataDir:       tempDir,
		BatchSize:     50,
		FlushInterval: 100 * time.Millisecond,
	}

	store, err := NewPartitionedKVStore(validConfig)
	if err != nil {
		t.Fatalf("Failed to create store with valid config: %v", err)
	}
	defer store.Close()

	// Verify configuration is applied
	if store.config.NumPartitions != 8 {
		t.Errorf("Expected NumPartitions 8, got %d", store.config.NumPartitions)
	}
	if store.config.BatchSize != 50 {
		t.Errorf("Expected BatchSize 50, got %d", store.config.BatchSize)
	}
}
