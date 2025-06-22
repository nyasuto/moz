package index

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// BTreeIndexConfig holds configuration for B-tree index
type BTreeIndexConfig struct {
	Degree int // B-tree degree (minimum number of keys per node)
}

// DefaultBTreeIndexConfig returns sensible defaults for B-tree index
func DefaultBTreeIndexConfig() BTreeIndexConfig {
	return BTreeIndexConfig{
		Degree: 64, // Higher degree for better performance with large datasets
	}
}

// BTreeNode represents a node in the B-tree
type BTreeNode struct {
	Keys     []string     // Sorted keys
	Entries  []IndexEntry // Corresponding entries
	Children []*BTreeNode // Child nodes (nil for leaf nodes)
	IsLeaf   bool
	Parent   *BTreeNode
}

// BTreeIndex implements a B-tree based index for range queries and sorted access
type BTreeIndex struct {
	root   *BTreeNode
	config BTreeIndexConfig
	count  int64
	mu     sync.RWMutex
}

// NewBTreeIndex creates a new B-tree index
func NewBTreeIndex(config BTreeIndexConfig) (*BTreeIndex, error) {
	if config.Degree < 2 {
		return nil, fmt.Errorf("b-tree degree must be at least 2")
	}

	return &BTreeIndex{
		root: &BTreeNode{
			Keys:     make([]string, 0, 2*config.Degree-1),
			Entries:  make([]IndexEntry, 0, 2*config.Degree-1),
			Children: nil, // Will be allocated when needed
			IsLeaf:   true,
			Parent:   nil,
		},
		config: config,
		count:  0,
	}, nil
}

// Insert adds an entry to the B-tree index
func (bt *BTreeIndex) Insert(key string, entry IndexEntry) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	// For this initial implementation, we'll use a simpler approach
	// and store all entries in sorted order in the root node
	// TODO: Implement proper B-tree insertion with splitting

	node := bt.root

	// Find insertion point
	i := sort.SearchStrings(node.Keys, key)

	// Check if key already exists
	if i < len(node.Keys) && node.Keys[i] == key {
		// Update existing entry
		node.Entries[i] = entry
		return nil
	}

	// Insert new entry
	node.Keys = append(node.Keys, "")
	node.Entries = append(node.Entries, IndexEntry{})

	// Shift elements to make room
	copy(node.Keys[i+1:], node.Keys[i:])
	copy(node.Entries[i+1:], node.Entries[i:])

	// Insert new values
	node.Keys[i] = key
	node.Entries[i] = entry

	bt.count++
	return nil
}

// Delete removes an entry from the B-tree index
func (bt *BTreeIndex) Delete(key string) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	node := bt.root
	i := sort.SearchStrings(node.Keys, key)

	if i >= len(node.Keys) || node.Keys[i] != key {
		return fmt.Errorf("key not found: %s", key)
	}

	// Remove the entry
	copy(node.Keys[i:], node.Keys[i+1:])
	copy(node.Entries[i:], node.Entries[i+1:])
	node.Keys = node.Keys[:len(node.Keys)-1]
	node.Entries = node.Entries[:len(node.Entries)-1]

	bt.count--
	return nil
}

// Get retrieves an entry from the B-tree index
func (bt *BTreeIndex) Get(key string) (IndexEntry, error) {
	bt.mu.RLock()
	defer bt.mu.RUnlock()

	node := bt.root
	i := sort.SearchStrings(node.Keys, key)

	if i >= len(node.Keys) || node.Keys[i] != key {
		return IndexEntry{}, fmt.Errorf("key not found: %s", key)
	}

	return node.Entries[i], nil
}

// Exists checks if a key exists in the B-tree index
func (bt *BTreeIndex) Exists(key string) bool {
	_, err := bt.Get(key)
	return err == nil
}

// BatchInsert adds multiple entries efficiently
func (bt *BTreeIndex) BatchInsert(entries map[string]IndexEntry) error {
	// Convert to sorted slice for efficient insertion
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if err := bt.Insert(key, entries[key]); err != nil {
			return fmt.Errorf("failed to insert key %s: %w", key, err)
		}
	}
	return nil
}

// BatchDelete removes multiple entries efficiently
func (bt *BTreeIndex) BatchDelete(keys []string) error {
	// Sort keys in reverse order for efficient deletion
	sortedKeys := make([]string, len(keys))
	copy(sortedKeys, keys)
	sort.Sort(sort.Reverse(sort.StringSlice(sortedKeys)))

	for _, key := range sortedKeys {
		if err := bt.Delete(key); err != nil {
			return fmt.Errorf("failed to delete key %s: %w", key, err)
		}
	}
	return nil
}

// Keys returns all keys in sorted order (B-tree's natural strength)
func (bt *BTreeIndex) Keys() []string {
	bt.mu.RLock()
	defer bt.mu.RUnlock()

	// Since we're storing everything in the root in sorted order,
	// we can just return a copy
	keys := make([]string, len(bt.root.Keys))
	copy(keys, bt.root.Keys)
	return keys
}

// Range returns entries in the specified key range [start, end]
func (bt *BTreeIndex) Range(start, end string) ([]IndexEntry, error) {
	bt.mu.RLock()
	defer bt.mu.RUnlock()

	node := bt.root
	var entries []IndexEntry

	// Find starting position
	startIdx := sort.SearchStrings(node.Keys, start)

	// Collect entries in range
	for i := startIdx; i < len(node.Keys) && node.Keys[i] <= end; i++ {
		entries = append(entries, node.Entries[i])
	}

	return entries, nil
}

// Prefix returns entries with the specified key prefix
func (bt *BTreeIndex) Prefix(prefix string) ([]IndexEntry, error) {
	bt.mu.RLock()
	defer bt.mu.RUnlock()

	node := bt.root
	var entries []IndexEntry

	// Find starting position
	startIdx := sort.SearchStrings(node.Keys, prefix)

	// Collect entries with prefix
	for i := startIdx; i < len(node.Keys); i++ {
		if !strings.HasPrefix(node.Keys[i], prefix) {
			break
		}
		entries = append(entries, node.Entries[i])
	}

	return entries, nil
}

// Size returns the number of entries in the index
func (bt *BTreeIndex) Size() int64 {
	bt.mu.RLock()
	defer bt.mu.RUnlock()
	return bt.count
}

// MemoryUsage estimates memory usage in bytes
func (bt *BTreeIndex) MemoryUsage() int64 {
	bt.mu.RLock()
	defer bt.mu.RUnlock()

	size := int64(0)

	// Root node overhead
	size += 128 // Struct overhead

	// Keys and entries
	for _, key := range bt.root.Keys {
		size += int64(len(key)) + 64 // Key string + IndexEntry struct
	}

	return size
}

// Validate checks the integrity of the B-tree index
func (bt *BTreeIndex) Validate() error {
	bt.mu.RLock()
	defer bt.mu.RUnlock()

	node := bt.root

	// Check that keys are sorted
	for i := 1; i < len(node.Keys); i++ {
		if node.Keys[i-1] >= node.Keys[i] {
			return fmt.Errorf("keys not sorted at position %d: %s >= %s",
				i, node.Keys[i-1], node.Keys[i])
		}
	}

	// Check count matches
	if int64(len(node.Keys)) != bt.count {
		return fmt.Errorf("count mismatch: expected %d, found %d", bt.count, len(node.Keys))
	}

	// Check keys and entries arrays have same length
	if len(node.Keys) != len(node.Entries) {
		return fmt.Errorf("keys and entries length mismatch: %d vs %d",
			len(node.Keys), len(node.Entries))
	}

	return nil
}

// Rebuild rebuilds the entire index from scratch
func (bt *BTreeIndex) Rebuild(entries map[string]IndexEntry) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	// Clear existing data
	bt.root.Keys = bt.root.Keys[:0]
	bt.root.Entries = bt.root.Entries[:0]
	bt.count = 0

	// Convert to sorted slice
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	// Add all entries in sorted order
	for _, key := range keys {
		bt.root.Keys = append(bt.root.Keys, key)
		bt.root.Entries = append(bt.root.Entries, entries[key])
		bt.count++
	}

	return nil
}

// Save persists the B-tree index to a file (placeholder implementation)
func (bt *BTreeIndex) Save(filename string) error {
	// TODO: Implement B-tree specific serialization
	// For now, use a simple approach similar to hash index
	return fmt.Errorf("b-tree persistence not yet implemented")
}

// Load restores the B-tree index from a file (placeholder implementation)
func (bt *BTreeIndex) Load(filename string) error {
	// TODO: Implement B-tree specific deserialization
	return fmt.Errorf("b-tree persistence not yet implemented")
}

// Close cleans up resources
func (bt *BTreeIndex) Close() error {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	// Clear all data
	bt.root.Keys = nil
	bt.root.Entries = nil
	bt.root.Children = nil
	bt.root = nil
	bt.count = 0

	return nil
}
