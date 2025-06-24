package kvstore

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// OpType represents the type of operation in WAL
type OpType uint8

const (
	OpTypePut OpType = iota
	OpTypeDelete
	OpTypeCompaction
)

// WALEntry represents a single entry in the Write-Ahead Log
type WALEntry struct {
	LSN       uint64 // Log Sequence Number
	Timestamp int64  // Unix timestamp in nanoseconds
	Operation OpType // Type of operation
	Key       []byte // Key data
	Value     []byte // Value data (empty for delete)
	Checksum  uint32 // CRC32 checksum for integrity
}

// WAL implements Write-Ahead Logging for durability and crash recovery
type WAL struct {
	mu       sync.RWMutex
	file     *os.File
	dataDir  string
	filename string

	// WAL state
	nextLSN  uint64 // Next Log Sequence Number
	fileSize int64  // Current file size

	// Write buffer for performance
	buffer  chan *WALEntry
	flushCh chan struct{}
	errorCh chan error
	stopCh  chan struct{}
	wg      sync.WaitGroup

	// Configuration
	bufferSize   int           // Size of write buffer
	flushTimeout time.Duration // Maximum time between flushes
	maxFileSize  int64         // Maximum WAL file size before rotation

	// Statistics
	stats WALStats
}

// WALStats holds statistics about WAL operations
type WALStats struct {
	TotalEntries   uint64
	BytesWritten   uint64
	FlushCount     uint64
	ErrorCount     uint64
	LastFlushTime  time.Time
	AverageLatency time.Duration
}

// WALConfig holds configuration for WAL
type WALConfig struct {
	DataDir      string
	BufferSize   int
	FlushTimeout time.Duration
	MaxFileSize  int64
}

// DefaultWALConfig returns default WAL configuration
func DefaultWALConfig() WALConfig {
	return WALConfig{
		DataDir:      ".",
		BufferSize:   1000,
		FlushTimeout: 100 * time.Millisecond,
		MaxFileSize:  64 * 1024 * 1024, // 64MB
	}
}

// NewWAL creates a new Write-Ahead Log
func NewWAL(config WALConfig) (*WAL, error) {
	if config.DataDir == "" {
		config.DataDir = "."
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(config.DataDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create WAL directory: %w", err)
	}

	filename := filepath.Join(config.DataDir, "moz.wal")

	wal := &WAL{
		dataDir:      config.DataDir,
		filename:     filename,
		buffer:       make(chan *WALEntry, config.BufferSize),
		flushCh:      make(chan struct{}, 1),
		errorCh:      make(chan error, 10),
		stopCh:       make(chan struct{}),
		bufferSize:   config.BufferSize,
		flushTimeout: config.FlushTimeout,
		maxFileSize:  config.MaxFileSize,
	}

	// Open or create WAL file
	if err := wal.open(); err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	// Recover from existing WAL if present
	if err := wal.recover(); err != nil {
		return nil, fmt.Errorf("failed to recover from WAL: %w", err)
	}

	// Start background flush worker
	wal.wg.Add(1)
	go wal.flushWorker()

	return wal, nil
}

// open opens the WAL file for writing
func (w *WAL) open() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	file, err := os.OpenFile(w.filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	w.file = file

	// Get current file size
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	w.fileSize = fileInfo.Size()

	return nil
}

// recover reads existing WAL entries and returns the highest LSN
func (w *WAL) recover() error {
	// Open file for reading
	file, err := os.Open(w.filename)
	if err != nil {
		if os.IsNotExist(err) {
			// No existing WAL file, start fresh
			w.nextLSN = 1
			return nil
		}
		return err
	}
	defer func() {
		if err := file.Close(); err != nil {
			fmt.Printf("Warning: WAL recovery file close failed: %v\n", err)
		}
	}()

	var maxLSN uint64
	reader := &walReader{file: file}

	for {
		entry, err := reader.ReadEntry()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Log error but continue recovery
			fmt.Printf("Warning: WAL recovery error: %v\n", err)
			continue
		}

		if entry.LSN > maxLSN {
			maxLSN = entry.LSN
		}
		w.stats.TotalEntries++
	}

	// Set next LSN
	w.nextLSN = maxLSN + 1

	return nil
}

// Append adds a new entry to the WAL
func (w *WAL) Append(operation OpType, key, value []byte) (uint64, error) {
	// Create WAL entry
	lsn := atomic.AddUint64(&w.nextLSN, 1) - 1
	entry := &WALEntry{
		LSN:       lsn,
		Timestamp: time.Now().UnixNano(),
		Operation: operation,
		Key:       make([]byte, len(key)),
		Value:     make([]byte, len(value)),
	}

	copy(entry.Key, key)
	copy(entry.Value, value)

	// Calculate checksum
	entry.Checksum = w.calculateChecksum(entry)

	// Send to buffer (non-blocking)
	select {
	case w.buffer <- entry:
		return lsn, nil
	default:
		// Buffer full, this indicates high write pressure
		// In production, you might want to implement backpressure
		return 0, fmt.Errorf("WAL buffer full")
	}
}

// Flush forces all buffered entries to disk
func (w *WAL) Flush() error {
	select {
	case w.flushCh <- struct{}{}:
		// Flush signal sent
	default:
		// Flush already pending
	}

	// Wait for flush to complete by checking error channel
	select {
	case err := <-w.errorCh:
		return err
	case <-time.After(w.flushTimeout * 2):
		return fmt.Errorf("flush timeout")
	}
}

// flushWorker runs in background and flushes buffered entries
func (w *WAL) flushWorker() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.flushTimeout)
	defer ticker.Stop()

	var pendingEntries []*WALEntry

	for {
		select {
		case <-w.stopCh:
			// Final flush before shutdown
			if len(pendingEntries) > 0 {
				if err := w.flushEntries(pendingEntries); err != nil {
					fmt.Printf("Warning: final flush failed during shutdown: %v\n", err)
				}
			}
			return

		case entry := <-w.buffer:
			pendingEntries = append(pendingEntries, entry)

			// Flush if buffer is getting full
			if len(pendingEntries) >= w.bufferSize/2 {
				if err := w.flushEntries(pendingEntries); err != nil {
					w.errorCh <- err
				} else {
					w.errorCh <- nil
				}
				pendingEntries = pendingEntries[:0]
			}

		case <-w.flushCh:
			// Manual flush requested
			if len(pendingEntries) > 0 {
				if err := w.flushEntries(pendingEntries); err != nil {
					w.errorCh <- err
				} else {
					w.errorCh <- nil
				}
				pendingEntries = pendingEntries[:0]
			} else {
				// No entries to flush, but still signal completion
				w.errorCh <- nil
			}

		case <-ticker.C:
			// Periodic flush
			if len(pendingEntries) > 0 {
				if err := w.flushEntries(pendingEntries); err != nil {
					fmt.Printf("Warning: periodic flush failed: %v\n", err)
				}
				pendingEntries = pendingEntries[:0]
			}
		}
	}
}

// flushEntries writes pending entries to disk
func (w *WAL) flushEntries(entries []*WALEntry) error {
	if len(entries) == 0 {
		return nil
	}

	start := time.Now()

	w.mu.Lock()
	defer w.mu.Unlock()

	// Write all entries
	for _, entry := range entries {
		if err := w.writeEntry(entry); err != nil {
			atomic.AddUint64(&w.stats.ErrorCount, 1)
			return err
		}
	}

	// Sync to disk for durability
	if err := w.file.Sync(); err != nil {
		atomic.AddUint64(&w.stats.ErrorCount, 1)
		return err
	}

	// Update statistics
	atomic.AddUint64(&w.stats.FlushCount, 1)
	w.stats.LastFlushTime = time.Now()
	w.stats.AverageLatency = time.Since(start)

	return nil
}

// writeEntry writes a single entry to the WAL file
func (w *WAL) writeEntry(entry *WALEntry) error {
	// Calculate entry size
	entrySize := 8 + 8 + 1 + 4 + 4 + len(entry.Key) + len(entry.Value) + 4

	// Write entry header
	header := make([]byte, 25) // LSN(8) + Timestamp(8) + Operation(1) + KeyLen(4) + ValueLen(4)
	binary.LittleEndian.PutUint64(header[0:8], entry.LSN)
	binary.LittleEndian.PutUint64(header[8:16], uint64(entry.Timestamp))
	header[16] = byte(entry.Operation)
	binary.LittleEndian.PutUint32(header[17:21], uint32(len(entry.Key)))
	binary.LittleEndian.PutUint32(header[21:25], uint32(len(entry.Value)))

	if _, err := w.file.Write(header); err != nil {
		return err
	}

	// Write key and value
	if len(entry.Key) > 0 {
		if _, err := w.file.Write(entry.Key); err != nil {
			return err
		}
	}
	if len(entry.Value) > 0 {
		if _, err := w.file.Write(entry.Value); err != nil {
			return err
		}
	}

	// Write checksum
	checksumBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(checksumBytes, entry.Checksum)
	if _, err := w.file.Write(checksumBytes); err != nil {
		return err
	}

	// Update statistics
	w.fileSize += int64(entrySize)
	atomic.AddUint64(&w.stats.TotalEntries, 1)
	atomic.AddUint64(&w.stats.BytesWritten, uint64(entrySize))

	return nil
}

// calculateChecksum calculates CRC32 checksum for an entry
func (w *WAL) calculateChecksum(entry *WALEntry) uint32 {
	hasher := crc32.NewIEEE()

	// Hash LSN, timestamp, operation
	if err := binary.Write(hasher, binary.LittleEndian, entry.LSN); err != nil {
		fmt.Printf("Warning: binary write LSN failed: %v\n", err)
	}
	if err := binary.Write(hasher, binary.LittleEndian, entry.Timestamp); err != nil {
		fmt.Printf("Warning: binary write timestamp failed: %v\n", err)
	}
	hasher.Write([]byte{byte(entry.Operation)})

	// Hash key and value
	hasher.Write(entry.Key)
	hasher.Write(entry.Value)

	return hasher.Sum32()
}

// GetStats returns current WAL statistics
func (w *WAL) GetStats() WALStats {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return WALStats{
		TotalEntries:   atomic.LoadUint64(&w.stats.TotalEntries),
		BytesWritten:   atomic.LoadUint64(&w.stats.BytesWritten),
		FlushCount:     atomic.LoadUint64(&w.stats.FlushCount),
		ErrorCount:     atomic.LoadUint64(&w.stats.ErrorCount),
		LastFlushTime:  w.stats.LastFlushTime,
		AverageLatency: w.stats.AverageLatency,
	}
}

// Close closes the WAL and flushes any pending entries
func (w *WAL) Close() error {
	// Signal shutdown
	close(w.stopCh)

	// Wait for background worker to finish
	w.wg.Wait()

	// Close file
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		return w.file.Close()
	}

	return nil
}

// walReader helps read WAL entries during recovery
type walReader struct {
	file *os.File
}

// ReadEntry reads the next WAL entry from file
func (r *walReader) ReadEntry() (*WALEntry, error) {
	// Read header
	header := make([]byte, 25)
	if _, err := io.ReadFull(r.file, header); err != nil {
		return nil, err
	}

	// Parse header
	entry := &WALEntry{
		LSN:       binary.LittleEndian.Uint64(header[0:8]),
		Timestamp: int64(binary.LittleEndian.Uint64(header[8:16])),
		Operation: OpType(header[16]),
	}

	keyLen := binary.LittleEndian.Uint32(header[17:21])
	valueLen := binary.LittleEndian.Uint32(header[21:25])

	// Read key and value
	if keyLen > 0 {
		entry.Key = make([]byte, keyLen)
		if _, err := io.ReadFull(r.file, entry.Key); err != nil {
			return nil, err
		}
	}

	if valueLen > 0 {
		entry.Value = make([]byte, valueLen)
		if _, err := io.ReadFull(r.file, entry.Value); err != nil {
			return nil, err
		}
	}

	// Read checksum
	checksumBytes := make([]byte, 4)
	if _, err := io.ReadFull(r.file, checksumBytes); err != nil {
		return nil, err
	}
	entry.Checksum = binary.LittleEndian.Uint32(checksumBytes)

	return entry, nil
}
