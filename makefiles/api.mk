# REST API関連のMakefileターゲット

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