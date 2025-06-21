#!/usr/bin/env bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

TEST_DATA_SIZE=${1:-10000}
RESULTS_DIR="../benchmark_results"
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
RESULT_FILE="$RESULTS_DIR/performance_${TIMESTAMP}.json"

mkdir -p "$RESULTS_DIR"

echo "🚀 Moz KVストア 性能テスト開始"
echo "📊 テストデータサイズ: $TEST_DATA_SIZE 件"
echo "📁 結果保存先: $RESULT_FILE"
echo ""

log_result() {
    local operation="$1"
    local duration="$2"
    local count="$3"
    local ops_per_sec
    ops_per_sec=$(echo "scale=2; $count / $duration" | bc -l 2>/dev/null || echo "0")
    
    echo "  ⏱️  実行時間: ${duration}s"
    echo "  📈 処理速度: ${ops_per_sec} ops/sec"
    
    cat >> "$RESULT_FILE" << EOF
    {
      "operation": "$operation",
      "duration": $duration,
      "count": $count,
      "ops_per_sec": $ops_per_sec,
      "timestamp": "$(date -Iseconds)"
    },
EOF
}

measure_time() {
    local start_time
    local end_time
    start_time=$(date +%s.%N)
    "$@"
    end_time=$(date +%s.%N)
    echo "$end_time - $start_time" | bc -l
}

generate_test_data() {
    echo "📝 テストデータ生成中..."
    local count=$1
    for i in $(seq 1 "$count"); do
        echo -e "key_${i}\tvalue_data_${i}_$(date +%s%N | tail -c 10)"
    done
}

test_put_operations() {
    echo "🔧 PUT 操作テスト (${TEST_DATA_SIZE}件)"
    rm -f moz.log
    
    local duration
    duration=$(measure_time bash -c "
        for i in \$(seq 1 $TEST_DATA_SIZE); do
            ./put.sh \"test_key_\$i\" \"test_value_\$i\"
        done
    ")
    
    log_result "put" "$duration" "$TEST_DATA_SIZE"
}

test_get_operations() {
    echo "🔍 GET 操作テスト (${TEST_DATA_SIZE}件)"
    
    local duration
    duration=$(measure_time bash -c "
        for i in \$(seq 1 $TEST_DATA_SIZE); do
            ./get.sh \"test_key_\$i\" > /dev/null
        done
    ")
    
    log_result "get" "$duration" "$TEST_DATA_SIZE"
}

test_list_operation() {
    echo "📋 LIST 操作テスト"
    
    local duration
    duration=$(measure_time ./list.sh > /dev/null)
    duration=${duration:-0.001}
    
    log_result "list" "$duration" "1"
}

test_filter_operation() {
    echo "🔎 FILTER 操作テスト"
    
    local duration
    duration=$(measure_time ./filter.sh "test_key_1" > /dev/null)
    duration=${duration:-0.001}
    
    log_result "filter" "$duration" "1"
}

test_compact_operation() {
    echo "🗜️ COMPACT 操作テスト"
    
    local duration
    duration=$(measure_time ./compact.sh)
    
    log_result "compact" "$duration" "1"
}

test_mixed_workload() {
    echo "🔄 混合ワークロードテスト"
    rm -f moz.log
    
    local half_size=$((TEST_DATA_SIZE / 2))
    local duration
    
    duration=$(measure_time bash -c "
        # PUT操作
        for i in \$(seq 1 $half_size); do
            ./put.sh \"mixed_key_\$i\" \"mixed_value_\$i\"
        done
        
        # GET操作
        for i in \$(seq 1 $half_size); do
            ./get.sh \"mixed_key_\$i\" > /dev/null
        done
        
        # 更新操作
        for i in \$(seq 1 $((half_size / 2))); do
            ./put.sh \"mixed_key_\$i\" \"updated_value_\$i\"
        done
        
        # 削除操作
        for i in \$(seq 1 $((half_size / 4))); do
            ./del.sh \"mixed_key_\$i\"
        done
        
        # コンパクション
        ./compact.sh
    ")
    
    local total_ops=$((half_size * 2 + half_size / 2 + half_size / 4 + 1))
    log_result "mixed_workload" "$duration" "$total_ops"
}

analyze_file_size() {
    if [ -f "moz.log" ]; then
        local file_size
        local line_count
        file_size=$(wc -c < moz.log)
        line_count=$(wc -l < moz.log)
        echo "📊 ファイルサイズ分析:"
        echo "  💾 ファイルサイズ: ${file_size} bytes"
        echo "  📄 行数: ${line_count} lines"
        
        cat >> "$RESULT_FILE" << EOF
    {
      "operation": "file_analysis",
      "file_size_bytes": $file_size,
      "line_count": $line_count,
      "timestamp": "$(date -Iseconds)"
    },
EOF
    fi
}

# メイン実行
{
    echo "{"
    echo "  \"test_run\": {"
    echo "    \"timestamp\": \"$(date -Iseconds)\","
    echo "    \"test_data_size\": $TEST_DATA_SIZE,"
    echo "    \"system_info\": {"
    echo "      \"os\": \"$(uname -s)\","
    echo "      \"bash_version\": \"$(bash --version | head -1)\""
    echo "    },"
    echo "    \"results\": ["
} > "$RESULT_FILE"

test_put_operations
test_get_operations
test_list_operation
test_filter_operation
test_compact_operation
test_mixed_workload
analyze_file_size

# JSONファイルの終了
sed -i '' '$s/,$//' "$RESULT_FILE" 2>/dev/null || sed -i '$s/,$//' "$RESULT_FILE"
cat >> "$RESULT_FILE" << EOF
    ]
  }
}
EOF

echo ""
echo "✅ 性能テスト完了!"
echo "📊 結果ファイル: $RESULT_FILE"
echo ""
echo "📈 性能サマリー:"
if command -v jq >/dev/null 2>&1; then
    jq -r '.test_run.results[] | select(.operation != "file_analysis") | "  \(.operation): \(.ops_per_sec) ops/sec"' "$RESULT_FILE"
else
    echo "  💡 詳細な分析にはjqをインストールしてください: brew install jq"
fi