package index

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

const (
	benchmarkDataSize = 10000
	keyPrefix         = "benchmark_key_"
)

// generateBenchmarkData creates test data for benchmarks
func generateBenchmarkData(size int) map[string]IndexEntry {
	entries := make(map[string]IndexEntry, size)
	for i := 0; i < size; i++ {
		key := fmt.Sprintf("%s%06d", keyPrefix, i)
		entries[key] = IndexEntry{
			Key:       key,
			Offset:    int64(i * 100),
			Size:      int32(50 + i%50), // Variable size 50-99
			Timestamp: time.Now().UnixNano(),
			Deleted:   false,
		}
	}
	return entries
}

// generateRandomKeys creates a slice of random keys for lookup benchmarks
func generateRandomKeys(size int, maxKey int) []string {
	keys := make([]string, size)
	for i := 0; i < size; i++ {
		keyNum := rand.Intn(maxKey)
		keys[i] = fmt.Sprintf("%s%06d", keyPrefix, keyNum)
	}
	return keys
}

// BenchmarkHashIndex_Insert benchmarks hash index insertion
func BenchmarkHashIndex_Insert(b *testing.B) {
	hi, err := NewHashIndex(DefaultHashIndexConfig())
	if err != nil {
		b.Fatalf("Failed to create hash index: %v", err)
	}
	defer hi.Close()

	entries := generateBenchmarkData(b.N)
	keys := make([]string, 0, b.N)
	values := make([]IndexEntry, 0, b.N)

	for key, entry := range entries {
		keys = append(keys, key)
		values = append(values, entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hi.Insert(keys[i], values[i])
	}
}

// BenchmarkBTreeIndex_Insert benchmarks B-tree index insertion
func BenchmarkBTreeIndex_Insert(b *testing.B) {
	bt, err := NewBTreeIndex(DefaultBTreeIndexConfig())
	if err != nil {
		b.Fatalf("Failed to create B-tree index: %v", err)
	}
	defer bt.Close()

	entries := generateBenchmarkData(b.N)
	keys := make([]string, 0, b.N)
	values := make([]IndexEntry, 0, b.N)

	for key, entry := range entries {
		keys = append(keys, key)
		values = append(values, entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bt.Insert(keys[i], values[i])
	}
}

// BenchmarkHashIndex_Get benchmarks hash index lookup
func BenchmarkHashIndex_Get(b *testing.B) {
	hi, err := NewHashIndex(DefaultHashIndexConfig())
	if err != nil {
		b.Fatalf("Failed to create hash index: %v", err)
	}
	defer hi.Close()

	// Pre-populate with data
	entries := generateBenchmarkData(benchmarkDataSize)
	for key, entry := range entries {
		hi.Insert(key, entry)
	}

	// Generate random lookup keys
	lookupKeys := generateRandomKeys(b.N, benchmarkDataSize)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hi.Get(lookupKeys[i])
	}
}

// BenchmarkBTreeIndex_Get benchmarks B-tree index lookup
func BenchmarkBTreeIndex_Get(b *testing.B) {
	bt, err := NewBTreeIndex(DefaultBTreeIndexConfig())
	if err != nil {
		b.Fatalf("Failed to create B-tree index: %v", err)
	}
	defer bt.Close()

	// Pre-populate with data
	entries := generateBenchmarkData(benchmarkDataSize)
	for key, entry := range entries {
		bt.Insert(key, entry)
	}

	// Generate random lookup keys
	lookupKeys := generateRandomKeys(b.N, benchmarkDataSize)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bt.Get(lookupKeys[i])
	}
}

// BenchmarkHashIndex_Range benchmarks hash index range queries
func BenchmarkHashIndex_Range(b *testing.B) {
	hi, err := NewHashIndex(DefaultHashIndexConfig())
	if err != nil {
		b.Fatalf("Failed to create hash index: %v", err)
	}
	defer hi.Close()

	// Pre-populate with data
	entries := generateBenchmarkData(benchmarkDataSize)
	for key, entry := range entries {
		hi.Insert(key, entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startKey := fmt.Sprintf("%s%06d", keyPrefix, i%1000)
		endKey := fmt.Sprintf("%s%06d", keyPrefix, (i%1000)+100)
		hi.Range(startKey, endKey)
	}
}

// BenchmarkBTreeIndex_Range benchmarks B-tree index range queries
func BenchmarkBTreeIndex_Range(b *testing.B) {
	bt, err := NewBTreeIndex(DefaultBTreeIndexConfig())
	if err != nil {
		b.Fatalf("Failed to create B-tree index: %v", err)
	}
	defer bt.Close()

	// Pre-populate with data
	entries := generateBenchmarkData(benchmarkDataSize)
	for key, entry := range entries {
		bt.Insert(key, entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		startKey := fmt.Sprintf("%s%06d", keyPrefix, i%1000)
		endKey := fmt.Sprintf("%s%06d", keyPrefix, (i%1000)+100)
		bt.Range(startKey, endKey)
	}
}

// BenchmarkHashIndex_Keys benchmarks getting all keys from hash index
func BenchmarkHashIndex_Keys(b *testing.B) {
	hi, err := NewHashIndex(DefaultHashIndexConfig())
	if err != nil {
		b.Fatalf("Failed to create hash index: %v", err)
	}
	defer hi.Close()

	// Pre-populate with data
	entries := generateBenchmarkData(1000) // Smaller dataset for Keys() benchmark
	for key, entry := range entries {
		hi.Insert(key, entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hi.Keys()
	}
}

// BenchmarkBTreeIndex_Keys benchmarks getting all keys from B-tree index
func BenchmarkBTreeIndex_Keys(b *testing.B) {
	bt, err := NewBTreeIndex(DefaultBTreeIndexConfig())
	if err != nil {
		b.Fatalf("Failed to create B-tree index: %v", err)
	}
	defer bt.Close()

	// Pre-populate with data
	entries := generateBenchmarkData(1000) // Smaller dataset for Keys() benchmark
	for key, entry := range entries {
		bt.Insert(key, entry)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bt.Keys()
	}
}

// BenchmarkHashIndex_BatchInsert benchmarks hash index batch insertion
func BenchmarkHashIndex_BatchInsert(b *testing.B) {
	entries := generateBenchmarkData(b.N)

	b.ResetTimer()
	hi, err := NewHashIndex(DefaultHashIndexConfig())
	if err != nil {
		b.Fatalf("Failed to create hash index: %v", err)
	}
	defer hi.Close()

	hi.BatchInsert(entries)
}

// BenchmarkBTreeIndex_BatchInsert benchmarks B-tree index batch insertion
func BenchmarkBTreeIndex_BatchInsert(b *testing.B) {
	entries := generateBenchmarkData(b.N)

	b.ResetTimer()
	bt, err := NewBTreeIndex(DefaultBTreeIndexConfig())
	if err != nil {
		b.Fatalf("Failed to create B-tree index: %v", err)
	}
	defer bt.Close()

	bt.BatchInsert(entries)
}

// BenchmarkIndexManager_HashVsBTree compares hash vs B-tree through manager
func BenchmarkIndexManager_HashVsBTree(b *testing.B) {
	testData := generateBenchmarkData(1000)

	b.Run("Hash", func(b *testing.B) {
		im, err := NewIndexManager(IndexTypeHash)
		if err != nil {
			b.Fatalf("Failed to create hash index manager: %v", err)
		}
		defer im.Close()

		// Insert data
		for key, entry := range testData {
			im.Insert(key, entry)
		}

		// Benchmark lookups
		lookupKeys := generateRandomKeys(b.N, 1000)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			im.Get(lookupKeys[i])
		}
	})

	b.Run("BTree", func(b *testing.B) {
		im, err := NewIndexManager(IndexTypeBTree)
		if err != nil {
			b.Fatalf("Failed to create B-tree index manager: %v", err)
		}
		defer im.Close()

		// Insert data
		for key, entry := range testData {
			im.Insert(key, entry)
		}

		// Benchmark lookups
		lookupKeys := generateRandomKeys(b.N, 1000)
		b.ResetTimer()

		for i := 0; i < b.N; i++ {
			im.Get(lookupKeys[i])
		}
	})
}

// BenchmarkMemoryUsage_Comparison compares memory usage between index types
func BenchmarkMemoryUsage_Comparison(b *testing.B) {
	testData := generateBenchmarkData(10000)

	b.Run("Hash_Memory", func(b *testing.B) {
		im, err := NewIndexManager(IndexTypeHash)
		if err != nil {
			b.Fatalf("Failed to create hash index manager: %v", err)
		}
		defer im.Close()

		for key, entry := range testData {
			im.Insert(key, entry)
		}

		b.ReportMetric(float64(im.MemoryUsage()), "bytes")
	})

	b.Run("BTree_Memory", func(b *testing.B) {
		im, err := NewIndexManager(IndexTypeBTree)
		if err != nil {
			b.Fatalf("Failed to create B-tree index manager: %v", err)
		}
		defer im.Close()

		for key, entry := range testData {
			im.Insert(key, entry)
		}

		b.ReportMetric(float64(im.MemoryUsage()), "bytes")
	})
}

// BenchmarkPrefixSearch compares prefix search performance
func BenchmarkPrefixSearch(b *testing.B) {
	// Create test data with various prefixes
	testData := make(map[string]IndexEntry)
	prefixes := []string{"user:", "admin:", "guest:", "system:", "temp:"}

	for _, prefix := range prefixes {
		for i := 0; i < 1000; i++ {
			key := fmt.Sprintf("%s%04d", prefix, i)
			testData[key] = IndexEntry{
				Key:    key,
				Offset: int64(i * 100),
				Size:   50,
			}
		}
	}

	b.Run("Hash_Prefix", func(b *testing.B) {
		im, err := NewIndexManager(IndexTypeHash)
		if err != nil {
			b.Fatalf("Failed to create hash index manager: %v", err)
		}
		defer im.Close()

		for key, entry := range testData {
			im.Insert(key, entry)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			prefix := prefixes[i%len(prefixes)]
			im.Prefix(prefix)
		}
	})

	b.Run("BTree_Prefix", func(b *testing.B) {
		im, err := NewIndexManager(IndexTypeBTree)
		if err != nil {
			b.Fatalf("Failed to create B-tree index manager: %v", err)
		}
		defer im.Close()

		for key, entry := range testData {
			im.Insert(key, entry)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			prefix := prefixes[i%len(prefixes)]
			im.Prefix(prefix)
		}
	})
}
