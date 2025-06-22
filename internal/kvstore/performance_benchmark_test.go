package kvstore

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

// BenchmarkResult holds performance measurement data
type BenchmarkResult struct {
	Name              string  `json:"name"`
	Implementation    string  `json:"implementation"`
	Operations        int64   `json:"operations"`
	NsPerOperation    int64   `json:"ns_per_operation"`
	AllocsPerOp       int64   `json:"allocs_per_op"`
	BytesPerOp        int64   `json:"bytes_per_op"`
	Duration          string  `json:"duration"`
	MemoryUsageMB     float64 `json:"memory_usage_mb"`
	Timestamp         string  `json:"timestamp"`
	DataSize          int     `json:"data_size"`
	ConcurrentWorkers int     `json:"concurrent_workers"`
}

// saveBenchmarkResult saves benchmark results to JSON file
func saveBenchmarkResult(result BenchmarkResult) error {
	// Ensure benchmark_results directory exists
	if err := os.MkdirAll("benchmark_results", 0755); err != nil {
		return err
	}

	filename := fmt.Sprintf("benchmark_results/go_performance_%s.json", 
		time.Now().Format("20060102_150405"))
	
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// getMemoryUsage returns current memory usage in MB
func getMemoryUsage() float64 {
	var m runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&m)
	return float64(m.Alloc) / 1024 / 1024
}

// BenchmarkGoPut tests Go PUT operation performance
func BenchmarkGoPut(b *testing.B) {
	tempDir := b.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)
	
	kv := New()
	
	var memBefore = getMemoryUsage()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := kv.Put(fmt.Sprintf("benchmark_key_%d", i), fmt.Sprintf("benchmark_value_%d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
	
	var memAfter = getMemoryUsage()
	
	// Save benchmark results
	result := BenchmarkResult{
		Name:           "Go PUT Operations",
		Implementation: "go",
		Operations:     int64(b.N),
		NsPerOperation: b.Elapsed().Nanoseconds() / int64(b.N),
		Duration:       b.Elapsed().String(),
		MemoryUsageMB:  memAfter - memBefore,
		Timestamp:      time.Now().Format(time.RFC3339),
		DataSize:       b.N,
	}
	
	if err := saveBenchmarkResult(result); err != nil {
		b.Logf("Failed to save benchmark result: %v", err)
	}
}

// BenchmarkGoGet tests Go GET operation performance
func BenchmarkGoGet(b *testing.B) {
	tempDir := b.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)
	
	kv := New()
	
	// Pre-populate with data
	dataSize := 10000
	for i := 0; i < dataSize; i++ {
		kv.Put(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
	}
	
	var memBefore = getMemoryUsage()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := kv.Get(fmt.Sprintf("key_%d", i%dataSize))
		if err != nil {
			b.Fatal(err)
		}
	}
	
	var memAfter = getMemoryUsage()
	
	// Save benchmark results
	result := BenchmarkResult{
		Name:           "Go GET Operations",
		Implementation: "go",
		Operations:     int64(b.N),
		NsPerOperation: b.Elapsed().Nanoseconds() / int64(b.N),
		Duration:       b.Elapsed().String(),
		MemoryUsageMB:  memAfter - memBefore,
		Timestamp:      time.Now().Format(time.RFC3339),
		DataSize:       dataSize,
	}
	
	if err := saveBenchmarkResult(result); err != nil {
		b.Logf("Failed to save benchmark result: %v", err)
	}
}

// BenchmarkGoList tests Go LIST operation performance
func BenchmarkGoList(b *testing.B) {
	tempDir := b.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)
	
	kv := New()
	
	// Pre-populate with data
	dataSize := 1000
	for i := 0; i < dataSize; i++ {
		kv.Put(fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
	}
	
	var memBefore = getMemoryUsage()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := kv.List()
		if err != nil {
			b.Fatal(err)
		}
	}
	
	var memAfter = getMemoryUsage()
	
	// Save benchmark results
	result := BenchmarkResult{
		Name:           "Go LIST Operations",
		Implementation: "go",
		Operations:     int64(b.N),
		NsPerOperation: b.Elapsed().Nanoseconds() / int64(b.N),
		Duration:       b.Elapsed().String(),
		MemoryUsageMB:  memAfter - memBefore,
		Timestamp:      time.Now().Format(time.RFC3339),
		DataSize:       dataSize,
	}
	
	if err := saveBenchmarkResult(result); err != nil {
		b.Logf("Failed to save benchmark result: %v", err)
	}
}

// BenchmarkGoDelete tests Go DELETE operation performance
func BenchmarkGoDelete(b *testing.B) {
	tempDir := b.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)
	
	kv := New()
	
	// Pre-populate with data
	dataSize := b.N * 2 // Ensure we have enough data to delete
	for i := 0; i < dataSize; i++ {
		kv.Put(fmt.Sprintf("delete_key_%d", i), fmt.Sprintf("delete_value_%d", i))
	}
	
	var memBefore = getMemoryUsage()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := kv.Delete(fmt.Sprintf("delete_key_%d", i))
		if err != nil {
			b.Fatal(err)
		}
	}
	
	var memAfter = getMemoryUsage()
	
	// Save benchmark results
	result := BenchmarkResult{
		Name:           "Go DELETE Operations",
		Implementation: "go",
		Operations:     int64(b.N),
		NsPerOperation: b.Elapsed().Nanoseconds() / int64(b.N),
		Duration:       b.Elapsed().String(),
		MemoryUsageMB:  memAfter - memBefore,
		Timestamp:      time.Now().Format(time.RFC3339),
		DataSize:       dataSize,
	}
	
	if err := saveBenchmarkResult(result); err != nil {
		b.Logf("Failed to save benchmark result: %v", err)
	}
}

// BenchmarkGoCompact tests Go COMPACT operation performance
func BenchmarkGoCompact(b *testing.B) {
	tempDir := b.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)
	
	kv := New()
	
	// Pre-populate and fragment data
	dataSize := 1000
	for i := 0; i < dataSize; i++ {
		kv.Put(fmt.Sprintf("compact_key_%d", i), fmt.Sprintf("compact_value_%d", i))
		if i%3 == 0 { // Delete every 3rd entry to create fragmentation
			kv.Delete(fmt.Sprintf("compact_key_%d", i))
		}
	}
	
	var memBefore = getMemoryUsage()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := kv.Compact()
		if err != nil {
			b.Fatal(err)
		}
		
		// Re-fragment for next iteration if needed
		if i < b.N-1 {
			for j := 0; j < 100; j++ {
				kv.Put(fmt.Sprintf("temp_%d_%d", i, j), fmt.Sprintf("temp_value_%d_%d", i, j))
				if j%2 == 0 {
					kv.Delete(fmt.Sprintf("temp_%d_%d", i, j))
				}
			}
		}
	}
	
	var memAfter = getMemoryUsage()
	
	// Save benchmark results
	result := BenchmarkResult{
		Name:           "Go COMPACT Operations",
		Implementation: "go",
		Operations:     int64(b.N),
		NsPerOperation: b.Elapsed().Nanoseconds() / int64(b.N),
		Duration:       b.Elapsed().String(),
		MemoryUsageMB:  memAfter - memBefore,
		Timestamp:      time.Now().Format(time.RFC3339),
		DataSize:       dataSize,
	}
	
	if err := saveBenchmarkResult(result); err != nil {
		b.Logf("Failed to save benchmark result: %v", err)
	}
}

// BenchmarkGoLargeData tests Go performance with large datasets
func BenchmarkGoLargeData(b *testing.B) {
	tempDir := b.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)
	
	kv := New()
	
	// Test different data sizes
	dataSizes := []int{1000, 10000, 100000}
	
	for _, size := range dataSizes {
		b.Run(fmt.Sprintf("Size_%d", size), func(b *testing.B) {
			// Pre-populate with data
			for i := 0; i < size; i++ {
				kv.Put(fmt.Sprintf("large_key_%d", i), fmt.Sprintf("large_value_%d", i))
			}
			
			var memBefore = getMemoryUsage()
			
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// Mixed operations on large dataset
				switch i % 4 {
				case 0:
					kv.Get(fmt.Sprintf("large_key_%d", i%size))
				case 1:
					kv.Put(fmt.Sprintf("new_key_%d", i), fmt.Sprintf("new_value_%d", i))
				case 2:
					kv.List()
				case 3:
					if i%100 == 0 { // Compact occasionally
						kv.Compact()
					}
				}
			}
			
			var memAfter = getMemoryUsage()
			
			// Save benchmark results
			result := BenchmarkResult{
				Name:           fmt.Sprintf("Go Large Data Operations (Size: %d)", size),
				Implementation: "go",
				Operations:     int64(b.N),
				NsPerOperation: b.Elapsed().Nanoseconds() / int64(b.N),
				Duration:       b.Elapsed().String(),
				MemoryUsageMB:  memAfter - memBefore,
				Timestamp:      time.Now().Format(time.RFC3339),
				DataSize:       size,
			}
			
			if err := saveBenchmarkResult(result); err != nil {
				b.Logf("Failed to save benchmark result: %v", err)
			}
		})
	}
}

// BenchmarkGoConcurrentOperations tests Go concurrent performance
func BenchmarkGoConcurrentOperations(b *testing.B) {
	tempDir := b.TempDir()
	os.Setenv("MOZ_DATA_DIR", tempDir)
	
	kv := New()
	
	// Pre-populate with data
	for i := 0; i < 1000; i++ {
		kv.Put(fmt.Sprintf("concurrent_key_%d", i), fmt.Sprintf("concurrent_value_%d", i))
	}
	
	var memBefore = getMemoryUsage()
	
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// Mostly reads with some writes (90% reads, 10% writes)
			if i%10 == 0 {
				kv.Put(fmt.Sprintf("concurrent_new_%d", i), fmt.Sprintf("concurrent_new_value_%d", i))
			} else {
				kv.Get(fmt.Sprintf("concurrent_key_%d", i%1000))
			}
			i++
		}
	})
	
	var memAfter = getMemoryUsage()
	
	// Save benchmark results
	result := BenchmarkResult{
		Name:              "Go Concurrent Operations",
		Implementation:    "go",
		Operations:        int64(b.N),
		NsPerOperation:    b.Elapsed().Nanoseconds() / int64(b.N),
		Duration:          b.Elapsed().String(),
		MemoryUsageMB:     memAfter - memBefore,
		Timestamp:         time.Now().Format(time.RFC3339),
		DataSize:          1000,
		ConcurrentWorkers: runtime.GOMAXPROCS(0),
	}
	
	if err := saveBenchmarkResult(result); err != nil {
		b.Logf("Failed to save benchmark result: %v", err)
	}
}

// Helper function to run shell script benchmarks for comparison
func runShellBenchmark(scriptPath string, operations int) (*BenchmarkResult, error) {
	start := time.Now()
	
	cmd := exec.Command("bash", scriptPath, fmt.Sprintf("%d", operations))
	cmd.Dir = "../.." // Run from project root
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("shell benchmark failed: %v, output: %s", err, output)
	}
	
	duration := time.Since(start)
	
	result := &BenchmarkResult{
		Name:           fmt.Sprintf("Shell %s", scriptPath),
		Implementation: "shell",
		Operations:     int64(operations),
		NsPerOperation: duration.Nanoseconds() / int64(operations),
		Duration:       duration.String(),
		Timestamp:      time.Now().Format(time.RFC3339),
		DataSize:       operations,
	}
	
	return result, nil
}