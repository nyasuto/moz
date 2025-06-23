# 性能測定・比較関連のMakefileターゲット

# Performance benchmarking targets
bench-go:
	@echo "📊 Go実装ベンチマーク実行中..."
	@mkdir -p benchmark_results
	@go test -bench=BenchmarkGo -benchmem ./internal/kvstore/
	@echo "✅ Goベンチマーク完了"

bench-shell:
	@echo "📊 シェル実装ベンチマーク実行中..."
	@mkdir -p benchmark_results
	@chmod +x scripts/shell_benchmark.sh
	@scripts/shell_benchmark.sh 1000 all
	@echo "✅ シェルベンチマーク完了"

bench-compare:
	@echo "📊 Go vs シェル性能比較実行中..."
	@mkdir -p benchmark_results
	@chmod +x scripts/performance_comparison.sh
	@scripts/performance_comparison.sh 1000 both
	@echo "✅ 性能比較完了"

# バイナリフォーマット性能ベンチマーク
bench-binary:
	@echo "🚀 バイナリフォーマット性能測定実行中..."
	@mkdir -p benchmark_results
	@chmod +x scripts/binary_benchmark.sh
	@scripts/binary_benchmark.sh 1000

bench-all: bench-go bench-shell bench-compare bench-binary
	@echo "🎯 全ベンチマーク完了"
	@echo "📁 結果はbenchmark_results/ディレクトリを確認してください"

bench-quick:
	@echo "⚡ クイック性能テスト実行中..."
	@mkdir -p benchmark_results
	@chmod +x scripts/performance_comparison.sh
	@scripts/performance_comparison.sh 100 json
	@echo "✅ クイック性能テスト完了"