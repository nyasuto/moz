package kvstore

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"time"
)

// BinaryEntry represents a single entry in binary format
type BinaryEntry struct {
	Timestamp   uint64 // 8 bytes - Unix nanoseconds
	Operation   uint8  // 1 byte  - PUT(1)/DELETE(2)
	KeyLength   uint16 // 2 bytes - Key length
	ValueLength uint32 // 4 bytes - Value length
	Key         []byte // variable - Key data
	Value       []byte // variable - Value data
	Checksum    uint32 // 4 bytes - CRC32 checksum
}

// Operation types
const (
	BinaryOpPut    uint8 = 1
	BinaryOpDelete uint8 = 2
)

// Binary format constants
const (
	BinaryHeaderSize = 19 // Fixed header size: 8+1+2+4+4 bytes
	BinaryMagicSize  = 4  // Magic number size
)

var (
	BinaryMagicNumber = [4]byte{'M', 'O', 'Z', 'B'} // "MOZB" magic number
)

// NewBinaryEntry creates a new binary entry
func NewBinaryEntry(operation uint8, key, value []byte) *BinaryEntry {
	entry := &BinaryEntry{
		Timestamp:   uint64(time.Now().UnixNano()),
		Operation:   operation,
		KeyLength:   uint16(len(key)),
		ValueLength: uint32(len(value)),
		Key:         make([]byte, len(key)),
		Value:       make([]byte, len(value)),
	}

	copy(entry.Key, key)
	copy(entry.Value, value)

	// Calculate checksum
	entry.Checksum = entry.calculateChecksum()

	return entry
}

// calculateChecksum calculates CRC32 checksum for the entry
func (e *BinaryEntry) calculateChecksum() uint32 {
	crc := crc32.NewIEEE()

	// Include all fields except checksum in calculation
	binary.Write(crc, binary.LittleEndian, e.Timestamp)
	binary.Write(crc, binary.LittleEndian, e.Operation)
	binary.Write(crc, binary.LittleEndian, e.KeyLength)
	binary.Write(crc, binary.LittleEndian, e.ValueLength)
	crc.Write(e.Key)
	crc.Write(e.Value)

	return crc.Sum32()
}

// Verify checks if the entry's checksum is valid
func (e *BinaryEntry) Verify() error {
	expectedChecksum := e.calculateChecksum()
	if e.Checksum != expectedChecksum {
		return fmt.Errorf("checksum mismatch: expected %d, got %d", expectedChecksum, e.Checksum)
	}
	return nil
}

// Size returns the total size of the entry in bytes
func (e *BinaryEntry) Size() int {
	return BinaryHeaderSize + int(e.KeyLength) + int(e.ValueLength)
}

// IsDeleted returns true if this is a delete operation
func (e *BinaryEntry) IsDeleted() bool {
	return e.Operation == BinaryOpDelete
}

// WriteTo writes the binary entry to a writer
func (e *BinaryEntry) WriteTo(w io.Writer) (int64, error) {
	var written int64

	// Write magic number
	n, err := w.Write(BinaryMagicNumber[:])
	if err != nil {
		return written, fmt.Errorf("failed to write magic number: %w", err)
	}
	written += int64(n)

	// Write header fields
	if err := binary.Write(w, binary.LittleEndian, e.Timestamp); err != nil {
		return written, fmt.Errorf("failed to write timestamp: %w", err)
	}
	written += 8

	if err := binary.Write(w, binary.LittleEndian, e.Operation); err != nil {
		return written, fmt.Errorf("failed to write operation: %w", err)
	}
	written += 1

	if err := binary.Write(w, binary.LittleEndian, e.KeyLength); err != nil {
		return written, fmt.Errorf("failed to write key length: %w", err)
	}
	written += 2

	if err := binary.Write(w, binary.LittleEndian, e.ValueLength); err != nil {
		return written, fmt.Errorf("failed to write value length: %w", err)
	}
	written += 4

	// Write key data
	n, err = w.Write(e.Key)
	if err != nil {
		return written, fmt.Errorf("failed to write key: %w", err)
	}
	written += int64(n)

	// Write value data
	n, err = w.Write(e.Value)
	if err != nil {
		return written, fmt.Errorf("failed to write value: %w", err)
	}
	written += int64(n)

	// Write checksum
	if err := binary.Write(w, binary.LittleEndian, e.Checksum); err != nil {
		return written, fmt.Errorf("failed to write checksum: %w", err)
	}
	written += 4

	return written, nil
}

// ReadBinaryEntry reads a binary entry from a reader
func ReadBinaryEntry(r io.Reader) (*BinaryEntry, error) {
	// Read and verify magic number
	var magic [4]byte
	if _, err := io.ReadFull(r, magic[:]); err != nil {
		return nil, fmt.Errorf("failed to read magic number: %w", err)
	}

	if magic != BinaryMagicNumber {
		return nil, fmt.Errorf("invalid magic number: expected %v, got %v", BinaryMagicNumber, magic)
	}

	entry := &BinaryEntry{}

	// Read header fields
	if err := binary.Read(r, binary.LittleEndian, &entry.Timestamp); err != nil {
		return nil, fmt.Errorf("failed to read timestamp: %w", err)
	}

	if err := binary.Read(r, binary.LittleEndian, &entry.Operation); err != nil {
		return nil, fmt.Errorf("failed to read operation: %w", err)
	}

	if err := binary.Read(r, binary.LittleEndian, &entry.KeyLength); err != nil {
		return nil, fmt.Errorf("failed to read key length: %w", err)
	}

	if err := binary.Read(r, binary.LittleEndian, &entry.ValueLength); err != nil {
		return nil, fmt.Errorf("failed to read value length: %w", err)
	}

	// Read key data
	entry.Key = make([]byte, entry.KeyLength)
	if _, err := io.ReadFull(r, entry.Key); err != nil {
		return nil, fmt.Errorf("failed to read key: %w", err)
	}

	// Read value data
	entry.Value = make([]byte, entry.ValueLength)
	if _, err := io.ReadFull(r, entry.Value); err != nil {
		return nil, fmt.Errorf("failed to read value: %w", err)
	}

	// Read checksum
	if err := binary.Read(r, binary.LittleEndian, &entry.Checksum); err != nil {
		return nil, fmt.Errorf("failed to read checksum: %w", err)
	}

	// Verify checksum
	if err := entry.Verify(); err != nil {
		return nil, fmt.Errorf("entry verification failed: %w", err)
	}

	return entry, nil
}

// BinaryFormat represents configuration for binary format
type BinaryFormat struct {
	Enabled     bool   // Whether binary format is enabled
	Filename    string // Binary log file name
	Compression bool   // Enable compression (future)
}

// DefaultBinaryFormat returns default binary format configuration
func DefaultBinaryFormat() *BinaryFormat {
	return &BinaryFormat{
		Enabled:     false, // Default to text format for compatibility
		Filename:    "moz.bin",
		Compression: false,
	}
}
