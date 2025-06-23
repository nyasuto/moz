# Main Makefile - モジュール化されたMakefileシステム
# CLAUDE.md準拠の開発ワークフロー

# カラー出力設定
export TERM := xterm-256color

# デフォルトターゲット
.DEFAULT_GOAL := help

# すべてのモジュールをインクルード
include makefiles/go.mk
include makefiles/quality.mk
include makefiles/api.mk
include makefiles/benchmark.mk
include makefiles/core.mk

# ヘルプシステム（自動生成）
help: ## 📚 利用可能なコマンドを表示
	@echo "🚀 Moz KVStore - 開発コマンド一覧"
	@echo ""
	@echo "📋 クイックスタート:"
	@echo "  make dev          - 開発環境セットアップ"
	@echo "  make quality      - 品質チェック実行"
	@echo "  make test         - テスト実行"
	@echo "  make server       - REST APIサーバー起動"
	@echo ""
	@echo "🛠️  開発ツール:"
	@echo "  make install      - 依存関係インストール"
	@echo "  make build        - アプリケーションビルド"
	@echo "  make clean        - クリーンアップ"
	@echo "  make git-hooks    - Git フック設定"
	@echo ""
	@echo "🔍 品質管理:"
	@echo "  make lint         - リンティング"
	@echo "  make format       - コードフォーマット"
	@echo "  make type-check   - タイプチェック"
	@echo "  make quality-fix  - 自動修正"
	@echo "  make quality-full - 包括的品質チェック（セキュリティ含む）"
	@echo ""
	@echo "🧪 テスト:"
	@echo "  make test         - 基本テスト"
	@echo "  make test-cov     - カバレッジ付きテスト"
	@echo "  make test-api     - REST API テスト"
	@echo "  make test-api-full - 包括的 API テスト"
	@echo ""
	@echo "📊 性能測定:"
	@echo "  make bench-go     - Go実装ベンチマーク"
	@echo "  make bench-shell  - シェル実装ベンチマーク"
	@echo "  make bench-compare - 性能比較"
	@echo "  make bench-optimization - 🚀 最適化性能検証（デーモン・バッチ・プール）"
	@echo "  make bench-all    - 全ベンチマーク実行"
	@echo "  make bench-quick  - クイック性能テスト"
	@echo ""
	@echo "🌐 REST API:"
	@echo "  make server       - サーバー起動（ポート8080）"
	@echo "  make test-api     - API統合テスト"
	@echo "  make test-api-full - 包括的APIテスト"
	@echo ""
	@echo "🔧 Go関連:"
	@echo "  make go-build     - Goビルド"
	@echo "  make go-test      - Goテスト"
	@echo "  make go-run       - Go実行（ARGS=引数指定）"
	@echo "  make go-tools-install - Go開発ツールインストール"
	@echo ""
	@echo "ℹ️  環境情報:"
	@echo "  make env-info     - 環境情報表示"
	@echo ""
	@echo "📖 詳細はCLAUDE.mdを参照してください"

# PR準備用統合ターゲット（最も重要）
pr-ready: quality test ## 🚀 プルリクエスト準備（品質チェック + テスト）

# すべてのターゲットをPHONYに設定
.PHONY: help pr-ready install dev build clean test test-cov lint format type-check quality quality-fix quality-full
.PHONY: go-build go-test go-run go-clean go-tools-install go-install go-mod-tidy go-lint go-fmt go-race go-bench go-security go-dep-check
.PHONY: server test-api test-api-full
.PHONY: bench-go bench-shell bench-compare bench-binary bench-optimization bench-all bench-quick
.PHONY: git-hooks env-info