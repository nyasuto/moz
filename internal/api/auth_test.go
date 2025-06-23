package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestLogin(t *testing.T) {
	server := NewServer("test.bin", "8080")
	defer os.Remove("test.bin")

	// Test successful login
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
		t.Errorf("Expected status 200, got %d", resp.Code)
	}

	var response APIResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.Status != "success" {
		t.Errorf("Expected success status, got %s", response.Status)
	}

	// Extract token from response
	loginData, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatal("Expected login data in response")
	}

	token, ok := loginData["token"].(string)
	if !ok || token == "" {
		t.Fatal("Expected token in login response")
	}

	// Test accessing protected endpoint with token
	req, _ = http.NewRequest("GET", "/api/v1/stats", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp = httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Protected endpoint with valid token: Expected status 200, got %d", resp.Code)
	}
}

func TestUnauthorizedAccess(t *testing.T) {
	server := NewServer("test.bin", "8080")
	defer os.Remove("test.bin")

	// Test accessing protected endpoint without token
	req, _ := http.NewRequest("GET", "/api/v1/stats", nil)
	resp := httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", resp.Code)
	}

	// Test accessing protected endpoint with invalid token
	req, _ = http.NewRequest("GET", "/api/v1/stats", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	resp = httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for invalid token, got %d", resp.Code)
	}
}

func TestHealthCheckNoAuth(t *testing.T) {
	server := NewServer("test.bin", "8080")
	defer os.Remove("test.bin")

	// Health check should not require authentication
	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	resp := httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Errorf("Health check should not require auth: Expected status 200, got %d", resp.Code)
	}
}

func TestInvalidCredentials(t *testing.T) {
	server := NewServer("test.bin", "8080")
	defer os.Remove("test.bin")

	// Test invalid credentials
	loginReq := LoginRequest{
		Username: "admin",
		Password: "wrong-password",
	}
	body, _ := json.Marshal(loginReq)
	req, _ := http.NewRequest("POST", "/api/v1/login", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401 for invalid credentials, got %d", resp.Code)
	}
}
