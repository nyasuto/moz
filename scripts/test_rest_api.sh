#!/bin/bash

# REST API Integration Test Script
# Tests all endpoints of the moz REST API server

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVER_HOST="${SERVER_HOST:-localhost}"
SERVER_PORT="${SERVER_PORT:-8080}"
BASE_URL="http://${SERVER_HOST}:${SERVER_PORT}"
API_BASE="${BASE_URL}/api/v1"

# Test results
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Utility functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
    ((PASSED_TESTS++))
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
    ((FAILED_TESTS++))
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

increment_test() {
    ((TOTAL_TESTS++))
}

# Test functions
test_health_check() {
    log_info "Testing health check endpoint..."
    increment_test
    
    # Make the request and capture the HTTP status code
    http_code=$(curl -s -w "%{http_code}" -o /tmp/health_response.json "${API_BASE}/health" 2>/dev/null)
    
    if [ "$http_code" = "200" ]; then
        if grep -q '"status".*:.*"ok"' /tmp/health_response.json 2>/dev/null; then
            log_success "Health check endpoint working"
        else
            log_error "Health check returned invalid response"
            cat /tmp/health_response.json 2>/dev/null || echo "No response file"
        fi
    else
        log_error "Health check failed (HTTP $http_code)"
        cat /tmp/health_response.json 2>/dev/null || echo "No response body"
    fi
}

test_login() {
    log_info "Testing login endpoint..."
    increment_test
    
    login_data='{"username":"admin","password":"password"}'
    http_code=$(curl -s -w "%{http_code}" -o /tmp/login_response.json \
        -X POST "${API_BASE}/login" \
        -H "Content-Type: application/json" \
        -d "$login_data" 2>/dev/null)
    
    if [ "$http_code" = "200" ]; then
        if grep -q '"token"' /tmp/login_response.json 2>/dev/null; then
            JWT_TOKEN=$(grep -o '"token":"[^"]*"' /tmp/login_response.json | cut -d'"' -f4)
            log_success "Login successful, token obtained"
        else
            log_error "Login successful but no token in response"
        fi
    else
        log_error "Login failed (HTTP $http_code)"
    fi
}

test_invalid_login() {
    log_info "Testing invalid login..."
    increment_test
    
    invalid_data='{"username":"admin","password":"wrong"}'
    response=$(curl -s -w "%{http_code}" -o /tmp/invalid_login.json \
        -X POST "${API_BASE}/login" \
        -H "Content-Type: application/json" \
        -d "$invalid_data" 2>/dev/null || echo "000")
    
    if [ "$response" = "401" ]; then
        log_success "Invalid login correctly rejected"
    else
        log_error "Invalid login should return 401 (got HTTP $response)"
    fi
}

test_unauthorized_access() {
    log_info "Testing unauthorized access..."
    increment_test
    
    response=$(curl -s -w "%{http_code}" -o /tmp/unauth.json \
        "${API_BASE}/stats" 2>/dev/null || echo "000")
    
    if [ "$response" = "401" ]; then
        log_success "Unauthorized access correctly blocked"
    else
        log_error "Unauthorized access should return 401 (got HTTP $response)"
    fi
}

test_put_data() {
    log_info "Testing PUT endpoint..."
    increment_test
    
    if [ -z "$JWT_TOKEN" ]; then
        log_error "No JWT token available for PUT test"
        return
    fi
    
    put_data='{"value":"test-value-123"}'
    response=$(curl -s -w "%{http_code}" -o /tmp/put_response.json \
        -X PUT "${API_BASE}/kv/test-key" \
        -H "Authorization: Bearer $JWT_TOKEN" \
        -H "Content-Type: application/json" \
        -d "$put_data" 2>/dev/null || echo "000")
    
    if [ "$response" = "200" ]; then
        if grep -q '"status".*:.*"success"' /tmp/put_response.json 2>/dev/null; then
            log_success "PUT request successful"
        else
            log_error "PUT returned 200 but invalid response format"
        fi
    else
        log_error "PUT failed (HTTP $response)"
    fi
}

test_get_data() {
    log_info "Testing GET endpoint..."
    increment_test
    
    if [ -z "$JWT_TOKEN" ]; then
        log_error "No JWT token available for GET test"
        return
    fi
    
    response=$(curl -s -w "%{http_code}" -o /tmp/get_response.json \
        -X GET "${API_BASE}/kv/test-key" \
        -H "Authorization: Bearer $JWT_TOKEN" 2>/dev/null || echo "000")
    
    if [ "$response" = "200" ]; then
        if grep -q '"value":"test-value-123"' /tmp/get_response.json 2>/dev/null; then
            log_success "GET request returned correct value"
        else
            log_error "GET returned 200 but wrong value"
        fi
    else
        log_error "GET failed (HTTP $response)"
    fi
}

test_get_nonexistent() {
    log_info "Testing GET for non-existent key..."
    increment_test
    
    if [ -z "$JWT_TOKEN" ]; then
        log_error "No JWT token available for GET test"
        return
    fi
    
    response=$(curl -s -w "%{http_code}" -o /tmp/get_404.json \
        -X GET "${API_BASE}/kv/nonexistent-key" \
        -H "Authorization: Bearer $JWT_TOKEN" 2>/dev/null || echo "000")
    
    if [ "$response" = "404" ]; then
        log_success "GET for non-existent key correctly returns 404"
    else
        log_error "GET for non-existent key should return 404 (got HTTP $response)"
    fi
}

test_list_data() {
    log_info "Testing LIST endpoint..."
    increment_test
    
    if [ -z "$JWT_TOKEN" ]; then
        log_error "No JWT token available for LIST test"
        return
    fi
    
    response=$(curl -s -w "%{http_code}" -o /tmp/list_response.json \
        -X GET "${API_BASE}/kv" \
        -H "Authorization: Bearer $JWT_TOKEN" 2>/dev/null || echo "000")
    
    if [ "$response" = "200" ]; then
        if grep -q '"keys"' /tmp/list_response.json 2>/dev/null; then
            log_success "LIST request successful"
        else
            log_error "LIST returned 200 but invalid format"
        fi
    else
        log_error "LIST failed (HTTP $response)"
    fi
}

test_stats() {
    log_info "Testing STATS endpoint..."
    increment_test
    
    if [ -z "$JWT_TOKEN" ]; then
        log_error "No JWT token available for STATS test"
        return
    fi
    
    response=$(curl -s -w "%{http_code}" -o /tmp/stats_response.json \
        -X GET "${API_BASE}/stats" \
        -H "Authorization: Bearer $JWT_TOKEN" 2>/dev/null || echo "000")
    
    if [ "$response" = "200" ]; then
        if grep -q '"status".*:.*"success"' /tmp/stats_response.json 2>/dev/null; then
            log_success "STATS request successful"
        else
            log_error "STATS returned 200 but invalid format"
        fi
    else
        log_error "STATS failed (HTTP $response)"
    fi
}

test_delete_data() {
    log_info "Testing DELETE endpoint..."
    increment_test
    
    if [ -z "$JWT_TOKEN" ]; then
        log_error "No JWT token available for DELETE test"
        return
    fi
    
    response=$(curl -s -w "%{http_code}" -o /tmp/delete_response.json \
        -X DELETE "${API_BASE}/kv/test-key" \
        -H "Authorization: Bearer $JWT_TOKEN" 2>/dev/null || echo "000")
    
    if [ "$response" = "200" ]; then
        if grep -q '"deleted":true' /tmp/delete_response.json 2>/dev/null; then
            log_success "DELETE request successful"
        else
            log_error "DELETE returned 200 but invalid format"
        fi
    else
        log_error "DELETE failed (HTTP $response)"
    fi
}

test_delete_verify() {
    log_info "Verifying DELETE worked..."
    increment_test
    
    if [ -z "$JWT_TOKEN" ]; then
        log_error "No JWT token available for DELETE verification"
        return
    fi
    
    response=$(curl -s -w "%{http_code}" -o /tmp/verify_delete.json \
        -X GET "${API_BASE}/kv/test-key" \
        -H "Authorization: Bearer $JWT_TOKEN" 2>/dev/null || echo "000")
    
    if [ "$response" = "404" ]; then
        log_success "DELETE verification: key no longer exists"
    else
        log_error "DELETE verification failed: key still exists (HTTP $response)"
    fi
}

# Wait for server to be ready
wait_for_server() {
    log_info "Waiting for server to be ready..."
    
    max_attempts=30
    attempt=0
    
    while [ $attempt -lt $max_attempts ]; do
        if curl -s "${API_BASE}/health" >/dev/null 2>&1; then
            log_success "Server is ready"
            return 0
        fi
        
        attempt=$((attempt + 1))
        echo -n "."
        sleep 1
    done
    
    log_error "Server failed to start within ${max_attempts} seconds"
    return 1
}

# Cleanup function
cleanup() {
    rm -f /tmp/health_response.json /tmp/login_response.json /tmp/invalid_login.json
    rm -f /tmp/unauth.json /tmp/put_response.json /tmp/get_response.json
    rm -f /tmp/get_404.json /tmp/list_response.json /tmp/stats_response.json
    rm -f /tmp/delete_response.json /tmp/verify_delete.json
}

# Main test execution
main() {
    echo "🌐 REST API Integration Test Suite"
    echo "=================================="
    echo "Server: ${BASE_URL}"
    echo
    
    # Wait for server to be ready
    if ! wait_for_server; then
        log_error "Cannot connect to server at ${BASE_URL}"
        log_info "Please start the server with: make server"
        exit 1
    fi
    
    echo
    log_info "Starting API tests..."
    echo
    
    # Run all tests
    test_health_check
    test_login
    test_invalid_login
    test_unauthorized_access
    test_put_data
    test_get_data
    test_get_nonexistent
    test_list_data
    test_stats
    test_delete_data
    test_delete_verify
    
    # Summary
    echo
    echo "🏁 Test Results Summary"
    echo "======================="
    echo -e "Total Tests:  ${BLUE}${TOTAL_TESTS}${NC}"
    echo -e "Passed:       ${GREEN}${PASSED_TESTS}${NC}"
    echo -e "Failed:       ${RED}${FAILED_TESTS}${NC}"
    
    if [ $FAILED_TESTS -eq 0 ]; then
        echo -e "\n${GREEN}🎉 All tests passed!${NC}"
        cleanup
        exit 0
    else
        echo -e "\n${RED}❌ Some tests failed!${NC}"
        cleanup
        exit 1
    fi
}

# Handle interruption
trap cleanup EXIT

# Run main function
main "$@"