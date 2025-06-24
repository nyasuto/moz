package kvstore

import (
	"sort"
	"sync"
	"time"
)

// MemTable represents an in-memory sorted table for recent writes
type MemTable struct {
	mu   sync.RWMutex
	data map[string]*MemTableEntry

	// Metadata
	size      int64 // Total memory usage in bytes
	maxSize   int64 // Maximum size before flush
	createdAt time.Time

	// Statistics
	stats MemTableStats
}

// MemTableEntry represents a single entry in the MemTable
type MemTableEntry struct {
	Key       string
	Value     string
	Timestamp int64  // Unix timestamp in nanoseconds
	Deleted   bool   // True if this is a deletion marker
	LSN       uint64 // WAL Log Sequence Number
}

// MemTableStats holds statistics about MemTable operations
type MemTableStats struct {
	Entries       int
	MemoryUsage   int64
	PutCount      uint64
	GetCount      uint64
	DeleteCount   uint64
	FlushCount    uint64
	LastFlushTime time.Time
}

// MemTableConfig holds configuration for MemTable
type MemTableConfig struct {
	MaxSize      int64 // Maximum size in bytes before forcing flush
	MaxEntries   int   // Maximum number of entries
	FlushTimeout time.Duration
}

// DefaultMemTableConfig returns default MemTable configuration
func DefaultMemTableConfig() MemTableConfig {
	return MemTableConfig{
		MaxSize:      16 * 1024 * 1024, // 16MB
		MaxEntries:   100000,           // 100K entries
		FlushTimeout: 30 * time.Second,
	}
}

// NewMemTable creates a new MemTable
func NewMemTable(config MemTableConfig) *MemTable {
	return &MemTable{
		data:      make(map[string]*MemTableEntry),
		maxSize:   config.MaxSize,
		createdAt: time.Now(),
	}
}

// Put adds a key-value pair to the MemTable
func (mt *MemTable) Put(key, value string, lsn uint64) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	// Calculate size changes
	oldEntry, exists := mt.data[key]
	oldSize := int64(0)
	if exists {
		oldSize = mt.calculateEntrySize(oldEntry)
	}

	// Create new entry
	entry := &MemTableEntry{
		Key:       key,
		Value:     value,
		Timestamp: time.Now().UnixNano(),
		Deleted:   false,
		LSN:       lsn,
	}

	newSize := mt.calculateEntrySize(entry)

	// Update data
	mt.data[key] = entry

	// Update size and statistics
	mt.size = mt.size - oldSize + newSize
	mt.stats.PutCount++
	mt.stats.Entries = len(mt.data)
	mt.stats.MemoryUsage = mt.size
}

// Get retrieves a value from the MemTable
func (mt *MemTable) Get(key string) (string, bool) {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	mt.stats.GetCount++

	entry, exists := mt.data[key]
	if !exists || entry.Deleted {
		return "", false
	}

	return entry.Value, true
}

// Delete marks a key as deleted in the MemTable
func (mt *MemTable) Delete(key string, lsn uint64) {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	// Create deletion marker
	entry := &MemTableEntry{
		Key:       key,
		Value:     "",
		Timestamp: time.Now().UnixNano(),
		Deleted:   true,
		LSN:       lsn,
	}

	// Calculate size changes
	oldEntry, exists := mt.data[key]
	oldSize := int64(0)
	if exists {
		oldSize = mt.calculateEntrySize(oldEntry)
	}

	newSize := mt.calculateEntrySize(entry)

	// Update data
	mt.data[key] = entry

	// Update size and statistics
	mt.size = mt.size - oldSize + newSize
	mt.stats.DeleteCount++
	mt.stats.Entries = len(mt.data)
	mt.stats.MemoryUsage = mt.size
}

// List returns all non-deleted keys in sorted order
func (mt *MemTable) List() []string {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	var keys []string
	for key, entry := range mt.data {
		if !entry.Deleted {
			keys = append(keys, key)
		}
	}

	sort.Strings(keys)
	return keys
}

// GetAll returns all entries (including deleted) for flushing
func (mt *MemTable) GetAll() []*MemTableEntry {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	entries := make([]*MemTableEntry, 0, len(mt.data))
	for _, entry := range mt.data {
		// Create copy to avoid race conditions
		entryCopy := &MemTableEntry{
			Key:       entry.Key,
			Value:     entry.Value,
			Timestamp: entry.Timestamp,
			Deleted:   entry.Deleted,
			LSN:       entry.LSN,
		}
		entries = append(entries, entryCopy)
	}

	// Sort by LSN to maintain order
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LSN < entries[j].LSN
	})

	return entries
}

// Size returns the current memory usage in bytes
func (mt *MemTable) Size() int64 {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return mt.size
}

// Count returns the number of entries
func (mt *MemTable) Count() int {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return len(mt.data)
}

// ShouldFlush returns true if the MemTable should be flushed
func (mt *MemTable) ShouldFlush(config MemTableConfig) bool {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	// Check size limit
	if mt.size >= config.MaxSize {
		return true
	}

	// Check entry count limit
	if len(mt.data) >= config.MaxEntries {
		return true
	}

	// Check time-based flush
	if time.Since(mt.createdAt) >= config.FlushTimeout {
		return true
	}

	return false
}

// Clear removes all entries from the MemTable
func (mt *MemTable) Clear() {
	mt.mu.Lock()
	defer mt.mu.Unlock()

	mt.data = make(map[string]*MemTableEntry)
	mt.size = 0
	mt.createdAt = time.Now()
	mt.stats.FlushCount++
	mt.stats.LastFlushTime = time.Now()
	mt.stats.Entries = 0
	mt.stats.MemoryUsage = 0
}

// GetStats returns current MemTable statistics
func (mt *MemTable) GetStats() MemTableStats {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	stats := mt.stats
	stats.Entries = len(mt.data)
	stats.MemoryUsage = mt.size

	return stats
}

// calculateEntrySize estimates the memory usage of an entry
func (mt *MemTable) calculateEntrySize(entry *MemTableEntry) int64 {
	// Base struct size + string lengths
	// This is an approximation; actual memory usage may vary
	baseSize := int64(64) // Estimated struct overhead
	keySize := int64(len(entry.Key))
	valueSize := int64(len(entry.Value))

	return baseSize + keySize + valueSize
}

// Range returns entries in the specified key range
func (mt *MemTable) Range(startKey, endKey string) []*MemTableEntry {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	var result []*MemTableEntry
	for key, entry := range mt.data {
		if key >= startKey && key <= endKey && !entry.Deleted {
			// Create copy
			entryCopy := &MemTableEntry{
				Key:       entry.Key,
				Value:     entry.Value,
				Timestamp: entry.Timestamp,
				Deleted:   entry.Deleted,
				LSN:       entry.LSN,
			}
			result = append(result, entryCopy)
		}
	}

	// Sort by key
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	return result
}

// PrefixSearch returns entries with keys matching the given prefix
func (mt *MemTable) PrefixSearch(prefix string) []*MemTableEntry {
	mt.mu.RLock()
	defer mt.mu.RUnlock()

	var result []*MemTableEntry
	for key, entry := range mt.data {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix && !entry.Deleted {
			// Create copy
			entryCopy := &MemTableEntry{
				Key:       entry.Key,
				Value:     entry.Value,
				Timestamp: entry.Timestamp,
				Deleted:   entry.Deleted,
				LSN:       entry.LSN,
			}
			result = append(result, entryCopy)
		}
	}

	// Sort by key
	sort.Slice(result, func(i, j int) bool {
		return result[i].Key < result[j].Key
	})

	return result
}

// IsEmpty returns true if the MemTable has no entries
func (mt *MemTable) IsEmpty() bool {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return len(mt.data) == 0
}

// Age returns the age of the MemTable since creation
func (mt *MemTable) Age() time.Duration {
	mt.mu.RLock()
	defer mt.mu.RUnlock()
	return time.Since(mt.createdAt)
}
