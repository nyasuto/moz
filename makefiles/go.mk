# Go関連のMakefileターゲット

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