#!/bin/bash

# レガシー vs 最新実装の包括的性能比較スクリプト
# Comprehensive Performance Comparison: Legacy vs Modern Implementation

set -e

echo "🚀 Moz KVストア 性能比較分析"
echo "==========================================="
echo "テスト開始時間: $(date)"
echo ""

# テストデータ設定
TEST_SIZE=100
RESULTS_FILE="performance_comparison_$(date +%Y%m%d_%H%M%S).md"

# 結果を記録する関数
log_result() {
    echo "$1" | tee -a "$RESULTS_FILE"
}

# ヘッダーを作成
cat > "$RESULTS_FILE" << EOF
# Moz KVストア 性能比較レポート

**生成日時:** $(date)  
**テストサイズ:** $TEST_SIZE 操作  
**システム:** $(uname -s) $(uname -m)  
**CPU:** Apple M4 Pro

## 性能比較結果

EOF

log_result "### 1. レガシーShell実装"
log_result ""

# レガシー実装テスト
rm -f legacy/moz.log moz.log moz.bin moz.idx

echo "📋 レガシーShell実装のテスト中..."

# PUT操作
echo "  - PUT操作テスト中..."
legacy_put_time=$(cd legacy && { time (for i in $(seq 1 $TEST_SIZE); do ./put.sh "key$i" "value$i"; done) } 2>&1 | grep total | awk '{print $1}')
legacy_put_ms=$(echo "$legacy_put_time" | sed 's/s$//' | awk '{print $1 * 1000}')

# GET操作
echo "  - GET操作テスト中..."
legacy_get_time=$(cd legacy && { time (for i in $(seq 1 $TEST_SIZE); do ./get.sh "key$i" >/dev/null; done) } 2>&1 | grep total | awk '{print $1}')
legacy_get_ms=$(echo "$legacy_get_time" | sed 's/s$//' | awk '{print $1 * 1000}')

log_result "- **PUT**: ${legacy_put_ms}ms (${TEST_SIZE}操作)"
log_result "- **GET**: ${legacy_get_ms}ms (${TEST_SIZE}操作)"
log_result "- **PUT平均**: $(echo "scale=2; $legacy_put_ms / $TEST_SIZE" | bc)ms/op"
log_result "- **GET平均**: $(echo "scale=2; $legacy_get_ms / $TEST_SIZE" | bc)ms/op"
log_result ""

log_result "### 2. Go実装（インデックスなし）"
log_result ""

# Go実装（インデックスなし）テスト
rm -f moz.log moz.bin moz.idx

echo "📋 Go実装（インデックスなし）のテスト中..."

# PUT操作
echo "  - PUT操作テスト中..."
go_put_result=$(go test -timeout=30s -bench=BenchmarkGoPut -benchtime=${TEST_SIZE}x ./internal/kvstore/ -run=^$ 2>/dev/null | grep BenchmarkGoPut | awk '{print $3}')
go_put_ns=$(echo "$go_put_result" | sed 's/ns\/op//')

# GET操作
echo "  - GET操作テスト中..."
go_get_result=$(go test -timeout=30s -bench=BenchmarkGoGet -benchtime=${TEST_SIZE}x ./internal/kvstore/ -run=^$ 2>/dev/null | grep BenchmarkGoGet | awk '{print $3}')
go_get_ns=$(echo "$go_get_result" | sed 's/ns\/op//')

log_result "- **PUT**: $(echo "scale=2; $go_put_ns / 1000000" | bc)ms/op"
log_result "- **GET**: $(echo "scale=2; $go_get_ns / 1000000" | bc)ms/op"
log_result ""

log_result "### 3. Go実装（Hash Index）"
log_result ""

echo "📋 Go実装（Hash Index）のテスト中..."

# Hash Index性能
hash_get_result=$(go test -bench=BenchmarkHashIndex_Get -benchtime=1000x ./internal/index/ -run=^$ 2>/dev/null | grep BenchmarkHashIndex_Get | awk '{print $3}')
hash_get_ns=$(echo "$hash_get_result" | sed 's/ns\/op//')

hash_insert_result=$(go test -bench=BenchmarkHashIndex_Insert -benchtime=1000x ./internal/index/ -run=^$ 2>/dev/null | grep BenchmarkHashIndex_Insert | awk '{print $3}')
hash_insert_ns=$(echo "$hash_insert_result" | sed 's/ns\/op//')

log_result "- **検索**: $(echo "scale=2; $hash_get_ns / 1000000" | bc)ms/op"
log_result "- **挿入**: $(echo "scale=2; $hash_insert_ns / 1000000" | bc)ms/op"
log_result ""

log_result "### 4. Go実装（B-Tree Index）"
log_result ""

echo "📋 Go実装（B-Tree Index）のテスト中..."

# B-Tree Index性能
btree_get_result=$(go test -bench=BenchmarkBTreeIndex_Get -benchtime=1000x ./internal/index/ -run=^$ 2>/dev/null | grep BenchmarkBTreeIndex_Get | awk '{print $3}')
btree_get_ns=$(echo "$btree_get_result" | sed 's/ns\/op//')

btree_insert_result=$(go test -bench=BenchmarkBTreeIndex_Insert -benchtime=1000x ./internal/index/ -run=^$ 2>/dev/null | grep BenchmarkBTreeIndex_Insert | awk '{print $3}')
btree_insert_ns=$(echo "$btree_insert_result" | sed 's/ns\/op//')

btree_range_result=$(go test -bench=BenchmarkBTreeIndex_Range -benchtime=100x ./internal/index/ -run=^$ 2>/dev/null | grep BenchmarkBTreeIndex_Range | awk '{print $3}')
btree_range_ns=$(echo "$btree_range_result" | sed 's/ns\/op//')

log_result "- **検索**: $(echo "scale=2; $btree_get_ns / 1000000" | bc)ms/op"
log_result "- **挿入**: $(echo "scale=2; $btree_insert_ns / 1000000" | bc)ms/op"
log_result "- **範囲検索**: $(echo "scale=2; $btree_range_ns / 1000000" | bc)ms/op"
log_result ""

log_result "### 5. 性能向上倍率"
log_result ""

# 性能向上計算
legacy_put_per_op=$(echo "scale=2; $legacy_put_ms / $TEST_SIZE" | bc)
legacy_get_per_op=$(echo "scale=2; $legacy_get_ms / $TEST_SIZE" | bc)
go_put_per_op=$(echo "scale=2; $go_put_ns / 1000000" | bc)
go_get_per_op=$(echo "scale=2; $go_get_ns / 1000000" | bc)

put_speedup=$(echo "scale=1; $legacy_put_per_op / $go_put_per_op" | bc)
get_speedup=$(echo "scale=1; $legacy_get_per_op / $go_get_per_op" | bc)

hash_get_speedup=$(echo "scale=1; $legacy_get_per_op / ($hash_get_ns / 1000000)" | bc)
btree_get_speedup=$(echo "scale=1; $legacy_get_per_op / ($btree_get_ns / 1000000)" | bc)

log_result "| 実装 | PUT | GET | 検索（Index） |"
log_result "|------|-----|-----|---------------|"
log_result "| Legacy Shell | ${legacy_put_per_op}ms | ${legacy_get_per_op}ms | - |"
log_result "| Go（基本） | ${go_put_per_op}ms (${put_speedup}x faster) | ${go_get_per_op}ms (${get_speedup}x faster) | - |"
log_result "| Go（Hash Index） | - | - | $(echo "scale=2; $hash_get_ns / 1000000" | bc)ms (${hash_get_speedup}x faster) |"
log_result "| Go（B-Tree Index） | - | - | $(echo "scale=2; $btree_get_ns / 1000000" | bc)ms (${btree_get_speedup}x faster) |"
log_result ""

log_result "### 6. まとめ"
log_result ""
log_result "#### 🚀 パフォーマンス向上"
log_result "- **Go基本実装**: Shell実装比で PUT ${put_speedup}x, GET ${get_speedup}x 高速"
log_result "- **Hash Index**: Shell実装比で検索 ${hash_get_speedup}x 高速"
log_result "- **B-Tree Index**: Shell実装比で検索 ${btree_get_speedup}x 高速、範囲検索サポート"
log_result ""
log_result "#### 🎯 技術的優位性"
log_result "- **コンパイル済みバイナリ**: 解釈実行オーバーヘッドなし"
log_result "- **メモリ内インデックス**: O(1) Hash, O(log n) B-Tree 検索"
log_result "- **並行安全性**: Mutex による安全な並行アクセス"
log_result "- **自動コンパクション**: ディスク効率の自動最適化"
log_result "- **型安全性**: コンパイル時エラー検出"
log_result ""
log_result "#### 📊 推奨用途"
log_result "- **Hash Index**: 高速キー検索が必要な場合"
log_result "- **B-Tree Index**: 範囲検索・ソート済み取得が必要な場合"
log_result "- **基本実装**: シンプルなK-V操作のみの場合"

echo ""
echo "✅ 性能比較完了！"
echo "📄 詳細レポート: $RESULTS_FILE"
echo ""
echo "🏆 結果サマリー:"
echo "  - Go実装は Shell実装より PUT ${put_speedup}x, GET ${get_speedup}x 高速"
echo "  - Hash Index検索は Shell GETより ${hash_get_speedup}x 高速"
echo "  - B-Tree Index検索は Shell GETより ${btree_get_speedup}x 高速"