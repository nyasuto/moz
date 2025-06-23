package batch

import (
	"fmt"
	"strings"
	"time"

	"github.com/nyasuto/moz/internal/kvstore"
)

// Operation represents a single batch operation
type Operation struct {
	Type      string   `json:"type"`
	Arguments []string `json:"arguments"`
}

// BatchResult represents the result of a batch operation
type BatchResult struct {
	Success   bool          `json:"success"`
	Error     string        `json:"error,omitempty"`
	Result    interface{}   `json:"result,omitempty"`
	Duration  time.Duration `json:"duration"`
	Operation Operation     `json:"operation"`
}

// BatchExecutor executes batch operations
type BatchExecutor struct {
	store *kvstore.KVStore
}

// NewBatchExecutor creates a new batch executor
func NewBatchExecutor(store *kvstore.KVStore) *BatchExecutor {
	return &BatchExecutor{
		store: store,
	}
}

// ParseBatchCommand parses a batch command string into operations
func ParseBatchCommand(args []string) ([]Operation, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("no batch operations specified")
	}

	var operations []Operation
	i := 0

	for i < len(args) {
		if i >= len(args) {
			break
		}

		command := args[i]
		i++

		switch command {
		case "put":
			if i+1 >= len(args) {
				return nil, fmt.Errorf("put operation requires key and value")
			}
			operations = append(operations, Operation{
				Type:      "put",
				Arguments: []string{args[i], args[i+1]},
			})
			i += 2

		case "get":
			if i >= len(args) {
				return nil, fmt.Errorf("get operation requires key")
			}
			operations = append(operations, Operation{
				Type:      "get",
				Arguments: []string{args[i]},
			})
			i++

		case "delete", "del":
			if i >= len(args) {
				return nil, fmt.Errorf("delete operation requires key")
			}
			operations = append(operations, Operation{
				Type:      "delete",
				Arguments: []string{args[i]},
			})
			i++

		case "list":
			operations = append(operations, Operation{
				Type:      "list",
				Arguments: []string{},
			})

		case "compact":
			operations = append(operations, Operation{
				Type:      "compact",
				Arguments: []string{},
			})

		case "stats":
			operations = append(operations, Operation{
				Type:      "stats",
				Arguments: []string{},
			})

		default:
			return nil, fmt.Errorf("unknown batch command: %s", command)
		}
	}

	return operations, nil
}

// Execute executes a batch of operations
func (be *BatchExecutor) Execute(operations []Operation) []BatchResult {
	results := make([]BatchResult, len(operations))

	for i, op := range operations {
		start := time.Now()
		result := be.executeOperation(op)
		result.Duration = time.Since(start)
		result.Operation = op
		results[i] = result
	}

	return results
}

// ExecuteTransactional executes operations as a transaction (all or nothing)
func (be *BatchExecutor) ExecuteTransactional(operations []Operation) ([]BatchResult, error) {
	// For now, implement simple all-or-nothing semantics
	// In a real implementation, this would use WAL or transaction log

	// First, validate all operations
	for _, op := range operations {
		if err := be.validateOperation(op); err != nil {
			return nil, fmt.Errorf("operation validation failed: %w", err)
		}
	}

	// Execute all operations
	results := be.Execute(operations)

	// Check if any operation failed
	for _, result := range results {
		if !result.Success {
			// In a real implementation, we would rollback here
			return results, fmt.Errorf("batch transaction failed: %s", result.Error)
		}
	}

	return results, nil
}

// executeOperation executes a single operation
func (be *BatchExecutor) executeOperation(op Operation) BatchResult {
	switch op.Type {
	case "put":
		if len(op.Arguments) != 2 {
			return BatchResult{
				Success: false,
				Error:   "put requires exactly 2 arguments: key and value",
			}
		}
		err := be.store.Put(op.Arguments[0], op.Arguments[1])
		if err != nil {
			return BatchResult{
				Success: false,
				Error:   err.Error(),
			}
		}
		return BatchResult{
			Success: true,
			Result:  "OK",
		}

	case "get":
		if len(op.Arguments) != 1 {
			return BatchResult{
				Success: false,
				Error:   "get requires exactly 1 argument: key",
			}
		}
		value, err := be.store.Get(op.Arguments[0])
		if err != nil {
			return BatchResult{
				Success: false,
				Error:   err.Error(),
			}
		}
		return BatchResult{
			Success: true,
			Result:  value,
		}

	case "delete":
		if len(op.Arguments) != 1 {
			return BatchResult{
				Success: false,
				Error:   "delete requires exactly 1 argument: key",
			}
		}
		err := be.store.Delete(op.Arguments[0])
		if err != nil {
			return BatchResult{
				Success: false,
				Error:   err.Error(),
			}
		}
		return BatchResult{
			Success: true,
			Result:  "OK",
		}

	case "list":
		entries, err := be.store.List()
		if err != nil {
			return BatchResult{
				Success: false,
				Error:   err.Error(),
			}
		}
		return BatchResult{
			Success: true,
			Result:  entries,
		}

	case "compact":
		err := be.store.Compact()
		if err != nil {
			return BatchResult{
				Success: false,
				Error:   err.Error(),
			}
		}
		return BatchResult{
			Success: true,
			Result:  "Compaction completed",
		}

	case "stats":
		stats, err := be.store.GetCompactionStats()
		if err != nil {
			return BatchResult{
				Success: false,
				Error:   err.Error(),
			}
		}
		return BatchResult{
			Success: true,
			Result:  stats,
		}

	default:
		return BatchResult{
			Success: false,
			Error:   fmt.Sprintf("unknown operation type: %s", op.Type),
		}
	}
}

// validateOperation validates an operation before execution
func (be *BatchExecutor) validateOperation(op Operation) error {
	switch op.Type {
	case "put":
		if len(op.Arguments) != 2 {
			return fmt.Errorf("put requires exactly 2 arguments")
		}
		if strings.TrimSpace(op.Arguments[0]) == "" {
			return fmt.Errorf("key cannot be empty")
		}

	case "get", "delete":
		if len(op.Arguments) != 1 {
			return fmt.Errorf("%s requires exactly 1 argument", op.Type)
		}
		if strings.TrimSpace(op.Arguments[0]) == "" {
			return fmt.Errorf("key cannot be empty")
		}

	case "list", "compact", "stats":
		if len(op.Arguments) != 0 {
			return fmt.Errorf("%s requires no arguments", op.Type)
		}

	default:
		return fmt.Errorf("unknown operation type: %s", op.Type)
	}

	return nil
}

// BatchSummary provides summary statistics for batch execution
type BatchSummary struct {
	TotalOperations  int           `json:"total_operations"`
	SuccessfulOps    int           `json:"successful_operations"`
	FailedOps        int           `json:"failed_operations"`
	TotalDuration    time.Duration `json:"total_duration"`
	AverageDuration  time.Duration `json:"average_duration"`
	OperationsPerSec float64       `json:"operations_per_second"`
}

// GenerateSummary generates a summary for batch results
func GenerateSummary(results []BatchResult) BatchSummary {
	summary := BatchSummary{
		TotalOperations: len(results),
	}

	var totalDuration time.Duration

	for _, result := range results {
		totalDuration += result.Duration
		if result.Success {
			summary.SuccessfulOps++
		} else {
			summary.FailedOps++
		}
	}

	summary.TotalDuration = totalDuration

	if len(results) > 0 {
		summary.AverageDuration = totalDuration / time.Duration(len(results))

		if totalDuration > 0 {
			summary.OperationsPerSec = float64(len(results)) / totalDuration.Seconds()
		}
	}

	return summary
}
