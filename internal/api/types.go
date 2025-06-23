package api

// APIResponse represents a standard API response
type APIResponse struct {
	Status   string      `json:"status"`
	Data     interface{} `json:"data,omitempty"`
	Metadata *Metadata   `json:"metadata,omitempty"`
	Error    *APIError   `json:"error,omitempty"`
}

// Metadata contains response metadata
type Metadata struct {
	Version         string  `json:"version"`
	RequestID       string  `json:"request_id,omitempty"`
	ExecutionTimeMs float64 `json:"execution_time_ms"`
	Timestamp       string  `json:"timestamp"`
}

// APIError represents an API error
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// KVEntry represents a key-value entry in API responses
type KVEntry struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Timestamp string `json:"timestamp,omitempty"`
}

// PutRequest represents a PUT request body
type PutRequest struct {
	Value string `json:"value" binding:"required"`
}

// BatchRequest represents a batch operation request
type BatchRequest struct {
	Operations []BatchOperation `json:"operations" binding:"required"`
}

// BatchOperation represents a single operation in a batch
type BatchOperation struct {
	Type  string `json:"type" binding:"required,oneof=PUT GET DELETE"`
	Key   string `json:"key" binding:"required"`
	Value string `json:"value,omitempty"`
}
