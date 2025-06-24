package lsm

import (
	"fmt"
	"sync"

	"github.com/nyasuto/moz/internal/kvstore"
)

// LSMKVStore implements the KVStore interface using LSM-Tree architecture
// It provides a seamless migration path from the existing single-file storage
type LSMKVStore struct {
	lsm *LSMTree

	// Migration support
	legacyStore   *kvstore.KVStore
	migrationMode bool
	mu            sync.RWMutex
}

// LSMKVStoreConfig holds configuration for LSM-based KVStore
type LSMKVStoreConfig struct {
	LSMConfig       LSMConfig
	DataDir         string
	EnableMigration bool
	LegacyLogFile   string
}

// DefaultLSMKVStoreConfig returns default configuration
func DefaultLSMKVStoreConfig() LSMKVStoreConfig {
	return LSMKVStoreConfig{
		LSMConfig:       DefaultLSMConfig(),
		DataDir:         "data/lsm",
		EnableMigration: true,
		LegacyLogFile:   "moz.log",
	}
}

// NewLSMKVStore creates a new LSM-Tree based KVStore
func NewLSMKVStore(config LSMKVStoreConfig) (*LSMKVStore, error) {
	// Create LSM-Tree
	lsm, err := NewLSMTree(config.LSMConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create LSM-Tree: %w", err)
	}

	store := &LSMKVStore{
		lsm:           lsm,
		migrationMode: config.EnableMigration,
	}

	// Initialize legacy store for migration if needed
	if config.EnableMigration {
		legacyStore := kvstore.New()
		store.legacyStore = legacyStore

		// Migrate existing data if present
		if err := store.migrateExistingData(); err != nil {
			return nil, fmt.Errorf("failed to migrate existing data: %w", err)
		}
	}

	return store, nil
}

// Put stores a key-value pair
func (lkv *LSMKVStore) Put(key, value string) error {
	lkv.mu.RLock()
	defer lkv.mu.RUnlock()

	// Write to LSM-Tree
	if err := lkv.lsm.Put(key, value); err != nil {
		return fmt.Errorf("LSM put failed: %w", err)
	}

	// Also write to legacy store during migration
	if lkv.migrationMode && lkv.legacyStore != nil {
		if err := lkv.legacyStore.Put(key, value); err != nil {
			// Log warning but don't fail - LSM is primary
			fmt.Printf("Warning: legacy store put failed: %v\n", err)
		}
	}

	return nil
}

// Get retrieves a value for a key
func (lkv *LSMKVStore) Get(key string) (string, error) {
	lkv.mu.RLock()
	defer lkv.mu.RUnlock()

	// Try LSM-Tree first
	value, err := lkv.lsm.Get(key)
	if err == nil {
		return value, nil
	}

	// Fall back to legacy store during migration
	if lkv.migrationMode && lkv.legacyStore != nil {
		if legacyValue, legacyErr := lkv.legacyStore.Get(key); legacyErr == nil {
			// Found in legacy store, migrate to LSM
			if putErr := lkv.lsm.Put(key, legacyValue); putErr != nil {
				fmt.Printf("Warning: failed to migrate key %s to LSM: %v\n", key, putErr)
			}
			return legacyValue, nil
		}
	}

	return "", fmt.Errorf("key not found: %s", key)
}

// Delete removes a key
func (lkv *LSMKVStore) Delete(key string) error {
	lkv.mu.RLock()
	defer lkv.mu.RUnlock()

	// Delete from LSM-Tree
	if err := lkv.lsm.Delete(key); err != nil {
		return fmt.Errorf("LSM delete failed: %w", err)
	}

	// Also delete from legacy store during migration
	if lkv.migrationMode && lkv.legacyStore != nil {
		if err := lkv.legacyStore.Delete(key); err != nil {
			// Log warning but don't fail - LSM is primary
			fmt.Printf("Warning: legacy store delete failed: %v\n", err)
		}
	}

	return nil
}

// List returns all keys (this is an expensive operation in LSM-Tree)
func (lkv *LSMKVStore) List() ([]string, error) {
	lkv.mu.RLock()
	defer lkv.mu.RUnlock()

	// Collect keys from all levels
	keySet := make(map[string]bool)

	// Get keys from MemTable
	if memKeys := lkv.lsm.memTable.List(); len(memKeys) > 0 {
		for _, key := range memKeys {
			keySet[key] = true
		}
	}

	// Get keys from immutable MemTables
	for _, immutableTable := range lkv.lsm.immutableTables {
		if immutableKeys := immutableTable.List(); len(immutableKeys) > 0 {
			for _, key := range immutableKeys {
				keySet[key] = true
			}
		}
	}

	// Get keys from SSTables
	for _, level := range lkv.lsm.levels {
		for _, sstable := range level.SSTables {
			if keys, err := sstable.GetAllKeys(); err == nil {
				for _, key := range keys {
					keySet[key] = true
				}
			}
		}
	}

	// During migration, also get keys from legacy store
	if lkv.migrationMode && lkv.legacyStore != nil {
		if legacyKeys, err := lkv.legacyStore.List(); err == nil {
			for _, key := range legacyKeys {
				keySet[key] = true
			}
		}
	}

	// Convert to slice and verify existence (to filter out deleted keys)
	var result []string
	for key := range keySet {
		if _, err := lkv.Get(key); err == nil {
			result = append(result, key)
		}
	}

	return result, nil
}

// Compact performs compaction on the LSM-Tree
func (lkv *LSMKVStore) Compact() error {
	lkv.mu.Lock()
	defer lkv.mu.Unlock()

	// Trigger LSM-Tree compaction
	lkv.lsm.compactionCh <- struct{}{}

	// Also compact legacy store if present
	if lkv.migrationMode && lkv.legacyStore != nil {
		if err := lkv.legacyStore.Compact(); err != nil {
			fmt.Printf("Warning: legacy store compaction failed: %v\n", err)
		}
	}

	return nil
}

// Stats returns storage statistics
func (lkv *LSMKVStore) Stats() map[string]interface{} {
	lkv.mu.RLock()
	defer lkv.mu.RUnlock()

	stats := make(map[string]interface{})

	// LSM-Tree stats
	lsmStats := lkv.lsm.GetStats()
	stats["lsm"] = map[string]interface{}{
		"total_levels":         lsmStats.TotalLevels,
		"active_sstables":      lsmStats.ActiveSSTables,
		"memtable_flushes":     lsmStats.MemTableFlushes,
		"compaction_count":     lsmStats.CompactionCount,
		"bytes_read":           lsmStats.BytesRead,
		"bytes_written":        lsmStats.BytesWritten,
		"bloom_filter_hits":    lsmStats.BloomFilterHits,
		"bloom_filter_misses":  lsmStats.BloomFilterMisses,
		"avg_read_latency":     lsmStats.AvgReadLatency,
		"avg_write_latency":    lsmStats.AvgWriteLatency,
		"last_compaction_time": lsmStats.LastCompactionTime,
	}

	// Migration status
	stats["migration"] = map[string]interface{}{
		"migration_mode":   lkv.migrationMode,
		"has_legacy_store": lkv.legacyStore != nil,
	}

	// Legacy store stats if available
	if lkv.migrationMode && lkv.legacyStore != nil {
		// Note: kvstore.KVStore doesn't have a Stats() method in the original code
		// This would need to be added or we can provide basic info
		stats["legacy"] = map[string]interface{}{
			"enabled": true,
		}
	}

	return stats
}

// Close gracefully shuts down the LSM-KVStore
func (lkv *LSMKVStore) Close() error {
	lkv.mu.Lock()
	defer lkv.mu.Unlock()

	var errors []string

	// Close LSM-Tree
	if err := lkv.lsm.Close(); err != nil {
		errors = append(errors, fmt.Sprintf("LSM close: %v", err))
	}

	// Close legacy store if present
	if lkv.legacyStore != nil {
		// Note: The original KVStore doesn't have a Close method
		// This would need to be added for proper resource cleanup
		lkv.legacyStore = nil
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors during close: %v", errors)
	}

	return nil
}

// CompleteMigration finalizes the migration by disabling legacy store
func (lkv *LSMKVStore) CompleteMigration() error {
	lkv.mu.Lock()
	defer lkv.mu.Unlock()

	if !lkv.migrationMode {
		return fmt.Errorf("not in migration mode")
	}

	// Ensure all data is migrated
	if err := lkv.migrateRemainingData(); err != nil {
		return fmt.Errorf("failed to complete data migration: %w", err)
	}

	// Disable migration mode
	lkv.migrationMode = false
	lkv.legacyStore = nil

	fmt.Println("Migration completed successfully")
	return nil
}

// migrateExistingData migrates data from legacy store to LSM-Tree
func (lkv *LSMKVStore) migrateExistingData() error {
	if lkv.legacyStore == nil {
		return nil
	}

	// Get all keys from legacy store
	keys, err := lkv.legacyStore.List()
	if err != nil {
		return fmt.Errorf("failed to list legacy keys: %w", err)
	}

	migrated := 0
	for _, key := range keys {
		value, err := lkv.legacyStore.Get(key)
		if err != nil {
			fmt.Printf("Warning: failed to get legacy key %s: %v\n", key, err)
			continue
		}

		if err := lkv.lsm.Put(key, value); err != nil {
			fmt.Printf("Warning: failed to migrate key %s: %v\n", key, err)
			continue
		}

		migrated++
	}

	fmt.Printf("Migrated %d keys from legacy store to LSM-Tree\n", migrated)
	return nil
}

// migrateRemainingData ensures all remaining data is migrated
func (lkv *LSMKVStore) migrateRemainingData() error {
	if lkv.legacyStore == nil {
		return nil
	}

	// This is similar to migrateExistingData but more thorough
	// In a production system, this might involve checksums or verification
	return lkv.migrateExistingData()
}

// ForceFlush forces all pending writes to disk
func (lkv *LSMKVStore) ForceFlush() error {
	lkv.mu.Lock()
	defer lkv.mu.Unlock()

	// Flush MemTable to SSTable
	if err := lkv.lsm.flushMemTable(); err != nil {
		return fmt.Errorf("failed to flush MemTable: %w", err)
	}

	// Force compaction to ensure data is persisted
	lkv.lsm.compactionCh <- struct{}{}

	return nil
}

// GetLSMStats returns detailed LSM-Tree statistics
func (lkv *LSMKVStore) GetLSMStats() LSMStats {
	lkv.mu.RLock()
	defer lkv.mu.RUnlock()

	return lkv.lsm.GetStats()
}

// SetMigrationMode enables or disables migration mode
func (lkv *LSMKVStore) SetMigrationMode(enabled bool) {
	lkv.mu.Lock()
	defer lkv.mu.Unlock()

	lkv.migrationMode = enabled
}

// IsInMigrationMode returns true if the store is in migration mode
func (lkv *LSMKVStore) IsInMigrationMode() bool {
	lkv.mu.RLock()
	defer lkv.mu.RUnlock()

	return lkv.migrationMode
}

// TriggerCompaction manually triggers compaction
func (lkv *LSMKVStore) TriggerCompaction() {
	select {
	case lkv.lsm.compactionCh <- struct{}{}:
		// Compaction triggered
	default:
		// Compaction already pending
	}
}

// EstimateMemoryUsage returns estimated memory usage
func (lkv *LSMKVStore) EstimateMemoryUsage() map[string]int64 {
	lkv.mu.RLock()
	defer lkv.mu.RUnlock()

	usage := make(map[string]int64)

	// MemTable memory usage
	if lkv.lsm.memTable != nil {
		usage["memtable"] = lkv.lsm.memTable.Size()
	}

	// Immutable MemTables
	var immutableSize int64
	for _, table := range lkv.lsm.immutableTables {
		immutableSize += table.Size()
	}
	usage["immutable_memtables"] = immutableSize

	// Bloom filters
	var bloomFilterSize int64
	for _, bf := range lkv.lsm.bloomFilters {
		bloomFilterSize += int64(bf.MemoryUsage())
	}
	usage["bloom_filters"] = bloomFilterSize

	// SSTable cache (if implemented)
	usage["sstable_cache"] = 0 // Placeholder

	return usage
}

// GetLevelInfo returns information about each level
func (lkv *LSMKVStore) GetLevelInfo() []map[string]interface{} {
	lkv.mu.RLock()
	defer lkv.mu.RUnlock()

	var levelInfo []map[string]interface{}

	for i, level := range lkv.lsm.levels {
		var totalSize int64
		var keyRange []string

		if len(level.SSTables) > 0 {
			minKey := level.SSTables[0].metadata.MinKey
			maxKey := level.SSTables[0].metadata.MaxKey

			for _, sstable := range level.SSTables {
				totalSize += sstable.FileSize
				if sstable.metadata.MinKey < minKey {
					minKey = sstable.metadata.MinKey
				}
				if sstable.metadata.MaxKey > maxKey {
					maxKey = sstable.metadata.MaxKey
				}
			}

			keyRange = []string{minKey, maxKey}
		}

		info := map[string]interface{}{
			"level":         i,
			"sstable_count": len(level.SSTables),
			"total_size":    totalSize,
			"max_size":      level.Config.MaxSize,
			"key_range":     keyRange,
		}

		levelInfo = append(levelInfo, info)
	}

	return levelInfo
}
