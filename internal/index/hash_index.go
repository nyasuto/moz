package index

import (
	"encoding/gob"
	"fmt"
	"hash/fnv"
	"os"
	"sort"
	"strings"
	"sync"
)

// HashIndexConfig holds configuration for hash index
type HashIndexConfig struct {
	InitialBuckets int     // Initial number of buckets
	LoadFactor     float64 // Maximum load factor before resize
	GrowthFactor   int     // Factor by which to grow buckets
}

// DefaultHashIndexConfig returns sensible defaults for hash index
func DefaultHashIndexConfig() HashIndexConfig {
	return HashIndexConfig{
		InitialBuckets: 1024,
		LoadFactor:     0.75,
		GrowthFactor:   2,
	}
}

// HashEntry represents a single entry in a hash bucket
type HashEntry struct {
	Key   string
	Entry IndexEntry
}

// HashBucket represents a bucket in the hash table (using chaining for collision resolution)
type HashBucket struct {
	entries []HashEntry
	mu      sync.RWMutex
}

// HashIndex implements a hash table based index
type HashIndex struct {
	buckets   []HashBucket
	config    HashIndexConfig
	count     int64
	mu        sync.RWMutex
	entryPool IndexEntryPool // Optional memory pool for IndexEntry objects
}

// NewHashIndex creates a new hash index with the given configuration
func NewHashIndex(config HashIndexConfig) (*HashIndex, error) {
	return NewHashIndexWithPool(config, nil)
}

// NewHashIndexWithPool creates a new hash index with optional memory pool
func NewHashIndexWithPool(config HashIndexConfig, entryPool IndexEntryPool) (*HashIndex, error) {
	if config.InitialBuckets <= 0 {
		return nil, fmt.Errorf("initial buckets must be positive")
	}
	if config.LoadFactor <= 0 || config.LoadFactor >= 1 {
		return nil, fmt.Errorf("load factor must be between 0 and 1")
	}
	if config.GrowthFactor < 2 {
		return nil, fmt.Errorf("growth factor must be at least 2")
	}

	hi := &HashIndex{
		buckets:   make([]HashBucket, config.InitialBuckets),
		config:    config,
		count:     0,
		entryPool: entryPool,
	}

	// Initialize bucket mutexes
	for i := range hi.buckets {
		hi.buckets[i].entries = make([]HashEntry, 0, 4) // Pre-allocate small capacity
	}

	return hi, nil
}

// SetEntryPool sets the memory pool for IndexEntry objects
func (hi *HashIndex) SetEntryPool(pool IndexEntryPool) {
	hi.mu.Lock()
	defer hi.mu.Unlock()
	hi.entryPool = pool
}

// hash computes hash value for a given key
func (hi *HashIndex) hash(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}

// getBucketIndex returns the bucket index for a given key
func (hi *HashIndex) getBucketIndex(key string) int {
	return int(hi.hash(key) % uint32(len(hi.buckets)))
}

// Insert adds an entry to the hash index
func (hi *HashIndex) Insert(key string, entry IndexEntry) error {
	hi.mu.Lock()
	defer hi.mu.Unlock()

	// Check if we need to resize
	if hi.shouldResize() {
		if err := hi.resize(); err != nil {
			return fmt.Errorf("failed to resize hash index: %w", err)
		}
	}

	bucketIdx := hi.getBucketIndex(key)
	bucket := &hi.buckets[bucketIdx]

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	// Check if key already exists and update
	for i, he := range bucket.entries {
		if he.Key == key {
			bucket.entries[i].Entry = entry
			return nil
		}
	}

	// Add new entry
	bucket.entries = append(bucket.entries, HashEntry{
		Key:   key,
		Entry: entry,
	})
	hi.count++

	return nil
}

// Delete removes an entry from the hash index
func (hi *HashIndex) Delete(key string) error {
	hi.mu.RLock()
	bucketIdx := hi.getBucketIndex(key)
	bucket := &hi.buckets[bucketIdx]
	hi.mu.RUnlock()

	bucket.mu.Lock()
	defer bucket.mu.Unlock()

	for i, he := range bucket.entries {
		if he.Key == key {
			// Remove entry by swapping with last and truncating
			bucket.entries[i] = bucket.entries[len(bucket.entries)-1]
			bucket.entries = bucket.entries[:len(bucket.entries)-1]

			hi.mu.Lock()
			hi.count--
			hi.mu.Unlock()
			return nil
		}
	}

	return fmt.Errorf("key not found: %s", key)
}

// Get retrieves an entry from the hash index
func (hi *HashIndex) Get(key string) (IndexEntry, error) {
	hi.mu.RLock()
	bucketIdx := hi.getBucketIndex(key)
	bucket := &hi.buckets[bucketIdx]
	hi.mu.RUnlock()

	bucket.mu.RLock()
	defer bucket.mu.RUnlock()

	for _, he := range bucket.entries {
		if he.Key == key {
			return he.Entry, nil
		}
	}

	return IndexEntry{}, fmt.Errorf("key not found: %s", key)
}

// Exists checks if a key exists in the hash index
func (hi *HashIndex) Exists(key string) bool {
	_, err := hi.Get(key)
	return err == nil
}

// BatchInsert adds multiple entries efficiently
func (hi *HashIndex) BatchInsert(entries map[string]IndexEntry) error {
	for key, entry := range entries {
		if err := hi.Insert(key, entry); err != nil {
			return fmt.Errorf("failed to insert key %s: %w", key, err)
		}
	}
	return nil
}

// BatchDelete removes multiple entries efficiently
func (hi *HashIndex) BatchDelete(keys []string) error {
	for _, key := range keys {
		if err := hi.Delete(key); err != nil {
			return fmt.Errorf("failed to delete key %s: %w", key, err)
		}
	}
	return nil
}

// Keys returns all keys in sorted order
func (hi *HashIndex) Keys() []string {
	hi.mu.RLock()
	defer hi.mu.RUnlock()

	keys := make([]string, 0, hi.count)

	for i := range hi.buckets {
		bucket := &hi.buckets[i]
		bucket.mu.RLock()
		for _, he := range bucket.entries {
			keys = append(keys, he.Key)
		}
		bucket.mu.RUnlock()
	}

	sort.Strings(keys)
	return keys
}

// KeysWithPrealloc returns all keys using pre-allocated slice for better memory efficiency
func (hi *HashIndex) KeysWithPrealloc(keys []string) []string {
	hi.mu.RLock()
	defer hi.mu.RUnlock()

	// Reset slice while preserving capacity
	keys = keys[:0]

	// Ensure we have enough capacity
	if cap(keys) < int(hi.count) {
		keys = make([]string, 0, hi.count)
	}

	for i := range hi.buckets {
		bucket := &hi.buckets[i]
		bucket.mu.RLock()
		for _, he := range bucket.entries {
			keys = append(keys, he.Key)
		}
		bucket.mu.RUnlock()
	}

	sort.Strings(keys)
	return keys
}

// Range returns entries in the specified key range [start, end]
// Note: Hash indexes are not optimal for range queries, but we provide basic support
func (hi *HashIndex) Range(start, end string) ([]IndexEntry, error) {
	hi.mu.RLock()
	defer hi.mu.RUnlock()

	var entries []IndexEntry

	for i := range hi.buckets {
		bucket := &hi.buckets[i]
		bucket.mu.RLock()
		for _, he := range bucket.entries {
			if he.Key >= start && he.Key <= end {
				entries = append(entries, he.Entry)
			}
		}
		bucket.mu.RUnlock()
	}

	// Sort by key since hash order is not meaningful
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})

	return entries, nil
}

// Prefix returns entries with the specified key prefix
func (hi *HashIndex) Prefix(prefix string) ([]IndexEntry, error) {
	hi.mu.RLock()
	defer hi.mu.RUnlock()

	var entries []IndexEntry

	for i := range hi.buckets {
		bucket := &hi.buckets[i]
		bucket.mu.RLock()
		for _, he := range bucket.entries {
			if strings.HasPrefix(he.Key, prefix) {
				entries = append(entries, he.Entry)
			}
		}
		bucket.mu.RUnlock()
	}

	// Sort by key
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Key < entries[j].Key
	})

	return entries, nil
}

// Size returns the number of entries in the index
func (hi *HashIndex) Size() int64 {
	hi.mu.RLock()
	defer hi.mu.RUnlock()
	return hi.count
}

// MemoryUsage estimates memory usage in bytes
func (hi *HashIndex) MemoryUsage() int64 {
	hi.mu.RLock()
	defer hi.mu.RUnlock()

	// Base structure size
	size := int64(len(hi.buckets)) * 64 // Approximate bucket overhead

	// Entry data size
	for i := range hi.buckets {
		bucket := &hi.buckets[i]
		bucket.mu.RLock()
		for _, he := range bucket.entries {
			size += int64(len(he.Key)) + 64 // Key + IndexEntry struct size
		}
		bucket.mu.RUnlock()
	}

	return size
}

// Validate checks the integrity of the hash index
func (hi *HashIndex) Validate() error {
	hi.mu.RLock()
	defer hi.mu.RUnlock()

	actualCount := int64(0)

	for i := range hi.buckets {
		bucket := &hi.buckets[i]
		bucket.mu.RLock()
		actualCount += int64(len(bucket.entries))

		// Check for duplicate keys within bucket
		seen := make(map[string]bool)
		for _, he := range bucket.entries {
			if seen[he.Key] {
				bucket.mu.RUnlock()
				return fmt.Errorf("duplicate key found in bucket %d: %s", i, he.Key)
			}
			seen[he.Key] = true

			// Verify key hashes to correct bucket
			expectedBucket := hi.getBucketIndex(he.Key)
			if expectedBucket != i {
				bucket.mu.RUnlock()
				return fmt.Errorf("key %s in wrong bucket: expected %d, found %d",
					he.Key, expectedBucket, i)
			}
		}
		bucket.mu.RUnlock()
	}

	if actualCount != hi.count {
		return fmt.Errorf("count mismatch: expected %d, found %d", hi.count, actualCount)
	}

	return nil
}

// Rebuild rebuilds the entire index from scratch
func (hi *HashIndex) Rebuild(entries map[string]IndexEntry) error {
	hi.mu.Lock()
	defer hi.mu.Unlock()

	// Clear all buckets
	for i := range hi.buckets {
		hi.buckets[i].entries = hi.buckets[i].entries[:0]
	}
	hi.count = 0

	// Re-insert all entries
	for key, entry := range entries {
		bucketIdx := hi.getBucketIndex(key)
		bucket := &hi.buckets[bucketIdx]

		bucket.entries = append(bucket.entries, HashEntry{
			Key:   key,
			Entry: entry,
		})
		hi.count++
	}

	return nil
}

// shouldResize checks if the hash table should be resized
func (hi *HashIndex) shouldResize() bool {
	currentLoad := float64(hi.count) / float64(len(hi.buckets))
	return currentLoad > hi.config.LoadFactor
}

// resize grows the hash table
func (hi *HashIndex) resize() error {
	oldBuckets := hi.buckets
	newSize := len(hi.buckets) * hi.config.GrowthFactor

	// Create new buckets
	hi.buckets = make([]HashBucket, newSize)
	for i := range hi.buckets {
		hi.buckets[i].entries = make([]HashEntry, 0, 4)
	}

	// Reset count
	oldCount := hi.count
	hi.count = 0

	// Rehash all entries
	for i := range oldBuckets {
		bucket := &oldBuckets[i]
		for _, he := range bucket.entries {
			newBucketIdx := hi.getBucketIndex(he.Key)
			newBucket := &hi.buckets[newBucketIdx]
			newBucket.entries = append(newBucket.entries, he)
			hi.count++
		}
	}

	if hi.count != oldCount {
		return fmt.Errorf("count mismatch during resize: expected %d, got %d", oldCount, hi.count)
	}

	return nil
}

// Save persists the hash index to a file
func (hi *HashIndex) Save(filename string) error {
	hi.mu.RLock()
	defer hi.mu.RUnlock()

	// Validate filename to prevent directory traversal
	if err := validateFilePath(filename); err != nil {
		return fmt.Errorf("invalid filename: %w", err)
	}

	file, err := os.Create(filename) // #nosec G304 - filename validated above
	if err != nil {
		return fmt.Errorf("failed to create index file: %w", err)
	}
	defer func() { _ = file.Close() }()

	encoder := gob.NewEncoder(file)

	// Save configuration and metadata
	if err := encoder.Encode(hi.config); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	if err := encoder.Encode(hi.count); err != nil {
		return fmt.Errorf("failed to encode count: %w", err)
	}

	// Save all entries
	allEntries := make(map[string]IndexEntry)
	for i := range hi.buckets {
		bucket := &hi.buckets[i]
		bucket.mu.RLock()
		for _, he := range bucket.entries {
			allEntries[he.Key] = he.Entry
		}
		bucket.mu.RUnlock()
	}

	if err := encoder.Encode(allEntries); err != nil {
		return fmt.Errorf("failed to encode entries: %w", err)
	}

	return nil
}

// Load restores the hash index from a file
func (hi *HashIndex) Load(filename string) error {
	// Validate filename to prevent directory traversal
	if err := validateFilePath(filename); err != nil {
		return fmt.Errorf("invalid filename: %w", err)
	}

	file, err := os.Open(filename) // #nosec G304 - filename validated above
	if err != nil {
		return fmt.Errorf("failed to open index file: %w", err)
	}
	defer func() { _ = file.Close() }()

	decoder := gob.NewDecoder(file)

	// Load configuration and metadata
	var config HashIndexConfig
	if err := decoder.Decode(&config); err != nil {
		return fmt.Errorf("failed to decode config: %w", err)
	}

	var count int64
	if err := decoder.Decode(&count); err != nil {
		return fmt.Errorf("failed to decode count: %w", err)
	}

	// Load entries
	var entries map[string]IndexEntry
	if err := decoder.Decode(&entries); err != nil {
		return fmt.Errorf("failed to decode entries: %w", err)
	}

	// Rebuild index with loaded data
	hi.config = config
	return hi.Rebuild(entries)
}

// Close cleans up resources
func (hi *HashIndex) Close() error {
	hi.mu.Lock()
	defer hi.mu.Unlock()

	// Clear all data
	for i := range hi.buckets {
		hi.buckets[i].entries = nil
	}
	hi.buckets = nil
	hi.count = 0

	return nil
}
