#!/usr/bin/env bash

RESULTS_DIR="../benchmark_results"

if [ ! -d "$RESULTS_DIR" ]; then
    echo "❌ 結果ディレクトリが見つかりません: $RESULTS_DIR"
    echo "💡 まず test_performance.sh を実行してください"
    exit 1
fi

echo "📊 Moz KVストア 性能分析レポート"
echo "=================================="
echo ""

latest_result=$(find "$RESULTS_DIR" -name "performance_*.json" -type f -exec ls -t {} + 2>/dev/null | head -1)

if [ -z "$latest_result" ]; then
    echo "❌ 性能テスト結果が見つかりません"
    exit 1
fi

echo "📄 最新結果ファイル: $(basename "$latest_result")"
echo ""

if command -v jq >/dev/null 2>&1; then
    # システム情報
    echo "🖥️  システム情報:"
    jq -r '.test_run.system_info | "  OS: \(.os)\n  Bash: \(.bash_version)"' "$latest_result"
    echo ""
    
    # テスト設定
    test_size=$(jq -r '.test_run.test_data_size' "$latest_result")
    echo "⚙️  テスト設定: ${test_size}件のデータ"
    echo ""
    
    # 性能結果
    echo "🚀 性能結果:"
    jq -r '.test_run.results[] | select(.operation != "file_analysis") | "  \(.operation | ascii_upcase): \(.ops_per_sec) ops/sec (\(.duration)s)"' "$latest_result"
    echo ""
    
    # ファイル分析
    echo "💾 ファイル分析:"
    jq -r '.test_run.results[] | select(.operation == "file_analysis") | "  サイズ: \(.file_size_bytes) bytes\n  行数: \(.line_count) lines"' "$latest_result"
    echo ""
    
    # 全結果の比較
    echo "📈 性能履歴比較:"
    find "$RESULTS_DIR" -name "performance_*.json" -type f -print0 | sort -z | while IFS= read -r -d '' result_file; do
        timestamp=$(jq -r '.test_run.timestamp' "$result_file" | cut -d'T' -f1)
        put_ops=$(jq -r '.test_run.results[] | select(.operation == "put") | .ops_per_sec' "$result_file")
        get_ops=$(jq -r '.test_run.results[] | select(.operation == "get") | .ops_per_sec' "$result_file")
        echo "  $timestamp: PUT ${put_ops} ops/sec, GET ${get_ops} ops/sec"
    done
    echo ""
    
    # 性能改善の提案
    echo "💡 性能改善の提案:"
    put_speed=$(jq -r '.test_run.results[] | select(.operation == "put") | .ops_per_sec' "$latest_result")
    get_speed=$(jq -r '.test_run.results[] | select(.operation == "get") | .ops_per_sec' "$latest_result")
    
    if (( $(echo "$put_speed < 1000" | bc -l) )); then
        echo "  📝 PUT操作: バッチ書き込みの実装を検討"
    fi
    
    if (( $(echo "$get_speed < 500" | bc -l) )); then
        echo "  🔍 GET操作: インデックス機能の実装を検討"
    fi
    
    echo "  🗜️  定期的なコンパクションでファイルサイズを最適化"
    echo "  🚀 フェーズ2のGo実装で大幅な性能向上が期待される"
    
else
    echo "⚠️  詳細分析にはjqが必要です"
    echo "💡 インストール: brew install jq"
    echo ""
    echo "📊 基本情報:"
    echo "  結果ファイル数: $(find "$RESULTS_DIR" -name "performance_*.json" -type f | wc -l)"
    echo "  最新テスト: $(basename "$latest_result")"
fi

echo ""
echo "📋 利用可能なコマンド:"
echo "  ./legacy/test_performance.sh 1000  # 1000件でテスト実行"
echo "  ./legacy/analyze_performance.sh    # このレポート表示"