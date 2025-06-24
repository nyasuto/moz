package lsm

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// SSTable represents a Sorted String Table on disk
type SSTable struct {
	mu sync.RWMutex

	// Identification and metadata
	ID       string
	Level    int
	FilePath string
	DataDir  string

	// File handles
	dataFile  *os.File
	indexFile *os.File

	// Metadata
	metadata SSTableMetadata
	index    []IndexEntry

	// State
	finalized bool
	closed    bool

	// Statistics
	NumEntries uint64
	FileSize   int64
}

// SSTableMetadata contains metadata about an SSTable
type SSTableMetadata struct {
	Version    uint32
	Level      int
	NumEntries uint64
	FileSize   int64
	MinKey     string
	MaxKey     string
	CreatedAt  int64
	Checksum   uint32
}

// IndexEntry represents an entry in the SSTable index
type IndexEntry struct {
	Key    string
	Offset int64
	Length int32
}

// SSTableEntry represents a single entry in the SSTable
type SSTableEntry struct {
	Key       string
	Value     string
	Deleted   bool
	Timestamp int64
	Checksum  uint32
}

const (
	SSTableVersion = 1
	IndexEntrySize = 8 + 4 // offset (8 bytes) + length (4 bytes)
)

// NewSSTable creates a new SSTable for writing
func NewSSTable(id, dataDir string, level int) (*SSTable, error) {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	sstable := &SSTable{
		ID:      id,
		Level:   level,
		DataDir: dataDir,
		metadata: SSTableMetadata{
			Version:   SSTableVersion,
			Level:     level,
			CreatedAt: int64(0), // Will be set when finalized
		},
		index: make([]IndexEntry, 0),
	}

	// Create file paths
	sstable.FilePath = filepath.Join(dataDir, fmt.Sprintf("%s.sst", id))
	indexPath := filepath.Join(dataDir, fmt.Sprintf("%s.idx", id))

	// Open data file for writing
	dataFile, err := os.Create(sstable.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create data file: %w", err)
	}
	sstable.dataFile = dataFile

	// Open index file for writing
	indexFile, err := os.Create(indexPath) // #nosec G304 - indexPath is safely constructed with filepath.Join
	if err != nil {
		_ = dataFile.Close()
		return nil, fmt.Errorf("failed to create index file: %w", err)
	}
	sstable.indexFile = indexFile

	// Write SSTable header
	if err := sstable.writeHeader(); err != nil {
		sstable.cleanup()
		return nil, fmt.Errorf("failed to write header: %w", err)
	}

	return sstable, nil
}

// OpenSSTable opens an existing SSTable for reading
func OpenSSTable(id, dataDir string) (*SSTable, error) {
	sstable := &SSTable{
		ID:        id,
		DataDir:   dataDir,
		finalized: true,
	}

	// Set file paths
	sstable.FilePath = filepath.Join(dataDir, fmt.Sprintf("%s.sst", id))
	indexPath := filepath.Join(dataDir, fmt.Sprintf("%s.idx", id))

	// Open data file for reading
	dataFile, err := os.Open(sstable.FilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open data file: %w", err)
	}
	sstable.dataFile = dataFile

	// Open index file for reading
	indexFile, err := os.Open(indexPath) // #nosec G304 - indexPath is safely constructed with filepath.Join
	if err != nil {
		_ = dataFile.Close()
		return nil, fmt.Errorf("failed to open index file: %w", err)
	}
	sstable.indexFile = indexFile

	// Read metadata and index
	if err := sstable.readMetadata(); err != nil {
		sstable.cleanup()
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	if err := sstable.readIndex(); err != nil {
		sstable.cleanup()
		return nil, fmt.Errorf("failed to read index: %w", err)
	}

	// Get file size
	if stat, err := sstable.dataFile.Stat(); err == nil {
		sstable.FileSize = stat.Size()
	}

	return sstable, nil
}

// writeHeader writes the SSTable header
func (sst *SSTable) writeHeader() error {
	// Write version
	if err := binary.Write(sst.dataFile, binary.LittleEndian, uint32(SSTableVersion)); err != nil {
		return err
	}

	// Write level
	if err := binary.Write(sst.dataFile, binary.LittleEndian, int32(sst.Level)); err != nil {
		return err
	}

	// Reserve space for metadata (will be written during finalization)
	metadataSize := int64(64) // Reserved space for metadata
	if _, err := sst.dataFile.Seek(metadataSize, io.SeekCurrent); err != nil {
		return err
	}

	return nil
}

// Put adds a key-value pair to the SSTable
func (sst *SSTable) Put(key, value string, deleted bool) error {
	if sst.finalized {
		return fmt.Errorf("cannot write to finalized SSTable")
	}

	sst.mu.Lock()
	defer sst.mu.Unlock()

	// Get current position
	offset, err := sst.dataFile.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("failed to get file position: %w", err)
	}

	// Create entry
	entry := SSTableEntry{
		Key:       key,
		Value:     value,
		Deleted:   deleted,
		Timestamp: 0, // Will be set from MemTable
	}

	// Calculate checksum
	entry.Checksum = sst.calculateChecksum(&entry)

	// Serialize and write entry
	entryData, err := sst.serializeEntry(&entry)
	if err != nil {
		return fmt.Errorf("failed to serialize entry: %w", err)
	}

	bytesWritten, err := sst.dataFile.Write(entryData)
	if err != nil {
		return fmt.Errorf("failed to write entry: %w", err)
	}

	// Add to index
	indexEntry := IndexEntry{
		Key:    key,
		Offset: offset,
		Length: int32(bytesWritten),
	}
	sst.index = append(sst.index, indexEntry)

	// Update metadata
	sst.NumEntries++
	if sst.metadata.MinKey == "" || key < sst.metadata.MinKey {
		sst.metadata.MinKey = key
	}
	if sst.metadata.MaxKey == "" || key > sst.metadata.MaxKey {
		sst.metadata.MaxKey = key
	}

	return nil
}

// serializeEntry serializes an SSTable entry to bytes
func (sst *SSTable) serializeEntry(entry *SSTableEntry) ([]byte, error) {
	keyLen := len(entry.Key)
	valueLen := len(entry.Value)

	// Calculate total size
	totalSize := 4 + // key length
		keyLen + // key
		4 + // value length
		valueLen + // value
		1 + // deleted flag
		8 + // timestamp
		4 // checksum

	data := make([]byte, totalSize)
	offset := 0

	// Write key length and key
	binary.LittleEndian.PutUint32(data[offset:], uint32(keyLen))
	offset += 4
	copy(data[offset:], entry.Key)
	offset += keyLen

	// Write value length and value
	binary.LittleEndian.PutUint32(data[offset:], uint32(valueLen))
	offset += 4
	copy(data[offset:], entry.Value)
	offset += valueLen

	// Write deleted flag
	if entry.Deleted {
		data[offset] = 1
	} else {
		data[offset] = 0
	}
	offset++

	// Write timestamp
	binary.LittleEndian.PutUint64(data[offset:], uint64(entry.Timestamp))
	offset += 8

	// Write checksum
	binary.LittleEndian.PutUint32(data[offset:], entry.Checksum)

	return data, nil
}

// calculateChecksum calculates CRC32 checksum for an entry
func (sst *SSTable) calculateChecksum(entry *SSTableEntry) uint32 {
	hasher := crc32.NewIEEE()
	hasher.Write([]byte(entry.Key))
	hasher.Write([]byte(entry.Value))
	if entry.Deleted {
		hasher.Write([]byte{1})
	} else {
		hasher.Write([]byte{0})
	}
	_ = binary.Write(hasher, binary.LittleEndian, entry.Timestamp)
	return hasher.Sum32()
}

// Get retrieves a value for a key from the SSTable
func (sst *SSTable) Get(key string) (string, bool, error) {
	if !sst.finalized {
		return "", false, fmt.Errorf("SSTable not finalized")
	}

	sst.mu.RLock()
	defer sst.mu.RUnlock()

	// Binary search in index
	idx := sort.Search(len(sst.index), func(i int) bool {
		return sst.index[i].Key >= key
	})

	if idx >= len(sst.index) || sst.index[idx].Key != key {
		return "", false, nil // Key not found
	}

	// Read entry from file
	indexEntry := sst.index[idx]
	entry, err := sst.readEntryAt(indexEntry.Offset, indexEntry.Length)
	if err != nil {
		return "", false, fmt.Errorf("failed to read entry: %w", err)
	}

	if entry.Deleted {
		return "", false, nil // Deleted entry
	}

	return entry.Value, true, nil
}

// readEntryAt reads an entry at a specific offset
func (sst *SSTable) readEntryAt(offset int64, length int32) (*SSTableEntry, error) {
	// Read entry data
	data := make([]byte, length)
	if _, err := sst.dataFile.ReadAt(data, offset); err != nil {
		return nil, err
	}

	// Deserialize entry
	return sst.deserializeEntry(data)
}

// deserializeEntry deserializes bytes to an SSTable entry
func (sst *SSTable) deserializeEntry(data []byte) (*SSTableEntry, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("invalid entry data")
	}

	offset := 0
	entry := &SSTableEntry{}

	// Read key length and key
	keyLen := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(keyLen) > len(data) {
		return nil, fmt.Errorf("invalid key length")
	}
	entry.Key = string(data[offset : offset+int(keyLen)])
	offset += int(keyLen)

	// Read value length and value
	if offset+4 > len(data) {
		return nil, fmt.Errorf("invalid value length position")
	}
	valueLen := binary.LittleEndian.Uint32(data[offset:])
	offset += 4
	if offset+int(valueLen) > len(data) {
		return nil, fmt.Errorf("invalid value length")
	}
	entry.Value = string(data[offset : offset+int(valueLen)])
	offset += int(valueLen)

	// Read deleted flag
	if offset >= len(data) {
		return nil, fmt.Errorf("missing deleted flag")
	}
	entry.Deleted = data[offset] == 1
	offset++

	// Read timestamp
	if offset+8 > len(data) {
		return nil, fmt.Errorf("missing timestamp")
	}
	entry.Timestamp = int64(binary.LittleEndian.Uint64(data[offset:]))
	offset += 8

	// Read checksum
	if offset+4 > len(data) {
		return nil, fmt.Errorf("missing checksum")
	}
	entry.Checksum = binary.LittleEndian.Uint32(data[offset:])

	// Verify checksum
	expectedChecksum := sst.calculateChecksum(entry)
	if entry.Checksum != expectedChecksum {
		return nil, fmt.Errorf("checksum mismatch")
	}

	return entry, nil
}

// ContainsKey checks if the SSTable might contain a key based on key range
func (sst *SSTable) ContainsKey(key string) bool {
	if !sst.finalized {
		return false
	}
	return key >= sst.metadata.MinKey && key <= sst.metadata.MaxKey
}

// GetAllKeys returns all keys in the SSTable
func (sst *SSTable) GetAllKeys() ([]string, error) {
	if !sst.finalized {
		return nil, fmt.Errorf("SSTable not finalized")
	}

	sst.mu.RLock()
	defer sst.mu.RUnlock()

	keys := make([]string, len(sst.index))
	for i, entry := range sst.index {
		keys[i] = entry.Key
	}

	return keys, nil
}

// Finalize finalizes the SSTable by writing metadata and index
func (sst *SSTable) Finalize() error {
	if sst.finalized {
		return fmt.Errorf("SSTable already finalized")
	}

	sst.mu.Lock()
	defer sst.mu.Unlock()

	// Sort index by key
	sort.Slice(sst.index, func(i, j int) bool {
		return sst.index[i].Key < sst.index[j].Key
	})

	// Update metadata
	sst.metadata.NumEntries = sst.NumEntries
	if stat, err := sst.dataFile.Stat(); err == nil {
		sst.metadata.FileSize = stat.Size()
		sst.FileSize = stat.Size()
	}

	// Write index to index file
	if err := sst.writeIndex(); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}

	// Write metadata to data file
	if err := sst.writeMetadata(); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	// Sync files
	if err := sst.dataFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync data file: %w", err)
	}
	if err := sst.indexFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync index file: %w", err)
	}

	sst.finalized = true
	return nil
}

// writeIndex writes the index to the index file
func (sst *SSTable) writeIndex() error {
	// Write number of entries
	if err := binary.Write(sst.indexFile, binary.LittleEndian, uint64(len(sst.index))); err != nil {
		return err
	}

	// Write each index entry
	for _, entry := range sst.index {
		// Write key length and key
		keyLen := uint32(len(entry.Key))
		if err := binary.Write(sst.indexFile, binary.LittleEndian, keyLen); err != nil {
			return err
		}
		if _, err := sst.indexFile.WriteString(entry.Key); err != nil {
			return err
		}

		// Write offset and length
		if err := binary.Write(sst.indexFile, binary.LittleEndian, entry.Offset); err != nil {
			return err
		}
		if err := binary.Write(sst.indexFile, binary.LittleEndian, entry.Length); err != nil {
			return err
		}
	}

	return nil
}

// writeMetadata writes metadata to the data file header
func (sst *SSTable) writeMetadata() error {
	// Seek to metadata position (after version and level)
	if _, err := sst.dataFile.Seek(8, io.SeekStart); err != nil {
		return err
	}

	// Write metadata
	if err := binary.Write(sst.dataFile, binary.LittleEndian, sst.metadata.NumEntries); err != nil {
		return err
	}
	if err := binary.Write(sst.dataFile, binary.LittleEndian, sst.metadata.FileSize); err != nil {
		return err
	}

	// Write min/max keys with length prefix
	minKeyLen := uint32(len(sst.metadata.MinKey))
	if err := binary.Write(sst.dataFile, binary.LittleEndian, minKeyLen); err != nil {
		return err
	}
	if _, err := sst.dataFile.WriteString(sst.metadata.MinKey); err != nil {
		return err
	}

	maxKeyLen := uint32(len(sst.metadata.MaxKey))
	if err := binary.Write(sst.dataFile, binary.LittleEndian, maxKeyLen); err != nil {
		return err
	}
	if _, err := sst.dataFile.WriteString(sst.metadata.MaxKey); err != nil {
		return err
	}

	return nil
}

// readMetadata reads metadata from the data file header
func (sst *SSTable) readMetadata() error {
	// Seek to beginning
	if _, err := sst.dataFile.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// Read version
	var version uint32
	if err := binary.Read(sst.dataFile, binary.LittleEndian, &version); err != nil {
		return err
	}
	sst.metadata.Version = version

	// Read level
	var level int32
	if err := binary.Read(sst.dataFile, binary.LittleEndian, &level); err != nil {
		return err
	}
	sst.metadata.Level = int(level)
	sst.Level = int(level)

	// Read metadata
	if err := binary.Read(sst.dataFile, binary.LittleEndian, &sst.metadata.NumEntries); err != nil {
		return err
	}
	sst.NumEntries = sst.metadata.NumEntries

	if err := binary.Read(sst.dataFile, binary.LittleEndian, &sst.metadata.FileSize); err != nil {
		return err
	}

	// Read min key
	var minKeyLen uint32
	if err := binary.Read(sst.dataFile, binary.LittleEndian, &minKeyLen); err != nil {
		return err
	}
	minKeyBytes := make([]byte, minKeyLen)
	if _, err := io.ReadFull(sst.dataFile, minKeyBytes); err != nil {
		return err
	}
	sst.metadata.MinKey = string(minKeyBytes)

	// Read max key
	var maxKeyLen uint32
	if err := binary.Read(sst.dataFile, binary.LittleEndian, &maxKeyLen); err != nil {
		return err
	}
	maxKeyBytes := make([]byte, maxKeyLen)
	if _, err := io.ReadFull(sst.dataFile, maxKeyBytes); err != nil {
		return err
	}
	sst.metadata.MaxKey = string(maxKeyBytes)

	return nil
}

// readIndex reads the index from the index file
func (sst *SSTable) readIndex() error {
	// Seek to beginning of index file
	if _, err := sst.indexFile.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// Read number of entries
	var numEntries uint64
	if err := binary.Read(sst.indexFile, binary.LittleEndian, &numEntries); err != nil {
		return err
	}

	// Read index entries
	sst.index = make([]IndexEntry, numEntries)
	for i := uint64(0); i < numEntries; i++ {
		// Read key length and key
		var keyLen uint32
		if err := binary.Read(sst.indexFile, binary.LittleEndian, &keyLen); err != nil {
			return err
		}
		keyBytes := make([]byte, keyLen)
		if _, err := io.ReadFull(sst.indexFile, keyBytes); err != nil {
			return err
		}

		// Read offset and length
		var offset int64
		var length int32
		if err := binary.Read(sst.indexFile, binary.LittleEndian, &offset); err != nil {
			return err
		}
		if err := binary.Read(sst.indexFile, binary.LittleEndian, &length); err != nil {
			return err
		}

		sst.index[i] = IndexEntry{
			Key:    string(keyBytes),
			Offset: offset,
			Length: length,
		}
	}

	return nil
}

// Close closes the SSTable files
func (sst *SSTable) Close() error {
	sst.mu.Lock()
	defer sst.mu.Unlock()

	if sst.closed {
		return nil
	}

	var errors []string

	if sst.dataFile != nil {
		if err := sst.dataFile.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("data file: %v", err))
		}
	}

	if sst.indexFile != nil {
		if err := sst.indexFile.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("index file: %v", err))
		}
	}

	sst.closed = true

	if len(errors) > 0 {
		return fmt.Errorf("errors closing SSTable: %s", strings.Join(errors, ", "))
	}

	return nil
}

// cleanup removes created files and closes handles
func (sst *SSTable) cleanup() {
	if sst.dataFile != nil {
		_ = sst.dataFile.Close()
		_ = os.Remove(sst.FilePath)
	}
	if sst.indexFile != nil {
		_ = sst.indexFile.Close()
		indexPath := filepath.Join(sst.DataDir, fmt.Sprintf("%s.idx", sst.ID))
		_ = os.Remove(indexPath)
	}
}

// Iterator returns an iterator for the SSTable
func (sst *SSTable) Iterator() *SSTableIterator {
	return &SSTableIterator{
		sstable: sst,
		index:   0,
	}
}

// SSTableIterator provides iteration over SSTable entries
type SSTableIterator struct {
	sstable *SSTable
	index   int
	current *SSTableEntry
}

// HasNext returns true if there are more entries
func (it *SSTableIterator) HasNext() bool {
	return it.index < len(it.sstable.index)
}

// Next advances to the next entry
func (it *SSTableIterator) Next() (*SSTableEntry, error) {
	if !it.HasNext() {
		return nil, fmt.Errorf("no more entries")
	}

	indexEntry := it.sstable.index[it.index]
	entry, err := it.sstable.readEntryAt(indexEntry.Offset, indexEntry.Length)
	if err != nil {
		return nil, err
	}

	it.current = entry
	it.index++
	return entry, nil
}

// Current returns the current entry
func (it *SSTableIterator) Current() *SSTableEntry {
	return it.current
}

// Reset resets the iterator to the beginning
func (it *SSTableIterator) Reset() {
	it.index = 0
	it.current = nil
}
