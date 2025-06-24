package kvstore

import (
	"runtime"
	"runtime/debug"
	"sync"
	"time"

	"github.com/nyasuto/moz/internal/index"
)

// MemoryPoolConfig holds memory pool configuration
type MemoryPoolConfig struct {
	EnableLogEntryPool bool // Enable LogEntry pooling
	EnableBufferPool   bool // Enable byte slice pooling
	EnableIndexPool    bool // Enable IndexEntry pooling
	InitialBufferSize  int  // Initial buffer size
	MaxBufferSize      int  // Maximum buffer size
	GCTargetPercent    int  // Target GC percentage
}

// DefaultMemoryPoolConfig returns default memory pool configuration
func DefaultMemoryPoolConfig() MemoryPoolConfig {
	return MemoryPoolConfig{
		EnableLogEntryPool: true,
		EnableBufferPool:   true,
		EnableIndexPool:    true,
		InitialBufferSize:  1024,
		MaxBufferSize:      64 * 1024, // 64KB
		GCTargetPercent:    100,
	}
}

// MemoryPools holds all memory pools for the KV store
type MemoryPools struct {
	config MemoryPoolConfig

	// Core pools
	logEntryPool   sync.Pool
	bufferPool     sync.Pool
	indexEntryPool sync.Pool

	// Statistics
	stats      MemoryPoolStats
	statsMutex sync.RWMutex
}

// MemoryPoolStats holds statistics about memory pool usage
type MemoryPoolStats struct {
	LogEntryGets       int64
	LogEntryPuts       int64
	BufferGets         int64
	BufferPuts         int64
	IndexEntryGets     int64
	IndexEntryPuts     int64
	TotalAllocations   int64
	TotalDeallocations int64
	LastGCStats        debug.GCStats
	MemoryUsage        runtime.MemStats
	LastUpdate         time.Time
}

// NewMemoryPools creates a new memory pool system
func NewMemoryPools(config MemoryPoolConfig) *MemoryPools {
	mp := &MemoryPools{
		config: config,
	}

	// Initialize LogEntry pool
	if config.EnableLogEntryPool {
		mp.logEntryPool = sync.Pool{
			New: func() interface{} {
				mp.incrementAllocation()
				return &LogEntry{}
			},
		}
	}

	// Initialize buffer pool with size-based allocation
	if config.EnableBufferPool {
		mp.bufferPool = sync.Pool{
			New: func() interface{} {
				mp.incrementAllocation()
				return make([]byte, 0, config.InitialBufferSize)
			},
		}
	}

	// Initialize IndexEntry pool
	if config.EnableIndexPool {
		mp.indexEntryPool = sync.Pool{
			New: func() interface{} {
				mp.incrementAllocation()
				return &index.IndexEntry{}
			},
		}
	}

	// Set GC target if specified
	if config.GCTargetPercent > 0 {
		debug.SetGCPercent(config.GCTargetPercent)
	}

	return mp
}

// GetLogEntry retrieves a LogEntry from the pool
func (mp *MemoryPools) GetLogEntry() *LogEntry {
	if !mp.config.EnableLogEntryPool {
		return &LogEntry{}
	}

	mp.statsMutex.Lock()
	mp.stats.LogEntryGets++
	mp.statsMutex.Unlock()

	entry, ok := mp.logEntryPool.Get().(*LogEntry)
	if !ok {
		return &LogEntry{}
	}

	// Reset the entry
	entry.Key = ""
	entry.Value = ""

	return entry
}

// PutLogEntry returns a LogEntry to the pool
func (mp *MemoryPools) PutLogEntry(entry *LogEntry) {
	if !mp.config.EnableLogEntryPool || entry == nil {
		return
	}

	mp.statsMutex.Lock()
	mp.stats.LogEntryPuts++
	mp.stats.TotalDeallocations++
	mp.statsMutex.Unlock()

	// Clear the entry before returning to pool
	entry.Key = ""
	entry.Value = ""

	mp.logEntryPool.Put(entry)
}

// GetBuffer retrieves a byte slice buffer from the pool
func (mp *MemoryPools) GetBuffer() []byte {
	if !mp.config.EnableBufferPool {
		return make([]byte, 0, mp.config.InitialBufferSize)
	}

	mp.statsMutex.Lock()
	mp.stats.BufferGets++
	mp.statsMutex.Unlock()

	poolItem := mp.bufferPool.Get()
	if bufferPtr, ok := poolItem.(*[]byte); ok {
		buffer := *bufferPtr
		return buffer[:0]
	}
	if buffer, ok := poolItem.([]byte); ok {
		return buffer[:0]
	}
	return make([]byte, 0, mp.config.InitialBufferSize)
}

// PutBuffer returns a byte slice buffer to the pool
func (mp *MemoryPools) PutBuffer(buffer []byte) {
	if !mp.config.EnableBufferPool || buffer == nil {
		return
	}

	mp.statsMutex.Lock()
	mp.stats.BufferPuts++
	mp.stats.TotalDeallocations++
	mp.statsMutex.Unlock()

	// Only return buffers that are not too large
	if cap(buffer) <= mp.config.MaxBufferSize {
		fullBuffer := buffer[:cap(buffer)]
		mp.bufferPool.Put(&fullBuffer) // Use pointer to avoid allocation warning
	}
}

// GetIndexEntry retrieves an IndexEntry from the pool
func (mp *MemoryPools) GetIndexEntry() *index.IndexEntry {
	if !mp.config.EnableIndexPool {
		return &index.IndexEntry{}
	}

	mp.statsMutex.Lock()
	mp.stats.IndexEntryGets++
	mp.statsMutex.Unlock()

	entry, ok := mp.indexEntryPool.Get().(*index.IndexEntry)
	if !ok {
		return &index.IndexEntry{}
	}

	// Reset the entry
	entry.Key = ""
	entry.Offset = 0
	entry.Size = 0
	entry.Timestamp = 0
	entry.Deleted = false

	return entry
}

// PutIndexEntry returns an IndexEntry to the pool
func (mp *MemoryPools) PutIndexEntry(entry *index.IndexEntry) {
	if !mp.config.EnableIndexPool || entry == nil {
		return
	}

	mp.statsMutex.Lock()
	mp.stats.IndexEntryPuts++
	mp.stats.TotalDeallocations++
	mp.statsMutex.Unlock()

	// Clear the entry before returning to pool
	entry.Key = ""
	entry.Offset = 0
	entry.Size = 0
	entry.Timestamp = 0
	entry.Deleted = false

	mp.indexEntryPool.Put(entry)
}

// incrementAllocation increments allocation counter (thread-safe)
func (mp *MemoryPools) incrementAllocation() {
	mp.statsMutex.Lock()
	mp.stats.TotalAllocations++
	mp.statsMutex.Unlock()
}

// GetStats returns current memory pool statistics
func (mp *MemoryPools) GetStats() MemoryPoolStats {
	mp.statsMutex.RLock()
	defer mp.statsMutex.RUnlock()

	// Update runtime stats
	stats := mp.stats
	runtime.ReadMemStats(&stats.MemoryUsage)
	debug.ReadGCStats(&stats.LastGCStats)
	stats.LastUpdate = time.Now()

	return stats
}

// ForceGC forces garbage collection and returns before/after memory stats
func (mp *MemoryPools) ForceGC() (before, after runtime.MemStats) {
	runtime.ReadMemStats(&before)
	runtime.GC()
	runtime.ReadMemStats(&after)
	return before, after
}

// OptimizeGC performs GC optimization based on current memory usage
func (mp *MemoryPools) OptimizeGC() {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	// Adjust GC target based on memory usage
	if ms.HeapInuse > ms.HeapSys/2 {
		// High memory usage - more aggressive GC
		debug.SetGCPercent(50)
	} else {
		// Normal memory usage - standard GC
		debug.SetGCPercent(mp.config.GCTargetPercent)
	}
}

// GetEfficiency calculates pool efficiency metrics
func (mp *MemoryPools) GetEfficiency() PoolEfficiency {
	stats := mp.GetStats()

	var logEntryEfficiency, bufferEfficiency, indexEntryEfficiency float64

	if stats.LogEntryGets > 0 {
		logEntryEfficiency = float64(stats.LogEntryPuts) / float64(stats.LogEntryGets)
	}

	if stats.BufferGets > 0 {
		bufferEfficiency = float64(stats.BufferPuts) / float64(stats.BufferGets)
	}

	if stats.IndexEntryGets > 0 {
		indexEntryEfficiency = float64(stats.IndexEntryPuts) / float64(stats.IndexEntryGets)
	}

	return PoolEfficiency{
		LogEntryEfficiency:   logEntryEfficiency,
		BufferEfficiency:     bufferEfficiency,
		IndexEntryEfficiency: indexEntryEfficiency,
		OverallEfficiency:    (logEntryEfficiency + bufferEfficiency + indexEntryEfficiency) / 3,
		PoolHitRate:          float64(stats.TotalDeallocations) / float64(stats.TotalAllocations),
	}
}

// PoolEfficiency holds efficiency metrics for memory pools
type PoolEfficiency struct {
	LogEntryEfficiency   float64 // Return rate for LogEntry objects
	BufferEfficiency     float64 // Return rate for buffer objects
	IndexEntryEfficiency float64 // Return rate for IndexEntry objects
	OverallEfficiency    float64 // Overall pool efficiency
	PoolHitRate          float64 // Pool hit rate (reuse rate)
}

// PreallocatedSlices provides pre-allocated slices for common operations
type PreallocatedSlices struct {
	LogEntries   []LogEntry
	IndexEntries []index.IndexEntry
	StringSlice  []string
	ByteSlice    []byte
}

// NewPreallocatedSlices creates pre-allocated slices with estimated sizes
func NewPreallocatedSlices(estimatedSize int) *PreallocatedSlices {
	return &PreallocatedSlices{
		LogEntries:   make([]LogEntry, 0, estimatedSize),
		IndexEntries: make([]index.IndexEntry, 0, estimatedSize),
		StringSlice:  make([]string, 0, estimatedSize),
		ByteSlice:    make([]byte, 0, estimatedSize*64), // Assume 64 bytes per entry
	}
}

// Reset resets all slices to zero length while preserving capacity
func (ps *PreallocatedSlices) Reset() {
	ps.LogEntries = ps.LogEntries[:0]
	ps.IndexEntries = ps.IndexEntries[:0]
	ps.StringSlice = ps.StringSlice[:0]
	ps.ByteSlice = ps.ByteSlice[:0]
}

// MemoryOptimizer provides memory optimization utilities
type MemoryOptimizer struct {
	pools              *MemoryPools
	preallocatedSlices *PreallocatedSlices
	gcTicker           *time.Ticker
	stopGC             chan struct{}
	config             MemoryPoolConfig
}

// NewMemoryOptimizer creates a new memory optimizer
func NewMemoryOptimizer(config MemoryPoolConfig) *MemoryOptimizer {
	mo := &MemoryOptimizer{
		pools:              NewMemoryPools(config),
		preallocatedSlices: NewPreallocatedSlices(1000), // Default estimated size
		stopGC:             make(chan struct{}),
		config:             config,
	}

	// Start periodic GC optimization (every 30 seconds)
	mo.gcTicker = time.NewTicker(30 * time.Second)
	go mo.periodicGCOptimization()

	return mo
}

// periodicGCOptimization runs periodic GC optimization
func (mo *MemoryOptimizer) periodicGCOptimization() {
	for {
		select {
		case <-mo.gcTicker.C:
			mo.pools.OptimizeGC()
		case <-mo.stopGC:
			return
		}
	}
}

// GetPools returns the memory pools
func (mo *MemoryOptimizer) GetPools() *MemoryPools {
	return mo.pools
}

// GetPreallocatedSlices returns pre-allocated slices
func (mo *MemoryOptimizer) GetPreallocatedSlices() *PreallocatedSlices {
	return mo.preallocatedSlices
}

// Close stops the memory optimizer
func (mo *MemoryOptimizer) Close() {
	if mo.gcTicker != nil {
		mo.gcTicker.Stop()
	}
	close(mo.stopGC)
}
