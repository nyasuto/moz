#!/bin/bash

# REST API Performance Benchmark Script
# Measures API performance for PUT, GET, DELETE, LIST operations

set -e

# Configuration
SERVER_PORT=${SERVER_PORT:-8083}
BASE_URL="http://localhost:${SERVER_PORT}"
OPERATIONS=${1:-1000}
RESULTS_DIR="benchmark_results"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🌐 REST API Performance Benchmark${NC}"
echo "========================================"
echo "Operation Count: ${OPERATIONS}"
echo "Server: ${BASE_URL}"
echo ""

# Create results directory
mkdir -p "${RESULTS_DIR}"

# Result file
RESULT_FILE="${RESULTS_DIR}/rest_api_benchmark_$(date +%Y%m%d_%H%M%S).json"

# Initialize results
echo "{" > "${RESULT_FILE}"
echo "  \"timestamp\": \"$(date -Iseconds)\"," >> "${RESULT_FILE}"
echo "  \"operations\": ${OPERATIONS}," >> "${RESULT_FILE}"
echo "  \"server_url\": \"${BASE_URL}\"," >> "${RESULT_FILE}"
echo "  \"results\": {" >> "${RESULT_FILE}"

# Function to measure operation time
measure_operation() {
    local operation_name="$1"
    local operation_count="$2"
    local operation_func="$3"
    
    echo -e "${YELLOW}📊 Benchmarking ${operation_name} operations...${NC}"
    
    local start_time=$(date +%s%N)
    
    for ((i=1; i<=operation_count; i++)); do
        $operation_func "$i" >/dev/null 2>&1 || {
            echo -e "${RED}❌ ${operation_name} operation failed at iteration $i${NC}"
            return 1
        }
        
        # Progress indicator every 100 operations
        if (( i % 100 == 0 )); then
            echo -ne "\r    Progress: $i/${operation_count} operations completed"
        fi
    done
    
    local end_time=$(date +%s%N)
    local duration_ns=$((end_time - start_time))
    local duration_ms=$((duration_ns / 1000000))
    local ops_per_sec=$(( operation_count * 1000000000 / duration_ns ))
    local ns_per_op=$((duration_ns / operation_count))
    
    echo -e "\r${GREEN}✅ ${operation_name}: ${ns_per_op} ns/op (${ops_per_sec} ops/sec)${NC}"
    
    # Add to results file
    echo "    \"${operation_name,,}\": {" >> "${RESULT_FILE}"
    echo "      \"operations\": ${operation_count}," >> "${RESULT_FILE}"
    echo "      \"total_duration_ns\": ${duration_ns}," >> "${RESULT_FILE}"
    echo "      \"ns_per_op\": ${ns_per_op}," >> "${RESULT_FILE}"
    echo "      \"ops_per_sec\": ${ops_per_sec}" >> "${RESULT_FILE}"
    echo "    }," >> "${RESULT_FILE}"
}

# JWT Token for authentication
get_jwt_token() {
    curl -s -X POST "${BASE_URL}/api/v1/login" \
        -H "Content-Type: application/json" \
        -d '{"username":"admin","password":"password"}' \
        | grep -o '"token":"[^"]*' | cut -d'"' -f4
}

# Operation functions
api_put() {
    local i="$1"
    curl -s -X PUT "${BASE_URL}/api/v1/kv/key${i}" \
        -H "Authorization: Bearer ${JWT_TOKEN}" \
        -H "Content-Type: application/json" \
        -d "{\"value\":\"value${i}\"}"
}

api_get() {
    local i="$1"
    curl -s -X GET "${BASE_URL}/api/v1/kv/key${i}" \
        -H "Authorization: Bearer ${JWT_TOKEN}"
}

api_delete() {
    local i="$1"
    curl -s -X DELETE "${BASE_URL}/api/v1/kv/key${i}" \
        -H "Authorization: Bearer ${JWT_TOKEN}"
}

api_list() {
    local i="$1"
    curl -s -X GET "${BASE_URL}/api/v1/kv" \
        -H "Authorization: Bearer ${JWT_TOKEN}"
}

# Get JWT token
echo -e "${YELLOW}🔑 Getting JWT token...${NC}"
JWT_TOKEN=$(get_jwt_token)
if [ -z "$JWT_TOKEN" ]; then
    echo -e "${RED}❌ Failed to get JWT token${NC}"
    exit 1
fi
echo -e "${GREEN}✅ JWT token obtained${NC}"
echo ""

# Run benchmarks
measure_operation "PUT" "$OPERATIONS" "api_put"
measure_operation "GET" "$OPERATIONS" "api_get"
measure_operation "LIST" "100" "api_list"  # LIST is expensive, use fewer operations
measure_operation "DELETE" "$OPERATIONS" "api_delete"

# Complete results file
sed -i '' '$ s/,$//' "${RESULT_FILE}"  # Remove last comma
echo "  }" >> "${RESULT_FILE}"
echo "}" >> "${RESULT_FILE}"

echo ""
echo -e "${GREEN}🎯 REST API benchmark completed!${NC}"
echo -e "${BLUE}📊 Results saved to: ${RESULT_FILE}${NC}"

# Display summary
echo ""
echo -e "${BLUE}📈 Performance Summary:${NC}"
grep -E '"ns_per_op"|"ops_per_sec"' "${RESULT_FILE}" | while IFS= read -r line; do
    if [[ $line == *"ns_per_op"* ]]; then
        ns_per_op=$(echo "$line" | grep -o '[0-9]*')
        operation=$(grep -B5 "$line" "${RESULT_FILE}" | grep '"[a-z]*": {' | tail -1 | cut -d'"' -f2)
        echo "  ${operation}: ${ns_per_op} ns/op"
    fi
done