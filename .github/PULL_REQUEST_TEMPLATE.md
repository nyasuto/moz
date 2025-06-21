# 🔄 Pull Request

## 📋 Summary
<!-- このPRの内容を簡潔に説明してください -->

## 🎯 Type
<!-- 該当するものにチェックを入れてください -->
- [ ] ✨ **feat**: 新機能
- [ ] 🐛 **fix**: バグ修正  
- [ ] 📝 **docs**: ドキュメント更新
- [ ] 🎨 **style**: コードスタイル修正（機能変更なし）
- [ ] ♻️ **refactor**: リファクタリング
- [ ] ✅ **test**: テスト追加・修正
- [ ] 🔧 **chore**: ビルド・補助ツール・ライブラリ更新
- [ ] 🚀 **ci**: CI/CD設定変更

## 🔗 Related Issues
<!-- 関連するIssueがあれば記載してください -->
Closes #<!-- Issue番号 -->

## 📊 Changes
<!-- 変更内容の詳細を記載してください -->
### Added (追加)
- 

### Changed (変更)
- 

### Fixed (修正)
- 

### Removed (削除)
- 

## 🧪 Test Plan
<!-- テスト方法を記載してください -->
### Manual Testing (手動テスト)
- [ ] 基本機能の動作確認
- [ ] エラーケースの確認
- [ ] 既存機能への影響確認

### Automated Testing (自動テスト)
- [ ] `make quality` が成功する
- [ ] `make test` が成功する  
- [ ] CI/CDパイプラインが成功する

### Performance Testing (性能テスト)
<!-- 性能に影響がある場合のみ記載 -->
- [ ] 性能測定を実行し、劣化がないことを確認
- [ ] ベンチマーク結果を比較

## 🔍 Code Review Checklist
### General (一般)
- [ ] コードが読みやすく、適切にコメントされている
- [ ] CLAUDE.mdの開発ガイドラインに従っている
- [ ] 適切なエラーハンドリングが実装されている
- [ ] セキュリティの考慮がなされている

### Shell Scripts (シェルスクリプト)
<!-- シェルスクリプトを変更した場合のみチェック -->
- [ ] shellcheckでリンティングが通る
- [ ] 適切な権限設定がされている
- [ ] エラーハンドリングが実装されている
- [ ] 引数バリデーションが適切

### Documentation (ドキュメント)
- [ ] 必要に応じてREADMEを更新
- [ ] 新機能の使用方法を記載
- [ ] コメントが適切に記載されている

### Compatibility (互換性)
- [ ] 既存のmoz.logファイル形式との互換性を保持
- [ ] レガシーコードとの共存を考慮
- [ ] 既存のMakefileターゲットに影響なし

## 📱 Screenshots / Demo
<!-- UI変更や新機能のデモがあれば追加してください -->

## 📚 Additional Notes
<!-- その他の注意点や補足情報があれば記載してください -->

## ✅ Final Checklist
### Pre-submission (提出前)
- [ ] 自分でコードレビューを実施
- [ ] ローカルで全てのテストが成功
- [ ] コミットメッセージがConventional Commits形式
- [ ] ブランチ名がCLAUDE.mdの命名規則に準拠

### Ready for Review (レビュー準備完了)
- [ ] このPRは完成しており、レビュー可能な状態
- [ ] 全ての関連Issueが適切にリンクされている
- [ ] 必要なドキュメント更新が完了している

---

<!-- 
🤖 Generated with [Claude Code](https://claude.ai/code)

Co-Authored-By: Claude <noreply@anthropic.com>
-->