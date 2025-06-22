#!/bin/bash

# Simple performance benchmark for Go vs Shell comparison
set -e

OPERATION_COUNT=${1:-100}

echo "🚀 Simple Performance Benchmark"
echo "================================"
echo "Operations: $OPERATION_COUNT"
echo ""

# Ensure directories exist
mkdir -p benchmark_results
rm -f moz.log

# Build Go binary
echo "Building Go binary..."
make go-build >/dev/null 2>&1

# Test Go PUT operations
echo "🐹 Testing Go PUT operations..."
start_time=$(date +%s%N)
for i in $(seq 1 $OPERATION_COUNT); do
    ./bin/moz put "go_key_$i" "go_value_$i" >/dev/null 2>&1
done
end_time=$(date +%s%N)
go_put_time=$((end_time - start_time))
go_put_ns_per_op=$((go_put_time / OPERATION_COUNT))

echo "✅ Go PUT: $go_put_ns_per_op ns/op"

# Clear data
rm -f moz.log

# Test Shell PUT operations
echo "🐚 Testing Shell PUT operations..."
chmod +x legacy/*.sh
start_time=$(date +%s%N)
for i in $(seq 1 $OPERATION_COUNT); do
    legacy/put.sh "shell_key_$i" "shell_value_$i" >/dev/null 2>&1
done
end_time=$(date +%s%N)
shell_put_time=$((end_time - start_time))
shell_put_ns_per_op=$((shell_put_time / OPERATION_COUNT))

echo "✅ Shell PUT: $shell_put_ns_per_op ns/op"

# Calculate speedup
if [ $go_put_ns_per_op -gt 0 ]; then
    if [ $shell_put_ns_per_op -gt $go_put_ns_per_op ]; then
        speedup=$((shell_put_ns_per_op / go_put_ns_per_op))
        echo "🏆 Go is ${speedup}x faster than Shell for PUT operations"
    else
        speedup=$((go_put_ns_per_op / shell_put_ns_per_op))
        echo "🏆 Shell is ${speedup}x faster than Go for PUT operations"
    fi
fi

# Save results
timestamp=$(date -Iseconds)
cat > "benchmark_results/simple_comparison_$(date +%Y%m%d_%H%M%S).json" << EOF
{
  "timestamp": "$timestamp",
  "operation_count": $OPERATION_COUNT,
  "results": {
    "go_put": {
      "ns_per_operation": $go_put_ns_per_op,
      "total_time_ns": $go_put_time
    },
    "shell_put": {
      "ns_per_operation": $shell_put_ns_per_op,
      "total_time_ns": $shell_put_time
    }
  }
}
EOF

echo ""
echo "✅ Benchmark completed! Results saved to benchmark_results/"