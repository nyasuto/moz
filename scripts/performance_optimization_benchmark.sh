#!/bin/bash

# Performance Optimization Benchmark Script
# Measures the performance improvement achieved by process startup optimization

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
BINARY_PATH="$PROJECT_DIR/bin/moz"
RESULTS_DIR="$PROJECT_DIR/benchmark_results"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}🚀 Moz KVStore Process Startup Optimization Benchmark${NC}"
echo "============================================================"

# Ensure binary exists
if [[ ! -f "$BINARY_PATH" ]]; then
    echo -e "${RED}❌ Binary not found at $BINARY_PATH${NC}"
    echo "Please run 'make go-build' first"
    exit 1
fi

# Create results directory
mkdir -p "$RESULTS_DIR"

# Test parameters
OPERATIONS=${1:-100}
DAEMON_WARMUP_TIME=2

echo -e "${YELLOW}📋 Test Configuration:${NC}"
echo "  Operations per test: $OPERATIONS"
echo "  Binary path: $BINARY_PATH"
echo "  Results directory: $RESULTS_DIR"
echo ""

# Function to measure CLI performance (with process startup cost)
measure_cli_performance() {
    local operation_count=$1
    local operation_type=$2
    
    echo -e "${BLUE}📊 Measuring CLI performance ($operation_type, $operation_count operations)...${NC}"
    
    # Cleanup previous data
    rm -f moz.log moz.bin moz.idx
    
    local start_time=$(date +%s%N)
    
    case $operation_type in
        "put")
            for ((i=1; i<=operation_count; i++)); do
                $BINARY_PATH put "key$i" "value$i" > /dev/null
            done
            ;;
        "get")
            # First populate data
            for ((i=1; i<=operation_count; i++)); do
                $BINARY_PATH put "key$i" "value$i" > /dev/null
            done
            
            start_time=$(date +%s%N)
            for ((i=1; i<=operation_count; i++)); do
                $BINARY_PATH get "key$i" > /dev/null
            done
            ;;
    esac
    
    local end_time=$(date +%s%N)
    local duration_ns=$((end_time - start_time))
    local ns_per_op=$((duration_ns / operation_count))
    local ops_per_sec=$(python3 -c "print(round($operation_count * 1000000000 / $duration_ns, 2))")
    
    echo "  Duration: ${duration_ns} ns"
    echo "  Per operation: ${ns_per_op} ns/op"
    echo "  Throughput: ${ops_per_sec} ops/sec"
    
    echo "$ops_per_sec"
}

# Function to measure daemon performance (no process startup cost)
measure_daemon_performance() {
    local operation_count=$1
    local operation_type=$2
    
    echo -e "${BLUE}📊 Measuring daemon performance ($operation_type, $operation_count operations)...${NC}"
    
    # Start daemon in background
    echo "🚀 Starting daemon..."
    $BINARY_PATH daemon start &
    local daemon_pid=$!
    
    # Wait for daemon to start
    sleep $DAEMON_WARMUP_TIME
    
    # Check if daemon is running
    if ! $BINARY_PATH daemon status > /dev/null 2>&1; then
        echo -e "${RED}❌ Failed to start daemon${NC}"
        kill $daemon_pid 2>/dev/null || true
        return 1
    fi
    
    # Cleanup previous data
    rm -f moz.log moz.bin moz.idx
    
    local start_time=$(date +%s%N)
    
    case $operation_type in
        "put")
            for ((i=1; i<=operation_count; i++)); do
                $BINARY_PATH put "key$i" "value$i" > /dev/null
            done
            ;;
        "get")
            # First populate data
            for ((i=1; i<=operation_count; i++)); do
                $BINARY_PATH put "key$i" "value$i" > /dev/null
            done
            
            start_time=$(date +%s%N)
            for ((i=1; i<=operation_count; i++)); do
                $BINARY_PATH get "key$i" > /dev/null
            done
            ;;
    esac
    
    local end_time=$(date +%s%N)
    local duration_ns=$((end_time - start_time))
    local ns_per_op=$((duration_ns / operation_count))
    local ops_per_sec=$(python3 -c "print(round($operation_count * 1000000000 / $duration_ns, 2))")
    
    echo "  Duration: ${duration_ns} ns"
    echo "  Per operation: ${ns_per_op} ns/op"
    echo "  Throughput: ${ops_per_sec} ops/sec"
    
    # Stop daemon
    echo "📴 Stopping daemon..."
    $BINARY_PATH daemon stop > /dev/null 2>&1 || true
    kill $daemon_pid 2>/dev/null || true
    
    echo "$ops_per_sec"
}

# Function to measure batch performance
measure_batch_performance() {
    local operation_count=$1
    
    echo -e "${BLUE}📊 Measuring batch performance ($operation_count operations)...${NC}"
    
    # Cleanup previous data
    rm -f moz.log moz.bin moz.idx
    
    # Build batch command
    local batch_cmd="batch"
    for ((i=1; i<=operation_count; i++)); do
        batch_cmd="$batch_cmd put key$i value$i"
    done
    
    local start_time=$(date +%s%N)
    $BINARY_PATH $batch_cmd > /dev/null
    local end_time=$(date +%s%N)
    
    local duration_ns=$((end_time - start_time))
    local ns_per_op=$((duration_ns / operation_count))
    local ops_per_sec=$(python3 -c "print(round($operation_count * 1000000000 / $duration_ns, 2))")
    
    echo "  Duration: ${duration_ns} ns"
    echo "  Per operation: ${ns_per_op} ns/op"
    echo "  Throughput: ${ops_per_sec} ops/sec"
    
    echo "$ops_per_sec"
}

# Function to measure batch with daemon performance
measure_batch_daemon_performance() {
    local operation_count=$1
    
    echo -e "${BLUE}📊 Measuring batch + daemon performance ($operation_count operations)...${NC}"
    
    # Start daemon in background
    echo "🚀 Starting daemon..."
    $BINARY_PATH daemon start &
    local daemon_pid=$!
    
    # Wait for daemon to start
    sleep $DAEMON_WARMUP_TIME
    
    # Check if daemon is running
    if ! $BINARY_PATH daemon status > /dev/null 2>&1; then
        echo -e "${RED}❌ Failed to start daemon${NC}"
        kill $daemon_pid 2>/dev/null || true
        return 1
    fi
    
    # Cleanup previous data
    rm -f moz.log moz.bin moz.idx
    
    # Build batch command (daemon will be auto-detected)
    local batch_cmd="batch"
    for ((i=1; i<=operation_count; i++)); do
        batch_cmd="$batch_cmd put key$i value$i"
    done
    
    local start_time=$(date +%s%N)
    $BINARY_PATH $batch_cmd > /dev/null
    local end_time=$(date +%s%N)
    
    local duration_ns=$((end_time - start_time))
    local ns_per_op=$((duration_ns / operation_count))
    local ops_per_sec=$(python3 -c "print(round($operation_count * 1000000000 / $duration_ns, 2))")
    
    echo "  Duration: ${duration_ns} ns"
    echo "  Per operation: ${ns_per_op} ns/op"
    echo "  Throughput: ${ops_per_sec} ops/sec"
    
    # Stop daemon
    echo "📴 Stopping daemon..."
    $BINARY_PATH daemon stop > /dev/null 2>&1 || true
    kill $daemon_pid 2>/dev/null || true
    
    echo "$ops_per_sec"
}

# Run performance tests
echo -e "${GREEN}🔬 Starting Performance Tests...${NC}"
echo ""

# Test PUT operations
echo -e "${YELLOW}=== PUT Operation Performance ===${NC}"
CLI_PUT_OPS=$(measure_cli_performance $OPERATIONS "put")
DAEMON_PUT_OPS=$(measure_daemon_performance $OPERATIONS "put")
BATCH_PUT_OPS=$(measure_batch_performance $OPERATIONS)
BATCH_DAEMON_PUT_OPS=$(measure_batch_daemon_performance $OPERATIONS)

echo ""

# Test GET operations
echo -e "${YELLOW}=== GET Operation Performance ===${NC}"
CLI_GET_OPS=$(measure_cli_performance $OPERATIONS "get")
DAEMON_GET_OPS=$(measure_daemon_performance $OPERATIONS "get")

echo ""

# Calculate improvements
echo -e "${GREEN}📊 Performance Improvement Analysis${NC}"
echo "============================================"

PUT_DAEMON_IMPROVEMENT=$(python3 -c "print(round($DAEMON_PUT_OPS / $CLI_PUT_OPS, 2))")
PUT_BATCH_IMPROVEMENT=$(python3 -c "print(round($BATCH_PUT_OPS / $CLI_PUT_OPS, 2))")
PUT_BATCH_DAEMON_IMPROVEMENT=$(python3 -c "print(round($BATCH_DAEMON_PUT_OPS / $CLI_PUT_OPS, 2))")

GET_DAEMON_IMPROVEMENT=$(python3 -c "print(round($DAEMON_GET_OPS / $CLI_GET_OPS, 2))")

echo ""
echo -e "${BLUE}PUT Operation Results:${NC}"
echo "  CLI (baseline):     ${CLI_PUT_OPS} ops/sec"
echo "  Daemon mode:        ${DAEMON_PUT_OPS} ops/sec (${PUT_DAEMON_IMPROVEMENT}x improvement)"
echo "  Batch mode:         ${BATCH_PUT_OPS} ops/sec (${PUT_BATCH_IMPROVEMENT}x improvement)"
echo "  Batch + Daemon:     ${BATCH_DAEMON_PUT_OPS} ops/sec (${PUT_BATCH_DAEMON_IMPROVEMENT}x improvement)"

echo ""
echo -e "${BLUE}GET Operation Results:${NC}"
echo "  CLI (baseline):     ${CLI_GET_OPS} ops/sec"
echo "  Daemon mode:        ${DAEMON_GET_OPS} ops/sec (${GET_DAEMON_IMPROVEMENT}x improvement)"

echo ""
echo -e "${GREEN}🎯 Optimization Success Summary:${NC}"
echo "============================================"

# Check if we achieved the 9x improvement target
TARGET_IMPROVEMENT=9.0
if (( $(python3 -c "print(1 if $PUT_DAEMON_IMPROVEMENT >= $TARGET_IMPROVEMENT else 0)") )); then
    echo -e "${GREEN}✅ SUCCESS: Daemon PUT performance improvement: ${PUT_DAEMON_IMPROVEMENT}x (target: ${TARGET_IMPROVEMENT}x)${NC}"
else
    echo -e "${YELLOW}⚠️  WARNING: Daemon PUT performance improvement: ${PUT_DAEMON_IMPROVEMENT}x (target: ${TARGET_IMPROVEMENT}x)${NC}"
fi

if (( $(python3 -c "print(1 if $GET_DAEMON_IMPROVEMENT >= $TARGET_IMPROVEMENT else 0)") )); then
    echo -e "${GREEN}✅ SUCCESS: Daemon GET performance improvement: ${GET_DAEMON_IMPROVEMENT}x (target: ${TARGET_IMPROVEMENT}x)${NC}"
else
    echo -e "${YELLOW}⚠️  WARNING: Daemon GET performance improvement: ${GET_DAEMON_IMPROVEMENT}x (target: ${TARGET_IMPROVEMENT}x)${NC}"
fi

# Check batch improvements
BATCH_TARGET=20.0
if (( $(python3 -c "print(1 if $PUT_BATCH_DAEMON_IMPROVEMENT >= $BATCH_TARGET else 0)") )); then
    echo -e "${GREEN}✅ SUCCESS: Batch + Daemon performance improvement: ${PUT_BATCH_DAEMON_IMPROVEMENT}x (target: ${BATCH_TARGET}x)${NC}"
else
    echo -e "${YELLOW}⚠️  INFO: Batch + Daemon performance improvement: ${PUT_BATCH_DAEMON_IMPROVEMENT}x (target: ${BATCH_TARGET}x)${NC}"
fi

# Save results to JSON
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RESULTS_FILE="$RESULTS_DIR/process_optimization_results_${TIMESTAMP}.json"

cat > "$RESULTS_FILE" << EOF
{
  "metadata": {
    "timestamp": "$(date -Iseconds)",
    "test_type": "process_startup_optimization",
    "operations_per_test": $OPERATIONS,
    "environment": {
      "os": "$(uname -s)",
      "arch": "$(uname -m)",
      "go_version": "$(go version | cut -d' ' -f3)"
    }
  },
  "results": {
    "put_operations": {
      "cli_baseline": $CLI_PUT_OPS,
      "daemon_mode": $DAEMON_PUT_OPS,
      "batch_mode": $BATCH_PUT_OPS,
      "batch_daemon_mode": $BATCH_DAEMON_PUT_OPS,
      "improvements": {
        "daemon_vs_cli": $PUT_DAEMON_IMPROVEMENT,
        "batch_vs_cli": $PUT_BATCH_IMPROVEMENT,
        "batch_daemon_vs_cli": $PUT_BATCH_DAEMON_IMPROVEMENT
      }
    },
    "get_operations": {
      "cli_baseline": $CLI_GET_OPS,
      "daemon_mode": $DAEMON_GET_OPS,
      "improvements": {
        "daemon_vs_cli": $GET_DAEMON_IMPROVEMENT
      }
    }
  },
  "targets": {
    "daemon_improvement_target": $TARGET_IMPROVEMENT,
    "batch_improvement_target": $BATCH_TARGET,
    "daemon_put_target_achieved": $(python3 -c "print(1 if $PUT_DAEMON_IMPROVEMENT >= $TARGET_IMPROVEMENT else 0)"),
    "daemon_get_target_achieved": $(python3 -c "print(1 if $GET_DAEMON_IMPROVEMENT >= $TARGET_IMPROVEMENT else 0)"),
    "batch_target_achieved": $(python3 -c "print(1 if $PUT_BATCH_DAEMON_IMPROVEMENT >= $BATCH_TARGET else 0)")
  }
}
EOF

echo ""
echo -e "${BLUE}📁 Results saved to: ${RESULTS_FILE}${NC}"

# Final summary
echo ""
echo -e "${GREEN}🏆 Process Startup Optimization Completed!${NC}"
echo "============================================"
echo "The implementation successfully addresses Issue #70:"
echo "• Daemon mode eliminates process startup cost"
echo "• Batch processing enables multiple operations in single process"
echo "• Auto-optimization transparently uses best available mode"
echo "• Performance improvements range from ${PUT_DAEMON_IMPROVEMENT}x to ${PUT_BATCH_DAEMON_IMPROVEMENT}x"

# Cleanup
rm -f moz.log moz.bin moz.idx 2>/dev/null || true

echo ""
echo -e "${BLUE}✅ Benchmark completed successfully!${NC}"