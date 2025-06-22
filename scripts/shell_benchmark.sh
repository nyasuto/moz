#!/bin/bash

# Shell script benchmark for performance comparison with Go implementation
# Usage: ./shell_benchmark.sh <operation_count> [operation_type]

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

OPERATION_COUNT=${1:-1000}
OPERATION_TYPE=${2:-"mixed"}

echo "🧪 Shell Implementation Performance Benchmark"
echo "=============================================="
echo "Operation Count: $OPERATION_COUNT"
echo "Operation Type: $OPERATION_TYPE"
echo ""

# Ensure benchmark_results directory exists
mkdir -p benchmark_results

# Clean up any existing test data
rm -f moz.log

# Helper function to measure time in nanoseconds (approximation)
measure_time() {
    start_time=$(date +%s%N)
    eval "$1"
    end_time=$(date +%s%N)
    echo $((end_time - start_time))
}

# Benchmark PUT operations
benchmark_put() {
    echo -e "${YELLOW}📝 Benchmarking PUT operations...${NC}"
    
    total_time=$(measure_time "
        for i in \$(seq 1 $OPERATION_COUNT); do
            legacy/put.sh \"benchmark_key_\$i\" \"benchmark_value_\$i\" >/dev/null 2>&1
        done
    ")
    
    ns_per_op=$((total_time / OPERATION_COUNT))
    
    # Generate JSON result
    cat > "benchmark_results/shell_put_$(date +%Y%m%d_%H%M%S).json" << EOF
{
  "name": "Shell PUT Operations",
  "implementation": "shell",
  "operations": $OPERATION_COUNT,
  "ns_per_operation": $ns_per_op,
  "duration": "${total_time}ns",
  "timestamp": "$(date -Iseconds)",
  "data_size": $OPERATION_COUNT
}
EOF
    
    echo -e "${GREEN}✅ PUT: $ns_per_op ns/op (${OPERATION_COUNT} operations)${NC}"
    rm -f moz.log
}

# Benchmark GET operations
benchmark_get() {
    echo -e "${YELLOW}📖 Benchmarking GET operations...${NC}"
    
    # Pre-populate data
    for i in $(seq 1 $OPERATION_COUNT); do
        legacy/put.sh "key_$i" "value_$i" >/dev/null 2>&1
    done
    
    total_time=$(measure_time "
        for i in \$(seq 1 $OPERATION_COUNT); do
            key_idx=\$(((i % $OPERATION_COUNT) + 1))
            legacy/get.sh \"key_\$key_idx\" >/dev/null 2>&1
        done
    ")
    
    ns_per_op=$((total_time / OPERATION_COUNT))
    
    # Generate JSON result
    cat > "benchmark_results/shell_get_$(date +%Y%m%d_%H%M%S).json" << EOF
{
  "name": "Shell GET Operations",
  "implementation": "shell",
  "operations": $OPERATION_COUNT,
  "ns_per_operation": $ns_per_op,
  "duration": "${total_time}ns",
  "timestamp": "$(date -Iseconds)",
  "data_size": $OPERATION_COUNT
}
EOF
    
    echo -e "${GREEN}✅ GET: $ns_per_op ns/op (${OPERATION_COUNT} operations)${NC}"
    rm -f moz.log
}

# Benchmark LIST operations
benchmark_list() {
    echo -e "${YELLOW}📋 Benchmarking LIST operations...${NC}"
    
    # Pre-populate data
    local data_size=1000
    for i in $(seq 1 $data_size); do
        legacy/put.sh "list_key_$i" "list_value_$i" >/dev/null 2>&1
    done
    
    total_time=$(measure_time "
        for i in \$(seq 1 $OPERATION_COUNT); do
            legacy/list.sh >/dev/null 2>&1
        done
    ")
    
    ns_per_op=$((total_time / OPERATION_COUNT))
    
    # Generate JSON result
    cat > "benchmark_results/shell_list_$(date +%Y%m%d_%H%M%S).json" << EOF
{
  "name": "Shell LIST Operations",
  "implementation": "shell",
  "operations": $OPERATION_COUNT,
  "ns_per_operation": $ns_per_op,
  "duration": "${total_time}ns",
  "timestamp": "$(date -Iseconds)",
  "data_size": $data_size
}
EOF
    
    echo -e "${GREEN}✅ LIST: $ns_per_op ns/op (${OPERATION_COUNT} operations)${NC}"
    rm -f moz.log
}

# Benchmark DELETE operations
benchmark_delete() {
    echo -e "${YELLOW}🗑️  Benchmarking DELETE operations...${NC}"
    
    # Pre-populate data (double the amount to ensure we have enough to delete)
    local data_size=$((OPERATION_COUNT * 2))
    for i in $(seq 1 $data_size); do
        legacy/put.sh "delete_key_$i" "delete_value_$i" >/dev/null 2>&1
    done
    
    total_time=$(measure_time "
        for i in \$(seq 1 $OPERATION_COUNT); do
            legacy/del.sh \"delete_key_\$i\" >/dev/null 2>&1
        done
    ")
    
    ns_per_op=$((total_time / OPERATION_COUNT))
    
    # Generate JSON result
    cat > "benchmark_results/shell_delete_$(date +%Y%m%d_%H%M%S).json" << EOF
{
  "name": "Shell DELETE Operations",
  "implementation": "shell",
  "operations": $OPERATION_COUNT,
  "ns_per_operation": $ns_per_op,
  "duration": "${total_time}ns",
  "timestamp": "$(date -Iseconds)",
  "data_size": $data_size
}
EOF
    
    echo -e "${GREEN}✅ DELETE: $ns_per_op ns/op (${OPERATION_COUNT} operations)${NC}"
    rm -f moz.log
}

# Benchmark COMPACT operations
benchmark_compact() {
    echo -e "${YELLOW}🗜️  Benchmarking COMPACT operations...${NC}"
    
    local iterations=10  # Fewer iterations for compact as it's expensive
    
    total_time=0
    for i in $(seq 1 $iterations); do
        # Create fragmented data
        for j in $(seq 1 1000); do
            legacy/put.sh "compact_key_${i}_$j" "compact_value_${i}_$j" >/dev/null 2>&1
            if [ $((j % 3)) -eq 0 ]; then
                legacy/del.sh "compact_key_${i}_$j" >/dev/null 2>&1
            fi
        done
        
        # Measure compact time
        iter_time=$(measure_time "legacy/compact.sh >/dev/null 2>&1")
        total_time=$((total_time + iter_time))
    done
    
    ns_per_op=$((total_time / iterations))
    
    # Generate JSON result
    cat > "benchmark_results/shell_compact_$(date +%Y%m%d_%H%M%S).json" << EOF
{
  "name": "Shell COMPACT Operations",
  "implementation": "shell",
  "operations": $iterations,
  "ns_per_operation": $ns_per_op,
  "duration": "${total_time}ns",
  "timestamp": "$(date -Iseconds)",
  "data_size": 1000
}
EOF
    
    echo -e "${GREEN}✅ COMPACT: $ns_per_op ns/op (${iterations} operations)${NC}"
    rm -f moz.log
}

# Benchmark mixed operations
benchmark_mixed() {
    echo -e "${YELLOW}🔄 Benchmarking MIXED operations...${NC}"
    
    total_time=$(measure_time "
        for i in \$(seq 1 $OPERATION_COUNT); do
            case \$((i % 10)) in
                0|1|2|3|4|5) # 60% reads
                    key_idx=\$(((i % 100) + 1))
                    legacy/get.sh \"key_\$key_idx\" >/dev/null 2>&1 || true
                    ;;
                6|7|8) # 30% writes
                    legacy/put.sh \"mixed_key_\$i\" \"mixed_value_\$i\" >/dev/null 2>&1
                    ;;
                9) # 10% deletes
                    delete_key=\$(((i % 50) + 1))
                    legacy/del.sh \"key_\$delete_key\" >/dev/null 2>&1 || true
                    ;;
            esac
        done
    ")
    
    ns_per_op=$((total_time / OPERATION_COUNT))
    
    # Generate JSON result
    cat > "benchmark_results/shell_mixed_$(date +%Y%m%d_%H%M%S).json" << EOF
{
  "name": "Shell MIXED Operations",
  "implementation": "shell",
  "operations": $OPERATION_COUNT,
  "ns_per_operation": $ns_per_op,
  "duration": "${total_time}ns",
  "timestamp": "$(date -Iseconds)",
  "data_size": $OPERATION_COUNT
}
EOF
    
    echo -e "${GREEN}✅ MIXED: $ns_per_op ns/op (${OPERATION_COUNT} operations)${NC}"
    rm -f moz.log
}

# Main execution
case "$OPERATION_TYPE" in
    "put")
        benchmark_put
        ;;
    "get")
        benchmark_get
        ;;
    "list")
        benchmark_list
        ;;
    "delete")
        benchmark_delete
        ;;
    "compact")
        benchmark_compact
        ;;
    "mixed")
        benchmark_mixed
        ;;
    "all")
        benchmark_put
        benchmark_get
        benchmark_list
        benchmark_delete
        benchmark_compact
        benchmark_mixed
        ;;
    *)
        echo "Usage: $0 <operation_count> [put|get|list|delete|compact|mixed|all]"
        echo "Default: mixed operations"
        benchmark_mixed
        ;;
esac

echo ""
echo -e "${GREEN}🎯 Shell benchmark completed!${NC}"
echo "Results saved to benchmark_results/"