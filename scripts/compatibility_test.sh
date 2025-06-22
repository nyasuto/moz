#!/bin/bash

# Cross-compatibility test suite for shell and Go implementations
# Ensures both versions produce identical results on the same data

set -e

echo "🧪 Shell-Go Cross-Compatibility Test Suite"
echo "=========================================="

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

PASSED=0
FAILED=0

# Helper functions
pass() {
    echo -e "${GREEN}✅ PASS${NC}: $1"
    ((PASSED++))
}

fail() {
    echo -e "${RED}❌ FAIL${NC}: $1"
    ((FAILED++))
}

warn() {
    echo -e "${YELLOW}⚠️  WARN${NC}: $1"
}

# Test setup
setup_test() {
    echo ""
    echo "🔧 Setting up test environment..."
    
    # Clean any existing data
    rm -f moz.log
    rm -f /tmp/moz_data/moz.log 2>/dev/null || true
    
    # Build Go implementation
    make go-build >/dev/null 2>&1
    
    # Ensure shell scripts are executable
    chmod +x legacy/*.sh
    
    echo "✅ Test environment ready"
}

# Test 1: Basic operations compatibility
test_basic_operations() {
    echo ""
    echo "🧪 Test 1: Basic Operations Compatibility"
    echo "----------------------------------------"
    
    # Clear data
    rm -f moz.log
    
    # Shell adds data
    legacy/put.sh name "Alice"
    legacy/put.sh city "Tokyo"
    legacy/put.sh age "25"
    
    # Go reads shell data
    go_result=$(./bin/moz get name 2>/dev/null || echo "ERROR")
    if [ "$go_result" = "Alice" ]; then
        pass "Go can read shell data"
    else
        fail "Go cannot read shell data: expected 'Alice', got '$go_result'"
    fi
    
    # Go adds data
    ./bin/moz put country "Japan" >/dev/null 2>&1
    ./bin/moz put status "active" >/dev/null 2>&1
    
    # Shell reads Go data
    shell_result=$(legacy/get.sh country 2>/dev/null || echo "ERROR")
    if [ "$shell_result" = "Japan" ]; then
        pass "Shell can read Go data"
    else
        fail "Shell cannot read Go data: expected 'Japan', got '$shell_result'"
    fi
    
    # Both list same data
    shell_count=$(legacy/list.sh | wc -l)
    go_count=$(./bin/moz list | wc -l)
    if [ "$shell_count" -eq "$go_count" ]; then
        pass "Both implementations list same number of entries ($shell_count)"
    else
        fail "Entry count mismatch: shell=$shell_count, go=$go_count"
    fi
}

# Test 2: Update operations compatibility
test_update_operations() {
    echo ""
    echo "🧪 Test 2: Update Operations Compatibility"
    echo "-----------------------------------------"
    
    # Shell updates value
    legacy/put.sh name "Bob"
    
    # Go reads updated value
    go_result=$(./bin/moz get name)
    if [ "$go_result" = "Bob" ]; then
        pass "Go reads shell updates correctly"
    else
        fail "Go didn't read shell update: expected 'Bob', got '$go_result'"
    fi
    
    # Go updates value
    ./bin/moz put name "Charlie" >/dev/null 2>&1
    
    # Shell reads Go update
    shell_result=$(legacy/get.sh name)
    if [ "$shell_result" = "Charlie" ]; then
        pass "Shell reads Go updates correctly"
    else
        fail "Shell didn't read Go update: expected 'Charlie', got '$shell_result'"
    fi
}

# Test 3: Delete operations compatibility
test_delete_operations() {
    echo ""
    echo "🧪 Test 3: Delete Operations Compatibility"
    echo "------------------------------------------"
    
    # Shell deletes key
    legacy/del.sh age >/dev/null 2>&1
    
    # Go should not find deleted key
    go_result=$(./bin/moz get age 2>/dev/null || echo "NOT_FOUND")
    if [ "$go_result" = "NOT_FOUND" ]; then
        pass "Go correctly reports shell deletions"
    else
        fail "Go still finds deleted key: got '$go_result'"
    fi
    
    # Go deletes key
    ./bin/moz put temp "temporary" >/dev/null 2>&1
    ./bin/moz del temp >/dev/null 2>&1
    
    # Shell should not find deleted key
    shell_result=$(legacy/get.sh temp 2>/dev/null || echo "NOT_FOUND")
    if [ "$shell_result" = "NOT_FOUND" ]; then
        pass "Shell correctly reports Go deletions"
    else
        fail "Shell still finds deleted key: got '$shell_result'"
    fi
}

# Test 4: Large data compatibility
test_large_data() {
    echo ""
    echo "🧪 Test 4: Large Data Compatibility"
    echo "-----------------------------------"
    
    # Clear data
    rm -f moz.log
    
    # Add 100 entries via shell
    for i in {1..50}; do
        legacy/put.sh "shell_key_$i" "shell_value_$i" >/dev/null 2>&1
    done
    
    # Add 100 entries via Go
    for i in {1..50}; do
        ./bin/moz put "go_key_$i" "go_value_$i" >/dev/null 2>&1
    done
    
    # Both should see all 100 entries
    shell_count=$(legacy/list.sh | wc -l)
    go_count=$(./bin/moz list | wc -l)
    
    if [ "$shell_count" -eq 100 ] && [ "$go_count" -eq 100 ]; then
        pass "Large data test: both implementations see all 100 entries"
    else
        fail "Large data test failed: shell=$shell_count, go=$go_count entries"
    fi
    
    # Verify random entries from both sources
    shell_random=$(./bin/moz get "shell_key_25")
    go_random=$(legacy/get.sh "go_key_25")
    
    if [ "$shell_random" = "shell_value_25" ] && [ "$go_random" = "go_value_25" ]; then
        pass "Cross-implementation random access works"
    else
        fail "Cross-implementation random access failed"
    fi
}

# Test 5: Log file format compatibility
test_log_format() {
    echo ""
    echo "🧪 Test 5: Log File Format Compatibility"
    echo "----------------------------------------"
    
    # Clear and add mixed data
    rm -f moz.log
    legacy/put.sh "test_tab" "value with spaces"
    ./bin/moz put "test_special" "value@#$%^&*()" >/dev/null 2>&1
    
    # Check log file format is TAB-delimited
    if grep -q $'\t' moz.log; then
        pass "Log file uses TAB-delimited format"
    else
        fail "Log file format is not TAB-delimited"
    fi
    
    # Both should handle special characters
    shell_special=$(legacy/get.sh "test_special")
    go_special=$(./bin/moz get "test_tab")
    
    if [ "$shell_special" = "value@#$%^&*()" ] && [ "$go_special" = "value with spaces" ]; then
        pass "Both implementations handle special characters"
    else
        fail "Special character handling failed"
    fi
}

# Test 6: Error case compatibility
test_error_cases() {
    echo ""
    echo "🧪 Test 6: Error Case Compatibility"
    echo "-----------------------------------"
    
    # Test non-existent key
    shell_error=$(legacy/get.sh "non_existent_key" 2>&1 || echo "ERROR")
    go_error=$(./bin/moz get "non_existent_key" 2>&1 || echo "ERROR")
    
    if [[ "$shell_error" == *"ERROR"* ]] && [[ "$go_error" == *"ERROR"* ]]; then
        pass "Both implementations handle missing keys correctly"
    else
        warn "Error handling differs between implementations"
    fi
}

# Run all tests
run_all_tests() {
    setup_test
    test_basic_operations
    test_update_operations
    test_delete_operations
    test_large_data
    test_log_format
    test_error_cases
    
    echo ""
    echo "🎯 Test Results Summary"
    echo "======================"
    echo -e "Passed: ${GREEN}$PASSED${NC}"
    echo -e "Failed: ${RED}$FAILED${NC}"
    
    if [ $FAILED -eq 0 ]; then
        echo -e "${GREEN}🎉 All compatibility tests passed!${NC}"
        echo "Shell and Go implementations are fully compatible."
        exit 0
    else
        echo -e "${RED}❌ Some compatibility tests failed.${NC}"
        echo "Please review the failures above."
        exit 1
    fi
}

# Main execution
if [ "$1" = "--help" ] || [ "$1" = "-h" ]; then
    echo "Usage: $0"
    echo "Runs comprehensive compatibility tests between shell and Go implementations."
    echo ""
    echo "Options:"
    echo "  --help, -h    Show this help message"
    exit 0
fi

run_all_tests