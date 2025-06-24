package index

import (
	"fmt"
	"sync"
)

// IndexType represents the type of index
type IndexType string

const (
	IndexTypeHash  IndexType = "hash"
	IndexTypeBTree IndexType = "btree"
	IndexTypeNone  IndexType = "none"
)

// IndexEntry represents an entry in the index pointing to data location
type IndexEntry struct {
	Key       string // The key being indexed
	Offset    int64  // File offset where the entry is stored
	Size      int32  // Size of the entry in bytes
	Timestamp int64  // Unix nanoseconds timestamp
	Deleted   bool   // Whether this entry is a deletion marker
}

// IndexEntryPool interface for memory pooling IndexEntry objects
type IndexEntryPool interface {
	GetIndexEntry() *IndexEntry
	PutIndexEntry(entry *IndexEntry)
}

// Index interface defines the contract for all index implementations
type Index interface {
	// Basic operations
	Insert(key string, entry IndexEntry) error
	Delete(key string) error
	Get(key string) (IndexEntry, error)
	Exists(key string) bool

	// Batch operations
	BatchInsert(entries map[string]IndexEntry) error
	BatchDelete(keys []string) error

	// Iteration and range operations
	Keys() []string                                // Get all keys in sorted order
	Range(start, end string) ([]IndexEntry, error) // Get entries in key range [start, end]
	Prefix(prefix string) ([]IndexEntry, error)    // Get entries with key prefix

	// Statistics and maintenance
	Size() int64                                 // Number of entries in index
	MemoryUsage() int64                          // Estimated memory usage in bytes
	Validate() error                             // Validate index integrity
	Rebuild(entries map[string]IndexEntry) error // Rebuild entire index

	// Persistence (for implementations that support it)
	Save(filename string) error // Save index to file
	Load(filename string) error // Load index from file

	// Cleanup
	Close() error // Cleanup resources
}

// IndexManager manages multiple index types and provides a unified interface
type IndexManager struct {
	indexType IndexType
	index     Index
	mu        sync.RWMutex
	enabled   bool
}

// NewIndexManager creates a new index manager with the specified type
func NewIndexManager(indexType IndexType) (*IndexManager, error) {
	return NewIndexManagerWithPool(indexType, nil)
}

// NewIndexManagerWithPool creates a new index manager with optional memory pool
func NewIndexManagerWithPool(indexType IndexType, entryPool IndexEntryPool) (*IndexManager, error) {
	var idx Index
	var err error

	switch indexType {
	case IndexTypeHash:
		if entryPool != nil {
			idx, err = NewHashIndexWithPool(DefaultHashIndexConfig(), entryPool)
		} else {
			idx, err = NewHashIndex(DefaultHashIndexConfig())
		}
	case IndexTypeBTree:
		if entryPool != nil {
			idx, err = NewBTreeIndexWithPool(DefaultBTreeIndexConfig(), entryPool)
		} else {
			idx, err = NewBTreeIndex(DefaultBTreeIndexConfig())
		}
	case IndexTypeNone:
		idx = NewNoIndex()
	default:
		return nil, fmt.Errorf("unsupported index type: %s", indexType)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}

	return &IndexManager{
		indexType: indexType,
		index:     idx,
		enabled:   indexType != IndexTypeNone,
	}, nil
}

// Insert adds an entry to the index
func (im *IndexManager) Insert(key string, entry IndexEntry) error {
	if !im.enabled {
		return nil
	}

	im.mu.Lock()
	defer im.mu.Unlock()
	return im.index.Insert(key, entry)
}

// Delete removes an entry from the index
func (im *IndexManager) Delete(key string) error {
	if !im.enabled {
		return nil
	}

	im.mu.Lock()
	defer im.mu.Unlock()
	return im.index.Delete(key)
}

// Get retrieves an entry from the index
func (im *IndexManager) Get(key string) (IndexEntry, error) {
	if !im.enabled {
		return IndexEntry{}, fmt.Errorf("index disabled")
	}

	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.index.Get(key)
}

// Exists checks if a key exists in the index
func (im *IndexManager) Exists(key string) bool {
	if !im.enabled {
		return false
	}

	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.index.Exists(key)
}

// Keys returns all keys in sorted order
func (im *IndexManager) Keys() []string {
	if !im.enabled {
		return nil
	}

	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.index.Keys()
}

// Range returns entries in the specified key range
func (im *IndexManager) Range(start, end string) ([]IndexEntry, error) {
	if !im.enabled {
		return nil, fmt.Errorf("index disabled")
	}

	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.index.Range(start, end)
}

// Prefix returns entries with the specified key prefix
func (im *IndexManager) Prefix(prefix string) ([]IndexEntry, error) {
	if !im.enabled {
		return nil, fmt.Errorf("index disabled")
	}

	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.index.Prefix(prefix)
}

// Size returns the number of entries in the index
func (im *IndexManager) Size() int64 {
	if !im.enabled {
		return 0
	}

	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.index.Size()
}

// MemoryUsage returns estimated memory usage
func (im *IndexManager) MemoryUsage() int64 {
	if !im.enabled {
		return 0
	}

	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.index.MemoryUsage()
}

// GetIndexType returns the current index type
func (im *IndexManager) GetIndexType() IndexType {
	return im.indexType
}

// IsEnabled returns whether indexing is enabled
func (im *IndexManager) IsEnabled() bool {
	return im.enabled
}

// Validate validates index integrity
func (im *IndexManager) Validate() error {
	if !im.enabled {
		return nil
	}

	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.index.Validate()
}

// Rebuild rebuilds the entire index from scratch
func (im *IndexManager) Rebuild(entries map[string]IndexEntry) error {
	if !im.enabled {
		return nil
	}

	im.mu.Lock()
	defer im.mu.Unlock()
	return im.index.Rebuild(entries)
}

// Save persists the index to a file
func (im *IndexManager) Save(filename string) error {
	if !im.enabled {
		return nil
	}

	im.mu.RLock()
	defer im.mu.RUnlock()
	return im.index.Save(filename)
}

// Load restores the index from a file
func (im *IndexManager) Load(filename string) error {
	if !im.enabled {
		return nil
	}

	im.mu.Lock()
	defer im.mu.Unlock()
	return im.index.Load(filename)
}

// Close cleans up index resources
func (im *IndexManager) Close() error {
	im.mu.Lock()
	defer im.mu.Unlock()

	if im.index != nil {
		return im.index.Close()
	}
	return nil
}
