package index

import "fmt"

// NoIndex is a no-op implementation of the Index interface
// Used when indexing is disabled (IndexTypeNone)
type NoIndex struct{}

// NewNoIndex creates a new no-op index
func NewNoIndex() *NoIndex {
	return &NoIndex{}
}

// Insert is a no-op
func (ni *NoIndex) Insert(key string, entry IndexEntry) error {
	return nil
}

// Delete is a no-op
func (ni *NoIndex) Delete(key string) error {
	return nil
}

// Get always returns an error since no indexing is available
func (ni *NoIndex) Get(key string) (IndexEntry, error) {
	return IndexEntry{}, fmt.Errorf("index disabled: no index available")
}

// Exists always returns false
func (ni *NoIndex) Exists(key string) bool {
	return false
}

// BatchInsert is a no-op
func (ni *NoIndex) BatchInsert(entries map[string]IndexEntry) error {
	return nil
}

// BatchDelete is a no-op
func (ni *NoIndex) BatchDelete(keys []string) error {
	return nil
}

// Keys returns an empty slice
func (ni *NoIndex) Keys() []string {
	return []string{}
}

// Range returns an error since range queries require indexing
func (ni *NoIndex) Range(start, end string) ([]IndexEntry, error) {
	return nil, fmt.Errorf("index disabled: range queries not available")
}

// Prefix returns an error since prefix searches require indexing
func (ni *NoIndex) Prefix(prefix string) ([]IndexEntry, error) {
	return nil, fmt.Errorf("index disabled: prefix searches not available")
}

// Size returns 0
func (ni *NoIndex) Size() int64 {
	return 0
}

// MemoryUsage returns 0
func (ni *NoIndex) MemoryUsage() int64 {
	return 0
}

// Validate is a no-op
func (ni *NoIndex) Validate() error {
	return nil
}

// Rebuild is a no-op
func (ni *NoIndex) Rebuild(entries map[string]IndexEntry) error {
	return nil
}

// Save is a no-op
func (ni *NoIndex) Save(filename string) error {
	return nil
}

// Load is a no-op
func (ni *NoIndex) Load(filename string) error {
	return nil
}

// Close is a no-op
func (ni *NoIndex) Close() error {
	return nil
}
