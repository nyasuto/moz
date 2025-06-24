package kvstore

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

// TestMemoryPoolIntegration tests the memory pool integration
func TestMemoryPoolIntegration(t *testing.T) {
	store := New()

	// Test that memory optimizer is enabled
	stats := store.GetMemoryStats()
	if !stats.OptimizationEnabled {
		t.Fatal("Memory optimization should be enabled")
	}

	// Perform operations to exercise memory pools
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("test-key-%d", i)
		value := fmt.Sprintf("test-value-%d", i)
		if err := store.Put(key, value); err != nil {
			t.Fatalf("Failed to put key %s: %v", key, err)
		}
	}

	// Check that pool statistics are being tracked
	finalStats := store.GetMemoryStats()
	if finalStats.PoolStats.LogEntryGets == 0 {
		t.Error("Expected LogEntry pool usage but got 0 gets")
	}
	if finalStats.PoolStats.BufferGets == 0 {
		t.Error("Expected Buffer pool usage but got 0 gets")
	}
	if finalStats.PoolStats.TotalAllocations == 0 {
		t.Error("Expected total allocations but got 0")
	}
}

// TestMemoryLeakDetection tests for memory leaks during sustained operations
func TestMemoryLeakDetection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory leak test in short mode")
	}

	store := New()

	// Baseline memory usage
	runtime.GC()
	var baseMemStats runtime.MemStats
	runtime.ReadMemStats(&baseMemStats)
	baseHeapInuse := baseMemStats.HeapInuse

	t.Logf("Baseline heap in use: %d bytes", baseHeapInuse)

	// Perform sustained operations
	const numCycles = 100
	const opsPerCycle = 50

	for cycle := 0; cycle < numCycles; cycle++ {
		// Write operations
		for i := 0; i < opsPerCycle; i++ {
			key := fmt.Sprintf("cycle-%d-key-%d", cycle, i)
			value := fmt.Sprintf("cycle-%d-value-%d", cycle, i)
			if err := store.Put(key, value); err != nil {
				t.Fatalf("Failed to put key %s: %v", key, err)
			}
		}

		// Read operations
		for i := 0; i < opsPerCycle; i++ {
			key := fmt.Sprintf("cycle-%d-key-%d", cycle, i)
			_, err := store.Get(key)
			if err != nil {
				t.Fatalf("Failed to get key %s: %v", key, err)
			}
		}

		// Delete some entries to exercise deletion
		for i := 0; i < opsPerCycle/2; i++ {
			key := fmt.Sprintf("cycle-%d-key-%d", cycle, i)
			if err := store.Delete(key); err != nil {
				t.Fatalf("Failed to delete key %s: %v", key, err)
			}
		}

		// Periodic memory check
		if cycle%10 == 0 {
			runtime.GC()
			var memStats runtime.MemStats
			runtime.ReadMemStats(&memStats)
			currentHeapInuse := memStats.HeapInuse
			growth := float64(currentHeapInuse-baseHeapInuse) / float64(baseHeapInuse)

			t.Logf("Cycle %d: heap in use: %d bytes, growth: %.2f%%",
				cycle, currentHeapInuse, growth*100)

			// Check for excessive memory growth (allow some growth but not too much)
			if growth > 5.0 { // 500% growth threshold
				t.Errorf("Excessive memory growth detected: %.2f%% after %d cycles", growth*100, cycle)
			}
		}
	}

	// Final memory check
	runtime.GC()
	var finalMemStats runtime.MemStats
	runtime.ReadMemStats(&finalMemStats)
	finalHeapInuse := finalMemStats.HeapInuse
	finalGrowth := float64(finalHeapInuse-baseHeapInuse) / float64(baseHeapInuse)

	t.Logf("Final heap in use: %d bytes, total growth: %.2f%%", finalHeapInuse, finalGrowth*100)

	// Verify memory pools are working efficiently
	memStats := store.GetMemoryStats()
	if memStats.Efficiency.PoolHitRate < 0.5 {
		t.Errorf("Low pool hit rate: %.2f%%, expected > 50%%", memStats.Efficiency.PoolHitRate*100)
	}

	t.Logf("Pool efficiency: %.2f%%, hit rate: %.2f%%",
		memStats.Efficiency.OverallEfficiency*100,
		memStats.Efficiency.PoolHitRate*100)
}

// TestLongRunningOperations tests sustained operations over time
func TestLongRunningOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long-running test in short mode")
	}

	store := New()

	// Test duration and operation parameters
	testDuration := 10 * time.Second
	operationInterval := 10 * time.Millisecond
	startTime := time.Now()

	operationCount := 0
	errorCount := 0

	t.Logf("Starting long-running test for %v", testDuration)

	for time.Since(startTime) < testDuration {
		key := fmt.Sprintf("long-test-key-%d", operationCount)
		value := fmt.Sprintf("long-test-value-%d", operationCount)

		// Put operation
		if err := store.Put(key, value); err != nil {
			errorCount++
			t.Logf("Put error for key %s: %v", key, err)
		}

		// Get operation
		if _, err := store.Get(key); err != nil {
			errorCount++
			t.Logf("Get error for key %s: %v", key, err)
		}

		operationCount++

		// Periodic status report
		if operationCount%1000 == 0 {
			elapsed := time.Since(startTime)
			memStats := store.GetMemoryStats()
			t.Logf("Operations: %d, Elapsed: %v, Errors: %d, Pool Hit Rate: %.2f%%",
				operationCount, elapsed, errorCount, memStats.Efficiency.PoolHitRate*100)

			// Force GC periodically to test stability
			store.OptimizeMemory()
		}

		time.Sleep(operationInterval)
	}

	elapsed := time.Since(startTime)
	opsPerSecond := float64(operationCount) / elapsed.Seconds()

	t.Logf("Completed long-running test:")
	t.Logf("  Duration: %v", elapsed)
	t.Logf("  Operations: %d", operationCount)
	t.Logf("  Errors: %d", errorCount)
	t.Logf("  Ops/sec: %.2f", opsPerSecond)

	// Verify error rate is acceptable
	errorRate := float64(errorCount) / float64(operationCount)
	if errorRate > 0.01 { // 1% error rate threshold
		t.Errorf("High error rate: %.2f%%, expected < 1%%", errorRate*100)
	}

	// Final memory optimization and stats
	store.OptimizeMemory()
	finalStats := store.GetMemoryStats()
	t.Logf("Final pool efficiency: %.2f%%, hit rate: %.2f%%",
		finalStats.Efficiency.OverallEfficiency*100,
		finalStats.Efficiency.PoolHitRate*100)
}

// TestDetailedMemoryStats tests the detailed memory statistics functionality
func TestDetailedMemoryStats(t *testing.T) {
	store := New()

	// Perform some operations
	for i := 0; i < 50; i++ {
		key := fmt.Sprintf("stats-test-key-%d", i)
		value := fmt.Sprintf("stats-test-value-%d", i)
		if err := store.Put(key, value); err != nil {
			t.Fatalf("Failed to put key %s: %v", key, err)
		}
	}

	// Test detailed stats
	detailedStats := store.GetDetailedMemoryStats()

	// Verify required sections exist
	requiredSections := []string{
		"optimization_enabled",
		"pool_stats",
		"efficiency",
		"memory_usage",
		"gc_stats",
		"kvstore_stats",
		"compaction_stats",
		"index_stats",
	}

	for _, section := range requiredSections {
		if _, exists := detailedStats[section]; !exists {
			t.Errorf("Missing required section in detailed stats: %s", section)
		}
	}

	// Verify optimization is enabled
	if enabled, ok := detailedStats["optimization_enabled"].(bool); !ok || !enabled {
		t.Error("Memory optimization should be enabled")
	}

	// Verify pool stats have expected structure
	if poolStats, ok := detailedStats["pool_stats"].(map[string]interface{}); ok {
		expectedKeys := []string{
			"log_entry_gets",
			"log_entry_puts",
			"buffer_gets",
			"buffer_puts",
			"total_allocations",
			"total_deallocations",
		}
		for _, key := range expectedKeys {
			if _, exists := poolStats[key]; !exists {
				t.Errorf("Missing pool stats key: %s", key)
			}
		}
	} else {
		t.Error("Pool stats should be a map[string]interface{}")
	}

	t.Logf("Detailed memory stats test completed successfully")
}

// TestGCOptimization tests garbage collection optimization features
func TestGCOptimization(t *testing.T) {
	store := New()

	// Get initial GC stats
	initialStats := store.GetMemoryStats()
	var initialMemStats runtime.MemStats
	runtime.ReadMemStats(&initialMemStats)

	// Perform memory-intensive operations
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("gc-test-key-%d", i)
		value := fmt.Sprintf("gc-test-value-%d-%s", i, string(make([]byte, 1024))) // 1KB values
		if err := store.Put(key, value); err != nil {
			t.Fatalf("Failed to put key %s: %v", key, err)
		}
	}

	// Force GC and optimization
	before, after := store.ForceGC()
	if before == nil || after == nil {
		t.Error("ForceGC should return before and after stats")
	}

	store.OptimizeMemory()

	// Get final stats
	finalStats := store.GetMemoryStats()
	var finalMemStats runtime.MemStats
	runtime.ReadMemStats(&finalMemStats)

	// Verify GC happened
	if finalMemStats.NumGC <= initialMemStats.NumGC {
		t.Error("Expected at least one GC cycle to have occurred")
	}

	t.Logf("GC optimization test completed:")
	t.Logf("  Initial GC count: %d", initialMemStats.NumGC)
	t.Logf("  Final GC count: %d", finalMemStats.NumGC)
	t.Logf("  Initial pool efficiency: %.2f%%", initialStats.Efficiency.OverallEfficiency*100)
	t.Logf("  Final pool efficiency: %.2f%%", finalStats.Efficiency.OverallEfficiency*100)
}
