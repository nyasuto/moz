# Moz KVStore Development Guide

開発標準とワークフローガイド（Claude Code用）

## 🔨 基本原則

### Rule Evolution Process

新しいルールの追加手順:
1. "これを標準のルールにしますか？" と確認
2. YESの場合、CLAUDE.mdに追加
3. 以後の開発で標準適用

## 🛠️ Development Workflow

### Essential Commands (Makefile)

```bash
make help          # コマンド一覧
make quality       # 全品質チェック (lint + format + type-check)  
make pr-ready      # PR準備（品質チェック完全版）
make test          # テスト実行
make dev           # 開発セットアップ
```

### Git Workflow

1. **ブランチ作成**: `feat/issue-X-feature-name`
2. **変更実装**
3. **品質チェック**: `make quality`
4. **コミット**: conventional commit format
5. **PR作成**: 全変更でPR必須
6. **CI通過後**: 手動マージ（自動マージ禁止）

### Commit Format

```
<type>: <description>

🤖 Generated with [Claude Code](https://claude.ai/code)
Co-Authored-By: Claude <noreply@anthropic.com>
```

## 📋 GitHub Issues

### 🔴 重要: 日本語必須

**全てのGitHub issueは日本語で記述**

### Required Labels

- **Priority**: `priority: critical/high/medium/low`
- **Type**: `type: feature/bug/enhancement/docs/test/refactor/ci/security`

### Issue Template

```markdown
## 🎯 [種類]: [説明]

### 優先度: [緊急/高/中/低]

### 問題の説明
[具体的内容]

### 推奨解決策  
[実装方法]

### 受け入れ基準
- [ ] [条件1]
- [ ] [条件2]
```

## 🏗️ Moz Architecture

### Async I/O & WAL Performance

#### 主要実装
- ⚡ **99.8%** 書き込み応答時間短縮
- 🚀 **5-10倍** 同時書き込み性能向上
- 🛡️ Write-Ahead Logging による耐障害性
- 🔄 非ブロッキング書き込みパイプライン

#### Core Usage

```go
// Async Store作成
config := kvstore.DefaultAsyncConfig()
store, err := kvstore.NewAsyncKVStore(config)
defer store.Close()

// 非同期操作
result := store.AsyncPut("key", "value")
lsn := result.Wait() // オプション: 永続化待機
```

### Partition Management

#### Directory Structure
```
data/partitions/
├── partition_0/
├── partition_1/
├── partition_2/
└── partition_3/
```

#### Configuration
```bash
# 環境変数設定（推奨）
export MOZ_PARTITION_DIR="/path/to/partitions"
moz --partitions=4 put key value
```

## 🔧 GitHub & Tools Integration

### MCP Tools for GitHub

**GitHub操作は常にMCPツール使用**

```bash
# GitHub CLI経由
gh pr view 123
gh issue create
gh pr create

# WebFetch経由
# URL: https://github.com/owner/repo/pull/123
# Prompt: "Extract PR details including status and reviews"
```

### Wiki Management

```bash
# Wiki更新手順
git clone https://github.com/owner/repo.wiki.git /tmp/repo-wiki
# ファイル編集: Page-Title.md
git -C /tmp/repo-wiki add Page-Title.md
git -C /tmp/repo-wiki commit -m "feat: 新しいページ追加"
git -C /tmp/repo-wiki push origin master
```

## 🛡️ Security & Quality

### Code Quality Standards

- **自動化**: Makefileターゲット経由
- **一貫性**: 全環境で同一チェック
- **強制**: pre-commit hooks + CI/CD
- **高速**: 頻繁利用を促進

### Security Practices

- 秘密情報のコミット禁止
- 環境変数での設定管理
- 依存関係の定期更新
- セキュリティスキャンの実行
