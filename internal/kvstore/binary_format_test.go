package kvstore

import (
	"bytes"
	"testing"
)

func TestBinaryEntry_NewBinaryEntry(t *testing.T) {
	key := []byte("test_key")
	value := []byte("test_value")

	entry := NewBinaryEntry(BinaryOpPut, key, value)

	if entry.Operation != BinaryOpPut {
		t.Errorf("Expected operation %d, got %d", BinaryOpPut, entry.Operation)
	}

	if entry.KeyLength != uint16(len(key)) {
		t.Errorf("Expected key length %d, got %d", len(key), entry.KeyLength)
	}

	if entry.ValueLength != uint32(len(value)) {
		t.Errorf("Expected value length %d, got %d", len(value), entry.ValueLength)
	}

	if !bytes.Equal(entry.Key, key) {
		t.Errorf("Expected key %s, got %s", key, entry.Key)
	}

	if !bytes.Equal(entry.Value, value) {
		t.Errorf("Expected value %s, got %s", value, entry.Value)
	}

	// Verify checksum is calculated
	if entry.Checksum == 0 {
		t.Error("Checksum should not be zero")
	}

	// Verify the entry
	if err := entry.Verify(); err != nil {
		t.Errorf("Entry verification failed: %v", err)
	}
}

func TestBinaryEntry_WriteTo_ReadFrom(t *testing.T) {
	tests := []struct {
		name      string
		operation uint8
		key       string
		value     string
	}{
		{"PUT operation", BinaryOpPut, "test_key", "test_value"},
		{"DELETE operation", BinaryOpDelete, "delete_key", ""},
		{"Empty key", BinaryOpPut, "", "value"},
		{"Empty value", BinaryOpPut, "key", ""},
		{"Large data", BinaryOpPut, "large_key_with_many_characters", "large_value_with_many_characters_and_data"},
		{"Unicode data", BinaryOpPut, "キー", "値"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create original entry
			original := NewBinaryEntry(tt.operation, []byte(tt.key), []byte(tt.value))

			// Write to buffer
			var buf bytes.Buffer
			written, err := original.WriteTo(&buf)
			if err != nil {
				t.Fatalf("WriteTo failed: %v", err)
			}

			expectedSize := int64(BinaryMagicSize + original.Size())
			if written != expectedSize {
				t.Errorf("Expected %d bytes written, got %d", expectedSize, written)
			}

			// Read from buffer
			read, err := ReadBinaryEntry(&buf)
			if err != nil {
				t.Fatalf("ReadBinaryEntry failed: %v", err)
			}

			// Compare entries
			if read.Timestamp != original.Timestamp {
				t.Errorf("Timestamp mismatch: expected %d, got %d", original.Timestamp, read.Timestamp)
			}

			if read.Operation != original.Operation {
				t.Errorf("Operation mismatch: expected %d, got %d", original.Operation, read.Operation)
			}

			if read.KeyLength != original.KeyLength {
				t.Errorf("KeyLength mismatch: expected %d, got %d", original.KeyLength, read.KeyLength)
			}

			if read.ValueLength != original.ValueLength {
				t.Errorf("ValueLength mismatch: expected %d, got %d", original.ValueLength, read.ValueLength)
			}

			if !bytes.Equal(read.Key, original.Key) {
				t.Errorf("Key mismatch: expected %s, got %s", original.Key, read.Key)
			}

			if !bytes.Equal(read.Value, original.Value) {
				t.Errorf("Value mismatch: expected %s, got %s", original.Value, read.Value)
			}

			if read.Checksum != original.Checksum {
				t.Errorf("Checksum mismatch: expected %d, got %d", original.Checksum, read.Checksum)
			}
		})
	}
}

func TestBinaryEntry_Checksum_Verification(t *testing.T) {
	entry := NewBinaryEntry(BinaryOpPut, []byte("key"), []byte("value"))

	// Valid checksum should pass
	if err := entry.Verify(); err != nil {
		t.Errorf("Valid entry should pass verification: %v", err)
	}

	// Corrupt checksum should fail
	entry.Checksum = 0xDEADBEEF
	if err := entry.Verify(); err == nil {
		t.Error("Corrupted entry should fail verification")
	}
}

func TestBinaryEntry_Size(t *testing.T) {
	tests := []struct {
		key   string
		value string
	}{
		{"", ""},
		{"key", ""},
		{"", "value"},
		{"key", "value"},
		{"long_key_with_many_characters", "long_value_with_many_characters_and_more_data"},
	}

	for _, tt := range tests {
		entry := NewBinaryEntry(BinaryOpPut, []byte(tt.key), []byte(tt.value))
		expectedSize := BinaryHeaderSize + len(tt.key) + len(tt.value)

		if entry.Size() != expectedSize {
			t.Errorf("Size mismatch for key=%s, value=%s: expected %d, got %d",
				tt.key, tt.value, expectedSize, entry.Size())
		}
	}
}

func TestBinaryEntry_IsDeleted(t *testing.T) {
	putEntry := NewBinaryEntry(BinaryOpPut, []byte("key"), []byte("value"))
	deleteEntry := NewBinaryEntry(BinaryOpDelete, []byte("key"), []byte(""))

	if putEntry.IsDeleted() {
		t.Error("PUT entry should not be marked as deleted")
	}

	if !deleteEntry.IsDeleted() {
		t.Error("DELETE entry should be marked as deleted")
	}
}

func TestReadBinaryEntry_InvalidMagic(t *testing.T) {
	buf := bytes.NewBuffer([]byte{0xFF, 0xFF, 0xFF, 0xFF}) // Invalid magic

	_, err := ReadBinaryEntry(buf)
	if err == nil {
		t.Error("Reading entry with invalid magic should fail")
	}
}

func TestReadBinaryEntry_IncompleteData(t *testing.T) {
	// Write partial magic number
	buf := bytes.NewBuffer([]byte{'M', 'O'}) // Incomplete magic

	_, err := ReadBinaryEntry(buf)
	if err == nil {
		t.Error("Reading incomplete entry should fail")
	}
}

func BenchmarkBinaryEntry_WriteTo(b *testing.B) {
	entry := NewBinaryEntry(BinaryOpPut, []byte("benchmark_key"), []byte("benchmark_value"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		_, err := entry.WriteTo(&buf)
		if err != nil {
			b.Fatalf("WriteTo failed: %v", err)
		}
	}
}

func BenchmarkBinaryEntry_ReadFrom(b *testing.B) {
	entry := NewBinaryEntry(BinaryOpPut, []byte("benchmark_key"), []byte("benchmark_value"))

	var buf bytes.Buffer
	_, err := entry.WriteTo(&buf)
	if err != nil {
		b.Fatalf("Setup failed: %v", err)
	}

	data := buf.Bytes()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := bytes.NewBuffer(data)
		_, err := ReadBinaryEntry(buf)
		if err != nil {
			b.Fatalf("ReadBinaryEntry failed: %v", err)
		}
	}
}

func BenchmarkBinaryEntry_NewBinaryEntry(b *testing.B) {
	key := []byte("benchmark_key")
	value := []byte("benchmark_value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NewBinaryEntry(BinaryOpPut, key, value)
	}
}

func BenchmarkBinaryEntry_Verify(b *testing.B) {
	entry := NewBinaryEntry(BinaryOpPut, []byte("benchmark_key"), []byte("benchmark_value"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := entry.Verify()
		if err != nil {
			b.Fatalf("Verify failed: %v", err)
		}
	}
}
