# CLAUDE.md Template

This template provides universal best practices for Claude Code (claude.ai/code) when working with code repositories.

## 🔨 Rule Evolution Process

When receiving user instructions that should become permanent standards:

1. Ask: "これを標準のルールにしますか？" (Should this become a standard rule?)
2. If YES, add the new rule to CLAUDE.md
3. Apply as standard rule going forward

This process enables continuous improvement of project rules.

## 🛠️ Development Tools

**Use the Makefile for all development tasks!** Standardize development workflows through a comprehensive Makefile.

Essential Makefile targets to implement:

- **Quick start:** `make help` - Show all available commands
- **Code quality:** `make quality` - Run all quality checks (lint + format + type-check)
- **Auto-fix:** `make quality-fix` - Auto-fix issues where possible
- **Development:** `make dev` - Quick setup and run cycle
- **PR preparation:** `make pr-ready` - Ensure code is ready for submission
- **Git hooks:** `make git-hooks` - Setup pre-commit hooks

### Individual Quality Targets

- `make lint` - Run linting
- `make format` - Format code
- `make type-check` - Type checking
- `make test` - Run tests
- `make test-cov` - Run tests with coverage

### Development Lifecycle

- `make install` - Install dependencies
- `make build` - Build package
- `make clean` - Clean artifacts
- `make env-info` - Show environment information

## GitHub Issue Management Rules

### 🔴 CRITICAL: Issue Language Requirement

**ALL GitHub issues MUST be written in Japanese (日本語) - This is a project rule.**

### Required Issue Format

All issues must follow this Japanese template:

```markdown
## 🎯 [問題の種類]: [簡潔な説明]

### **優先度: [緊急/高/中/低]**

**影響:** [影響範囲]
**コンポーネント:** [関連コンポーネント]  
**ファイル:** [関連ファイル]

### 問題の説明

[具体的な問題内容と背景]

### 推奨解決策

[実装すべき解決策の詳細]

### 受け入れ基準

- [ ] [具体的な完了条件1]
- [ ] [具体的な完了条件2]

**[プロジェクトへの価値説明]**
```

### Required Label System

All issues MUST have both Priority and Type labels:

#### Priority Labels (優先度ラベル)

- `priority: critical` - 緊急 (アプリクラッシュ、セキュリティ問題)
- `priority: high` - 高 (コア機能、重要なバグ)
- `priority: medium` - 中 (改善、軽微なバグ)
- `priority: low` - 低 (将来機能、ドキュメント)

#### Type Labels (種類ラベル)

- `type: feature` - 新機能
- `type: bug` - バグ修正
- `type: enhancement` - 既存機能の改善
- `type: docs` - ドキュメント
- `type: test` - テスト関連
- `type: refactor` - コードリファクタリング
- `type: ci/cd` - CI/CDパイプライン
- `type: security` - セキュリティ関連

### Issue Title Examples (日本語例)

```
title: "🚨 緊急: テスト設定でのAPIキー露出問題を修正"
labels: ["priority: critical", "type: bug"]

title: "⚡ 高優先度: UIブロッキングを防ぐ非同期API呼び出しを実装"
labels: ["priority: high", "type: enhancement"]

title: "📚 低優先度: ドキュメントの正確性と完全性を更新"
labels: ["priority: low", "type: docs"]
```

## Git Workflow and Branch Management

### Core Git Rules

- **NEVER commit directly to main branch**
- Always create feature branches for changes
- Create Pull Requests for ALL changes, regardless of size
- All commits must follow conventional commit format
- Include issue references in PR descriptions: `Closes #X`

### Branch Naming Convention

Use descriptive, consistent branch names:

- Feature: `feat/issue-X-feature-name`
- Bug fix: `fix/issue-X-description`
- Hotfix: `hotfix/X-description`
- Test: `test/X-description`
- Docs: `docs/X-description`
- CI/CD: `ci/X-description` or `cicd/X-description`
- Refactor: `refactor/X-description`
- Performance: `perf/X-description`
- Security: `security/X-description`
- Dependencies: `deps/X-description`
- Automated dependency updates: `dependabot/*` (handled by Dependabot)

### Commit Message Format

```
<type>: <description>

<optional body explaining what and why>

<optional footer with issue references>
🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>
```

**Commit Types:** feat, fix, docs, style, refactor, test, chore, ci

### Required Development Workflow

1. Create feature branch from main
2. Make changes
3. **Run quality checks before commit:**
   - `make quality` (comprehensive checks)
   - OR `make quality-fix` (auto-fix + check)
4. Commit only after all checks pass
5. Push branch to remote
6. Create Pull Request with descriptive title and body
7. Wait for CI checks to pass
8. Merge via GitHub interface (not locally)

### Pre-commit Hook Setup

- Run `make git-hooks` to setup automatic quality checks
- Prevents committing code that fails quality standards
- Saves time by catching issues early

## Code Quality Standards

### Quality Check Integration

Quality checks should be:

- **Automated** through Makefile targets
- **Consistent** across all development environments
- **Enforceable** through pre-commit hooks and CI/CD
- **Fast** to encourage frequent use

### Essential Quality Tools

- **Linting:** Language-specific linters (ruff for Python, eslint for JS, etc.)
- **Formatting:** Code formatters (black/ruff for Python, prettier for JS, etc.)
- **Type Checking:** Static type analysis (mypy for Python, TypeScript, etc.)
- **Testing:** Unit and integration tests with coverage reporting

### CI/CD Integration

- All quality checks must pass in CI before merge
- Separate CI jobs for different check types (lint, test, type-check)
- Coverage reporting and tracking
- Security scanning where applicable

## Testing Standards

### Test Organization

- Unit tests for individual components
- Integration tests for system interactions
- Mocking external dependencies to avoid platform issues
- Clear test naming: `test_<function>_<scenario>_<expected_result>`

### CI Test Environment

- Mock platform-specific dependencies for cross-platform compatibility
- Use consistent test databases/fixtures
- Parallel test execution where possible
- Clear error reporting and debugging information

## Error Handling and Debugging

### Logging Standards

- Structured logging with appropriate levels
- Context-rich error messages
- Avoid logging sensitive information
- Performance-conscious logging (lazy evaluation)

### Error Recovery

- Graceful degradation for non-critical failures
- Clear error messages for users
- Retry mechanisms with exponential backoff
- Circuit breaker patterns for external services

## Documentation Standards

### Code Documentation

- Clear docstrings for public APIs
- Type hints for better IDE support
- README with setup and usage instructions
- CHANGELOG for version tracking

### Process Documentation

- This CLAUDE.md file for development standards
- Contributing guidelines for external contributors
- Architecture decision records (ADRs) for major decisions
- Troubleshooting guides for common issues

## GitHub Integration with MCP Tools

### 🔧 MCP Tool Usage for GitHub Access

**ALWAYS use MCP tools when accessing GitHub** - This provides better integration and functionality.

#### Preferred GitHub Access Methods

1. **WebFetch Tool for GitHub URLs**
   ```
   Use WebFetch tool to access GitHub pages:
   - Pull requests: https://github.com/owner/repo/pull/123
   - Issues: https://github.com/owner/repo/issues/123
   - Commits: https://github.com/owner/repo/commit/hash
   - Releases: https://github.com/owner/repo/releases
   ```

2. **GitHub CLI Integration**
   ```bash
   # Use gh command via Bash tool for GitHub operations
   gh pr view 123           # View pull request details
   gh issue view 123        # View issue details
   gh pr create             # Create pull requests
   gh issue create          # Create issues
   gh repo view             # View repository information
   ```

3. **WebFetch Prompt Guidelines**
   ```
   When using WebFetch for GitHub URLs, use specific prompts:
   - "Extract pull request details including title, status, description, comments, and reviews"
   - "Get issue information including labels, assignees, status, and discussion"
   - "Summarize commit details including changes, files modified, and commit message"
   ```

#### GitHub Access Workflow

1. **Always try MCP tools first** before manual URL construction
2. **Use WebFetch** for viewing GitHub content (PRs, issues, commits)
3. **Use Bash + gh command** for GitHub operations (create, update, merge)
4. **Verify results** by re-fetching the updated content via MCP

#### Examples

```bash
# Create PR using gh command
gh pr create --title "feat: new feature" --body "Description"

# View PR using WebFetch
WebFetch: https://github.com/owner/repo/pull/123
Prompt: "Extract all PR details including status and reviews"

# Check issue status
gh issue view 123 --json state,labels,assignees
```

### MCP Tool Benefits for GitHub

- **Real-time data**: Always gets current GitHub state
- **Rich content**: Extracts formatted information and metadata
- **Integrated workflow**: Seamless integration with development process
- **Error handling**: Better error messages and retry capabilities

## Asynchronous I/O & Write-Ahead Logging (WAL)

### 🚀 High-Performance Async Architecture

**Moz KVStore now includes enterprise-level asynchronous I/O and WAL capabilities** for dramatic performance improvements:

#### Performance Achievements
- ⚡ **99.8% reduction** in write response time  
- 🚀 **5-10x improvement** in concurrent write throughput
- 🛡️ Enhanced data durability and crash recovery
- 🔄 Non-blocking write pipeline

#### Core Components

1. **Write-Ahead Logging (WAL)**
   ```go
   // WAL ensures durability and crash recovery
   config := kvstore.DefaultWALConfig()
   wal, err := kvstore.NewWAL(config)
   ```

2. **In-Memory Buffer (MemTable)**
   ```go
   // Fast in-memory writes with background flush
   memTable := kvstore.NewMemTable(kvstore.DefaultMemTableConfig())
   ```

3. **Async Write Pipeline**
   ```go
   // Non-blocking async operations
   result := store.AsyncPut("key", "value")
   lsn := result.Wait() // Optional wait for durability
   ```

#### Usage Examples

```go
// Create async store
config := kvstore.DefaultAsyncConfig()
store, err := kvstore.NewAsyncKVStore(config)
defer store.Close()

// Async operations (immediate response)
result := store.AsyncPut("user:123", "alice")
lsn, err := result.LSN, result.Wait() // Non-blocking + optional wait

// Reads from MemTable + disk
value, err := store.Get("user:123") // Fast retrieval

// Force durability
store.ForceFlush() // Ensures all data is persisted
```

#### Configuration Options

```go
config := kvstore.AsyncConfig{
    WALConfig: kvstore.WALConfig{
        DataDir:      "data/wal",
        BufferSize:   10000,
        FlushTimeout: 100 * time.Millisecond,
        MaxFileSize:  64 * 1024 * 1024, // 64MB
    },
    MemTableConfig: kvstore.MemTableConfig{
        MaxSize:      16 * 1024 * 1024, // 16MB
        MaxEntries:   100000,
        FlushTimeout: 30 * time.Second,
    },
    EnableAsync: true, // false for sync fallback
}
```

#### Crash Recovery

```go
// Automatic recovery on startup
recoveryManager := kvstore.NewRecoveryManager(wal, baseStore, memTable)
err := recoveryManager.RecoverFromWAL()

// Integrity validation
err = recoveryManager.ValidateWALIntegrity()
```

#### Benefits
- **Immediate Response**: Write operations return instantly
- **High Throughput**: Concurrent writes with minimal blocking
- **Data Safety**: WAL ensures no data loss on crashes
- **Automatic Recovery**: Seamless restart after failures
- **Configurable Durability**: Balance performance vs safety

## Partition Directory Management

### 🗂️ Partition Storage Configuration

**Partition directories are automatically organized** to avoid cluttering the workspace:

#### Default Configuration
- **Partition Location**: `data/partitions/` (instead of current directory)
- **Environment Override**: `MOZ_PARTITION_DIR` environment variable
- **Directory Structure**: 
  ```
  data/partitions/
  ├── partition_0/
  ├── partition_1/
  ├── partition_2/
  └── partition_3/
  ```

#### Configuration Methods

1. **Environment Variable (Recommended)**
   ```bash
   export MOZ_PARTITION_DIR="/path/to/partitions"
   moz --partitions=4 put key value
   ```

2. **Default Behavior**
   ```bash
   # Creates partitions in data/partitions/ subdirectory
   moz --partitions=4 put key value
   ```

3. **Programmatic Configuration**
   ```go
   config := kvstore.PartitionConfig{
       NumPartitions: 4,
       DataDir:       "custom/partition/path",
       BatchSize:     100,
       FlushInterval: 100 * time.Millisecond,
   }
   ```

#### Benefits
- **Clean Workspace**: No partition clutter in project root
- **Organized Storage**: Dedicated directory structure
- **Environment Flexibility**: Configurable storage location
- **Easy Cleanup**: Single directory to remove when needed

#### Partition File Structure
Each partition contains:
- `moz_p{id}.log` - Text format log file
- `moz_p{id}.bin` - Binary format file (if enabled)  
- `moz_p{id}.idx` - Index file (if enabled)

## GitHub Wiki Management

### 📝 Wiki Update Workflow

**GitHub Wiki は専用リポジトリとして管理** - Wiki更新は以下の標準的手順に従う:

#### Standard Wiki Update Process

1. **Wiki Repository Clone**
   ```bash
   # Wiki は独立したGitリポジトリとして管理される
   git clone https://github.com/owner/repo.wiki.git /tmp/repo-wiki
   ```

2. **Content Creation/Update**
   ```bash
   # Wikiページをマークダウンファイルとして作成・編集
   # ファイル名: Page-Title.md (スペースはハイフンに変換)
   vim /tmp/repo-wiki/New-Wiki-Page.md
   ```

3. **Git Operations**
   ```bash
   # 通常のGit操作でWikiを更新
   git -C /tmp/repo-wiki add New-Wiki-Page.md
   git -C /tmp/repo-wiki commit -m "feat: 新しいWikiページを追加"
   git -C /tmp/repo-wiki push origin master
   ```

#### Wiki Naming Conventions

- **ファイル名**: `Page-Title.md` (スペースは `-` で置換)
- **URL**: `https://github.com/owner/repo/wiki/Page-Title`
- **Home Page**: `Home.md` (リポジトリWikiのトップページ)

#### Wiki Content Standards

1. **構造化マークダウン**
   - 明確な見出し階層 (`#`, `##`, `###`)
   - 適切な表・リスト・コードブロック使用
   - 内部リンク: `[[Other-Wiki-Page]]`

2. **メタデータ情報**
   ```markdown
   **📝 作成者**: Claude Code
   **📅 作成日時**: YYYY年MM月DD日
   **🔍 分析手法**: 実測ベンチマーク・プロファイリング
   **📊 データ信頼性**: 複数回測定による平均値・再現性確認済み
   ```

3. **日本語コンテンツ標準**
   - 技術文書は日本語で記述
   - 重要な用語は **太字** で強調
   - 絵文字による視覚的区分 (📊, 🚀, ⚡, 🎯 等)

#### Common Wiki Operations

```bash
# 新規Wiki作成
git clone https://github.com/owner/repo.wiki.git /tmp/repo-wiki
echo "# New Page Content" > /tmp/repo-wiki/New-Page.md
git -C /tmp/repo-wiki add New-Page.md
git -C /tmp/repo-wiki commit -m "feat: 新しいページ追加"
git -C /tmp/repo-wiki push origin master

# 既存Wiki更新
git -C /tmp/repo-wiki pull origin master
# Edit files...
git -C /tmp/repo-wiki add .
git -C /tmp/repo-wiki commit -m "docs: ページ内容更新"
git -C /tmp/repo-wiki push origin master

# Wiki削除
git -C /tmp/repo-wiki rm Old-Page.md
git -C /tmp/repo-wiki commit -m "docs: 不要ページ削除"
git -C /tmp/repo-wiki push origin master
```

#### Wiki Security Notes

- **Public Access**: GitHub Wikiは通常パブリックアクセス
- **Edit Permissions**: リポジトリ書き込み権限が必要
- **Sensitive Information**: 秘密情報・APIキー等は記載禁止
- **Archive Policy**: 重要文書は定期的にローカルバックアップ

#### Wiki vs Documentation Strategy

| Content Type | Location | Purpose |
|--------------|----------|---------|
| **API Reference** | `/docs` in main repo | コード同期・バージョン管理 |
| **User Guides** | GitHub Wiki | 使用方法・チュートリアル |
| **Analysis Reports** | GitHub Wiki | 性能分析・調査結果 |
| **Development Process** | `CLAUDE.md` | 開発ルール・手順 |

## Security Considerations

### Secrets Management

- Never commit secrets to version control
- Use environment variables for configuration from .env
- Scan for accidentally committed secrets

### Dependency Management

- Regular dependency updates by dependabot
- Security vulnerability scanning
- Pin versions for reproducible builds
