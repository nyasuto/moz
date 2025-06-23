package query

import (
	"fmt"
	"strings"
)

// Node represents a node in the Abstract Syntax Tree
type Node interface {
	String() string
}

// Statement represents a SQL statement
type Statement interface {
	Node
	statementNode()
}

// Expression represents an expression in the query
type Expression interface {
	Node
	expressionNode()
}

// Operator represents comparison and logical operators
type Operator int

const (
	UNKNOWN_OP  Operator = iota
	EQ                   // =
	NEQ                  // !=
	LT_OP                // <
	GT_OP                // >
	LTE                  // <=
	GTE                  // >=
	LIKE_OP              // LIKE
	CONTAINS_OP          // CONTAINS
	BETWEEN_OP           // BETWEEN
	IN_OP                // IN
	REGEX_OP             // REGEX
	AND_OP               // AND
	OR_OP                // OR
	NOT_OP               // NOT
)

// String returns string representation of operator
func (op Operator) String() string {
	switch op {
	case EQ:
		return "="
	case NEQ:
		return "!="
	case LT_OP:
		return "<"
	case GT_OP:
		return ">"
	case LTE:
		return "<="
	case GTE:
		return ">="
	case LIKE_OP:
		return "LIKE"
	case CONTAINS_OP:
		return "CONTAINS"
	case BETWEEN_OP:
		return "BETWEEN"
	case IN_OP:
		return "IN"
	case REGEX_OP:
		return "REGEX"
	case AND_OP:
		return "AND"
	case OR_OP:
		return "OR"
	case NOT_OP:
		return "NOT"
	default:
		return "UNKNOWN"
	}
}

// SelectStatement represents a SELECT query
type SelectStatement struct {
	Fields  []Expression // SELECT fields (* or specific fields)
	From    string       // FROM table (always "moz" for our case)
	Where   Expression   // WHERE clause
	OrderBy *OrderClause // ORDER BY clause
	Limit   *LimitClause // LIMIT clause
}

func (ss *SelectStatement) statementNode() {}
func (ss *SelectStatement) String() string {
	var out strings.Builder

	out.WriteString("SELECT ")
	if len(ss.Fields) > 0 {
		fields := make([]string, len(ss.Fields))
		for i, field := range ss.Fields {
			fields[i] = field.String()
		}
		out.WriteString(strings.Join(fields, ", "))
	}

	if ss.From != "" {
		out.WriteString(" FROM ")
		out.WriteString(ss.From)
	}

	if ss.Where != nil {
		out.WriteString(" WHERE ")
		out.WriteString(ss.Where.String())
	}

	if ss.OrderBy != nil {
		out.WriteString(" ")
		out.WriteString(ss.OrderBy.String())
	}

	if ss.Limit != nil {
		out.WriteString(" ")
		out.WriteString(ss.Limit.String())
	}

	return out.String()
}

// OrderClause represents ORDER BY clause
type OrderClause struct {
	Field     string
	Direction string // ASC or DESC
}

func (oc *OrderClause) String() string {
	return fmt.Sprintf("ORDER BY %s %s", oc.Field, oc.Direction)
}

// LimitClause represents LIMIT clause
type LimitClause struct {
	Count  int
	Offset int
}

func (lc *LimitClause) String() string {
	if lc.Offset > 0 {
		return fmt.Sprintf("LIMIT %d OFFSET %d", lc.Count, lc.Offset)
	}
	return fmt.Sprintf("LIMIT %d", lc.Count)
}

// Identifier represents field names (key, value)
type Identifier struct {
	Value string
}

func (i *Identifier) expressionNode() {}
func (i *Identifier) String() string  { return i.Value }

// StringLiteral represents string values
type StringLiteral struct {
	Value string
}

func (sl *StringLiteral) expressionNode() {}
func (sl *StringLiteral) String() string  { return fmt.Sprintf("\"%s\"", sl.Value) }

// NumberLiteral represents numeric values
type NumberLiteral struct {
	Value string
}

func (nl *NumberLiteral) expressionNode() {}
func (nl *NumberLiteral) String() string  { return nl.Value }

// WildcardExpression represents * in SELECT
type WildcardExpression struct{}

func (we *WildcardExpression) expressionNode() {}
func (we *WildcardExpression) String() string  { return "*" }

// BinaryExpression represents binary operations (field = value, etc.)
type BinaryExpression struct {
	Left     Expression
	Operator Operator
	Right    Expression
}

func (be *BinaryExpression) expressionNode() {}
func (be *BinaryExpression) String() string {
	return fmt.Sprintf("(%s %s %s)", be.Left.String(), be.Operator.String(), be.Right.String())
}

// UnaryExpression represents unary operations (NOT condition)
type UnaryExpression struct {
	Operator Operator
	Right    Expression
}

func (ue *UnaryExpression) expressionNode() {}
func (ue *UnaryExpression) String() string {
	return fmt.Sprintf("(%s %s)", ue.Operator.String(), ue.Right.String())
}

// BetweenExpression represents BETWEEN operations
type BetweenExpression struct {
	Field Expression
	Start Expression
	End   Expression
}

func (be *BetweenExpression) expressionNode() {}
func (be *BetweenExpression) String() string {
	return fmt.Sprintf("%s BETWEEN %s AND %s", be.Field.String(), be.Start.String(), be.End.String())
}

// InExpression represents IN operations
type InExpression struct {
	Field  Expression
	Values []Expression
}

func (ie *InExpression) expressionNode() {}
func (ie *InExpression) String() string {
	values := make([]string, len(ie.Values))
	for i, v := range ie.Values {
		values[i] = v.String()
	}
	return fmt.Sprintf("%s IN (%s)", ie.Field.String(), strings.Join(values, ", "))
}

// FunctionExpression represents function calls (COUNT, etc.)
type FunctionExpression struct {
	Name      string
	Arguments []Expression
}

func (fe *FunctionExpression) expressionNode() {}
func (fe *FunctionExpression) String() string {
	if len(fe.Arguments) == 0 {
		return fmt.Sprintf("%s()", fe.Name)
	}
	args := make([]string, len(fe.Arguments))
	for i, arg := range fe.Arguments {
		args[i] = arg.String()
	}
	return fmt.Sprintf("%s(%s)", fe.Name, strings.Join(args, ", "))
}
