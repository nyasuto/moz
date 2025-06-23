package query

import (
	"unicode"
	"unicode/utf8"
)

// Lexer performs lexical analysis on query strings
type Lexer struct {
	input        string
	position     int  // current position in input (points to current char)
	readPosition int  // current reading position in input (after current char)
	ch           byte // current char under examination
}

// NewLexer creates a new lexer instance
func NewLexer(input string) *Lexer {
	l := &Lexer{input: input}
	l.readChar()
	return l
}

// readChar reads the next character and advances position
func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0 // ASCII NUL character represents "EOF"
	} else {
		l.ch = l.input[l.readPosition]
	}
	l.position = l.readPosition
	l.readPosition++
}

// peekChar returns the next character without advancing position
func (l *Lexer) peekChar() byte {
	if l.readPosition >= len(l.input) {
		return 0
	}
	return l.input[l.readPosition]
}

// NextToken scans the input and returns the next token
func (l *Lexer) NextToken() Token {
	var tok Token

	l.skipWhitespace()

	switch l.ch {
	case '=':
		tok = newToken(ASSIGN, l.ch, l.position)
	case '!':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: NOT_EQ, Literal: string(ch) + string(l.ch), Position: l.position - 1}
		} else {
			tok = newToken(ILLEGAL, l.ch, l.position)
		}
	case '<':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: LT_EQ, Literal: string(ch) + string(l.ch), Position: l.position - 1}
		} else {
			tok = newToken(LT, l.ch, l.position)
		}
	case '>':
		if l.peekChar() == '=' {
			ch := l.ch
			l.readChar()
			tok = Token{Type: GT_EQ, Literal: string(ch) + string(l.ch), Position: l.position - 1}
		} else {
			tok = newToken(GT, l.ch, l.position)
		}
	case '+':
		tok = newToken(PLUS, l.ch, l.position)
	case '-':
		tok = newToken(MINUS, l.ch, l.position)
	case '*':
		tok = newToken(ASTERISK, l.ch, l.position)
	case '/':
		tok = newToken(SLASH, l.ch, l.position)
	case '%':
		tok = newToken(PERCENT, l.ch, l.position)
	case ',':
		tok = newToken(COMMA, l.ch, l.position)
	case ';':
		tok = newToken(SEMICOLON, l.ch, l.position)
	case '(':
		tok = newToken(LPAREN, l.ch, l.position)
	case ')':
		tok = newToken(RPAREN, l.ch, l.position)
	case '"':
		tok.Type = STRING
		tok.Literal = l.readString()
		tok.Position = l.position
	case '\'':
		tok.Type = STRING
		tok.Literal = l.readSingleQuotedString()
		tok.Position = l.position
	case 0:
		tok.Literal = ""
		tok.Type = EOF
		tok.Position = l.position
	default:
		if isLetter(l.ch) {
			tok.Position = l.position
			tok.Literal = l.readIdentifier()
			tok.Type = LookupIdent(tok.Literal)
			return tok // early return to avoid readChar()
		} else if isDigit(l.ch) {
			tok.Type = NUMBER
			tok.Literal = l.readNumber()
			tok.Position = l.position
			return tok // early return to avoid readChar()
		} else {
			tok = newToken(ILLEGAL, l.ch, l.position)
		}
	}

	l.readChar()
	return tok
}

// newToken creates a new token
func newToken(tokenType TokenType, ch byte, position int) Token {
	return Token{Type: tokenType, Literal: string(ch), Position: position}
}

// readIdentifier reads identifier (keywords and field names)
func (l *Lexer) readIdentifier() string {
	position := l.position
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}
	return l.input[position:l.position]
}

// readNumber reads numeric literals
func (l *Lexer) readNumber() string {
	position := l.position
	for isDigit(l.ch) {
		l.readChar()
	}
	// Support decimal numbers
	if l.ch == '.' && isDigit(l.peekChar()) {
		l.readChar()
		for isDigit(l.ch) {
			l.readChar()
		}
	}
	return l.input[position:l.position]
}

// readString reads double-quoted string literals
func (l *Lexer) readString() string {
	position := l.position + 1
	for {
		l.readChar()
		if l.ch == '"' || l.ch == 0 {
			break
		}
		// Handle escaped quotes
		if l.ch == '\\' && l.peekChar() == '"' {
			l.readChar() // skip backslash
			l.readChar() // skip escaped quote
		}
	}
	return l.input[position:l.position]
}

// readSingleQuotedString reads single-quoted string literals
func (l *Lexer) readSingleQuotedString() string {
	position := l.position + 1
	for {
		l.readChar()
		if l.ch == '\'' || l.ch == 0 {
			break
		}
		// Handle escaped quotes
		if l.ch == '\\' && l.peekChar() == '\'' {
			l.readChar() // skip backslash
			l.readChar() // skip escaped quote
		}
	}
	return l.input[position:l.position]
}

// skipWhitespace skips whitespace characters
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.readChar()
	}
}

// isLetter checks if a character is a letter
func isLetter(ch byte) bool {
	r, _ := utf8.DecodeRune([]byte{ch})
	return unicode.IsLetter(r) || ch == '_'
}

// isDigit checks if a character is a digit
func isDigit(ch byte) bool {
	return '0' <= ch && ch <= '9'
}
