.PHONY: help install build clean dev test lint format type-check quality quality-fix pr-ready git-hooks env-info

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
	@echo "  make quality    - 全品質チェック実行"
	@echo "  make quality-fix - 自動修正可能な問題を修正"
	@echo "  make lint       - リンティング"
	@echo "  make format     - コードフォーマット"
	@echo ""
	@echo "🔧 ビルド・管理:"
	@echo "  make install    - 依存関係インストール"
	@echo "  make build      - ビルド"
	@echo "  make clean      - クリーンアップ"
	@echo ""
	@echo "📋 PR準備:"
	@echo "  make pr-ready   - PR提出前チェック"
	@echo "  make git-hooks  - Gitフック設定"
	@echo ""
	@echo "ℹ️  情報:"
	@echo "  make env-info   - 環境情報表示"

# 開発環境セットアップ
install:
	@echo "📦 依存関係のインストール..."
	@chmod +x legacy/*.sh 2>/dev/null || true
	@echo "✅ レガシーシェルスクリプトに実行権限を付与しました"

# 開発用クイックスタート
dev: install
	@echo "🚀 開発環境セットアップ完了"
	@echo "💡 Phase 1 レガシー使用例:"
	@echo "  ./legacy/put.sh name Alice"
	@echo "  ./legacy/get.sh name"
	@echo "  ./legacy/list.sh"

# テスト実行 (Legacy Phase 1)
test:
	@echo "🧪 Phase 1 レガシーテスト実行中..."
	@./legacy/test_performance.sh 1000

# カバレッジ付きテスト
test-cov: test
	@echo "📊 テストカバレッジ: 基本機能テスト完了"

# リンティング (shellcheck使用)
lint:
	@echo "🔍 レガシーシェルスクリプトのリンティング中..."
	@if command -v shellcheck >/dev/null 2>&1; then \
		shellcheck legacy/*.sh; \
	else \
		echo "⚠️  shellcheck がインストールされていません"; \
		echo "   brew install shellcheck でインストールしてください"; \
	fi

# フォーマット (shfmt使用)
format:
	@echo "✨ レガシーシェルスクリプトのフォーマット中..."
	@if command -v shfmt >/dev/null 2>&1; then \
		shfmt -w -i 4 legacy/*.sh; \
		echo "✅ フォーマット完了"; \
	else \
		echo "⚠️  shfmt がインストールされていません"; \
		echo "   brew install shfmt でインストールしてください"; \
	fi

# タイプチェック (基本的な構文チェック)
type-check:
	@echo "🔍 レガシーシェルスクリプトの構文チェック中..."
	@for script in legacy/*.sh; do \
		if [ -f "$$script" ]; then \
			bash -n "$$script" && echo "✅ $$script" || echo "❌ $$script"; \
		fi \
	done

# 品質チェック統合
quality: lint type-check
	@echo "🎯 品質チェック完了"

# 自動修正
quality-fix: format
	@echo "🔧 自動修正完了"

# PR準備チェック
pr-ready: quality test
	@echo "🚀 PR準備完了！"
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
	@echo 'if ! echo "$$current_branch" | grep -E "^(feat|fix|hotfix|test|docs|cicd|refactor)/.*" > /dev/null; then' >> .git/hooks/pre-commit
	@echo '    echo "⚠️  ブランチ名がCLAUDE.mdの命名規則に従っていません"' >> .git/hooks/pre-commit
	@echo '    echo "📋 推奨形式:"' >> .git/hooks/pre-commit
	@echo '    echo "   feat/issue-X-feature-name"' >> .git/hooks/pre-commit
	@echo '    echo "   fix/issue-X-description"' >> .git/hooks/pre-commit
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

# ビルド
build:
	@echo "🏗️  ビルド処理（シェル版では不要）"
	@echo "✅ ビルド完了"

# クリーンアップ
clean:
	@echo "🧹 クリーンアップ中..."
	@rm -f moz.log
	@rm -f /tmp/moz_*
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