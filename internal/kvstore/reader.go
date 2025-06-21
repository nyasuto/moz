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
}

// NewLogReader creates a new LogReader for the specified file
func NewLogReader(filename string) *LogReader {
	return &LogReader{
		filename: filename,
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
	defer file.Close()

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
	defer file.Close()

	var entries []LogEntry
	scanner := bufio.NewScanner(file)
	
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
		
		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	return entries, nil
}

// parseLogFile parses the log file and builds the current state
func (lr *LogReader) parseLogFile(reader io.Reader) (map[string]string, error) {
	data := make(map[string]string)
	scanner := bufio.NewScanner(reader)

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