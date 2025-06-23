package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
)

func getAuthToken(t *testing.T, server *Server) string {
	loginReq := LoginRequest{
		Username: "admin",
		Password: "password",
	}
	body, _ := json.Marshal(loginReq)
	req, _ := http.NewRequest("POST", "/api/v1/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("Login failed: %d", resp.Code)
	}

	var response APIResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal login response: %v", err)
	}

	loginData, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Expected login data in response")
	}

	token, ok := loginData["token"].(string)
	if !ok || token == "" {
		t.Fatal("Expected token in login response")
	}

	return token
}

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func TestHealthCheck(t *testing.T) {
	server := NewServer("test.bin", "8080")
	defer os.Remove("test.bin")

	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	resp := httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(resp.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got %v", response["status"])
	}
}

func TestPutAndGet(t *testing.T) {
	server := NewServer("test.bin", "8080")
	defer os.Remove("test.bin")

	// Get auth token first
	token := getAuthToken(t, server)

	// Test PUT
	putReq := PutRequest{Value: "test-value"}
	putBody, _ := json.Marshal(putReq)
	req, _ := http.NewRequest("PUT", "/api/v1/kv/test-key", bytes.NewBuffer(putBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("PUT: Expected status 200, got %d", resp.Code)
	}

	// Test GET
	req, _ = http.NewRequest("GET", "/api/v1/kv/test-key", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp = httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("GET: Expected status 200, got %d", resp.Code)
	}

	var response APIResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Status != "success" {
		t.Errorf("Expected success status, got %s", response.Status)
	}
}

func TestGetNonExistentKey(t *testing.T) {
	server := NewServer("test.bin", "8080")
	defer os.Remove("test.bin")

	token := getAuthToken(t, server)

	req, _ := http.NewRequest("GET", "/api/v1/kv/nonexistent", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", resp.Code)
	}

	var response APIResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Status != "error" {
		t.Errorf("Expected error status, got %s", response.Status)
	}
}

func TestDelete(t *testing.T) {
	server := NewServer("test.bin", "8080")
	defer os.Remove("test.bin")

	token := getAuthToken(t, server)

	// First PUT a key
	putReq := PutRequest{Value: "test-value"}
	putBody, _ := json.Marshal(putReq)
	req, _ := http.NewRequest("PUT", "/api/v1/kv/test-key", bytes.NewBuffer(putBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)

	// Then DELETE it
	req, _ = http.NewRequest("DELETE", "/api/v1/kv/test-key", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp = httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("DELETE: Expected status 200, got %d", resp.Code)
	}

	// Verify it's gone
	req, _ = http.NewRequest("GET", "/api/v1/kv/test-key", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp = httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNotFound {
		t.Errorf("GET after DELETE: Expected status 404, got %d", resp.Code)
	}
}

func TestList(t *testing.T) {
	server := NewServer("test.bin", "8080")
	defer os.Remove("test.bin")

	token := getAuthToken(t, server)

	// Add some keys
	keys := []string{"key1", "key2", "key3"}
	for _, key := range keys {
		putReq := PutRequest{Value: "value-" + key}
		putBody, _ := json.Marshal(putReq)
		req, _ := http.NewRequest("PUT", "/api/v1/kv/"+key, bytes.NewBuffer(putBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp := httptest.NewRecorder()
		server.router.ServeHTTP(resp, req)
	}

	// Test LIST
	req, _ := http.NewRequest("GET", "/api/v1/kv", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp := httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("LIST: Expected status 200, got %d", resp.Code)
	}

	var response APIResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Status != "success" {
		t.Errorf("Expected success status, got %s", response.Status)
	}
}
