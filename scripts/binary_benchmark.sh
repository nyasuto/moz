#!/bin/bash

# Binary format performance benchmark
set -e

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

OPERATION_COUNT=${1:-1000}

echo "🚀 Binary Format Performance Benchmark"
echo "======================================"
echo "Operation Count: $OPERATION_COUNT"
echo ""

# Ensure benchmark_results directory exists
mkdir -p benchmark_results

# Clean up any existing test data
rm -f moz.log moz.bin

# Build Go binary
echo "Building Go binary..."
make go-build >/dev/null 2>&1

echo -e "${BLUE}🔧 Testing Binary Format Performance...${NC}"

# Test Binary PUT operations
echo "  - Binary PUT operations..."
start_time=$(date +%s%N)
for i in $(seq 1 $OPERATION_COUNT); do
    ./bin/moz --format=binary put "binary_key_$i" "binary_value_$i" >/dev/null 2>&1
done
end_time=$(date +%s%N)
binary_put_time=$((end_time - start_time))
binary_put_ns_per_op=$((binary_put_time / OPERATION_COUNT))

echo -e "${GREEN}✅ Binary PUT: $binary_put_ns_per_op ns/op${NC}"

# Test Binary GET operations
echo "  - Binary GET operations..."
start_time=$(date +%s%N)
for i in $(seq 1 $OPERATION_COUNT); do
    ./bin/moz --format=binary get "binary_key_$i" >/dev/null 2>&1
done
end_time=$(date +%s%N)
binary_get_time=$((end_time - start_time))
binary_get_ns_per_op=$((binary_get_time / OPERATION_COUNT))

echo -e "${GREEN}✅ Binary GET: $binary_get_ns_per_op ns/op${NC}"

# Test Binary LIST operation
echo "  - Binary LIST operation..."
start_time=$(date +%s%N)
for i in $(seq 1 10); do  # Fewer iterations for LIST
    ./bin/moz --format=binary list >/dev/null 2>&1
done
end_time=$(date +%s%N)
binary_list_time=$((end_time - start_time))
binary_list_ns_per_op=$((binary_list_time / 10))

echo -e "${GREEN}✅ Binary LIST: $binary_list_ns_per_op ns/op${NC}"

# Get file sizes
binary_file_size=$(wc -c < moz.bin 2>/dev/null || echo 0)

# Clean up and test Text format for comparison
rm -f moz.log moz.bin

echo -e "${BLUE}🔧 Testing Text Format Performance (for comparison)...${NC}"

# Test Text PUT operations
echo "  - Text PUT operations..."
start_time=$(date +%s%N)
for i in $(seq 1 $OPERATION_COUNT); do
    ./bin/moz --format=text put "text_key_$i" "text_value_$i" >/dev/null 2>&1
done
end_time=$(date +%s%N)
text_put_time=$((end_time - start_time))
text_put_ns_per_op=$((text_put_time / OPERATION_COUNT))

echo -e "${GREEN}✅ Text PUT: $text_put_ns_per_op ns/op${NC}"

# Test Text GET operations
echo "  - Text GET operations..."
start_time=$(date +%s%N)
for i in $(seq 1 $OPERATION_COUNT); do
    ./bin/moz --format=text get "text_key_$i" >/dev/null 2>&1
done
end_time=$(date +%s%N)
text_get_time=$((end_time - start_time))
text_get_ns_per_op=$((text_get_time / OPERATION_COUNT))

echo -e "${GREEN}✅ Text GET: $text_get_ns_per_op ns/op${NC}"

# Test Text LIST operation
echo "  - Text LIST operation..."
start_time=$(date +%s%N)
for i in $(seq 1 10); do  # Fewer iterations for LIST
    ./bin/moz --format=text list >/dev/null 2>&1
done
end_time=$(date +%s%N)
text_list_time=$((end_time - start_time))
text_list_ns_per_op=$((text_list_time / 10))

echo -e "${GREEN}✅ Text LIST: $text_list_ns_per_op ns/op${NC}"

# Get file sizes
text_file_size=$(wc -c < moz.log 2>/dev/null || echo 0)

# Calculate performance differences
put_speedup=$(echo "scale=2; $text_put_ns_per_op / $binary_put_ns_per_op" | bc 2>/dev/null || echo "N/A")
get_speedup=$(echo "scale=2; $text_get_ns_per_op / $binary_get_ns_per_op" | bc 2>/dev/null || echo "N/A")
list_speedup=$(echo "scale=2; $text_list_ns_per_op / $binary_list_ns_per_op" | bc 2>/dev/null || echo "N/A")

# Calculate space efficiency
if [ $text_file_size -gt 0 ]; then
    space_efficiency=$(echo "scale=2; (1 - $binary_file_size / $text_file_size) * 100" | bc 2>/dev/null || echo "N/A")
else
    space_efficiency="N/A"
fi

# Display results
echo ""
echo -e "${YELLOW}📊 Performance Comparison Summary:${NC}"
echo "=================================="
echo "PUT operations:"
echo "  Binary: $binary_put_ns_per_op ns/op"
echo "  Text:   $text_put_ns_per_op ns/op"
echo "  Binary speedup: ${put_speedup}x"
echo ""
echo "GET operations:"
echo "  Binary: $binary_get_ns_per_op ns/op"
echo "  Text:   $text_get_ns_per_op ns/op"
echo "  Binary speedup: ${get_speedup}x"
echo ""
echo "LIST operations:"
echo "  Binary: $binary_list_ns_per_op ns/op"
echo "  Text:   $text_list_ns_per_op ns/op"
echo "  Binary speedup: ${list_speedup}x"
echo ""
echo "Storage efficiency:"
echo "  Binary file: $binary_file_size bytes"
echo "  Text file:   $text_file_size bytes"
echo "  Space saved: ${space_efficiency}%"

# Save results to JSON
timestamp=$(date -Iseconds)
cat > "benchmark_results/binary_format_$(date +%Y%m%d_%H%M%S).json" << EOF
{
  "timestamp": "$timestamp",
  "operation_count": $OPERATION_COUNT,
  "binary_format": {
    "put_ns_per_op": $binary_put_ns_per_op,
    "get_ns_per_op": $binary_get_ns_per_op,
    "list_ns_per_op": $binary_list_ns_per_op,
    "file_size": $binary_file_size
  },
  "text_format": {
    "put_ns_per_op": $text_put_ns_per_op,
    "get_ns_per_op": $text_get_ns_per_op,
    "list_ns_per_op": $text_list_ns_per_op,
    "file_size": $text_file_size
  },
  "performance_gains": {
    "put_speedup": "$put_speedup",
    "get_speedup": "$get_speedup",
    "list_speedup": "$list_speedup",
    "space_efficiency_percent": "$space_efficiency"
  }
}
EOF

echo ""
echo -e "${GREEN}✅ Binary format benchmark completed!${NC}"
echo "Results saved to benchmark_results/"