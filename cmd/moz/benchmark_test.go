package main

import (
	"fmt"
	"testing"

	"github.com/nyasuto/moz/internal/kvstore"
)

// setupBenchmarkStore creates a KVStore for benchmarking
func setupBenchmarkStore(b *testing.B) *kvstore.KVStore {
	// Use default configuration for simplicity
	store := kvstore.New()
	return store
}

// BenchmarkCMDPut measures PUT operation performance
func BenchmarkCMDPut(b *testing.B) {
	store := setupBenchmarkStore(b)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		err := store.Put(key, value)
		if err != nil {
			b.Fatalf("PUT failed: %v", err)
		}
	}
}

// BenchmarkCMDGet measures GET operation performance
func BenchmarkCMDGet(b *testing.B) {
	store := setupBenchmarkStore(b)
	
	// Pre-populate with test data
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		store.Put(key, value)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i%10000)
		_, err := store.Get(key)
		if err != nil {
			b.Fatalf("GET failed: %v", err)
		}
	}
}

// BenchmarkCMDList measures LIST operation performance
func BenchmarkCMDList(b *testing.B) {
	store := setupBenchmarkStore(b)
	
	// Pre-populate with test data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		store.Put(key, value)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := store.List()
		if err != nil {
			b.Fatalf("LIST failed: %v", err)
		}
	}
}

// BenchmarkCMDDelete measures DELETE operation performance
func BenchmarkCMDDelete(b *testing.B) {
	store := setupBenchmarkStore(b)
	
	// Pre-populate with test data (more than benchmark iterations)
	for i := 0; i < b.N+1000; i++ {
		key := fmt.Sprintf("key%d", i)
		value := fmt.Sprintf("value%d", i)
		store.Put(key, value)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key%d", i)
		err := store.Delete(key)
		if err != nil {
			b.Fatalf("DELETE failed: %v", err)
		}
	}
}

// BenchmarkCMDMixedWorkload measures mixed operation performance
func BenchmarkCMDMixedWorkload(b *testing.B) {
	store := setupBenchmarkStore(b)
	
	// Pre-populate with initial data
	for i := 0; i < 1000; i++ {
		key := fmt.Sprintf("init_key%d", i)
		value := fmt.Sprintf("init_value%d", i)
		store.Put(key, value)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		switch i % 4 {
		case 0: // PUT
			key := fmt.Sprintf("mixed_key%d", i)
			value := fmt.Sprintf("mixed_value%d", i)
			store.Put(key, value)
		case 1: // GET
			key := fmt.Sprintf("init_key%d", i%1000)
			store.Get(key)
		case 2: // LIST (every 4th operation)
			store.List()
		case 3: // DELETE
			key := fmt.Sprintf("mixed_key%d", i-3)
			store.Delete(key)
		}
	}
}

// BenchmarkCMDLargeValue measures performance with larger values
func BenchmarkCMDLargeValue(b *testing.B) {
	store := setupBenchmarkStore(b)
	
	// Create a 1KB value
	largeValue := make([]byte, 1024)
	for i := range largeValue {
		largeValue[i] = byte('A' + (i % 26))
	}
	valueStr := string(largeValue)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("large_key%d", i)
		err := store.Put(key, valueStr)
		if err != nil {
			b.Fatalf("PUT large value failed: %v", err)
		}
	}
}

// BenchmarkCMDWithHashIndex measures performance with hash indexing
func BenchmarkCMDWithHashIndex(b *testing.B) {
	compactionConfig := kvstore.CompactionConfig{
		Enabled:         false,
		MaxFileSize:     1024 * 1024 * 10,
		MaxOperations:   10000,
		CompactionRatio: 0.7,
	}
	
	storageConfig := kvstore.StorageConfig{
		Format:    "text",
		TextFile:  "benchmark_hash.log",
		IndexType: "hash",
		IndexFile: "benchmark_hash.idx",
	}
	
	store := kvstore.NewWithConfig(compactionConfig, storageConfig)
	
	// Pre-populate
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("idx_key%d", i)
		value := fmt.Sprintf("idx_value%d", i)
		store.Put(key, value)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("idx_key%d", i%5000)
		_, err := store.Get(key)
		if err != nil {
			b.Fatalf("GET with hash index failed: %v", err)
		}
	}
}

// BenchmarkCMDWithBTreeIndex measures performance with B-Tree indexing
func BenchmarkCMDWithBTreeIndex(b *testing.B) {
	compactionConfig := kvstore.CompactionConfig{
		Enabled:         false,
		MaxFileSize:     1024 * 1024 * 10,
		MaxOperations:   10000,
		CompactionRatio: 0.7,
	}
	
	storageConfig := kvstore.StorageConfig{
		Format:    "text",
		TextFile:  "benchmark_btree.log",
		IndexType: "btree",
		IndexFile: "benchmark_btree.idx",
	}
	
	store := kvstore.NewWithConfig(compactionConfig, storageConfig)
	
	// Pre-populate
	for i := 0; i < 5000; i++ {
		key := fmt.Sprintf("btree_key%d", i)
		value := fmt.Sprintf("btree_value%d", i)
		store.Put(key, value)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("btree_key%d", i%5000)
		_, err := store.Get(key)
		if err != nil {
			b.Fatalf("GET with B-Tree index failed: %v", err)
		}
	}
}