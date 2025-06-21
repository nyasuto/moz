package kvstore

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLegacyCompatibility(t *testing.T) {
	// This test verifies that our Go implementation can read files
	// created by the legacy shell scripts

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "legacy_test.log")

	// Create a file that mimics what legacy scripts would create
	legacyContent := "test_key\ttest_value\nname\tAlice\ntest_key\t__DELETED__\ncity\tTokyo\nage\t25\nname\tBob\n"

	err := os.WriteFile(testFile, []byte(legacyContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Read using our new LogReader
	reader := NewLogReader(testFile)
	data, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	expected := map[string]string{
		"name": "Bob", // Latest value after Alice
		"city": "Tokyo",
		"age":  "25",
		// test_key should be deleted
	}

	if !reflect.DeepEqual(data, expected) {
		t.Errorf("Expected %v, got %v", expected, data)
	}

	// Verify that test_key is indeed deleted
	if _, exists := data["test_key"]; exists {
		t.Error("test_key should be deleted but still exists in data")
	}
}

func TestMixedFormatCompatibility(t *testing.T) {
	// Test that we can handle both legacy TAB format and new space format
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "mixed_test.log")

	// Mixed content: some TAB-delimited (legacy), some space-delimited (new)
	mixedContent := "legacy_key\tlegacy_value\nPUT new_key new_value\nlegacy_key\t__DELETED__\nDEL new_key\nfinal_key\tfinal_value\n"

	err := os.WriteFile(testFile, []byte(mixedContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	reader := NewLogReader(testFile)
	data, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	expected := map[string]string{
		"final_key": "final_value",
		// legacy_key and new_key should both be deleted
	}

	if !reflect.DeepEqual(data, expected) {
		t.Errorf("Expected %v, got %v", expected, data)
	}
}

func TestCurrentLogFileReading(t *testing.T) {
	// Test reading the actual moz.log file if it exists
	// This is an integration test that verifies we can read real legacy data

	// Check if moz.log exists in project root directory
	logFile := "../../moz.log"
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		t.Skip("moz.log does not exist in project root, skipping current log file test")
	}

	reader := NewLogReader(logFile)
	data, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to read moz.log: %v", err)
	}

	// Just verify that we can read it without errors
	// The content will vary depending on what tests have been run
	t.Logf("Successfully read moz.log with %d entries", len(data))

	// If there's data, verify it looks reasonable
	for key, value := range data {
		if key == "" {
			t.Error("Found empty key in moz.log")
		}
		if value == "__DELETED__" {
			t.Error("Found __DELETED__ value that should have been removed during parsing")
		}
	}
}
