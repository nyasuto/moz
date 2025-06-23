.PHONY: help install build clean dev test lint format type-check quality quality-fix pr-ready git-hooks env-info go-build go-test go-run go-clean go-mod-tidy go-lint go-fmt go-test-cov go-race go-bench go-install go-tools-install go-security go-dep-check bench-go bench-shell bench-compare bench-binary bench-all bench-quick server test-api test-api-full

# Default target
help:
	@echo "🔨 Moz KVストア - 利用可能なコマンド"
	@echo ""
	@echo "🚀 開発コマンド:"
	@echo "  make dev        - 開発環境セットアップと実行"
	@echo "  make test       - テスト実行"
	@echo "  make test-cov   - カバレッジ付きテスト実行"
	@echo ""
	@echo "🧹 品質チェック:"
	@echo "  make quality    - 基本品質チェック実行 (CI互換)"
	@echo "  make quality-full - 包括的品質チェック (セキュリティ含む)"
	@echo "  make quality-fix - 自動修正可能な問題を修正"
	@echo "  make lint       - リンティング"
	@echo "  make format     - コードフォーマット"
	@echo ""
	@echo "🔧 ビルド・管理:"
	@echo "  make install    - 依存関係インストール"
	@echo "  make build      - ビルド"
	@echo "  make clean      - クリーンアップ"
	@echo ""
	@echo "🐹 Go関連コマンド:"
	@echo "  make go-build   - Goアプリケーションビルド"
	@echo "  make go-test    - Goテスト実行"
	@echo "  make go-run     - Goアプリケーション実行"
	@echo "  make go-clean   - Goビルド成果物クリーンアップ"
	@echo "  make go-mod-tidy - Go依存関係整理"
	@echo "  make server     - REST APIサーバー起動"
	@echo "  make test-api   - REST API統合テスト実行"
	@echo "  make test-api-full - 包括的REST API統合テスト実行"
	@echo ""
	@echo "🔍 Go品質ツール:"
	@echo "  make go-lint    - Goコードリンティング (golangci-lint)"
	@echo "  make go-fmt     - Goコードフォーマット"
	@echo "  make go-test-cov - Goテストカバレッジ"
	@echo "  make go-race    - レース条件検出テスト"
	@echo "  make go-bench   - ベンチマークテスト"
	@echo ""
	@echo "📊 性能測定・比較:"
	@echo "  make bench-go   - Go実装ベンチマーク実行"
	@echo "  make bench-shell - シェル実装ベンチマーク実行" 
	@echo "  make bench-compare - Go vs シェル性能比較"
	@echo "  make bench-binary - バイナリフォーマット性能測定"
	@echo "  make bench-all  - 全ベンチマーク実行"
	@echo ""
	@echo "🛠️ Go開発ツール:"
	@echo "  make go-install - バイナリをGOPATH/binにインストール"
	@echo "  make go-tools-install - 開発ツールインストール"
	@echo "  make go-security - セキュリティスキャン (gosec)"
	@echo "  make go-dep-check - 脆弱性チェック (govulncheck)"
	@echo ""
	@echo "📋 PR準備:"
	@echo "  make pr-ready   - PR提出前チェック"
	@echo "  make git-hooks  - Gitフック設定"
	@echo ""
	@echo "ℹ️  情報:"
	@echo "  make env-info   - 環境情報表示"

# 開発環境セットアップ
install: go-tools-install
	@echo "📦 依存関係のインストール..."
	@chmod +x legacy/*.sh 2>/dev/null || true
	@go mod download
	@echo "✅ Go依存関係ダウンロード完了"
	@echo "✅ レガシーシェルスクリプトに実行権限を付与しました"

# 開発用クイックスタート
dev: install
	@echo "🚀 開発環境セットアップ完了"
	@echo "💡 Phase 1 レガシー使用例:"
	@echo "  ./legacy/put.sh name Alice"
	@echo "  ./legacy/get.sh name"
	@echo "  ./legacy/list.sh"

# テスト実行 (統合)
test: go-test
	@echo "🧪 レガシーテスト実行中..."
	@./legacy/test_performance.sh 1000
	@echo "🎯 全テスト完了"

# カバレッジ付きテスト
test-cov: go-test-cov test
	@echo "📊 テストカバレッジ: 基本機能テスト完了"

# リンティング (統合)
lint: go-lint
	@echo "🔍 レガシーシェルスクリプトのリンティング中..."
	@if command -v shellcheck >/dev/null 2>&1; then \
		shellcheck legacy/*.sh; \
	else \
		echo "⚠️  shellcheck がインストールされていません"; \
		echo "   brew install shellcheck でインストールしてください"; \
	fi

# フォーマット (統合)
format: go-fmt
	@echo "✨ レガシーシェルスクリプトのフォーマット中..."
	@if command -v shfmt >/dev/null 2>&1; then \
		shfmt -w -i 4 legacy/*.sh; \
		echo "✅ フォーマット完了"; \
	else \
		echo "⚠️  shfmt がインストールされていません"; \
		echo "   brew install shfmt でインストールしてください"; \
	fi

# タイプチェック (統合)
type-check:
	@echo "🔍 Goコード解析中..."
	@go fmt ./... > /dev/null
	@echo "✅ Go解析完了"
	@echo "🔍 レガシーシェルスクリプトの構文チェック中..."
	@for script in legacy/*.sh; do \
		if [ -f "$$script" ]; then \
			bash -n "$$script" && echo "✅ $$script" || echo "❌ $$script"; \
		fi \
	done

# 品質チェック統合 (ローカル用)
quality: lint type-check
	@echo "🎯 品質チェック完了"

# 包括的品質チェック (セキュリティ含む - ローカル用)
quality-full: lint type-check go-security
	@echo "🎯 包括的品質チェック完了"

# 自動修正
quality-fix: format
	@echo "🔧 自動修正完了"

# PR準備チェック (CI互換)
pr-ready: quality test
	@echo "🚀 PR準備完了！"
	@echo "💡 Note: セキュリティチェックはCI/CDで実行されます"
	@echo "📝 次のステップ:"
	@echo "  1. git add ."
	@echo "  2. git commit -m 'feat: 新機能追加'"
	@echo "  3. git push origin feature-branch"

# Gitフック設定
git-hooks:
	@echo "🔗 Gitフック設定中..."
	@mkdir -p .git/hooks
	@echo '#!/bin/bash' > .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo '# Branch protection rules from CLAUDE.md' >> .git/hooks/pre-commit
	@echo 'current_branch=$$(git rev-parse --abbrev-ref HEAD)' >> .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo '# NEVER commit directly to main branch' >> .git/hooks/pre-commit
	@echo 'if [ "$$current_branch" = "main" ]; then' >> .git/hooks/pre-commit
	@echo '    echo "❌ 直接mainブランチにコミットすることは禁止されています"' >> .git/hooks/pre-commit
	@echo '    echo "💡 フィーチャーブランチを作成してください:"' >> .git/hooks/pre-commit
	@echo '    echo "   git checkout -b feat/issue-X-feature-name"' >> .git/hooks/pre-commit
	@echo '    exit 1' >> .git/hooks/pre-commit
	@echo 'fi' >> .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo '# Check branch naming convention' >> .git/hooks/pre-commit
	@echo 'if ! echo "$$current_branch" | grep -E "^(feat|fix|hotfix|test|docs|ci|cicd|refactor|perf|security|deps|dependabot)/.*" > /dev/null; then' >> .git/hooks/pre-commit
	@echo '    echo "⚠️  ブランチ名がCLAUDE.mdの命名規則に従っていません"' >> .git/hooks/pre-commit
	@echo '    echo "📋 推奨形式:"' >> .git/hooks/pre-commit
	@echo '    echo "   feat/issue-X-feature-name"' >> .git/hooks/pre-commit
	@echo '    echo "   fix/issue-X-description"' >> .git/hooks/pre-commit
	@echo '    echo "   ci/X-description"' >> .git/hooks/pre-commit
	@echo '    echo "   docs/X-description"' >> .git/hooks/pre-commit
	@echo '    echo "   test/X-description"' >> .git/hooks/pre-commit
	@echo '    echo "   refactor/X-description"' >> .git/hooks/pre-commit
	@echo '    echo "継続しますか？ [y/N]"' >> .git/hooks/pre-commit
	@echo '    read -r response' >> .git/hooks/pre-commit
	@echo '    if [ "$$response" != "y" ] && [ "$$response" != "Y" ]; then' >> .git/hooks/pre-commit
	@echo '        exit 1' >> .git/hooks/pre-commit
	@echo '    fi' >> .git/hooks/pre-commit
	@echo 'fi' >> .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo '# Run quality checks before commit' >> .git/hooks/pre-commit
	@echo 'echo "🔍 品質チェック実行中..."' >> .git/hooks/pre-commit
	@echo 'make quality' >> .git/hooks/pre-commit
	@echo 'if [ $$? -ne 0 ]; then' >> .git/hooks/pre-commit
	@echo '    echo "❌ 品質チェックに失敗しました"' >> .git/hooks/pre-commit
	@echo '    echo "💡 修正してから再度コミットしてください"' >> .git/hooks/pre-commit
	@echo '    exit 1' >> .git/hooks/pre-commit
	@echo 'fi' >> .git/hooks/pre-commit
	@echo '' >> .git/hooks/pre-commit
	@echo 'echo "✅ 品質チェック完了"' >> .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "✅ pre-commitフック設定完了"
	@echo "📋 設定されたルール:"
	@echo "  - mainブランチへの直接コミット禁止"
	@echo "  - ブランチ命名規則チェック"
	@echo "  - 品質チェック自動実行"

# ビルド (統合)
build: go-build
	@echo "✅ ビルド完了"

# クリーンアップ (統合)
clean: go-clean
	@echo "🧹 クリーンアップ中..."
	@rm -f moz.log
	@rm -f /tmp/moz_*
	@rm -f coverage.out coverage.html
	@echo "✅ クリーンアップ完了"

# 環境情報表示
env-info:
	@echo "🔍 環境情報:"
	@echo "  OS: $$(uname -s)"
	@echo "  Shell: $$SHELL"
	@echo "  Bash: $$(bash --version | head -1)"
	@echo "  作業ディレクトリ: $$(pwd)"
	@echo "  利用可能ツール:"
	@command -v shellcheck >/dev/null 2>&1 && echo "    ✅ shellcheck" || echo "    ❌ shellcheck"
	@command -v shfmt >/dev/null 2>&1 && echo "    ✅ shfmt" || echo "    ❌ shfmt"
	@command -v awk >/dev/null 2>&1 && echo "    ✅ awk" || echo "    ❌ awk"
	@command -v go >/dev/null 2>&1 && echo "    ✅ go ($$(go version))" || echo "    ❌ go"
	@command -v golangci-lint >/dev/null 2>&1 && echo "    ✅ golangci-lint" || echo "    ❌ golangci-lint"
	@command -v gosec >/dev/null 2>&1 && echo "    ✅ gosec" || echo "    ❌ gosec"
	@command -v govulncheck >/dev/null 2>&1 && echo "    ✅ govulncheck" || echo "    ❌ govulncheck"

# Go関連ターゲット
go-build:
	@echo "🐹 Goアプリケーションビルド中..."
	@go build -o bin/moz ./cmd/moz
	@go build -o bin/moz-server ./cmd/moz-server
	@echo "✅ ビルド完了: bin/moz, bin/moz-server"

go-test:
	@echo "🧪 Goテスト実行中..."
	@go test -v ./...
	@echo "✅ テスト完了"

go-run:
	@echo "🐹 Goアプリケーション実行中..."
	@if [ -z "$(ARGS)" ]; then \
		echo "使用例: make go-run ARGS='put name Alice'"; \
		echo "      make go-run ARGS='get name'"; \
		echo "      make go-run ARGS='list'"; \
	else \
		go run ./cmd/moz $(ARGS); \
	fi

go-clean:
	@echo "🧹 Goビルド成果物クリーンアップ中..."
	@rm -rf bin/
	@go clean
	@echo "✅ クリーンアップ完了"

go-mod-tidy:
	@echo "🐹 Go依存関係整理中..."
	@go mod tidy
	@echo "✅ 依存関係整理完了"

# Go品質ツール
go-lint:
	@echo "🔍 Goコードリンティング中..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		if golangci-lint run ./...; then \
			echo "✅ golangci-lint 完了"; \
		else \
			echo "❌ golangci-lint で問題が検出されました"; \
			exit 1; \
		fi; \
	elif [ -f "$$(go env GOPATH)/bin/golangci-lint" ]; then \
		if $$(go env GOPATH)/bin/golangci-lint run ./...; then \
			echo "✅ golangci-lint 完了"; \
		else \
			echo "❌ golangci-lint で問題が検出されました"; \
			exit 1; \
		fi; \
	else \
		echo "❌ golangci-lint がインストールされていません"; \
		echo "   make go-tools-install を実行してください"; \
		exit 1; \
	fi

go-fmt:
	@echo "🎨 Goコードフォーマット中..."
	@go fmt ./...
	@echo "✅ フォーマット完了"

go-test-cov:
	@echo "📊 Goテストカバレッジ測定中..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "✅ カバレッジレポート生成完了: coverage.html"

go-race:
	@echo "🏃 レース条件検出テスト実行中..."
	@go test -race ./...
	@echo "✅ レース条件検出テスト完了"

go-bench:
	@echo "⚡ ベンチマークテスト実行中..."
	@go test -bench=. -benchmem ./...
	@echo "✅ ベンチマークテスト完了"

# Go開発ツール
go-install:
	@echo "📦 バイナリインストール中..."
	@go install ./cmd/moz
	@echo "✅ インストール完了: $$(go env GOPATH)/bin/moz"

go-tools-install:
	@echo "🛠️ Go開発ツールインストール中..."
	@echo "📦 golangci-lint インストール中..."
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest || echo "⚠️  golangci-lint インストール失敗"
	@echo "📦 govulncheck インストール中..." 
	@go install golang.org/x/vuln/cmd/govulncheck@latest || echo "⚠️  govulncheck インストール失敗"
	@echo "📦 gosec インストール中..." 
	@go install github.com/securego/gosec/v2/cmd/gosec@latest || echo "⚠️  gosec インストール失敗"
	@echo "✅ 開発ツールインストール完了"

go-security:
	@echo "🔒 セキュリティスキャン実行中..."
	@if command -v gosec >/dev/null 2>&1; then \
		if gosec ./...; then \
			echo "✅ gosec セキュリティスキャン完了 - 問題なし"; \
		else \
			echo "❌ gosec で重要なセキュリティ問題が検出されました"; \
			echo "🔍 修正が必要です"; \
			exit 1; \
		fi; \
	elif [ -f "$$(go env GOPATH)/bin/gosec" ]; then \
		if $$(go env GOPATH)/bin/gosec ./...; then \
			echo "✅ gosec セキュリティスキャン完了 - 問題なし"; \
		else \
			echo "❌ gosec で重要なセキュリティ問題が検出されました"; \
			echo "🔍 修正が必要です"; \
			exit 1; \
		fi; \
	else \
		echo "❌ gosec がインストールされていません"; \
		echo "   make go-tools-install を実行してください"; \
		exit 1; \
	fi

go-dep-check:
	@echo "🛡️ 脆弱性チェック実行中..."
	@if [ -f "$$(go env GOPATH)/bin/govulncheck" ]; then \
		$$(go env GOPATH)/bin/govulncheck ./...; \
	elif command -v govulncheck >/dev/null 2>&1; then \
		govulncheck ./...; \
	else \
		echo "⚠️  govulncheck がインストールされていません"; \
		echo "   make go-tools-install を実行してください"; \
	fi
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

# REST API Server
server:
	@echo "🌐 REST APIサーバー起動中..."
	@if [ ! -f bin/moz-server ]; then \
		echo "📦 moz-serverをビルド中..."; \
		go build -o bin/moz-server ./cmd/moz-server; \
	fi
	@echo "🚀 サーバー起動: http://localhost:8080"
	@echo "💡 使用例:"
	@echo "  curl -X POST http://localhost:8080/api/v1/login \\"
	@echo "    -H 'Content-Type: application/json' \\"
	@echo "    -d '{\"username\":\"admin\",\"password\":\"password\"}'"
	@echo ""
	@echo "🔑 認証情報:"
	@echo "  Username: admin"
	@echo "  Password: password"
	@echo ""
	@echo "📋 利用可能エンドポイント:"
	@echo "  POST /api/v1/login           - JWT認証"
	@echo "  GET  /api/v1/health          - ヘルスチェック"
	@echo "  PUT  /api/v1/kv/{key}        - データ作成・更新"
	@echo "  GET  /api/v1/kv/{key}        - データ取得"
	@echo "  DELETE /api/v1/kv/{key}      - データ削除"
	@echo "  GET  /api/v1/kv              - 全データ一覧"
	@echo "  GET  /api/v1/stats           - 統計情報"
	@echo ""
	@echo "⚠️  Ctrl+C で停止"
	@./bin/moz-server --port 8080

# REST API Integration Test  
test-api:
	@echo "🧪 REST API統合テスト実行中..."
	@if [ ! -f bin/moz-server ]; then \
		echo "📦 moz-serverをビルド中..."; \
		go build -o bin/moz-server ./cmd/moz-server; \
	fi
	@echo "🚀 テスト用サーバー起動中..."
	@./bin/moz-server --port 8081 & \
	SERVER_PID=$$!; \
	echo "⏳ サーバー起動待機中..."; \
	sleep 3; \
	echo "🔗 テスト実行中..."; \
	if SERVER_PORT=8081 ./scripts/simple_api_test.sh; then \
		echo "✅ REST API統合テスト完了"; \
		kill $$SERVER_PID 2>/dev/null || true; \
		wait $$SERVER_PID 2>/dev/null || true; \
	else \
		echo "❌ REST API統合テスト失敗"; \
		kill $$SERVER_PID 2>/dev/null || true; \
		wait $$SERVER_PID 2>/dev/null || true; \
		exit 1; \
	fi

# Comprehensive REST API Test (all endpoints)
test-api-full:
	@echo "🧪 包括的REST API統合テスト実行中..."
	@if [ ! -f bin/moz-server ]; then \
		echo "📦 moz-serverをビルド中..."; \
		go build -o bin/moz-server ./cmd/moz-server; \
	fi
	@echo "🚀 テスト用サーバー起動中..."
	@./bin/moz-server --port 8082 & \
	SERVER_PID=$$!; \
	echo "⏳ サーバー起動待機中..."; \
	sleep 3; \
	echo "🔗 包括的テスト実行中..."; \
	if SERVER_PORT=8082 ./scripts/test_rest_api.sh; then \
		echo "✅ 包括的REST API統合テスト完了"; \
		kill $$SERVER_PID 2>/dev/null || true; \
		wait $$SERVER_PID 2>/dev/null || true; \
	else \
		echo "❌ 包括的REST API統合テスト失敗"; \
		kill $$SERVER_PID 2>/dev/null || true; \
		wait $$SERVER_PID 2>/dev/null || true; \
		exit 1; \
	fi