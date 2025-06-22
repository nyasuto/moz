package kvstore

import (
	"fmt"
	"os"
	"testing"
)

func BenchmarkMemoryMapPut(b *testing.B) {
	tempDir := b.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)

	kv := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kv.Put(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
	}
}

func BenchmarkMemoryMapGet(b *testing.B) {
	tempDir := b.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)

	kv := New()

	// Pre-populate with data
	for i := 0; i < 1000; i++ {
		kv.Put(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kv.Get(fmt.Sprintf("key_%d", i%1000))
	}
}

func BenchmarkMemoryMapList(b *testing.B) {
	tempDir := b.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)

	kv := New()

	// Pre-populate with data
	for i := 0; i < 1000; i++ {
		kv.Put(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kv.List()
	}
}

func BenchmarkMemoryMapMixed(b *testing.B) {
	tempDir := b.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)

	kv := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Mixed workload: 60% reads, 30% writes, 10% deletes
		switch i % 10 {
		case 0, 1, 2, 3, 4, 5: // 60% reads
			kv.Get(fmt.Sprintf("key_%d", i%100))
		case 6, 7, 8: // 30% writes
			kv.Put(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
		case 9: // 10% deletes
			kv.Delete(fmt.Sprintf("key_%d", i%50))
		}
	}
}

func BenchmarkMemoryMapConcurrentReads(b *testing.B) {
	tempDir := b.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)

	kv := New()

	// Pre-populate with data
	for i := 0; i < 1000; i++ {
		kv.Put(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			kv.Get(fmt.Sprintf("key_%d", i%1000))
			i++
		}
	})
}

func BenchmarkMemoryMapStats(b *testing.B) {
	tempDir := b.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)

	kv := New()

	// Pre-populate with data
	for i := 0; i < 1000; i++ {
		kv.Put(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kv.GetStats()
	}
}