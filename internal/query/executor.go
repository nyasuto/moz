package query

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/nyasuto/moz/internal/kvstore"
)

// Executor executes parsed queries against the KV store
type Executor struct {
	store *kvstore.KVStore
}

// NewExecutor creates a new query executor
func NewExecutor(store *kvstore.KVStore) *Executor {
	return &Executor{store: store}
}

// ExecuteResult represents the result of query execution
type ExecuteResult struct {
	Rows  []map[string]string // Result rows
	Count int                 // Count for aggregation queries
	Error error               // Execution error
}

// Execute executes a parsed query statement
func (e *Executor) Execute(stmt Statement) *ExecuteResult {
	switch s := stmt.(type) {
	case *SelectStatement:
		return e.executeSelect(s)
	default:
		return &ExecuteResult{Error: fmt.Errorf("unsupported statement type")}
	}
}

// executeSelect executes SELECT statements
func (e *Executor) executeSelect(stmt *SelectStatement) *ExecuteResult {
	result := &ExecuteResult{Rows: []map[string]string{}}

	// Get all keys from store
	keys, err := e.store.List()
	if err != nil {
		return &ExecuteResult{Error: fmt.Errorf("failed to list keys: %v", err)}
	}

	// Build result set
	for _, key := range keys {
		value, err := e.store.Get(key)
		if err != nil {
			continue // Skip keys that can't be retrieved
		}

		row := map[string]string{
			"key":   key,
			"value": value,
		}

		// Apply WHERE clause filtering
		if stmt.Where != nil {
			if !e.evaluateExpression(stmt.Where, row) {
				continue
			}
		}

		// Check if this is an aggregation query
		if e.isAggregationQuery(stmt) {
			result.Count++
		} else {
			// Apply field selection
			filteredRow := e.applyFieldSelection(stmt.Fields, row)
			result.Rows = append(result.Rows, filteredRow)
		}
	}

	// Apply ORDER BY
	if stmt.OrderBy != nil && !e.isAggregationQuery(stmt) {
		e.applyOrderBy(result.Rows, stmt.OrderBy)
	}

	// Apply LIMIT
	if stmt.Limit != nil && !e.isAggregationQuery(stmt) {
		e.applyLimit(result, stmt.Limit)
	}

	return result
}

// isAggregationQuery checks if the query contains aggregation functions
func (e *Executor) isAggregationQuery(stmt *SelectStatement) bool {
	for _, field := range stmt.Fields {
		if _, ok := field.(*FunctionExpression); ok {
			return true
		}
	}
	return false
}

// applyFieldSelection filters the row based on selected fields
func (e *Executor) applyFieldSelection(fields []Expression, row map[string]string) map[string]string {
	if len(fields) == 0 {
		return row
	}

	// Check for wildcard
	for _, field := range fields {
		if _, ok := field.(*WildcardExpression); ok {
			return row
		}
	}

	// Select specific fields
	result := make(map[string]string)
	for _, field := range fields {
		if ident, ok := field.(*Identifier); ok {
			if value, exists := row[ident.Value]; exists {
				result[ident.Value] = value
			}
		}
	}

	return result
}

// evaluateExpression evaluates WHERE clause expressions
func (e *Executor) evaluateExpression(expr Expression, row map[string]string) bool {
	switch exp := expr.(type) {
	case *BinaryExpression:
		return e.evaluateBinaryExpression(exp, row)
	case *UnaryExpression:
		return e.evaluateUnaryExpression(exp, row)
	case *BetweenExpression:
		return e.evaluateBetweenExpression(exp, row)
	case *InExpression:
		return e.evaluateInExpression(exp, row)
	default:
		return false
	}
}

// evaluateBinaryExpression evaluates binary expressions
func (e *Executor) evaluateBinaryExpression(expr *BinaryExpression, row map[string]string) bool {
	switch expr.Operator {
	case AND_OP:
		return e.evaluateExpression(expr.Left, row) && e.evaluateExpression(expr.Right, row)
	case OR_OP:
		return e.evaluateExpression(expr.Left, row) || e.evaluateExpression(expr.Right, row)
	default:
		return e.evaluateComparison(expr, row)
	}
}

// evaluateUnaryExpression evaluates unary expressions
func (e *Executor) evaluateUnaryExpression(expr *UnaryExpression, row map[string]string) bool {
	switch expr.Operator {
	case NOT_OP:
		return !e.evaluateExpression(expr.Right, row)
	default:
		return false
	}
}

// evaluateComparison evaluates comparison operations
func (e *Executor) evaluateComparison(expr *BinaryExpression, row map[string]string) bool {
	leftValue := e.getValueFromExpression(expr.Left, row)
	rightValue := e.getValueFromExpression(expr.Right, row)

	switch expr.Operator {
	case EQ:
		return leftValue == rightValue
	case NEQ:
		return leftValue != rightValue
	case LT_OP:
		return e.compareValues(leftValue, rightValue) < 0
	case GT_OP:
		return e.compareValues(leftValue, rightValue) > 0
	case LTE:
		return e.compareValues(leftValue, rightValue) <= 0
	case GTE:
		return e.compareValues(leftValue, rightValue) >= 0
	case LIKE_OP:
		return e.evaluateLike(leftValue, rightValue)
	case CONTAINS_OP:
		return strings.Contains(leftValue, rightValue)
	case REGEX_OP:
		return e.evaluateRegex(leftValue, rightValue)
	default:
		return false
	}
}

// evaluateBetweenExpression evaluates BETWEEN expressions
func (e *Executor) evaluateBetweenExpression(expr *BetweenExpression, row map[string]string) bool {
	fieldValue := e.getValueFromExpression(expr.Field, row)
	startValue := e.getValueFromExpression(expr.Start, row)
	endValue := e.getValueFromExpression(expr.End, row)

	return e.compareValues(fieldValue, startValue) >= 0 && e.compareValues(fieldValue, endValue) <= 0
}

// evaluateInExpression evaluates IN expressions
func (e *Executor) evaluateInExpression(expr *InExpression, row map[string]string) bool {
	fieldValue := e.getValueFromExpression(expr.Field, row)

	for _, valueExpr := range expr.Values {
		if fieldValue == e.getValueFromExpression(valueExpr, row) {
			return true
		}
	}
	return false
}

// getValueFromExpression extracts the actual value from an expression
func (e *Executor) getValueFromExpression(expr Expression, row map[string]string) string {
	switch exp := expr.(type) {
	case *Identifier:
		if value, exists := row[exp.Value]; exists {
			return value
		}
		return ""
	case *StringLiteral:
		return exp.Value
	case *NumberLiteral:
		return exp.Value
	default:
		return ""
	}
}

// compareValues compares two string values (tries numeric comparison first)
func (e *Executor) compareValues(left, right string) int {
	// Try numeric comparison first
	if leftNum, err1 := strconv.ParseFloat(left, 64); err1 == nil {
		if rightNum, err2 := strconv.ParseFloat(right, 64); err2 == nil {
			if leftNum < rightNum {
				return -1
			} else if leftNum > rightNum {
				return 1
			} else {
				return 0
			}
		}
	}

	// Fall back to string comparison
	return strings.Compare(left, right)
}

// evaluateLike evaluates LIKE pattern matching
func (e *Executor) evaluateLike(value, pattern string) bool {
	// Convert SQL LIKE pattern to regex
	regexPattern := strings.ReplaceAll(pattern, "%", ".*")
	regexPattern = strings.ReplaceAll(regexPattern, "_", ".")
	regexPattern = "^" + regexPattern + "$"

	matched, err := regexp.MatchString(regexPattern, value)
	return err == nil && matched
}

// evaluateRegex evaluates regex pattern matching
func (e *Executor) evaluateRegex(value, pattern string) bool {
	matched, err := regexp.MatchString(pattern, value)
	return err == nil && matched
}

// applyOrderBy sorts the result rows
func (e *Executor) applyOrderBy(rows []map[string]string, orderBy *OrderClause) {
	if len(rows) <= 1 {
		return
	}

	// Simple bubble sort for demonstration (could be optimized)
	for i := 0; i < len(rows)-1; i++ {
		for j := 0; j < len(rows)-i-1; j++ {
			val1 := rows[j][orderBy.Field]
			val2 := rows[j+1][orderBy.Field]

			var shouldSwap bool
			if orderBy.Direction == "ASC" {
				shouldSwap = e.compareValues(val1, val2) > 0
			} else {
				shouldSwap = e.compareValues(val1, val2) < 0
			}

			if shouldSwap {
				rows[j], rows[j+1] = rows[j+1], rows[j]
			}
		}
	}
}

// applyLimit applies LIMIT and OFFSET to the result
func (e *Executor) applyLimit(result *ExecuteResult, limitClause *LimitClause) {
	start := limitClause.Offset
	end := start + limitClause.Count

	if start >= len(result.Rows) {
		result.Rows = []map[string]string{}
		return
	}

	if end > len(result.Rows) {
		end = len(result.Rows)
	}

	result.Rows = result.Rows[start:end]
}
