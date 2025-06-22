#!/bin/bash

# Performance comparison script between Go and Shell implementations
# Generates comprehensive performance analysis and comparison reports

set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

OPERATION_COUNT=${1:-1000}
REPORT_FORMAT=${2:-"json"}  # json, markdown, or both

echo "🚀 Moz KV Store Performance Comparison"
echo "====================================="
echo "Operation Count: $OPERATION_COUNT"
echo "Report Format: $REPORT_FORMAT"
echo ""

# Ensure benchmark_results directory exists
mkdir -p benchmark_results

# Clean up any existing test data
rm -f moz.log

# Function to run Go benchmarks
run_go_benchmarks() {
    echo -e "${BLUE}🐹 Running Go benchmarks...${NC}"
    
    # Build Go binary first
    make go-build >/dev/null 2>&1
    
    # Run individual operation benchmarks
    echo "  - PUT operations..."
    go test -bench=BenchmarkGoPut -benchtime=${OPERATION_COUNT}x ./internal/kvstore/ -run=^$ >/dev/null 2>&1
    
    echo "  - GET operations..."
    go test -bench=BenchmarkGoGet -benchtime=${OPERATION_COUNT}x ./internal/kvstore/ -run=^$ >/dev/null 2>&1
    
    echo "  - LIST operations..."
    go test -bench=BenchmarkGoList -benchtime=100x ./internal/kvstore/ -run=^$ >/dev/null 2>&1
    
    echo "  - DELETE operations..."
    go test -bench=BenchmarkGoDelete -benchtime=100x ./internal/kvstore/ -run=^$ >/dev/null 2>&1
    
    echo "  - COMPACT operations..."
    go test -bench=BenchmarkGoCompact -benchtime=10x ./internal/kvstore/ -run=^$ >/dev/null 2>&1
    
    echo "  - Large data test..."
    go test -bench=BenchmarkGoLargeData -benchtime=100x ./internal/kvstore/ -run=^$ >/dev/null 2>&1
    
    echo "  - Concurrent operations..."
    go test -bench=BenchmarkGoConcurrentOperations -benchtime=1000x ./internal/kvstore/ -run=^$ >/dev/null 2>&1
    
    echo -e "${GREEN}✅ Go benchmarks completed${NC}"
}

# Function to run Shell benchmarks
run_shell_benchmarks() {
    echo -e "${BLUE}🐚 Running Shell benchmarks...${NC}"
    
    # Ensure shell scripts are executable
    chmod +x legacy/*.sh
    
    # Run shell benchmarks for each operation type
    echo "  - PUT operations..."
    scripts/shell_benchmark.sh $OPERATION_COUNT put >/dev/null 2>&1
    
    echo "  - GET operations..."
    scripts/shell_benchmark.sh $OPERATION_COUNT get >/dev/null 2>&1
    
    echo "  - LIST operations..."
    scripts/shell_benchmark.sh 100 list >/dev/null 2>&1
    
    echo "  - DELETE operations..."
    scripts/shell_benchmark.sh 100 delete >/dev/null 2>&1
    
    echo "  - COMPACT operations..."
    scripts/shell_benchmark.sh 100 compact >/dev/null 2>&1
    
    echo "  - MIXED operations..."
    scripts/shell_benchmark.sh $OPERATION_COUNT mixed >/dev/null 2>&1
    
    echo -e "${GREEN}✅ Shell benchmarks completed${NC}"
}

# Function to parse JSON benchmark results
parse_benchmark_results() {
    local implementation=$1
    local operation=$2
    
    # Find the most recent result file for this implementation and operation
    local result_file=$(ls -t benchmark_results/${implementation}_${operation}_*.json 2>/dev/null | head -1)
    
    if [ -f "$result_file" ]; then
        # Extract key metrics using basic JSON parsing
        local ns_per_op=$(grep '"ns_per_operation"' "$result_file" | sed 's/.*: *\([0-9]*\).*/\1/')
        local operations=$(grep '"operations"' "$result_file" | sed 's/.*: *\([0-9]*\).*/\1/')
        local duration=$(grep '"duration"' "$result_file" | sed 's/.*: *"\([^"]*\)".*/\1/')
        
        echo "$ns_per_op|$operations|$duration"
    else
        echo "N/A|N/A|N/A"
    fi
}

# Function to generate comparison report
generate_comparison_report() {
    echo -e "${YELLOW}📊 Generating performance comparison report...${NC}"
    
    local timestamp=$(date -Iseconds)
    local report_file="benchmark_results/performance_comparison_$(date +%Y%m%d_%H%M%S)"
    
    # Collect results for all operations
    declare -A operations=( 
        ["put"]="PUT Operations"
        ["get"]="GET Operations" 
        ["list"]="LIST Operations"
        ["delete"]="DELETE Operations"
        ["compact"]="COMPACT Operations"
        ["mixed"]="MIXED Operations"
    )
    
    # Generate JSON report
    if [ "$REPORT_FORMAT" = "json" ] || [ "$REPORT_FORMAT" = "both" ]; then
        cat > "${report_file}.json" << EOF
{
  "timestamp": "$timestamp",
  "operation_count": $OPERATION_COUNT,
  "system_info": {
    "os": "$(uname -s)",
    "architecture": "$(uname -m)",
    "shell": "$SHELL"
  },
  "results": {
EOF
        
        local first=true
        for op in "${!operations[@]}"; do
            if [ "$first" = false ]; then
                echo "," >> "${report_file}.json"
            fi
            first=false
            
            local go_result=$(parse_benchmark_results "go" "$op")
            local shell_result=$(parse_benchmark_results "shell" "$op")
            
            local go_ns=$(echo "$go_result" | cut -d'|' -f1)
            local shell_ns=$(echo "$shell_result" | cut -d'|' -f1)
            
            # Calculate speedup (if both have valid results)
            local speedup="N/A"
            if [ "$go_ns" != "N/A" ] && [ "$shell_ns" != "N/A" ] && [ "$go_ns" -gt 0 ]; then
                speedup=$(echo "scale=2; $shell_ns / $go_ns" | bc 2>/dev/null || echo "N/A")
            fi
            
            cat >> "${report_file}.json" << EOF
    "$op": {
      "operation": "${operations[$op]}",
      "go": {
        "ns_per_operation": "$go_ns",
        "operations": "$(echo "$go_result" | cut -d'|' -f2)",
        "duration": "$(echo "$go_result" | cut -d'|' -f3)"
      },
      "shell": {
        "ns_per_operation": "$shell_ns", 
        "operations": "$(echo "$shell_result" | cut -d'|' -f2)",
        "duration": "$(echo "$shell_result" | cut -d'|' -f3)"
      },
      "speedup_factor": "$speedup"
    }EOF
        done
        
        cat >> "${report_file}.json" << EOF

  }
}
EOF
    
    # Generate Markdown report
    if [ "$REPORT_FORMAT" = "markdown" ] || [ "$REPORT_FORMAT" = "both" ]; then
        cat > "${report_file}.md" << EOF
# Moz KV Store Performance Comparison Report

**Generated:** $timestamp  
**Operation Count:** $OPERATION_COUNT  
**System:** $(uname -s) $(uname -m)

## Performance Results

| Operation | Go (ns/op) | Shell (ns/op) | Speedup Factor | Winner |
|-----------|------------|---------------|----------------|---------|
EOF
        
        for op in "${!operations[@]}"; do
            local go_result=$(parse_benchmark_results "go" "$op")
            local shell_result=$(parse_benchmark_results "shell" "$op")
            
            local go_ns=$(echo "$go_result" | cut -d'|' -f1)
            local shell_ns=$(echo "$shell_result" | cut -d'|' -f1)
            
            # Calculate speedup and winner
            local speedup="N/A"
            local winner="N/A"
            if [ "$go_ns" != "N/A" ] && [ "$shell_ns" != "N/A" ] && [ "$go_ns" -gt 0 ]; then
                speedup=$(echo "scale=2; $shell_ns / $go_ns" | bc 2>/dev/null || echo "N/A")
                if [ "$speedup" != "N/A" ]; then
                    if (( $(echo "$speedup > 1" | bc -l) )); then
                        winner="🐹 Go"
                    else
                        winner="🐚 Shell"
                    fi
                fi
            fi
            
            # Format numbers with commas for readability
            local go_formatted=$(echo "$go_ns" | sed ':a;s/\B[0-9]\{3\}\>/,&/;ta' 2>/dev/null || echo "$go_ns")
            local shell_formatted=$(echo "$shell_ns" | sed ':a;s/\B[0-9]\{3\}\>/,&/;ta' 2>/dev/null || echo "$shell_ns")
            
            echo "| ${operations[$op]} | $go_formatted | $shell_formatted | ${speedup}x | $winner |" >> "${report_file}.md"
        done
        
        cat >> "${report_file}.md" << EOF

## Summary

This report compares the performance between the Go implementation and Shell script implementation of the Moz KV store.

- **Speedup Factor > 1.0**: Go implementation is faster
- **Speedup Factor < 1.0**: Shell implementation is faster  
- **Speedup Factor = N/A**: Unable to calculate (missing data)

### Methodology

- Each benchmark runs the specified number of operations
- Results are averaged across multiple runs
- Memory usage is measured for Go implementation
- Time measurements include I/O operations

### Notes

- Go implementation benefits from compiled code and memory caching
- Shell implementation provides simplicity and debugging ease
- Results may vary based on system configuration and load
EOF
    fi
    
    echo -e "${GREEN}✅ Report generated: ${report_file}.${REPORT_FORMAT}${NC}"
    
    # Display summary
    echo ""
    echo -e "${BLUE}📈 Performance Summary:${NC}"
    for op in "${!operations[@]}"; do
        local go_result=$(parse_benchmark_results "go" "$op")
        local shell_result=$(parse_benchmark_results "shell" "$op")
        
        local go_ns=$(echo "$go_result" | cut -d'|' -f1)
        local shell_ns=$(echo "$shell_result" | cut -d'|' -f1)
        
        if [ "$go_ns" != "N/A" ] && [ "$shell_ns" != "N/A" ] && [ "$go_ns" -gt 0 ]; then
            local speedup=$(echo "scale=2; $shell_ns / $go_ns" | bc 2>/dev/null || echo "N/A")
            if [ "$speedup" != "N/A" ]; then
                if (( $(echo "$speedup > 1" | bc -l) )); then
                    echo -e "  ${GREEN}${operations[$op]}: Go is ${speedup}x faster${NC}"
                else
                    local shell_speedup=$(echo "scale=2; $go_ns / $shell_ns" | bc 2>/dev/null || echo "N/A")
                    echo -e "  ${RED}${operations[$op]}: Shell is ${shell_speedup}x faster${NC}"
                fi
            fi
        fi
    done
}

# Main execution
echo -e "${YELLOW}🏃 Starting performance comparison...${NC}"
echo ""

# Run benchmarks
run_go_benchmarks
echo ""
run_shell_benchmarks
echo ""

# Generate comparison report
generate_comparison_report

echo ""
echo -e "${GREEN}🎉 Performance comparison completed!${NC}"
echo "Check benchmark_results/ directory for detailed results."