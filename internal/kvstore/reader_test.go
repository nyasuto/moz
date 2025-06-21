package kvstore

import (
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestLogReader_ReadAll_TABFormat(t *testing.T) {
	// Create temporary test file with TAB-delimited format
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.log")

	content := "name\tAlice\ncity\tTokyo\nname\tBob\nage\t25\ncity\t__DELETED__\n"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	reader := NewLogReader(testFile)
	data, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	expected := map[string]string{
		"name": "Bob", // Latest value
		"age":  "25",
		// city should be deleted due to __DELETED__ marker
	}

	if !reflect.DeepEqual(data, expected) {
		t.Errorf("Expected %v, got %v", expected, data)
	}
}

func TestLogReader_ReadAll_SpaceFormat(t *testing.T) {
	// Create temporary test file with space-delimited format
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.log")

	content := "PUT name Alice\nPUT city Tokyo\nPUT name Bob\nPUT age 25\nDEL city\n"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	reader := NewLogReader(testFile)
	data, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	expected := map[string]string{
		"name": "Bob", // Latest value
		"age":  "25",
		// city should be deleted due to DEL operation
	}

	if !reflect.DeepEqual(data, expected) {
		t.Errorf("Expected %v, got %v", expected, data)
	}
}

func TestLogReader_ReadAll_MixedFormat(t *testing.T) {
	// Test mixed TAB and space formats
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.log")

	content := "name\tAlice\nPUT city Tokyo\nname\tBob\nDEL city\nage\t25\n"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	reader := NewLogReader(testFile)
	data, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	expected := map[string]string{
		"name": "Bob",
		"age":  "25",
		// city should be deleted
	}

	if !reflect.DeepEqual(data, expected) {
		t.Errorf("Expected %v, got %v", expected, data)
	}
}

func TestLogReader_ReadAll_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "empty.log")

	// Create empty file
	err := os.WriteFile(testFile, []byte(""), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	reader := NewLogReader(testFile)
	data, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(data) != 0 {
		t.Errorf("Expected empty map, got %v", data)
	}
}

func TestLogReader_ReadAll_NonExistentFile(t *testing.T) {
	reader := NewLogReader("/non/existent/file.log")
	data, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll should not fail for non-existent file: %v", err)
	}

	if len(data) != 0 {
		t.Errorf("Expected empty map, got %v", data)
	}
}

func TestLogReader_ReadAllEntries(t *testing.T) {
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.log")

	content := "name\tAlice\ncity\tTokyo\nname\tBob\n"
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	reader := NewLogReader(testFile)
	entries, err := reader.ReadAllEntries()
	if err != nil {
		t.Fatalf("ReadAllEntries failed: %v", err)
	}

	expected := []LogEntry{
		{Key: "name", Value: "Alice"},
		{Key: "city", Value: "Tokyo"},
		{Key: "name", Value: "Bob"},
	}

	if !reflect.DeepEqual(entries, expected) {
		t.Errorf("Expected %v, got %v", expected, entries)
	}
}

func TestLogReader_parseLine_TABFormat(t *testing.T) {
	reader := NewLogReader("")

	tests := []struct {
		line     string
		expected LogEntry
		hasError bool
	}{
		{"name\tAlice", LogEntry{Key: "name", Value: "Alice"}, false},
		{"city\t__DELETED__", LogEntry{Key: "city", Value: "__DELETED__"}, false},
		{"key\tvalue with spaces", LogEntry{Key: "key", Value: "value with spaces"}, false},
		{"invalid_tab_format", LogEntry{}, true},
		{"key\t", LogEntry{Key: "key", Value: ""}, false},
		{"\tvalue", LogEntry{Key: "", Value: "value"}, false},
	}

	for _, test := range tests {
		entry, err := reader.parseLine(test.line)

		if test.hasError {
			if err == nil {
				t.Errorf("Expected error for line: %s", test.line)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for line %s: %v", test.line, err)
			}
			if !reflect.DeepEqual(entry, test.expected) {
				t.Errorf("For line %s, expected %v, got %v", test.line, test.expected, entry)
			}
		}
	}
}

func TestLogReader_parseLine_SpaceFormat(t *testing.T) {
	reader := NewLogReader("")

	tests := []struct {
		line     string
		expected LogEntry
		hasError bool
	}{
		{"PUT name Alice", LogEntry{Key: "name", Value: "Alice"}, false},
		{"DEL city", LogEntry{Key: "city", Value: "__DELETED__"}, false},
		{"PUT key value with spaces", LogEntry{Key: "key", Value: "value with spaces"}, false},
		{"PUT key", LogEntry{}, true},           // Missing value
		{"DEL", LogEntry{}, true},               // Missing key
		{"INVALID key value", LogEntry{}, true}, // Invalid operation
		{"", LogEntry{}, true},                  // Empty line
	}

	for _, test := range tests {
		entry, err := reader.parseLine(test.line)

		if test.hasError {
			if err == nil {
				t.Errorf("Expected error for line: %s", test.line)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for line %s: %v", test.line, err)
			}
			if !reflect.DeepEqual(entry, test.expected) {
				t.Errorf("For line %s, expected %v, got %v", test.line, test.expected, entry)
			}
		}
	}
}

func TestValidateKey(t *testing.T) {
	tests := []struct {
		key      string
		hasError bool
	}{
		{"validkey", false},
		{"valid_key", false},
		{"valid-key", false},
		{"valid123", false},
		{"key\twith\ttab", true},
		{"key\twith\tone\ttab", true},
		{"", false}, // Empty key is technically valid
	}

	for _, test := range tests {
		err := ValidateKey(test.key)

		if test.hasError {
			if err == nil {
				t.Errorf("Expected error for key: %s", test.key)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for key %s: %v", test.key, err)
			}
		}
	}
}

func TestIsDeleted(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"__DELETED__", true},
		{"regular_value", false},
		{"", false},
		{"__deleted__", false}, // Case sensitive
		{"DELETED", false},
	}

	for _, test := range tests {
		result := IsDeleted(test.value)
		if result != test.expected {
			t.Errorf("For value %s, expected %v, got %v", test.value, test.expected, result)
		}
	}
}

func TestLogReader_LargeFile(t *testing.T) {
	// Test with a reasonably large file to ensure streaming works
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "large.log")

	var content strings.Builder
	for i := 0; i < 1000; i++ {
		content.WriteString("key")
		if i < 10 {
			content.WriteString("00")
		} else if i < 100 {
			content.WriteString("0")
		}
		content.WriteString(strconv.Itoa(i))
		content.WriteString("\tvalue")
		content.WriteString(strconv.Itoa(i))
		content.WriteString("\n")
	}

	err := os.WriteFile(testFile, []byte(content.String()), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	reader := NewLogReader(testFile)
	data, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if len(data) != 1000 {
		t.Errorf("Expected 1000 entries, got %d", len(data))
	}

	// Verify a few sample entries
	if data["key000"] != "value0" {
		t.Errorf("Expected key000=value0, got %s", data["key000"])
	}
	if data["key999"] != "value999" {
		t.Errorf("Expected key999=value999, got %s", data["key999"])
	}
}
