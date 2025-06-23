package query

import (
	"fmt"
	"strconv"
	"strings"
)

// Parser performs syntax analysis on tokenized queries
type Parser struct {
	l *Lexer

	curToken  Token
	peekToken Token

	errors []string
}

// NewParser creates a new parser instance
func NewParser(l *Lexer) *Parser {
	p := &Parser{
		l:      l,
		errors: []string{},
	}

	// Read two tokens, so curToken and peekToken are both set
	p.nextToken()
	p.nextToken()

	return p
}

// Errors returns parser errors
func (p *Parser) Errors() []string {
	return p.errors
}

// nextToken advances token positions
func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

// ParseQuery parses a complete query and returns a Statement
func (p *Parser) ParseQuery() Statement {
	switch p.curToken.Type {
	case SELECT:
		return p.parseSelectStatement()
	default:
		p.addError(fmt.Sprintf("unexpected token %s, expected SELECT", p.curToken.Type))
		return nil
	}
}

// parseSelectStatement parses SELECT statements
func (p *Parser) parseSelectStatement() *SelectStatement {
	stmt := &SelectStatement{}

	// Current token should already be SELECT
	if p.curToken.Type != SELECT {
		p.addError(fmt.Sprintf("expected SELECT, got %s", p.curToken.Type))
		return nil
	}

	// Parse SELECT fields
	stmt.Fields = p.parseSelectFields()

	// Parse FROM clause
	if !p.expectPeek(FROM) {
		return nil
	}

	if !p.expectPeek(IDENT) {
		return nil
	}
	stmt.From = p.curToken.Literal

	// Parse optional WHERE clause
	if p.peekToken.Type == WHERE {
		p.nextToken()
		p.nextToken()
		stmt.Where = p.parseExpression()
	}

	// Parse optional ORDER BY clause
	if p.peekToken.Type == ORDER {
		stmt.OrderBy = p.parseOrderClause()
	}

	// Parse optional LIMIT clause
	if p.peekToken.Type == LIMIT {
		stmt.Limit = p.parseLimitClause()
	}

	return stmt
}

// parseSelectFields parses the field list in SELECT
func (p *Parser) parseSelectFields() []Expression {
	fields := []Expression{}

	p.nextToken()

	// Handle wildcard
	if p.curToken.Type == ASTERISK {
		fields = append(fields, &WildcardExpression{})
		return fields
	}

	// Handle function calls (COUNT, etc.)
	if (p.curToken.Type == IDENT || p.curToken.Type == COUNT) && p.peekToken.Type == LPAREN {
		fields = append(fields, p.parseFunctionExpression())

		// Handle comma-separated fields after function
		for p.peekToken.Type == COMMA {
			p.nextToken()
			p.nextToken()
			if (p.curToken.Type == IDENT || p.curToken.Type == COUNT) && p.peekToken.Type == LPAREN {
				fields = append(fields, p.parseFunctionExpression())
			} else if p.curToken.Type == IDENT {
				fields = append(fields, &Identifier{Value: p.curToken.Literal})
			}
		}
		return fields
	}

	// Handle regular identifiers
	if p.curToken.Type == IDENT {
		fields = append(fields, &Identifier{Value: p.curToken.Literal})
	}

	// Handle comma-separated fields
	for p.peekToken.Type == COMMA {
		p.nextToken()
		p.nextToken()
		if p.curToken.Type == IDENT {
			fields = append(fields, &Identifier{Value: p.curToken.Literal})
		}
	}

	return fields
}

// parseOrderClause parses ORDER BY clause
func (p *Parser) parseOrderClause() *OrderClause {
	if !p.expectPeek(ORDER) {
		return nil
	}
	if !p.expectPeek(BY) {
		return nil
	}
	if !p.expectPeek(IDENT) {
		return nil
	}

	orderBy := &OrderClause{
		Field:     p.curToken.Literal,
		Direction: "ASC", // default
	}

	// Check for ASC/DESC
	if p.peekToken.Type == ASC || p.peekToken.Type == DESC {
		p.nextToken()
		orderBy.Direction = strings.ToUpper(p.curToken.Literal)
	}

	return orderBy
}

// parseLimitClause parses LIMIT clause
func (p *Parser) parseLimitClause() *LimitClause {
	if !p.expectPeek(LIMIT) {
		return nil
	}
	if !p.expectPeek(NUMBER) {
		return nil
	}

	count, err := strconv.Atoi(p.curToken.Literal)
	if err != nil {
		p.addError(fmt.Sprintf("invalid LIMIT value: %s", p.curToken.Literal))
		return nil
	}

	limitClause := &LimitClause{Count: count}

	// Check for OFFSET
	if p.peekToken.Type == OFFSET {
		p.nextToken()
		if !p.expectPeek(NUMBER) {
			return nil
		}
		offset, err := strconv.Atoi(p.curToken.Literal)
		if err != nil {
			p.addError(fmt.Sprintf("invalid OFFSET value: %s", p.curToken.Literal))
			return nil
		}
		limitClause.Offset = offset
	}

	return limitClause
}

// parseExpression parses expressions with precedence
func (p *Parser) parseExpression() Expression {
	return p.parseOrExpression()
}

// parseOrExpression parses OR expressions (lowest precedence)
func (p *Parser) parseOrExpression() Expression {
	left := p.parseAndExpression()

	for p.peekToken.Type == OR {
		p.nextToken()
		operator := OR_OP
		p.nextToken()
		right := p.parseAndExpression()
		left = &BinaryExpression{Left: left, Operator: operator, Right: right}
	}

	return left
}

// parseAndExpression parses AND expressions
func (p *Parser) parseAndExpression() Expression {
	left := p.parseNotExpression()

	for p.peekToken.Type == AND {
		p.nextToken()
		operator := AND_OP
		p.nextToken()
		right := p.parseNotExpression()
		left = &BinaryExpression{Left: left, Operator: operator, Right: right}
	}

	return left
}

// parseNotExpression parses NOT expressions
func (p *Parser) parseNotExpression() Expression {
	if p.curToken.Type == NOT {
		operator := NOT_OP
		p.nextToken()
		right := p.parseComparisonExpression()
		return &UnaryExpression{Operator: operator, Right: right}
	}

	return p.parseComparisonExpression()
}

// parseComparisonExpression parses comparison expressions
func (p *Parser) parseComparisonExpression() Expression {
	left := p.parsePrimaryExpression()

	switch p.peekToken.Type {
	case ASSIGN:
		p.nextToken()
		operator := EQ
		p.nextToken()
		right := p.parsePrimaryExpression()
		return &BinaryExpression{Left: left, Operator: operator, Right: right}
	case NOT_EQ:
		p.nextToken()
		operator := NEQ
		p.nextToken()
		right := p.parsePrimaryExpression()
		return &BinaryExpression{Left: left, Operator: operator, Right: right}
	case LT:
		p.nextToken()
		operator := LT_OP
		p.nextToken()
		right := p.parsePrimaryExpression()
		return &BinaryExpression{Left: left, Operator: operator, Right: right}
	case GT:
		p.nextToken()
		operator := GT_OP
		p.nextToken()
		right := p.parsePrimaryExpression()
		return &BinaryExpression{Left: left, Operator: operator, Right: right}
	case LT_EQ:
		p.nextToken()
		operator := LTE
		p.nextToken()
		right := p.parsePrimaryExpression()
		return &BinaryExpression{Left: left, Operator: operator, Right: right}
	case GT_EQ:
		p.nextToken()
		operator := GTE
		p.nextToken()
		right := p.parsePrimaryExpression()
		return &BinaryExpression{Left: left, Operator: operator, Right: right}
	case LIKE:
		p.nextToken()
		operator := LIKE_OP
		p.nextToken()
		right := p.parsePrimaryExpression()
		return &BinaryExpression{Left: left, Operator: operator, Right: right}
	case CONTAINS:
		p.nextToken()
		operator := CONTAINS_OP
		p.nextToken()
		right := p.parsePrimaryExpression()
		return &BinaryExpression{Left: left, Operator: operator, Right: right}
	case REGEX:
		p.nextToken()
		operator := REGEX_OP
		p.nextToken()
		right := p.parsePrimaryExpression()
		return &BinaryExpression{Left: left, Operator: operator, Right: right}
	case BETWEEN:
		return p.parseBetweenExpression(left)
	case IN:
		return p.parseInExpression(left)
	}

	return left
}

// parseBetweenExpression parses BETWEEN expressions
func (p *Parser) parseBetweenExpression(field Expression) Expression {
	if !p.expectPeek(BETWEEN) {
		return nil
	}

	p.nextToken()
	start := p.parsePrimaryExpression()

	if !p.expectPeek(AND) {
		return nil
	}

	p.nextToken()
	end := p.parsePrimaryExpression()

	return &BetweenExpression{Field: field, Start: start, End: end}
}

// parseInExpression parses IN expressions
func (p *Parser) parseInExpression(field Expression) Expression {
	if !p.expectPeek(IN) {
		return nil
	}

	if !p.expectPeek(LPAREN) {
		return nil
	}

	values := []Expression{}
	p.nextToken()

	if p.curToken.Type != RPAREN {
		values = append(values, p.parsePrimaryExpression())

		for p.peekToken.Type == COMMA {
			p.nextToken()
			p.nextToken()
			values = append(values, p.parsePrimaryExpression())
		}
	}

	if !p.expectPeek(RPAREN) {
		return nil
	}

	return &InExpression{Field: field, Values: values}
}

// parseFunctionExpression parses function calls
func (p *Parser) parseFunctionExpression() Expression {
	name := p.curToken.Literal

	if !p.expectPeek(LPAREN) {
		return nil
	}

	args := []Expression{}
	p.nextToken()

	if p.curToken.Type != RPAREN {
		args = append(args, p.parsePrimaryExpression())

		for p.peekToken.Type == COMMA {
			p.nextToken()
			p.nextToken()
			args = append(args, p.parsePrimaryExpression())
		}
	}

	if !p.expectPeek(RPAREN) {
		return nil
	}

	return &FunctionExpression{Name: name, Arguments: args}
}

// parsePrimaryExpression parses primary expressions (identifiers, literals)
func (p *Parser) parsePrimaryExpression() Expression {
	switch p.curToken.Type {
	case IDENT:
		return &Identifier{Value: p.curToken.Literal}
	case STRING:
		return &StringLiteral{Value: p.curToken.Literal}
	case NUMBER:
		return &NumberLiteral{Value: p.curToken.Literal}
	case ASTERISK:
		return &WildcardExpression{}
	case LPAREN:
		p.nextToken()
		exp := p.parseExpression()
		if !p.expectPeek(RPAREN) {
			return nil
		}
		return exp
	default:
		p.addError(fmt.Sprintf("unexpected token %s in expression", p.curToken.Type))
		return nil
	}
}

// expectPeek checks if the next token is of expected type
func (p *Parser) expectPeek(t TokenType) bool {
	if p.peekToken.Type == t {
		p.nextToken()
		return true
	} else {
		p.addError(fmt.Sprintf("expected next token to be %s, got %s instead", t, p.peekToken.Type))
		return false
	}
}

// addError adds a parser error
func (p *Parser) addError(msg string) {
	p.errors = append(p.errors, msg)
}
