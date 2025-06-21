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
- CI/CD: `cicd/X-description`

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

## Security Considerations

### Secrets Management

- Never commit secrets to version control
- Use environment variables for configuration from .env
- Scan for accidentally committed secrets

### Dependency Management

- Regular dependency updates by dependabot
- Security vulnerability scanning
- Pin versions for reproducible builds
