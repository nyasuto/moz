package query

import (
	"os"
	"testing"

	"github.com/nyasuto/moz/internal/kvstore"
)

func TestExecutor_SelectAll(t *testing.T) {
	// Create test store with temporary file
	compactionConfig := kvstore.CompactionConfig{Enabled: false}
	storageConfig := kvstore.StorageConfig{
		Format:   "text",
		TextFile: "test_query_all.log",
	}
	store := kvstore.NewWithConfig(compactionConfig, storageConfig)
	defer func() {
		// Clean up test file
		os.Remove("test_query_all.log")
	}()

	store.Put("user1", "Alice")
	store.Put("user2", "Bob")
	store.Put("admin1", "Charlie Admin")

	executor := NewExecutor(store)

	// Test SELECT * FROM moz
	l := NewLexer("SELECT * FROM moz")
	p := NewParser(l)
	stmt := p.ParseQuery()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	result := executor.Execute(stmt)
	if result.Error != nil {
		t.Fatalf("Execution error: %v", result.Error)
	}

	if len(result.Rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(result.Rows))
	}
}

func TestExecutor_WhereConditions(t *testing.T) {
	// Create test store with temporary file
	compactionConfig := kvstore.CompactionConfig{Enabled: false}
	storageConfig := kvstore.StorageConfig{
		Format:   "text",
		TextFile: "test_query_where.log",
	}
	store := kvstore.NewWithConfig(compactionConfig, storageConfig)
	defer func() {
		// Clean up test file
		os.Remove("test_query_where.log")
	}()

	store.Put("user1", "Alice")
	store.Put("user2", "Bob")
	store.Put("admin1", "Charlie Admin")

	executor := NewExecutor(store)

	tests := []struct {
		query        string
		expectedRows int
	}{
		{
			"SELECT * FROM moz WHERE key = 'user1'",
			1,
		},
		{
			"SELECT * FROM moz WHERE key LIKE 'user%'",
			2,
		},
		{
			"SELECT * FROM moz WHERE value CONTAINS 'Admin'",
			1,
		},
		{
			"SELECT * FROM moz WHERE key BETWEEN 'admin1' AND 'user2'",
			3,
		},
	}

	for _, tt := range tests {
		l := NewLexer(tt.query)
		p := NewParser(l)
		stmt := p.ParseQuery()

		if len(p.Errors()) > 0 {
			t.Errorf("Parser errors for %q: %v", tt.query, p.Errors())
			continue
		}

		result := executor.Execute(stmt)
		if result.Error != nil {
			t.Errorf("Execution error for %q: %v", tt.query, result.Error)
			continue
		}

		if len(result.Rows) != tt.expectedRows {
			t.Errorf("Query %q: expected %d rows, got %d", tt.query, tt.expectedRows, len(result.Rows))
		}
	}
}

func TestExecutor_CountFunction(t *testing.T) {
	// Create test store with temporary file
	compactionConfig := kvstore.CompactionConfig{Enabled: false}
	storageConfig := kvstore.StorageConfig{
		Format:   "text",
		TextFile: "test_query_count.log",
	}
	store := kvstore.NewWithConfig(compactionConfig, storageConfig)
	defer func() {
		// Clean up test file
		os.Remove("test_query_count.log")
	}()

	store.Put("user1", "Alice Admin")
	store.Put("user2", "Bob User")
	store.Put("admin1", "Charlie Admin")

	executor := NewExecutor(store)

	l := NewLexer("SELECT COUNT(*) FROM moz WHERE value CONTAINS 'Admin'")
	p := NewParser(l)
	stmt := p.ParseQuery()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	result := executor.Execute(stmt)
	if result.Error != nil {
		t.Fatalf("Execution error: %v", result.Error)
	}

	if result.Count != 2 {
		t.Errorf("Expected count 2, got %d", result.Count)
	}
}

func TestExecutor_ComplexConditions(t *testing.T) {
	// Create test store with temporary file
	compactionConfig := kvstore.CompactionConfig{Enabled: false}
	storageConfig := kvstore.StorageConfig{
		Format:   "text",
		TextFile: "test_query_complex.log",
	}
	store := kvstore.NewWithConfig(compactionConfig, storageConfig)
	defer func() {
		// Clean up test file
		os.Remove("test_query_complex.log")
	}()

	store.Put("user1", "Alice Admin")
	store.Put("user2", "Bob User")
	store.Put("admin1", "Charlie Admin")
	store.Put("guest1", "David Guest")

	executor := NewExecutor(store)

	// Test AND condition
	l := NewLexer("SELECT * FROM moz WHERE key LIKE 'user%' AND value CONTAINS 'Admin'")
	p := NewParser(l)
	stmt := p.ParseQuery()

	if len(p.Errors()) > 0 {
		t.Fatalf("Parser errors: %v", p.Errors())
	}

	result := executor.Execute(stmt)
	if result.Error != nil {
		t.Fatalf("Execution error: %v", result.Error)
	}

	if len(result.Rows) != 1 {
		t.Errorf("Expected 1 row for AND condition, got %d", len(result.Rows))
	}

	// Verify the correct row
	if len(result.Rows) > 0 {
		row := result.Rows[0]
		if row["key"] != "user1" || row["value"] != "Alice Admin" {
			t.Errorf("Wrong row returned: %v", row)
		}
	}
}
