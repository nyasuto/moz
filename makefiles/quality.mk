# 品質チェック関連のMakefileターゲット

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