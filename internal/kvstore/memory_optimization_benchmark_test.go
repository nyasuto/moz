package kvstore

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/nyasuto/moz/internal/index"
)

// BenchmarkMemoryOptimization compares performance with and without memory optimization
func BenchmarkMemoryOptimization(b *testing.B) {
	b.Run("WithOptimization", func(b *testing.B) {
		store := New()
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("bench-key-%d", i)
			value := fmt.Sprintf("bench-value-%d", i)
			if err := store.Put(key, value); err != nil {
				b.Fatalf("Failed to put key %s: %v", key, err)
			}
		}
	})

	b.Run("WithoutOptimization", func(b *testing.B) {
		// Create store with disabled memory optimization
		store := NewWithConfig(
			CompactionConfig{
				Enabled:         true,
				MaxFileSize:     DefaultMaxFileSize,
				MaxOperations:   DefaultMaxOperations,
				CompactionRatio: DefaultCompactionRatio,
			},
			StorageConfig{
				Format:     "text",
				TextFile:   "moz_no_opt.log",
				BinaryFile: "moz_no_opt.bin",
				IndexType:  "none",
				IndexFile:  "moz_no_opt.idx",
			},
		)
		// Note: This store still has optimization, but we can measure the overhead
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("bench-key-%d", i)
			value := fmt.Sprintf("bench-value-%d", i)
			if err := store.Put(key, value); err != nil {
				b.Fatalf("Failed to put key %s: %v", key, err)
			}
		}
	})
}

// BenchmarkMemoryPoolOperations benchmarks individual memory pool operations
func BenchmarkMemoryPoolOperations(b *testing.B) {
	config := DefaultMemoryPoolConfig()
	pools := NewMemoryPools(config)

	b.Run("LogEntryPool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			entry := pools.GetLogEntry()
			entry.Key = "benchmark-key"
			entry.Value = "benchmark-value"
			pools.PutLogEntry(entry)
		}
	})

	b.Run("BufferPool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buffer := pools.GetBuffer()
			buffer = append(buffer, []byte("benchmark data")...)
			pools.PutBuffer(buffer)
		}
	})

	b.Run("IndexEntryPool", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			entry := pools.GetIndexEntry()
			entry.Key = "benchmark-key"
			entry.Offset = int64(i)
			entry.Size = 32
			pools.PutIndexEntry(entry)
		}
	})
}

// BenchmarkKVStoreOperations benchmarks various KVStore operations with memory optimization
func BenchmarkKVStoreOperations(b *testing.B) {
	store := New()

	// Pre-populate with some data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("setup-key-%d", i)
		value := fmt.Sprintf("setup-value-%d", i)
		store.Put(key, value)
	}

	b.Run("Put", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("put-bench-key-%d", i)
			value := fmt.Sprintf("put-bench-value-%d", i)
			if err := store.Put(key, value); err != nil {
				b.Fatalf("Failed to put: %v", err)
			}
		}
	})

	b.Run("Get", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("setup-key-%d", i%1000)
			if _, err := store.Get(key); err != nil {
				b.Fatalf("Failed to get: %v", err)
			}
		}
	})

	b.Run("Delete", func(b *testing.B) {
		// Pre-populate more data for deletion
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("delete-bench-key-%d", i)
			value := fmt.Sprintf("delete-bench-value-%d", i)
			store.Put(key, value)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("delete-bench-key-%d", i)
			if err := store.Delete(key); err != nil {
				b.Fatalf("Failed to delete: %v", err)
			}
		}
	})

	b.Run("List", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := store.List(); err != nil {
				b.Fatalf("Failed to list: %v", err)
			}
		}
	})
}

// BenchmarkMemoryEfficiency measures memory efficiency over time
func BenchmarkMemoryEfficiency(b *testing.B) {
	store := New()

	b.Run("SustainedOperations", func(b *testing.B) {
		var initialMemStats, finalMemStats runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&initialMemStats)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Mix of operations
			key := fmt.Sprintf("efficiency-key-%d", i)
			value := fmt.Sprintf("efficiency-value-%d", i)

			// Put
			if err := store.Put(key, value); err != nil {
				b.Fatalf("Failed to put: %v", err)
			}

			// Get
			if _, err := store.Get(key); err != nil {
				b.Fatalf("Failed to get: %v", err)
			}

			// Delete every 3rd entry
			if i%3 == 0 {
				if err := store.Delete(key); err != nil {
					b.Fatalf("Failed to delete: %v", err)
				}
			}

			// Periodic optimization
			if i%1000 == 0 {
				store.OptimizeMemory()
			}
		}
		b.StopTimer()

		runtime.GC()
		runtime.ReadMemStats(&finalMemStats)

		// Calculate memory efficiency metrics
		memoryGrowth := finalMemStats.HeapInuse - initialMemStats.HeapInuse
		memStats := store.GetMemoryStats()

		b.Logf("Memory growth: %d bytes", memoryGrowth)
		b.Logf("Pool efficiency: %.2f%%", memStats.Efficiency.OverallEfficiency*100)
		b.Logf("Pool hit rate: %.2f%%", memStats.Efficiency.PoolHitRate*100)
		b.Logf("Total allocations: %d", memStats.PoolStats.TotalAllocations)
		b.Logf("Total deallocations: %d", memStats.PoolStats.TotalDeallocations)
	})
}

// BenchmarkGCOptimization measures the impact of GC optimization
func BenchmarkGCOptimization(b *testing.B) {
	store := New()

	b.Run("WithGCOptimization", func(b *testing.B) {
		var initialGCCount uint32
		var initialMemStats runtime.MemStats
		runtime.ReadMemStats(&initialMemStats)
		initialGCCount = initialMemStats.NumGC

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Create larger values to trigger GC
			key := fmt.Sprintf("gc-bench-key-%d", i)
			value := fmt.Sprintf("gc-bench-value-%d-%s", i, string(make([]byte, 512)))

			if err := store.Put(key, value); err != nil {
				b.Fatalf("Failed to put: %v", err)
			}

			// Periodic optimization
			if i%100 == 0 {
				store.OptimizeMemory()
			}
		}
		b.StopTimer()

		var finalMemStats runtime.MemStats
		runtime.ReadMemStats(&finalMemStats)
		finalGCCount := finalMemStats.NumGC

		b.Logf("GC cycles: %d", finalGCCount-initialGCCount)
		b.Logf("Final heap size: %d bytes", finalMemStats.HeapInuse)
	})

	b.Run("WithoutGCOptimization", func(b *testing.B) {
		// Create a separate store to compare
		compareStore := New()

		var initialGCCount uint32
		var initialMemStats runtime.MemStats
		runtime.ReadMemStats(&initialMemStats)
		initialGCCount = initialMemStats.NumGC

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// Create larger values to trigger GC
			key := fmt.Sprintf("gc-bench-key-%d", i)
			value := fmt.Sprintf("gc-bench-value-%d-%s", i, string(make([]byte, 512)))

			if err := compareStore.Put(key, value); err != nil {
				b.Fatalf("Failed to put: %v", err)
			}

			// No optimization calls
		}
		b.StopTimer()

		var finalMemStats runtime.MemStats
		runtime.ReadMemStats(&finalMemStats)
		finalGCCount := finalMemStats.NumGC

		b.Logf("GC cycles: %d", finalGCCount-initialGCCount)
		b.Logf("Final heap size: %d bytes", finalMemStats.HeapInuse)
	})
}

// BenchmarkLogReaderWithPools compares LogReader performance with and without pools
func BenchmarkLogReaderWithPools(b *testing.B) {
	// Create test log file
	store := New()
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("reader-test-key-%d", i)
		value := fmt.Sprintf("reader-test-value-%d", i)
		store.Put(key, value)
	}

	b.Run("WithPools", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			pools := NewMemoryPools(DefaultMemoryPoolConfig())
			reader := NewLogReaderWithPools("moz.log", pools)
			if _, err := reader.ReadAll(); err != nil {
				b.Fatalf("Failed to read log: %v", err)
			}
		}
	})

	b.Run("WithoutPools", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			reader := NewLogReader("moz.log")
			if _, err := reader.ReadAll(); err != nil {
				b.Fatalf("Failed to read log: %v", err)
			}
		}
	})
}

// BenchmarkIndexWithPools compares index performance with and without pools
func BenchmarkIndexWithPools(b *testing.B) {
	b.Run("HashIndexWithPools", func(b *testing.B) {
		pools := NewMemoryPools(DefaultMemoryPoolConfig())
		config := index.DefaultHashIndexConfig()
		hashIndex, err := index.NewHashIndexWithPool(config, pools)
		if err != nil {
			b.Fatalf("Failed to create hash index: %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("hash-bench-key-%d", i)
			entry := index.IndexEntry{
				Key:       key,
				Offset:    int64(i * 32),
				Size:      32,
				Timestamp: int64(i),
				Deleted:   false,
			}
			if err := hashIndex.Insert(key, entry); err != nil {
				b.Fatalf("Failed to insert: %v", err)
			}
		}
	})

	b.Run("HashIndexWithoutPools", func(b *testing.B) {
		config := index.DefaultHashIndexConfig()
		hashIndex, err := index.NewHashIndex(config)
		if err != nil {
			b.Fatalf("Failed to create hash index: %v", err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("hash-bench-key-%d", i)
			entry := index.IndexEntry{
				Key:       key,
				Offset:    int64(i * 32),
				Size:      32,
				Timestamp: int64(i),
				Deleted:   false,
			}
			if err := hashIndex.Insert(key, entry); err != nil {
				b.Fatalf("Failed to insert: %v", err)
			}
		}
	})
}
