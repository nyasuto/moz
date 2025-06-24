package kvstore

import (
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// PartitionConfig holds configuration for partitioned storage
type PartitionConfig struct {
	NumPartitions int           // Number of partitions (1-16)
	DataDir       string        // Base directory for partition files
	BatchSize     int           // Batch size for group writes
	FlushInterval time.Duration // Interval for auto-flush
}

// DefaultPartitionConfig returns default partition configuration
func DefaultPartitionConfig() PartitionConfig {
	// Check for environment variable first
	dataDir := os.Getenv("MOZ_PARTITION_DIR")
	if dataDir == "" {
		dataDir = "data/partitions" // Use dedicated subdirectory instead of current directory
	}
	
	return PartitionConfig{
		NumPartitions: 4,
		DataDir:       dataDir,
		BatchSize:     100,
		FlushInterval: 100 * time.Millisecond,
	}
}

// Partition represents a single partition
type Partition struct {
	id          int
	store       *KVStore
	batchBuffer []*BatchEntry
	bufferMutex sync.Mutex
	lastFlush   time.Time
}

// BatchEntry represents an entry in the batch buffer
type BatchEntry struct {
	Key       string
	Value     string
	Operation string // "PUT" or "DELETE"
	Timestamp time.Time
}

// PartitionedKVStore implements high-performance partitioned key-value storage
type PartitionedKVStore struct {
	config     PartitionConfig
	partitions []*Partition
	hashFunc   func(string) uint32

	// Memory pools for reduced GC pressure
	entryPool  sync.Pool
	bufferPool sync.Pool

	// Background flush management
	flushTicker *time.Ticker
	flushStop   chan struct{}
	flushWg     sync.WaitGroup
}

// NewPartitionedKVStore creates a new partitioned KV store
func NewPartitionedKVStore(config PartitionConfig) (*PartitionedKVStore, error) {
	if config.NumPartitions < 1 || config.NumPartitions > 16 {
		return nil, fmt.Errorf("invalid partition count: %d (must be 1-16)", config.NumPartitions)
	}

	store := &PartitionedKVStore{
		config:     config,
		partitions: make([]*Partition, config.NumPartitions),
		hashFunc:   hashString,
		flushStop:  make(chan struct{}),
		entryPool: sync.Pool{
			New: func() interface{} { return &BatchEntry{} },
		},
		bufferPool: sync.Pool{
			New: func() interface{} { return make([]byte, 0, 1024) },
		},
	}

	// Initialize partitions
	for i := 0; i < config.NumPartitions; i++ {
		partition, err := store.createPartition(i)
		if err != nil {
			return nil, fmt.Errorf("failed to create partition %d: %w", i, err)
		}
		store.partitions[i] = partition
	}

	// Start background flush goroutine
	store.flushTicker = time.NewTicker(config.FlushInterval)
	store.flushWg.Add(1)
	go store.backgroundFlush()

	return store, nil
}

// createPartition creates a single partition
func (pks *PartitionedKVStore) createPartition(id int) (*Partition, error) {
	// Create partition-specific directory
	partitionDir := filepath.Join(pks.config.DataDir, fmt.Sprintf("partition_%d", id))
	if err := os.MkdirAll(partitionDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create partition directory: %w", err)
	}

	// Create KVStore for this partition
	storageConfig := StorageConfig{
		Format:     "text", // Start with text format for compatibility
		TextFile:   fmt.Sprintf("moz_p%d.log", id),
		BinaryFile: fmt.Sprintf("moz_p%d.bin", id),
		IndexType:  "none", // Disable per-partition indexing for now
		IndexFile:  fmt.Sprintf("moz_p%d.idx", id),
	}

	compactionConfig := CompactionConfig{
		Enabled:         true,
		MaxFileSize:     DefaultMaxFileSize / int64(pks.config.NumPartitions), // Scale down per partition
		MaxOperations:   DefaultMaxOperations / pks.config.NumPartitions,
		CompactionRatio: DefaultCompactionRatio,
	}

	store := NewWithConfig(compactionConfig, storageConfig)
	store.dataDir = partitionDir

	// Update log file path
	store.logFile = filepath.Join(partitionDir, storageConfig.TextFile)

	partition := &Partition{
		id:          id,
		store:       store,
		batchBuffer: make([]*BatchEntry, 0, pks.config.BatchSize),
		lastFlush:   time.Now(),
	}

	return partition, nil
}

// getPartition returns the partition for a given key
func (pks *PartitionedKVStore) getPartition(key string) *Partition {
	hash := pks.hashFunc(key)
	partitionID := int(hash % uint32(pks.config.NumPartitions))
	return pks.partitions[partitionID]
}

// Put stores a key-value pair with high performance
func (pks *PartitionedKVStore) Put(key, value string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}

	partition := pks.getPartition(key)

	// Get entry from pool
	entry, ok := pks.entryPool.Get().(*BatchEntry)
	if !ok {
		entry = &BatchEntry{}
	}
	entry.Key = key
	entry.Value = value
	entry.Operation = "PUT"
	entry.Timestamp = time.Now()

	// Add to partition's batch buffer
	partition.bufferMutex.Lock()
	partition.batchBuffer = append(partition.batchBuffer, entry)
	shouldFlush := len(partition.batchBuffer) >= pks.config.BatchSize
	partition.bufferMutex.Unlock()

	// Immediate flush if batch is full
	if shouldFlush {
		return pks.flushPartition(partition)
	}

	return nil
}

// Get retrieves a value by key
func (pks *PartitionedKVStore) Get(key string) (string, error) {
	partition := pks.getPartition(key)

	// Check batch buffer first (most recent writes)
	partition.bufferMutex.Lock()
	for i := len(partition.batchBuffer) - 1; i >= 0; i-- {
		entry := partition.batchBuffer[i]
		if entry.Key == key {
			if entry.Operation == "DELETE" {
				partition.bufferMutex.Unlock()
				return "", fmt.Errorf("key not found: %s", key)
			}
			value := entry.Value
			partition.bufferMutex.Unlock()
			return value, nil
		}
	}
	partition.bufferMutex.Unlock()

	// Fall back to partition store
	return partition.store.Get(key)
}

// Delete removes a key
func (pks *PartitionedKVStore) Delete(key string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}

	partition := pks.getPartition(key)

	// Get entry from pool
	entry, ok := pks.entryPool.Get().(*BatchEntry)
	if !ok {
		entry = &BatchEntry{}
	}
	entry.Key = key
	entry.Value = ""
	entry.Operation = "DELETE"
	entry.Timestamp = time.Now()

	// Add to partition's batch buffer
	partition.bufferMutex.Lock()
	partition.batchBuffer = append(partition.batchBuffer, entry)
	shouldFlush := len(partition.batchBuffer) >= pks.config.BatchSize
	partition.bufferMutex.Unlock()

	// Immediate flush if batch is full
	if shouldFlush {
		return pks.flushPartition(partition)
	}

	return nil
}

// List returns all keys across all partitions
func (pks *PartitionedKVStore) List() ([]string, error) {
	// Ensure all partitions are flushed
	if err := pks.FlushAll(); err != nil {
		return nil, fmt.Errorf("failed to flush partitions: %w", err)
	}

	var allKeys []string
	keySet := make(map[string]bool)

	// Collect keys from all partitions
	for _, partition := range pks.partitions {
		keys, err := partition.store.List()
		if err != nil {
			return nil, fmt.Errorf("failed to list partition %d: %w", partition.id, err)
		}

		for _, key := range keys {
			if !keySet[key] {
				keySet[key] = true
				allKeys = append(allKeys, key)
			}
		}
	}

	sort.Strings(allKeys)
	return allKeys, nil
}

// flushPartition flushes a single partition's batch buffer
func (pks *PartitionedKVStore) flushPartition(partition *Partition) error {
	partition.bufferMutex.Lock()
	if len(partition.batchBuffer) == 0 {
		partition.bufferMutex.Unlock()
		return nil
	}

	// Copy buffer for processing
	buffer := make([]*BatchEntry, len(partition.batchBuffer))
	copy(buffer, partition.batchBuffer)

	// Clear buffer and update flush time
	partition.batchBuffer = partition.batchBuffer[:0]
	partition.lastFlush = time.Now()
	partition.bufferMutex.Unlock()

	// Process batch entries
	for _, entry := range buffer {
		var err error
		switch entry.Operation {
		case "PUT":
			err = partition.store.Put(entry.Key, entry.Value)
		case "DELETE":
			err = partition.store.Delete(entry.Key)
		}

		if err != nil {
			// Return entry to pool and return error
			pks.entryPool.Put(entry)
			return fmt.Errorf("batch operation failed on partition %d: %w", partition.id, err)
		}

		// Return entry to pool
		pks.entryPool.Put(entry)
	}

	return nil
}

// FlushAll flushes all partition buffers
func (pks *PartitionedKVStore) FlushAll() error {
	for _, partition := range pks.partitions {
		if err := pks.flushPartition(partition); err != nil {
			return err
		}
	}
	return nil
}

// backgroundFlush runs periodic flush operations
func (pks *PartitionedKVStore) backgroundFlush() {
	defer pks.flushWg.Done()

	for {
		select {
		case <-pks.flushTicker.C:
			// Flush partitions that haven't been flushed recently
			now := time.Now()
			for _, partition := range pks.partitions {
				partition.bufferMutex.Lock()
				needsFlush := len(partition.batchBuffer) > 0 &&
					now.Sub(partition.lastFlush) >= pks.config.FlushInterval
				partition.bufferMutex.Unlock()

				if needsFlush {
					if err := pks.flushPartition(partition); err != nil {
						fmt.Printf("Warning: background flush failed for partition %d: %v\n",
							partition.id, err)
					}
				}
			}
		case <-pks.flushStop:
			// Final flush before shutdown
			_ = pks.FlushAll()
			return
		}
	}
}

// Close gracefully shuts down the partitioned store
func (pks *PartitionedKVStore) Close() error {
	// Stop background flush
	if pks.flushTicker != nil {
		pks.flushTicker.Stop()
	}
	close(pks.flushStop)
	pks.flushWg.Wait()

	// Final flush
	return pks.FlushAll()
}

// Compact performs compaction on all partitions
func (pks *PartitionedKVStore) Compact() error {
	if err := pks.FlushAll(); err != nil {
		return fmt.Errorf("failed to flush before compaction: %w", err)
	}

	// Compact all partitions in parallel
	var wg sync.WaitGroup
	errors := make(chan error, len(pks.partitions))

	for _, partition := range pks.partitions {
		wg.Add(1)
		go func(p *Partition) {
			defer wg.Done()
			if err := p.store.Compact(); err != nil {
				errors <- fmt.Errorf("partition %d compaction failed: %w", p.id, err)
			}
		}(partition)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	for err := range errors {
		return err
	}

	return nil
}

// GetStats returns aggregated statistics
func (pks *PartitionedKVStore) GetStats() (map[string]interface{}, error) {
	if err := pks.FlushAll(); err != nil {
		return nil, fmt.Errorf("failed to flush for stats: %w", err)
	}

	stats := make(map[string]interface{})
	stats["num_partitions"] = pks.config.NumPartitions
	stats["batch_size"] = pks.config.BatchSize
	stats["flush_interval_ms"] = pks.config.FlushInterval.Milliseconds()

	// Aggregate partition stats
	totalEntries := 0
	var partitionStats []map[string]interface{}

	for i, partition := range pks.partitions {
		pStats, err := partition.store.GetStats()
		if err != nil {
			return nil, fmt.Errorf("failed to get stats for partition %d: %w", i, err)
		}

		partitionInfo := map[string]interface{}{
			"id":        i,
			"entries":   pStats.MemoryMapSize,
			"is_loaded": pStats.IsLoaded,
		}

		// Add buffer info
		partition.bufferMutex.Lock()
		partitionInfo["buffer_size"] = len(partition.batchBuffer)
		partition.bufferMutex.Unlock()

		partitionStats = append(partitionStats, partitionInfo)
		totalEntries += pStats.MemoryMapSize
	}

	stats["total_entries"] = totalEntries
	stats["partitions"] = partitionStats

	return stats, nil
}

// GetStats returns aggregated statistics with partitioned store information
func (pks *PartitionedKVStore) GetPartitionedStats() (map[string]interface{}, error) {
	if err := pks.FlushAll(); err != nil {
		return nil, fmt.Errorf("failed to flush for stats: %w", err)
	}

	stats := make(map[string]interface{})
	stats["store_type"] = "partitioned"
	stats["num_partitions"] = pks.config.NumPartitions
	stats["batch_size"] = pks.config.BatchSize
	stats["flush_interval_ms"] = pks.config.FlushInterval.Milliseconds()

	// Aggregate partition stats
	totalEntries := 0
	var partitionStats []map[string]interface{}

	for i, partition := range pks.partitions {
		pStats, err := partition.store.GetStats()
		if err != nil {
			return nil, fmt.Errorf("failed to get stats for partition %d: %w", i, err)
		}

		partitionInfo := map[string]interface{}{
			"id":        i,
			"entries":   pStats.MemoryMapSize,
			"is_loaded": pStats.IsLoaded,
		}

		// Add buffer info
		partition.bufferMutex.Lock()
		partitionInfo["buffer_size"] = len(partition.batchBuffer)
		partition.bufferMutex.Unlock()

		partitionStats = append(partitionStats, partitionInfo)
		totalEntries += pStats.MemoryMapSize
	}

	stats["total_entries"] = totalEntries
	stats["partitions"] = partitionStats

	return stats, nil
}

// hashString computes FNV-1a hash for a string
func hashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}
