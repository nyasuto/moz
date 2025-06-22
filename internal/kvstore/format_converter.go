package kvstore

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FormatConverter handles conversion between text and binary formats
type FormatConverter struct {
	// Configuration
	textFile   string
	binaryFile string
}

// validateFilePath validates that the file path is safe to use
func validateFilePath(filename string) error {
	// Clean the path to prevent directory traversal
	cleanPath := filepath.Clean(filename)

	// Check for directory traversal attempts
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("invalid file path: contains directory traversal")
	}

	// Ensure the path is not absolute (for security)
	if filepath.IsAbs(cleanPath) {
		return fmt.Errorf("invalid file path: absolute paths not allowed")
	}

	return nil
}

// NewFormatConverter creates a new format converter
func NewFormatConverter(textFile, binaryFile string) *FormatConverter {
	return &FormatConverter{
		textFile:   textFile,
		binaryFile: binaryFile,
	}
}

// TextToBinary converts text format log to binary format
func (fc *FormatConverter) TextToBinary() error {
	// Open text file for reading
	textF, err := os.Open(fc.textFile)
	if err != nil {
		return fmt.Errorf("failed to open text file %s: %w", fc.textFile, err)
	}
	defer func() { _ = textF.Close() }()

	// Create binary file for writing
	binaryF, err := os.Create(fc.binaryFile)
	if err != nil {
		return fmt.Errorf("failed to create binary file %s: %w", fc.binaryFile, err)
	}
	defer func() { _ = binaryF.Close() }()

	scanner := bufio.NewScanner(textF)
	entryCount := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// Parse text format: key\tvalue or key\t__DELETED__
		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			return fmt.Errorf("invalid text format at line: %s", line)
		}

		key := parts[0]
		value := parts[1]

		var entry *BinaryEntry
		if value == "__DELETED__" {
			// Delete operation
			entry = NewBinaryEntry(BinaryOpDelete, []byte(key), []byte(""))
		} else {
			// Put operation
			entry = NewBinaryEntry(BinaryOpPut, []byte(key), []byte(value))
		}

		// Write binary entry
		if _, err := entry.WriteTo(binaryF); err != nil {
			return fmt.Errorf("failed to write binary entry: %w", err)
		}

		entryCount++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading text file: %w", err)
	}

	fmt.Printf("Successfully converted %d entries from text to binary format\n", entryCount)
	return nil
}

// BinaryToText converts binary format log to text format
func (fc *FormatConverter) BinaryToText() error {
	// Open binary file for reading
	binaryF, err := os.Open(fc.binaryFile)
	if err != nil {
		return fmt.Errorf("failed to open binary file %s: %w", fc.binaryFile, err)
	}
	defer func() { _ = binaryF.Close() }()

	// Create text file for writing
	textF, err := os.Create(fc.textFile)
	if err != nil {
		return fmt.Errorf("failed to create text file %s: %w", fc.textFile, err)
	}
	defer func() { _ = textF.Close() }()

	entryCount := 0

	for {
		entry, err := ReadBinaryEntry(binaryF)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read binary entry: %w", err)
		}

		// Convert to text format
		var line string
		if entry.IsDeleted() {
			line = fmt.Sprintf("%s\t__DELETED__\n", string(entry.Key))
		} else {
			line = fmt.Sprintf("%s\t%s\n", string(entry.Key), string(entry.Value))
		}

		if _, err := textF.WriteString(line); err != nil {
			return fmt.Errorf("failed to write text entry: %w", err)
		}

		entryCount++
	}

	fmt.Printf("Successfully converted %d entries from binary to text format\n", entryCount)
	return nil
}

// TextEntryToBinary converts a single text entry to binary entry
func TextEntryToBinary(key, value string) *BinaryEntry {
	if value == "__DELETED__" {
		return NewBinaryEntry(BinaryOpDelete, []byte(key), []byte(""))
	}
	return NewBinaryEntry(BinaryOpPut, []byte(key), []byte(value))
}

// BinaryEntryToText converts a binary entry to text format string
func BinaryEntryToText(entry *BinaryEntry) string {
	if entry.IsDeleted() {
		return fmt.Sprintf("%s\t__DELETED__", string(entry.Key))
	}
	return fmt.Sprintf("%s\t%s", string(entry.Key), string(entry.Value))
}

// ValidateBinaryFile checks if a binary file is valid
func ValidateBinaryFile(filename string) error {
	if err := validateFilePath(filename); err != nil {
		return fmt.Errorf("file path validation failed: %w", err)
	}

	file, err := os.Open(filepath.Clean(filename))
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer func() { _ = file.Close() }()

	entryCount := 0
	for {
		entry, err := ReadBinaryEntry(file)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("validation failed at entry %d: %w", entryCount+1, err)
		}

		// Verify entry integrity
		if err := entry.Verify(); err != nil {
			return fmt.Errorf("checksum verification failed at entry %d: %w", entryCount+1, err)
		}

		entryCount++
	}

	fmt.Printf("Binary file %s is valid (%d entries)\n", filename, entryCount)
	return nil
}

// GetBinaryFileStats returns statistics about a binary file
func GetBinaryFileStats(filename string) (*BinaryFileStats, error) {
	if err := validateFilePath(filename); err != nil {
		return nil, fmt.Errorf("file path validation failed: %w", err)
	}

	file, err := os.Open(filepath.Clean(filename))
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer func() { _ = file.Close() }()

	stats := &BinaryFileStats{
		Filename: filename,
	}

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	stats.FileSize = fileInfo.Size()

	for {
		entry, err := ReadBinaryEntry(file)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to read entry: %w", err)
		}

		stats.EntryCount++
		stats.TotalKeySize += int64(len(entry.Key))
		stats.TotalValueSize += int64(len(entry.Value))

		if entry.IsDeleted() {
			stats.DeletedCount++
		} else {
			stats.ActiveCount++
		}

		// Track timestamp range
		timestamp := time.Unix(0, int64(entry.Timestamp))
		if stats.EarliestTimestamp.IsZero() || timestamp.Before(stats.EarliestTimestamp) {
			stats.EarliestTimestamp = timestamp
		}
		if timestamp.After(stats.LatestTimestamp) {
			stats.LatestTimestamp = timestamp
		}
	}

	return stats, nil
}

// BinaryFileStats contains statistics about a binary file
type BinaryFileStats struct {
	Filename          string
	FileSize          int64
	EntryCount        int64
	ActiveCount       int64
	DeletedCount      int64
	TotalKeySize      int64
	TotalValueSize    int64
	EarliestTimestamp time.Time
	LatestTimestamp   time.Time
}

// String returns a formatted string representation of the stats
func (s *BinaryFileStats) String() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Binary File Statistics: %s\n", s.Filename))
	sb.WriteString(fmt.Sprintf("File Size: %s\n", formatBytes(s.FileSize)))
	sb.WriteString(fmt.Sprintf("Total Entries: %d\n", s.EntryCount))
	sb.WriteString(fmt.Sprintf("Active Entries: %d\n", s.ActiveCount))
	sb.WriteString(fmt.Sprintf("Deleted Entries: %d\n", s.DeletedCount))
	sb.WriteString(fmt.Sprintf("Total Key Size: %s\n", formatBytes(s.TotalKeySize)))
	sb.WriteString(fmt.Sprintf("Total Value Size: %s\n", formatBytes(s.TotalValueSize)))

	if !s.EarliestTimestamp.IsZero() {
		sb.WriteString(fmt.Sprintf("Time Range: %s to %s\n",
			s.EarliestTimestamp.Format(time.RFC3339),
			s.LatestTimestamp.Format(time.RFC3339)))
	}

	if s.EntryCount > 0 {
		avgKeySize := float64(s.TotalKeySize) / float64(s.EntryCount)
		avgValueSize := float64(s.TotalValueSize) / float64(s.EntryCount)
		deletionRate := float64(s.DeletedCount) / float64(s.EntryCount) * 100

		sb.WriteString(fmt.Sprintf("Average Key Size: %.1f bytes\n", avgKeySize))
		sb.WriteString(fmt.Sprintf("Average Value Size: %.1f bytes\n", avgValueSize))
		sb.WriteString(fmt.Sprintf("Deletion Rate: %.1f%%\n", deletionRate))
	}

	return sb.String()
}

// formatBytes formats byte size in human readable format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
