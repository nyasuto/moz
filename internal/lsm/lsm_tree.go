package lsm

import (
	"fmt"
	"sync"
	"time"

	"github.com/nyasuto/moz/internal/kvstore"
)

// LSMTree implements a Log-Structured Merge-Tree for high-performance storage
type LSMTree struct {
	mu sync.RWMutex

	// L0: Active in-memory table
	memTable *kvstore.MemTable

	// L0: Immutable in-memory tables (ready for flush)
	immutableTables []*kvstore.MemTable

	// L1-LN: Hierarchical SSTables on disk
	levels []Level

	// Bloom filters for fast negative lookups
	bloomFilters map[string]*BloomFilter

	// Configuration and state
	config        LSMConfig
	dataDir       string
	nextSSTableID uint64

	// Background processes
	compactionCh chan struct{}
	stopCh       chan struct{}
	wg           sync.WaitGroup

	// Statistics
	stats LSMStats
}

// Level represents a single level in the LSM-Tree hierarchy
type Level struct {
	Level    int
	SSTables []*SSTable
	MaxSize  int64
	Config   LevelConfig
}

// LevelConfig holds configuration for a specific level
type LevelConfig struct {
	MaxSSTables    int
	MaxSize        int64
	CompactionSize int64
	TargetFileSize int64
}

// LSMConfig holds configuration for the LSM-Tree
type LSMConfig struct {
	DataDir         string
	MemTableConfig  kvstore.MemTableConfig
	NumLevels       int
	L0MaxSSTables   int
	LevelSizeRatio  int     // Size ratio between levels (default: 10)
	BloomFilterFPR  float64 // False positive rate (default: 0.01)
	CompactionStyle CompactionStyle
}

// CompactionStyle defines the compaction strategy
type CompactionStyle int

const (
	SizeTieredCompaction CompactionStyle = iota
	LeveledCompaction
	HybridCompaction
)

// LSMStats holds statistics about LSM-Tree operations
type LSMStats struct {
	MemTableFlushes    uint64
	CompactionCount    uint64
	BytesRead          uint64
	BytesWritten       uint64
	BloomFilterHits    uint64
	BloomFilterMisses  uint64
	AvgReadLatency     time.Duration
	AvgWriteLatency    time.Duration
	LastCompactionTime time.Time
	TotalLevels        int
	ActiveSSTables     int
}

// DefaultLSMConfig returns a default LSM-Tree configuration
func DefaultLSMConfig() LSMConfig {
	return LSMConfig{
		DataDir:         "data/lsm",
		MemTableConfig:  kvstore.DefaultMemTableConfig(),
		NumLevels:       7, // L0-L6, similar to RocksDB
		L0MaxSSTables:   4, // Trigger compaction after 4 SSTables in L0
		LevelSizeRatio:  10,
		BloomFilterFPR:  0.01, // 1% false positive rate
		CompactionStyle: LeveledCompaction,
	}
}

// NewLSMTree creates a new LSM-Tree instance
func NewLSMTree(config LSMConfig) (*LSMTree, error) {
	if config.DataDir == "" {
		config.DataDir = "data/lsm"
	}

	lsm := &LSMTree{
		config:       config,
		dataDir:      config.DataDir,
		levels:       make([]Level, config.NumLevels),
		bloomFilters: make(map[string]*BloomFilter),
		compactionCh: make(chan struct{}, 1),
		stopCh:       make(chan struct{}),
	}

	// Initialize MemTable
	memTable := kvstore.NewMemTable(config.MemTableConfig)
	lsm.memTable = memTable

	// Initialize levels with appropriate configurations
	for i := range lsm.levels {
		lsm.levels[i] = Level{
			Level:    i,
			SSTables: make([]*SSTable, 0),
			Config:   lsm.calculateLevelConfig(i),
		}
	}

	// Start background compaction process
	lsm.wg.Add(1)
	go lsm.compactionWorker()

	return lsm, nil
}

// calculateLevelConfig calculates configuration for a specific level
func (lsm *LSMTree) calculateLevelConfig(level int) LevelConfig {
	if level == 0 {
		// L0 has special handling (size-tiered)
		return LevelConfig{
			MaxSSTables:    lsm.config.L0MaxSSTables,
			MaxSize:        int64(lsm.config.L0MaxSSTables) * lsm.config.MemTableConfig.MaxSize,
			CompactionSize: lsm.config.MemTableConfig.MaxSize,
			TargetFileSize: lsm.config.MemTableConfig.MaxSize,
		}
	}

	// L1+ levels follow exponential growth
	baseSize := lsm.config.MemTableConfig.MaxSize * int64(lsm.config.LevelSizeRatio)
	levelMultiplier := int64(1)
	for i := 1; i < level; i++ {
		levelMultiplier *= int64(lsm.config.LevelSizeRatio)
	}

	maxSize := baseSize * levelMultiplier
	targetFileSize := maxSize / 10 // Target ~10 files per level

	return LevelConfig{
		MaxSSTables:    50, // Reasonable upper bound
		MaxSize:        maxSize,
		CompactionSize: maxSize / 2,
		TargetFileSize: targetFileSize,
	}
}

// Put writes a key-value pair to the LSM-Tree
func (lsm *LSMTree) Put(key, value string) error {
	start := time.Now()
	defer func() {
		lsm.stats.AvgWriteLatency = time.Since(start)
	}()

	lsm.mu.Lock()
	defer lsm.mu.Unlock()

	// Check if MemTable needs to be flushed
	if lsm.memTable.ShouldFlush(lsm.config.MemTableConfig) {
		if err := lsm.flushMemTable(); err != nil {
			return fmt.Errorf("failed to flush MemTable: %w", err)
		}
	}

	// Write to active MemTable
	lsm.memTable.Put(key, value, 0) // LSN will be handled by WAL integration

	return nil
}

// Get retrieves a value for a key from the LSM-Tree
func (lsm *LSMTree) Get(key string) (string, error) {
	start := time.Now()
	defer func() {
		lsm.stats.AvgReadLatency = time.Since(start)
	}()

	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	// 1. Check active MemTable first
	if value, found := lsm.memTable.Get(key); found {
		return value, nil
	}

	// 2. Check immutable MemTables
	for i := len(lsm.immutableTables) - 1; i >= 0; i-- {
		if value, found := lsm.immutableTables[i].Get(key); found {
			return value, nil
		}
	}

	// 3. Check SSTables from newest to oldest (L0 to LN)
	for levelIdx := 0; levelIdx < len(lsm.levels); levelIdx++ {
		level := &lsm.levels[levelIdx]

		// For L0, check all SSTables (they can overlap)
		if levelIdx == 0 {
			for i := len(level.SSTables) - 1; i >= 0; i-- {
				sstable := level.SSTables[i]
				if lsm.bloomFilterMightContain(sstable, key) {
					if value, found, err := sstable.Get(key); err != nil {
						return "", err
					} else if found {
						return value, nil
					}
				}
			}
		} else {
			// For L1+, SSTables don't overlap, so binary search by key range
			for _, sstable := range level.SSTables {
				if sstable.ContainsKey(key) && lsm.bloomFilterMightContain(sstable, key) {
					if value, found, err := sstable.Get(key); err != nil {
						return "", err
					} else if found {
						return value, nil
					}
					break // Only one SSTable can contain the key in L1+
				}
			}
		}
	}

	return "", fmt.Errorf("key not found: %s", key)
}

// Delete marks a key as deleted in the LSM-Tree
func (lsm *LSMTree) Delete(key string) error {
	lsm.mu.Lock()
	defer lsm.mu.Unlock()

	// Check if MemTable needs to be flushed
	if lsm.memTable.ShouldFlush(lsm.config.MemTableConfig) {
		if err := lsm.flushMemTable(); err != nil {
			return fmt.Errorf("failed to flush MemTable: %w", err)
		}
	}

	// Add deletion marker to MemTable
	lsm.memTable.Delete(key, 0) // LSN will be handled by WAL integration

	return nil
}

// flushMemTable flushes the current MemTable to disk as an SSTable
func (lsm *LSMTree) flushMemTable() error {
	if lsm.memTable.IsEmpty() {
		return nil
	}

	// Move current MemTable to immutable list
	lsm.immutableTables = append(lsm.immutableTables, lsm.memTable)

	// Create new active MemTable
	lsm.memTable = kvstore.NewMemTable(lsm.config.MemTableConfig)

	// Trigger background flush
	select {
	case lsm.compactionCh <- struct{}{}:
	default:
		// Compaction already queued
	}

	lsm.stats.MemTableFlushes++
	return nil
}

// bloomFilterMightContain checks if a bloom filter might contain a key
func (lsm *LSMTree) bloomFilterMightContain(sstable *SSTable, key string) bool {
	if bf, exists := lsm.bloomFilters[sstable.ID]; exists {
		might := bf.MightContain([]byte(key))
		if might {
			lsm.stats.BloomFilterHits++
		} else {
			lsm.stats.BloomFilterMisses++
		}
		return might
	}
	return true // No bloom filter, assume it might contain
}

// compactionWorker runs background compaction operations
func (lsm *LSMTree) compactionWorker() {
	defer lsm.wg.Done()

	ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-lsm.stopCh:
			return

		case <-lsm.compactionCh:
			lsm.performCompaction()

		case <-ticker.C:
			// Periodic compaction check
			if lsm.needsCompaction() {
				lsm.performCompaction()
			}
		}
	}
}

// needsCompaction checks if any level needs compaction
func (lsm *LSMTree) needsCompaction() bool {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	// Check if there are immutable MemTables to flush
	if len(lsm.immutableTables) > 0 {
		return true
	}

	// Check if any level exceeds its size/count limits
	for i := range lsm.levels {
		level := &lsm.levels[i]
		if len(level.SSTables) > level.Config.MaxSSTables {
			return true
		}

		totalSize := int64(0)
		for _, sstable := range level.SSTables {
			totalSize += sstable.FileSize
		}
		if totalSize > level.Config.MaxSize {
			return true
		}
	}

	return false
}

// performCompaction performs the actual compaction work
func (lsm *LSMTree) performCompaction() {
	lsm.mu.Lock()
	defer lsm.mu.Unlock()

	start := time.Now()
	defer func() {
		lsm.stats.LastCompactionTime = time.Now()
		lsm.stats.CompactionCount++
	}()

	// 1. Flush any immutable MemTables to L0
	if err := lsm.flushImmutableMemTables(); err != nil {
		fmt.Printf("Error flushing immutable MemTables: %v\n", err)
		return
	}

	// 2. Perform level compaction if needed
	for level := 0; level < len(lsm.levels)-1; level++ {
		if lsm.shouldCompactLevel(level) {
			if err := lsm.compactLevel(level); err != nil {
				fmt.Printf("Error compacting level %d: %v\n", level, err)
				return
			}
		}
	}

	fmt.Printf("Compaction completed in %v\n", time.Since(start))
}

// shouldCompactLevel checks if a specific level needs compaction
func (lsm *LSMTree) shouldCompactLevel(level int) bool {
	if level >= len(lsm.levels) {
		return false
	}

	levelData := &lsm.levels[level]

	// Check SSTable count
	if len(levelData.SSTables) > levelData.Config.MaxSSTables {
		return true
	}

	// Check total size
	totalSize := int64(0)
	for _, sstable := range levelData.SSTables {
		totalSize += sstable.FileSize
	}

	return totalSize > levelData.Config.CompactionSize
}

// flushImmutableMemTables flushes all immutable MemTables to L0 SSTables
func (lsm *LSMTree) flushImmutableMemTables() error {
	for len(lsm.immutableTables) > 0 {
		// Take the oldest immutable MemTable
		memTable := lsm.immutableTables[0]
		lsm.immutableTables = lsm.immutableTables[1:]

		// Create SSTable from MemTable
		sstable, err := lsm.createSSTableFromMemTable(memTable)
		if err != nil {
			return fmt.Errorf("failed to create SSTable: %w", err)
		}

		// Add to L0
		lsm.levels[0].SSTables = append(lsm.levels[0].SSTables, sstable)

		// Create bloom filter for the new SSTable
		bf, err := lsm.createBloomFilter(sstable)
		if err != nil {
			return fmt.Errorf("failed to create bloom filter: %w", err)
		}
		lsm.bloomFilters[sstable.ID] = bf
	}

	return nil
}

// createSSTableFromMemTable creates an SSTable from a MemTable
func (lsm *LSMTree) createSSTableFromMemTable(memTable *kvstore.MemTable) (*SSTable, error) {
	lsm.nextSSTableID++
	sstableID := fmt.Sprintf("sstable_%d", lsm.nextSSTableID)

	sstable, err := NewSSTable(sstableID, lsm.dataDir, 0)
	if err != nil {
		return nil, err
	}

	// Get all entries from MemTable in sorted order
	entries := memTable.GetAll()

	// Write entries to SSTable
	for _, entry := range entries {
		if err := sstable.Put(entry.Key, entry.Value, entry.Deleted); err != nil {
			return nil, fmt.Errorf("failed to write to SSTable: %w", err)
		}
	}

	// Finalize SSTable
	if err := sstable.Finalize(); err != nil {
		return nil, fmt.Errorf("failed to finalize SSTable: %w", err)
	}

	return sstable, nil
}

// createBloomFilter creates a bloom filter for an SSTable
func (lsm *LSMTree) createBloomFilter(sstable *SSTable) (*BloomFilter, error) {
	// Estimate number of keys in SSTable
	numKeys := sstable.NumEntries

	bf := NewBloomFilter(numKeys, lsm.config.BloomFilterFPR)

	// Add all keys to bloom filter
	keys, err := sstable.GetAllKeys()
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		bf.Add([]byte(key))
	}

	return bf, nil
}

// compactLevel compacts a specific level with the next level
func (lsm *LSMTree) compactLevel(level int) error {
	if level >= len(lsm.levels)-1 {
		return nil // Can't compact the last level
	}

	fmt.Printf("Compacting level %d\n", level)

	// Implementation depends on compaction strategy
	switch lsm.config.CompactionStyle {
	case LeveledCompaction:
		return lsm.performLeveledCompaction(level)
	case SizeTieredCompaction:
		return lsm.performSizeTieredCompaction(level)
	case HybridCompaction:
		if level == 0 {
			return lsm.performSizeTieredCompaction(level)
		}
		return lsm.performLeveledCompaction(level)
	default:
		return lsm.performLeveledCompaction(level)
	}
}

// performLeveledCompaction performs leveled compaction between two levels
func (lsm *LSMTree) performLeveledCompaction(level int) error {
	// This is a simplified implementation
	// In a full implementation, this would involve:
	// 1. Select SSTables to compact from current level
	// 2. Find overlapping SSTables in next level
	// 3. Merge all selected SSTables
	// 4. Write new SSTables to next level
	// 5. Delete old SSTables

	fmt.Printf("Performing leveled compaction on level %d (placeholder)\n", level)
	return nil
}

// performSizeTieredCompaction performs size-tiered compaction
func (lsm *LSMTree) performSizeTieredCompaction(level int) error {
	// This is a simplified implementation
	// In a full implementation, this would:
	// 1. Group SSTables by similar size
	// 2. Merge groups that exceed size threshold
	// 3. Write merged results to next level

	fmt.Printf("Performing size-tiered compaction on level %d (placeholder)\n", level)
	return nil
}

// GetStats returns current LSM-Tree statistics
func (lsm *LSMTree) GetStats() LSMStats {
	lsm.mu.RLock()
	defer lsm.mu.RUnlock()

	stats := lsm.stats
	stats.TotalLevels = len(lsm.levels)

	// Count active SSTables
	for _, level := range lsm.levels {
		stats.ActiveSSTables += len(level.SSTables)
	}

	return stats
}

// Close shuts down the LSM-Tree gracefully
func (lsm *LSMTree) Close() error {
	close(lsm.stopCh)
	lsm.wg.Wait()

	// Perform final flush
	lsm.mu.Lock()
	defer lsm.mu.Unlock()

	if err := lsm.flushMemTable(); err != nil {
		return fmt.Errorf("failed to flush MemTable during close: %w", err)
	}

	if err := lsm.flushImmutableMemTables(); err != nil {
		return fmt.Errorf("failed to flush immutable MemTables during close: %w", err)
	}

	// Close all SSTables
	for _, level := range lsm.levels {
		for _, sstable := range level.SSTables {
			if err := sstable.Close(); err != nil {
				return fmt.Errorf("failed to close SSTable %s: %w", sstable.ID, err)
			}
		}
	}

	return nil
}
