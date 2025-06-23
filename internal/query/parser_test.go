package query

import (
	"testing"
)

func TestParser_SelectStatement(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"SELECT * FROM moz",
			"SELECT * FROM moz",
		},
		{
			"SELECT key FROM moz WHERE key = 'test'",
			"SELECT key FROM moz WHERE (key = \"test\")",
		},
		{
			"SELECT * FROM moz WHERE key LIKE 'user%'",
			"SELECT * FROM moz WHERE (key LIKE \"user%\")",
		},
		{
			"SELECT COUNT(*) FROM moz WHERE value CONTAINS 'admin'",
			"SELECT COUNT(*) FROM moz WHERE (value CONTAINS \"admin\")",
		},
		{
			"SELECT * FROM moz WHERE key BETWEEN 'a' AND 'z'",
			"SELECT * FROM moz WHERE key BETWEEN \"a\" AND \"z\"",
		},
	}

	for _, tt := range tests {
		l := NewLexer(tt.input)
		p := NewParser(l)
		stmt := p.ParseQuery()

		if len(p.Errors()) > 0 {
			t.Errorf("parser errors: %v", p.Errors())
			continue
		}

		if stmt.String() != tt.expected {
			t.Errorf("expected %q, got %q", tt.expected, stmt.String())
		}
	}
}

func TestParser_ComplexQueries(t *testing.T) {
	tests := []struct {
		input       string
		shouldError bool
	}{
		{
			"SELECT * FROM moz WHERE key = 'test' AND value = 'data'",
			false,
		},
		{
			"SELECT * FROM moz WHERE key LIKE 'user%' OR key LIKE 'admin%'",
			false,
		},
		{
			"SELECT * FROM moz WHERE NOT key = 'test'",
			false,
		},
		{
			"SELECT * FROM moz ORDER BY key ASC",
			false,
		},
		{
			"SELECT * FROM moz LIMIT 10",
			false,
		},
		{
			"SELECT * FROM moz LIMIT 10 OFFSET 5",
			false,
		},
		{
			"SELECT INVALID",
			true,
		},
	}

	for _, tt := range tests {
		l := NewLexer(tt.input)
		p := NewParser(l)
		stmt := p.ParseQuery()

		hasErrors := len(p.Errors()) > 0

		if hasErrors != tt.shouldError {
			if tt.shouldError {
				t.Errorf("expected parser errors for %q, but got none", tt.input)
			} else {
				t.Errorf("unexpected parser errors for %q: %v", tt.input, p.Errors())
			}
		}

		if !tt.shouldError && stmt == nil {
			t.Errorf("expected statement for %q, got nil", tt.input)
		}
	}
}
