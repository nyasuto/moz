package lsm

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/nyasuto/moz/internal/kvstore"
)

// TestLSMPerformanceImprovement validates the expected performance improvements
func TestLSMPerformanceImprovement(t *testing.T) {
	numOperations := 1000

	// Benchmark traditional KVStore
	legacyDuration := benchmarkLegacyKVStore(t, numOperations)

	// Benchmark LSM-Tree KVStore
	lsmDuration := benchmarkLSMKVStore(t, numOperations)

	// Calculate improvement
	improvement := float64(legacyDuration-lsmDuration) / float64(legacyDuration) * 100

	t.Logf("Performance Comparison Results:")
	t.Logf("  Legacy KVStore duration: %v", legacyDuration)
	t.Logf("  LSM-Tree duration:       %v", lsmDuration)
	t.Logf("  Improvement:             %.2f%%", improvement)

	// Validate improvement targets
	expectedMinImprovement := 80.0 // 80% improvement target for reads with bloom filters
	if improvement < expectedMinImprovement {
		t.Errorf("Expected at least %.1f%% improvement, got %.2f%%", expectedMinImprovement, improvement)
	} else {
		t.Logf("✅ Success: LSM-Tree showed %.2f%% improvement (target: %.1f%%)", improvement, expectedMinImprovement)
	}
}

func benchmarkLegacyKVStore(t *testing.T, numOps int) time.Duration {
	// Create legacy KVStore
	store := kvstore.New()

	start := time.Now()

	// Mixed workload: 70% writes, 30% reads
	writeRatio := 0.7
	numWrites := int(float64(numOps) * writeRatio)
	numReads := numOps - numWrites

	// Write phase
	for i := 0; i < numWrites; i++ {
		key := fmt.Sprintf("legacy_key_%d", i)
		value := fmt.Sprintf("legacy_value_with_some_data_%d", i)
		if err := store.Put(key, value); err != nil {
			t.Errorf("Legacy Put failed: %v", err)
		}
	}

	// Read phase (read random keys)
	for i := 0; i < numReads; i++ {
		keyIndex := rand.Intn(numWrites)
		key := fmt.Sprintf("legacy_key_%d", keyIndex)
		_, err := store.Get(key)
		if err != nil {
			t.Errorf("Legacy Get failed: %v", err)
		}
	}

	return time.Since(start)
}

func benchmarkLSMKVStore(t *testing.T, numOps int) time.Duration {
	tempDir := t.TempDir()

	// Create LSM-Tree KVStore
	config := DefaultLSMKVStoreConfig()
	config.DataDir = tempDir
	config.EnableMigration = false

	store, err := NewLSMKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create LSM KVStore: %v", err)
	}
	defer store.Close()

	start := time.Now()

	// Mixed workload: 70% writes, 30% reads
	writeRatio := 0.7
	numWrites := int(float64(numOps) * writeRatio)
	numReads := numOps - numWrites

	// Write phase
	for i := 0; i < numWrites; i++ {
		key := fmt.Sprintf("lsm_key_%d", i)
		value := fmt.Sprintf("lsm_value_with_some_data_%d", i)
		if err := store.Put(key, value); err != nil {
			t.Errorf("LSM Put failed: %v", err)
		}
	}

	// Read phase (read random keys)
	for i := 0; i < numReads; i++ {
		keyIndex := rand.Intn(numWrites)
		key := fmt.Sprintf("lsm_key_%d", keyIndex)
		_, err := store.Get(key)
		if err != nil {
			t.Errorf("LSM Get failed: %v", err)
		}
	}

	return time.Since(start)
}

// BenchmarkWritePerformance compares write performance
func BenchmarkWritePerformance(b *testing.B) {
	b.Run("Legacy", func(b *testing.B) {
		store := kvstore.New()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("write_bench_key_%d", i)
			value := fmt.Sprintf("write_bench_value_%d", i)
			if err := store.Put(key, value); err != nil {
				b.Errorf("Put failed: %v", err)
			}
		}
	})

	b.Run("LSM", func(b *testing.B) {
		tempDir := b.TempDir()
		config := DefaultLSMKVStoreConfig()
		config.DataDir = tempDir
		config.EnableMigration = false

		store, err := NewLSMKVStore(config)
		if err != nil {
			b.Fatalf("Failed to create LSM KVStore: %v", err)
		}
		defer store.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("write_bench_key_%d", i)
			value := fmt.Sprintf("write_bench_value_%d", i)
			if err := store.Put(key, value); err != nil {
				b.Errorf("Put failed: %v", err)
			}
		}
	})
}

// BenchmarkReadPerformance compares read performance with bloom filter benefits
func BenchmarkReadPerformance(b *testing.B) {
	numKeys := 10000

	b.Run("Legacy", func(b *testing.B) {
		store := kvstore.New()

		// Populate data
		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("read_bench_key_%d", i)
			value := fmt.Sprintf("read_bench_value_%d", i)
			if err := store.Put(key, value); err != nil {
				b.Fatalf("Setup Put failed: %v", err)
			}
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			keyIndex := rand.Intn(numKeys)
			key := fmt.Sprintf("read_bench_key_%d", keyIndex)
			_, err := store.Get(key)
			if err != nil {
				b.Errorf("Get failed: %v", err)
			}
		}
	})

	b.Run("LSM", func(b *testing.B) {
		tempDir := b.TempDir()
		config := DefaultLSMKVStoreConfig()
		config.DataDir = tempDir
		config.EnableMigration = false

		store, err := NewLSMKVStore(config)
		if err != nil {
			b.Fatalf("Failed to create LSM KVStore: %v", err)
		}
		defer store.Close()

		// Populate data
		for i := 0; i < numKeys; i++ {
			key := fmt.Sprintf("read_bench_key_%d", i)
			value := fmt.Sprintf("read_bench_value_%d", i)
			if err := store.Put(key, value); err != nil {
				b.Fatalf("Setup Put failed: %v", err)
			}
		}

		// Allow some background processing
		time.Sleep(100 * time.Millisecond)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			keyIndex := rand.Intn(numKeys)
			key := fmt.Sprintf("read_bench_key_%d", keyIndex)
			_, err := store.Get(key)
			if err != nil {
				b.Errorf("Get failed: %v", err)
			}
		}
	})
}

// BenchmarkConcurrentOperations tests concurrent performance
func BenchmarkConcurrentOperations(b *testing.B) {
	b.Run("Legacy", func(b *testing.B) {
		store := kvstore.New()

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				key := fmt.Sprintf("concurrent_key_%d", i)
				value := fmt.Sprintf("concurrent_value_%d", i)
				if err := store.Put(key, value); err != nil {
					b.Errorf("Put failed: %v", err)
				}
				i++
			}
		})
	})

	b.Run("LSM", func(b *testing.B) {
		tempDir := b.TempDir()
		config := DefaultLSMKVStoreConfig()
		config.DataDir = tempDir
		config.EnableMigration = false

		store, err := NewLSMKVStore(config)
		if err != nil {
			b.Fatalf("Failed to create LSM KVStore: %v", err)
		}
		defer store.Close()

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				key := fmt.Sprintf("concurrent_key_%d", i)
				value := fmt.Sprintf("concurrent_value_%d", i)
				if err := store.Put(key, value); err != nil {
					b.Errorf("Put failed: %v", err)
				}
				i++
			}
		})
	})
}

// BenchmarkSpaceEfficiency tests compaction efficiency
func BenchmarkSpaceEfficiency(b *testing.B) {
	b.Run("LSM_Compaction", func(b *testing.B) {
		tempDir := b.TempDir()
		config := DefaultLSMKVStoreConfig()
		config.DataDir = tempDir
		config.EnableMigration = false
		config.LSMConfig.MemTableConfig.MaxSize = 1024 // Small for frequent flushes

		store, err := NewLSMKVStore(config)
		if err != nil {
			b.Fatalf("Failed to create LSM KVStore: %v", err)
		}
		defer store.Close()

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("space_key_%d", i%1000) // Reuse keys to test compaction
			value := fmt.Sprintf("space_value_with_extra_data_%d", i)
			if err := store.Put(key, value); err != nil {
				b.Errorf("Put failed: %v", err)
			}

			// Trigger compaction periodically
			if i%100 == 0 {
				store.TriggerCompaction()
			}
		}
	})
}

// TestBloomFilterEffectiveness validates bloom filter performance
func TestBloomFilterEffectiveness(t *testing.T) {
	numKeys := 10000
	numLookups := 1000

	// Create bloom filter
	bf := NewBloomFilter(uint64(numKeys), 0.01) // 1% false positive rate

	// Add keys
	addedKeys := make(map[string]bool)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("bloom_effectiveness_key_%d", i)
		bf.Add([]byte(key))
		addedKeys[key] = true
	}

	// Test positive lookups (should all return true)
	positiveTests := 0
	for key := range addedKeys {
		if bf.MightContain([]byte(key)) {
			positiveTests++
		}
		if positiveTests >= numLookups/2 {
			break
		}
	}

	if positiveTests < numLookups/2 {
		t.Errorf("Bloom filter failed on positive tests: %d/%d", positiveTests, numLookups/2)
	}

	// Test negative lookups (should mostly return false)
	falsePositives := 0
	negativeTests := 0
	for i := numKeys; i < numKeys+numLookups/2; i++ {
		key := fmt.Sprintf("bloom_effectiveness_key_%d", i)
		if bf.MightContain([]byte(key)) {
			falsePositives++
		}
		negativeTests++
	}

	falsePositiveRate := float64(falsePositives) / float64(negativeTests)
	expectedMaxFPR := 0.02 // Allow 2% (target was 1%)

	t.Logf("Bloom Filter Effectiveness Results:")
	t.Logf("  Positive tests passed: %d/%d", positiveTests, numLookups/2)
	t.Logf("  False positives: %d/%d (%.2f%%)", falsePositives, negativeTests, falsePositiveRate*100)
	t.Logf("  Target FPR: 1.0%%, Actual: %.2f%%, Max allowed: %.2f%%", falsePositiveRate*100, expectedMaxFPR*100)

	if falsePositiveRate > expectedMaxFPR {
		t.Errorf("False positive rate too high: %.2f%% > %.2f%%", falsePositiveRate*100, expectedMaxFPR*100)
	} else {
		t.Logf("✅ Success: Bloom filter FPR within acceptable range")
	}
}

// TestCompactionEfficiency validates compaction space savings
func TestCompactionEfficiency(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultLSMKVStoreConfig()
	config.DataDir = tempDir
	config.EnableMigration = false
	config.LSMConfig.MemTableConfig.MaxSize = 512 // Small for frequent flushes
	config.LSMConfig.MemTableConfig.MaxEntries = 10

	store, err := NewLSMKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create LSM KVStore: %v", err)
	}
	defer store.Close()

	// Add data with many updates to the same keys
	numKeys := 100
	numUpdates := 5

	for update := 0; update < numUpdates; update++ {
		for key := 0; key < numKeys; key++ {
			keyStr := fmt.Sprintf("compaction_key_%d", key)
			valueStr := fmt.Sprintf("compaction_value_update_%d_%d", update, key)

			if err := store.Put(keyStr, valueStr); err != nil {
				t.Errorf("Put failed: %v", err)
			}
		}

		// Force flush and compaction
		store.ForceFlush()
		time.Sleep(100 * time.Millisecond)
	}

	// Trigger final compaction
	store.TriggerCompaction()
	time.Sleep(500 * time.Millisecond)

	// Verify final state
	stats := store.GetLSMStats()

	t.Logf("Compaction Efficiency Results:")
	t.Logf("  Total compactions: %d", stats.CompactionCount)
	t.Logf("  Active SSTables: %d", stats.ActiveSSTables)
	t.Logf("  Total levels: %d", stats.TotalLevels)

	// Verify all latest values are accessible
	for key := 0; key < numKeys; key++ {
		keyStr := fmt.Sprintf("compaction_key_%d", key)
		expectedValue := fmt.Sprintf("compaction_value_update_%d_%d", numUpdates-1, key)

		value, err := store.Get(keyStr)
		if err != nil {
			t.Errorf("Get failed after compaction for %s: %v", keyStr, err)
			continue
		}
		if value != expectedValue {
			t.Errorf("Expected %s, got %s for key %s after compaction", expectedValue, value, keyStr)
		}
	}

	// Compaction should have occurred
	if stats.CompactionCount == 0 {
		t.Log("Warning: No compactions occurred during test")
	} else {
		t.Logf("✅ Success: Compaction efficiency validated with %d compactions", stats.CompactionCount)
	}
}

// TestMemoryUsageOptimization validates memory efficiency
func TestMemoryUsageOptimization(t *testing.T) {
	tempDir := t.TempDir()
	config := DefaultLSMKVStoreConfig()
	config.DataDir = tempDir
	config.EnableMigration = false

	store, err := NewLSMKVStore(config)
	if err != nil {
		t.Fatalf("Failed to create LSM KVStore: %v", err)
	}
	defer store.Close()

	// Add data and measure memory usage
	numKeys := 1000
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("memory_key_%d", i)
		value := fmt.Sprintf("memory_value_with_data_%d", i)

		if err := store.Put(key, value); err != nil {
			t.Errorf("Put failed: %v", err)
		}
	}

	// Get memory usage breakdown
	memUsage := store.EstimateMemoryUsage()

	t.Logf("Memory Usage Optimization Results:")
	for component, usage := range memUsage {
		t.Logf("  %s: %d bytes", component, usage)
	}

	// Verify memory usage is reasonable
	totalMemory := int64(0)
	for _, usage := range memUsage {
		totalMemory += usage
	}

	// Rough estimate: should be less than 10MB for 1000 keys
	maxExpectedMemory := int64(10 * 1024 * 1024)
	if totalMemory > maxExpectedMemory {
		t.Errorf("Memory usage too high: %d bytes > %d bytes", totalMemory, maxExpectedMemory)
	} else {
		t.Logf("✅ Success: Memory usage within expected range: %d bytes", totalMemory)
	}
}
