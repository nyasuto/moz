#!/bin/bash

# CMD Go Implementation Performance Benchmark Script
set -e

OPERATIONS=${1:-500}
RESULTS_DIR="benchmark_results"
BLUE='\033[0;34m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${BLUE}⚡ CMD Go Implementation Performance Benchmark${NC}"
echo "=============================================="
echo "Operation Count: ${OPERATIONS}"
echo ""

mkdir -p "${RESULTS_DIR}"
RESULT_FILE="${RESULTS_DIR}/cmd_benchmark_$(date +%Y%m%d_%H%M%S).json"

# Clean data directory
rm -f moz.log

# Function to measure operation time
measure_cmd_operation() {
    local operation_name="$1"
    local operation_count="$2"
    local operation_func="$3"
    
    echo -e "${YELLOW}📊 Benchmarking ${operation_name} operations...${NC}"
    
    local start_time=$(date +%s%N)
    
    for ((i=1; i<=operation_count; i++)); do
        $operation_func "$i" >/dev/null 2>&1
        
        # Progress indicator every 100 operations
        if (( i % 100 == 0 )); then
            echo -ne "\r    Progress: $i/${operation_count} operations completed"
        fi
    done
    
    local end_time=$(date +%s%N)
    local duration_ns=$((end_time - start_time))
    local ops_per_sec=$(( operation_count * 1000000000 / duration_ns ))
    local ns_per_op=$((duration_ns / operation_count))
    
    echo -e "\r${GREEN}✅ ${operation_name}: ${ns_per_op} ns/op (${ops_per_sec} ops/sec)${NC}"
    
    # Store results
    echo "  ${operation_name,,}: ${ns_per_op} ns/op"
}

# Operation functions
cmd_put() {
    local i="$1"
    ./bin/moz put "key${i}" "value${i}"
}

cmd_get() {
    local i="$1"
    ./bin/moz get "key${i}"
}

cmd_delete() {
    local i="$1" 
    ./bin/moz del "key${i}"
}

cmd_list() {
    local i="$1"
    ./bin/moz list
}

# Initialize results
echo "{" > "${RESULT_FILE}"
echo "  \"timestamp\": \"$(date -Iseconds)\"," >> "${RESULT_FILE}"
echo "  \"operations\": ${OPERATIONS}," >> "${RESULT_FILE}"
echo "  \"implementation\": \"cmd-go\"," >> "${RESULT_FILE}"
echo "  \"results\": {" >> "${RESULT_FILE}"

# Run benchmarks
measure_cmd_operation "PUT" "$OPERATIONS" "cmd_put"
measure_cmd_operation "GET" "$OPERATIONS" "cmd_get"
measure_cmd_operation "LIST" "100" "cmd_list"  # LIST is expensive
measure_cmd_operation "DELETE" "$OPERATIONS" "cmd_delete"

# Complete results file
echo "  }" >> "${RESULT_FILE}"
echo "}" >> "${RESULT_FILE}"

echo ""
echo -e "${GREEN}🎯 CMD Go benchmark completed!${NC}"
echo -e "${BLUE}📊 Results saved to: ${RESULT_FILE}${NC}"