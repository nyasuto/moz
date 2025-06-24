package kvstore

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// LogEntry represents a single entry in the moz.log file
type LogEntry struct {
	Key   string
	Value string
}

// LogReader provides functionality to read and parse moz.log files
type LogReader struct {
	filename string
	pools    *MemoryPools
}

// NewLogReader creates a new LogReader for the specified file
func NewLogReader(filename string) *LogReader {
	return &LogReader{
		filename: filename,
		pools:    NewMemoryPools(DefaultMemoryPoolConfig()),
	}
}

// NewLogReaderWithPools creates a new LogReader with existing memory pools
func NewLogReaderWithPools(filename string, pools *MemoryPools) *LogReader {
	return &LogReader{
		filename: filename,
		pools:    pools,
	}
}

// ReadAll reads all entries from the log file and returns the current state
// This method handles both TAB-delimited format (legacy) and space-delimited format
func (lr *LogReader) ReadAll() (map[string]string, error) {
	file, err := os.Open(lr.filename)
	if os.IsNotExist(err) {
		return make(map[string]string), nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log close error but don't override main error
			fmt.Printf("Warning: failed to close file: %v\n", closeErr)
		}
	}()

	return lr.parseLogFile(file)
}

// ReadAllEntries reads all entries from the log file as a slice
// Useful for debugging and testing
func (lr *LogReader) ReadAllEntries() ([]LogEntry, error) {
	file, err := os.Open(lr.filename)
	if os.IsNotExist(err) {
		return []LogEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log close error but don't override main error
			fmt.Printf("Warning: failed to close file: %v\n", closeErr)
		}
	}()

	// Pre-allocate slice with estimated capacity
	fileInfo, _ := file.Stat()
	estimatedEntries := int(fileInfo.Size() / 50) // Estimate ~50 bytes per entry
	if estimatedEntries < 100 {
		estimatedEntries = 100
	}

	entries := make([]LogEntry, 0, estimatedEntries)
	scanner := bufio.NewScanner(file)

	// Use memory pool for scan buffer optimization
	if lr.pools != nil {
		scanBuffer := lr.pools.GetBuffer()
		defer lr.pools.PutBuffer(scanBuffer)

		// Resize buffer if needed for scanner
		if cap(scanBuffer) < 64*1024 {
			scanBuffer = make([]byte, 0, 64*1024) // 64KB scan buffer
		}
		scanner.Buffer(scanBuffer, cap(scanBuffer))
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Use memory pool for log entries if available
		var entry LogEntry
		if lr.pools != nil {
			pooledEntry := lr.pools.GetLogEntry()
			entryResult, err := lr.parseLineToPooledEntry(line, pooledEntry)
			if err != nil {
				lr.pools.PutLogEntry(pooledEntry)
				continue
			}
			entry = *entryResult
			lr.pools.PutLogEntry(pooledEntry)
		} else {
			entryResult, err := lr.parseLine(line)
			if err != nil {
				continue
			}
			entry = entryResult
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return entries, nil
}

// parseLogFile parses the log file and builds the current state
func (lr *LogReader) parseLogFile(reader io.Reader) (map[string]string, error) {
	// Pre-allocate map with estimated capacity
	data := make(map[string]string, 1000) // Start with reasonable default
	scanner := bufio.NewScanner(reader)

	// Use memory pool for scan buffer optimization
	if lr.pools != nil {
		scanBuffer := lr.pools.GetBuffer()
		defer lr.pools.PutBuffer(scanBuffer)

		// Resize buffer if needed for scanner
		if cap(scanBuffer) < 64*1024 {
			scanBuffer = make([]byte, 0, 64*1024) // 64KB scan buffer
		}
		scanner.Buffer(scanBuffer, cap(scanBuffer))
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		entry, err := lr.parseLine(line)
		if err != nil {
			// Skip invalid lines instead of failing
			continue
		}

		// Handle deletion marker
		if entry.Value == "__DELETED__" {
			delete(data, entry.Key)
		} else {
			data[entry.Key] = entry.Value
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return data, nil
}

// parseLineToPooledEntry parses a single line into a pooled LogEntry
func (lr *LogReader) parseLineToPooledEntry(line string, entry *LogEntry) (*LogEntry, error) {
	// Try TAB-delimited format first (legacy compatibility)
	if strings.Contains(line, "\t") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid TAB-delimited format: %s", line)
		}
		entry.Key = parts[0]
		entry.Value = parts[1]
		return entry, nil
	}

	// Try space-delimited format (PUT key value or DEL key)
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid line format: %s", line)
	}

	operation := parts[0]
	key := parts[1]

	switch operation {
	case "PUT":
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid PUT format: %s", line)
		}
		entry.Key = key
		entry.Value = parts[2]
		return entry, nil
	case "DEL":
		entry.Key = key
		entry.Value = "__DELETED__"
		return entry, nil
	default:
		return nil, fmt.Errorf("unknown operation: %s", operation)
	}
}

// parseLine parses a single line from the log file
// Supports both formats:
// - TAB-delimited: key\tvalue (legacy format)
// - Space-delimited: PUT key value or DEL key (new format)
func (lr *LogReader) parseLine(line string) (LogEntry, error) {
	// Try TAB-delimited format first (legacy compatibility)
	if strings.Contains(line, "\t") {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			return LogEntry{}, fmt.Errorf("invalid TAB-delimited format: %s", line)
		}
		return LogEntry{
			Key:   parts[0],
			Value: parts[1],
		}, nil
	}

	// Try space-delimited format (PUT key value or DEL key)
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return LogEntry{}, fmt.Errorf("invalid line format: %s", line)
	}

	operation := parts[0]
	key := parts[1]

	switch operation {
	case "PUT":
		if len(parts) != 3 {
			return LogEntry{}, fmt.Errorf("invalid PUT format: %s", line)
		}
		return LogEntry{
			Key:   key,
			Value: parts[2],
		}, nil
	case "DEL":
		return LogEntry{
			Key:   key,
			Value: "__DELETED__",
		}, nil
	default:
		return LogEntry{}, fmt.Errorf("unknown operation: %s", operation)
	}
}

// ValidateKey checks if a key is valid (no tab characters)
func ValidateKey(key string) error {
	if strings.Contains(key, "\t") {
		return fmt.Errorf("key cannot contain tab characters: %s", key)
	}
	return nil
}

// IsDeleted checks if a value represents a deletion marker
func IsDeleted(value string) bool {
	return value == "__DELETED__"
}
