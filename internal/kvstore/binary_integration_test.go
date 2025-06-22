package kvstore

import (
	"fmt"
	"os"
	"testing"
)

func TestBinaryFormatIntegration(t *testing.T) {
	// Clean up test files
	defer func() {
		os.Remove("test_binary.bin")
		os.Remove("test_text.log")
	}()

	t.Run("Binary Format Basic Operations", func(t *testing.T) {
		// Test binary format operations
		storageConfig := StorageConfig{
			Format:     "binary",
			TextFile:   "test_text.log",
			BinaryFile: "test_binary.bin",
		}

		compactionConfig := CompactionConfig{
			Enabled:         false, // Disable for predictable testing
			MaxFileSize:     1024 * 1024,
			MaxOperations:   1000,
			CompactionRatio: 0.5,
		}

		store := NewWithConfig(compactionConfig, storageConfig)

		// Test PUT
		err := store.Put("test_key", "test_value")
		if err != nil {
			t.Fatalf("PUT failed: %v", err)
		}

		// Test GET
		value, err := store.Get("test_key")
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}

		if value != "test_value" {
			t.Errorf("Expected 'test_value', got '%s'", value)
		}

		// Test DELETE
		err = store.Delete("test_key")
		if err != nil {
			t.Fatalf("DELETE failed: %v", err)
		}

		// Verify deleted
		_, err = store.Get("test_key")
		if err == nil {
			t.Error("Expected error for deleted key")
		}
	})

	t.Run("Format Conversion", func(t *testing.T) {
		// Create test data in text format
		textStore := NewWithConfig(
			CompactionConfig{Enabled: false, MaxFileSize: 1024 * 1024, MaxOperations: 1000, CompactionRatio: 0.5},
			StorageConfig{Format: "text", TextFile: "test_text.log", BinaryFile: "test_binary.bin"},
		)

		// Add test data
		testData := map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		}

		for k, v := range testData {
			if err := textStore.Put(k, v); err != nil {
				t.Fatalf("Failed to put %s: %v", k, err)
			}
		}

		// Convert text to binary
		converter := NewFormatConverter("test_text.log", "test_binary.bin")
		if err := converter.TextToBinary(); err != nil {
			t.Fatalf("Text to binary conversion failed: %v", err)
		}

		// Verify binary file
		if err := ValidateBinaryFile("test_binary.bin"); err != nil {
			t.Fatalf("Binary file validation failed: %v", err)
		}

		// Create binary store and verify data
		binaryStore := NewWithConfig(
			CompactionConfig{Enabled: false, MaxFileSize: 1024 * 1024, MaxOperations: 1000, CompactionRatio: 0.5},
			StorageConfig{Format: "binary", TextFile: "test_text.log", BinaryFile: "test_binary.bin"},
		)

		for k, expected := range testData {
			value, err := binaryStore.Get(k)
			if err != nil {
				t.Errorf("Failed to get %s from binary store: %v", k, err)
				continue
			}
			if value != expected {
				t.Errorf("Key %s: expected %s, got %s", k, expected, value)
			}
		}

		// Convert back to text
		os.Remove("test_text.log") // Clean up original
		if err := converter.BinaryToText(); err != nil {
			t.Fatalf("Binary to text conversion failed: %v", err)
		}

		// Verify converted text data
		textStore2 := NewWithConfig(
			CompactionConfig{Enabled: false, MaxFileSize: 1024 * 1024, MaxOperations: 1000, CompactionRatio: 0.5},
			StorageConfig{Format: "text", TextFile: "test_text.log", BinaryFile: "test_binary.bin"},
		)

		for k, expected := range testData {
			value, err := textStore2.Get(k)
			if err != nil {
				t.Errorf("Failed to get %s from converted text store: %v", k, err)
				continue
			}
			if value != expected {
				t.Errorf("Key %s: expected %s, got %s", k, expected, value)
			}
		}
	})

	t.Run("File Size Comparison", func(t *testing.T) {
		// Clean up
		os.Remove("test_text.log")
		os.Remove("test_binary.bin")

		testData := make(map[string]string)
		for i := 0; i < 100; i++ {
			testData[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d_with_some_longer_content", i)
		}

		// Create text version
		textStore := NewWithConfig(
			CompactionConfig{Enabled: false, MaxFileSize: 1024 * 1024, MaxOperations: 1000, CompactionRatio: 0.5},
			StorageConfig{Format: "text", TextFile: "test_text.log", BinaryFile: "test_binary.bin"},
		)

		for k, v := range testData {
			if err := textStore.Put(k, v); err != nil {
				t.Fatalf("Failed to put %s: %v", k, err)
			}
		}

		// Get text file size
		textInfo, err := os.Stat("test_text.log")
		if err != nil {
			t.Fatalf("Failed to stat text file: %v", err)
		}
		textSize := textInfo.Size()

		// Convert to binary
		converter := NewFormatConverter("test_text.log", "test_binary.bin")
		if err := converter.TextToBinary(); err != nil {
			t.Fatalf("Conversion failed: %v", err)
		}

		// Get binary file size
		binaryInfo, err := os.Stat("test_binary.bin")
		if err != nil {
			t.Fatalf("Failed to stat binary file: %v", err)
		}
		binarySize := binaryInfo.Size()

		t.Logf("Text file size: %d bytes", textSize)
		t.Logf("Binary file size: %d bytes", binarySize)

		if binarySize >= textSize {
			t.Logf("Warning: Binary file (%d bytes) is not smaller than text file (%d bytes)", binarySize, textSize)
			// This might happen with small datasets due to overhead, but log it
		}

		spaceEfficiency := float64(textSize-binarySize) / float64(textSize) * 100
		t.Logf("Space efficiency: %.2f%%", spaceEfficiency)
	})
}

func TestBinaryFileStats(t *testing.T) {
	// Clean up
	defer os.Remove("test_stats.bin")

	// Create test data directly

	// Create some test entries directly
	entries := []*BinaryEntry{
		NewBinaryEntry(BinaryOpPut, []byte("key1"), []byte("value1")),
		NewBinaryEntry(BinaryOpPut, []byte("key2"), []byte("value2")),
		NewBinaryEntry(BinaryOpDelete, []byte("key1"), []byte("")),
		NewBinaryEntry(BinaryOpPut, []byte("key3"), []byte("value3")),
	}

	// Write entries to file
	file, err := os.Create("test_stats.bin")
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer file.Close()

	for _, entry := range entries {
		if _, err := entry.WriteTo(file); err != nil {
			t.Fatalf("Failed to write entry: %v", err)
		}
	}
	file.Close()

	// Get stats
	stats, err := GetBinaryFileStats("test_stats.bin")
	if err != nil {
		t.Fatalf("Failed to get stats: %v", err)
	}

	// Verify stats
	if stats.EntryCount != 4 {
		t.Errorf("Expected 4 entries, got %d", stats.EntryCount)
	}

	if stats.ActiveCount != 2 { // key2, key3
		t.Errorf("Expected 2 active entries, got %d", stats.ActiveCount)
	}

	if stats.DeletedCount != 1 { // key1 deleted
		t.Errorf("Expected 1 deleted entry, got %d", stats.DeletedCount)
	}

	if stats.FileSize <= 0 {
		t.Error("File size should be greater than 0")
	}

	// Test string representation
	statsStr := stats.String()
	if len(statsStr) == 0 {
		t.Error("Stats string should not be empty")
	}

	t.Logf("Stats:\n%s", statsStr)
}
