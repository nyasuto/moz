package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *Server) putKey(c *gin.Context) {
	start := time.Now()
	key := c.Param("key")

	var req PutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.errorResponse(c, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}

	if key == "" {
		s.errorResponse(c, http.StatusBadRequest, "INVALID_KEY", "Key cannot be empty")
		return
	}

	if err := s.store.Put(key, req.Value); err != nil {
		s.errorResponse(c, http.StatusInternalServerError, "PUT_FAILED", err.Error())
		return
	}

	s.successResponse(c, http.StatusOK, KVEntry{
		Key:       key,
		Value:     req.Value,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}, time.Since(start))
}

func (s *Server) getKey(c *gin.Context) {
	start := time.Now()
	key := c.Param("key")

	if key == "" {
		s.errorResponse(c, http.StatusBadRequest, "INVALID_KEY", "Key cannot be empty")
		return
	}

	value, err := s.store.Get(key)
	if err != nil {
		s.errorResponse(c, http.StatusNotFound, "KEY_NOT_FOUND", err.Error())
		return
	}

	s.successResponse(c, http.StatusOK, KVEntry{
		Key:   key,
		Value: value,
	}, time.Since(start))
}

func (s *Server) deleteKey(c *gin.Context) {
	start := time.Now()
	key := c.Param("key")

	if key == "" {
		s.errorResponse(c, http.StatusBadRequest, "INVALID_KEY", "Key cannot be empty")
		return
	}

	if err := s.store.Delete(key); err != nil {
		s.errorResponse(c, http.StatusNotFound, "KEY_NOT_FOUND", err.Error())
		return
	}

	s.successResponse(c, http.StatusOK, gin.H{
		"key":     key,
		"deleted": true,
	}, time.Since(start))
}

func (s *Server) listKeys(c *gin.Context) {
	start := time.Now()

	keys, err := s.store.List()
	if err != nil {
		s.errorResponse(c, http.StatusInternalServerError, "LIST_FAILED", err.Error())
		return
	}

	entries := make([]KVEntry, 0, len(keys))
	for _, key := range keys {
		if value, err := s.store.Get(key); err == nil {
			entries = append(entries, KVEntry{
				Key:   key,
				Value: value,
			})
		}
	}

	s.successResponse(c, http.StatusOK, gin.H{
		"keys":    keys,
		"count":   len(keys),
		"entries": entries,
	}, time.Since(start))
}

func (s *Server) successResponse(c *gin.Context, status int, data interface{}, duration time.Duration) {
	c.JSON(status, APIResponse{
		Status: "success",
		Data:   data,
		Metadata: &Metadata{
			Version:         "1.0",
			ExecutionTimeMs: float64(duration.Nanoseconds()) / 1e6,
			Timestamp:       time.Now().UTC().Format(time.RFC3339),
		},
	})
}

func (s *Server) errorResponse(c *gin.Context, status int, code, message string) {
	c.JSON(status, APIResponse{
		Status: "error",
		Error: &APIError{
			Code:    code,
			Message: message,
		},
		Metadata: &Metadata{
			Version:   "1.0",
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		},
	})
}
