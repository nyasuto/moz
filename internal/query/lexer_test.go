package query

import (
	"testing"
)

func TestLexer_BasicTokens(t *testing.T) {
	input := `SELECT COUNT(*) FROM moz WHERE value CONTAINS 'Admin'`

	tests := []struct {
		expectedType    TokenType
		expectedLiteral string
	}{
		{SELECT, "SELECT"},
		{COUNT, "COUNT"},
		{LPAREN, "("},
		{ASTERISK, "*"},
		{RPAREN, ")"},
		{FROM, "FROM"},
		{IDENT, "moz"},
		{WHERE, "WHERE"},
		{IDENT, "value"},
		{CONTAINS, "CONTAINS"},
		{STRING, "Admin"},
		{EOF, ""},
	}

	l := NewLexer(input)

	for i, tt := range tests {
		tok := l.NextToken()

		if tok.Type != tt.expectedType {
			t.Fatalf("tests[%d] - tokentype wrong. expected=%q, got=%q", i, tt.expectedType, tok.Type)
		}

		if tok.Literal != tt.expectedLiteral {
			t.Fatalf("tests[%d] - literal wrong. expected=%q, got=%q", i, tt.expectedLiteral, tok.Literal)
		}
	}
}
