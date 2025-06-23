package query

import (
	"fmt"
	"strings"
)

// TokenType represents the type of a lexical token
type TokenType int

const (
	// Special tokens
	ILLEGAL TokenType = iota
	EOF

	// Identifiers and literals
	IDENT  // field names, table names
	STRING // "string"
	NUMBER // 123

	// Keywords
	SELECT
	FROM
	WHERE
	AND
	OR
	NOT
	LIKE
	BETWEEN
	CONTAINS
	IN
	COUNT
	DISTINCT
	ORDER
	BY
	ASC
	DESC
	LIMIT
	OFFSET

	// Operators
	ASSIGN   // =
	NOT_EQ   // !=
	LT       // <
	GT       // >
	LT_EQ    // <=
	GT_EQ    // >=
	PLUS     // +
	MINUS    // -
	ASTERISK // *
	SLASH    // /
	PERCENT  // %

	// Delimiters
	COMMA     // ,
	SEMICOLON // ;
	LPAREN    // (
	RPAREN    // )

	// Special operators
	REGEX // REGEX
)

// Token represents a lexical token
type Token struct {
	Type     TokenType
	Literal  string
	Position int
}

// String returns the string representation of the token type
func (t TokenType) String() string {
	switch t {
	case ILLEGAL:
		return "ILLEGAL"
	case EOF:
		return "EOF"
	case IDENT:
		return "IDENT"
	case STRING:
		return "STRING"
	case NUMBER:
		return "NUMBER"
	case SELECT:
		return "SELECT"
	case FROM:
		return "FROM"
	case WHERE:
		return "WHERE"
	case AND:
		return "AND"
	case OR:
		return "OR"
	case NOT:
		return "NOT"
	case LIKE:
		return "LIKE"
	case BETWEEN:
		return "BETWEEN"
	case CONTAINS:
		return "CONTAINS"
	case IN:
		return "IN"
	case COUNT:
		return "COUNT"
	case DISTINCT:
		return "DISTINCT"
	case ORDER:
		return "ORDER"
	case BY:
		return "BY"
	case ASC:
		return "ASC"
	case DESC:
		return "DESC"
	case LIMIT:
		return "LIMIT"
	case OFFSET:
		return "OFFSET"
	case ASSIGN:
		return "="
	case NOT_EQ:
		return "!="
	case LT:
		return "<"
	case GT:
		return ">"
	case LT_EQ:
		return "<="
	case GT_EQ:
		return ">="
	case PLUS:
		return "+"
	case MINUS:
		return "-"
	case ASTERISK:
		return "*"
	case SLASH:
		return "/"
	case PERCENT:
		return "%"
	case COMMA:
		return ","
	case SEMICOLON:
		return ";"
	case LPAREN:
		return "("
	case RPAREN:
		return ")"
	case REGEX:
		return "REGEX"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", int(t))
	}
}

// Keywords maps string literals to their token types
var Keywords = map[string]TokenType{
	"SELECT":   SELECT,
	"FROM":     FROM,
	"WHERE":    WHERE,
	"AND":      AND,
	"OR":       OR,
	"NOT":      NOT,
	"LIKE":     LIKE,
	"BETWEEN":  BETWEEN,
	"CONTAINS": CONTAINS,
	"IN":       IN,
	"COUNT":    COUNT,
	"DISTINCT": DISTINCT,
	"ORDER":    ORDER,
	"BY":       BY,
	"ASC":      ASC,
	"DESC":     DESC,
	"LIMIT":    LIMIT,
	"OFFSET":   OFFSET,
	"REGEX":    REGEX,
}

// LookupIdent checks if an identifier is a keyword
func LookupIdent(ident string) TokenType {
	// Make case-insensitive lookup
	upperIdent := strings.ToUpper(ident)
	if tok, ok := Keywords[upperIdent]; ok {
		return tok
	}
	return IDENT
}
