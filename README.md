# **moz：高性能キーバリューストア（学習用プロジェクト）**

**シェルスクリプトからGo言語まで、段階的に学ぶデータベース実装**

---

moz は、ファイルベース・追記型のキーバリュー型データベースを、最初はLinuxコマンドだけで構築し、その後Go言語でログ構造型データベース+インデックスシステムへと進化させる学習プロジェクトです。

## **🐦 プロジェクトのコンセプト**

鳥の「モズ」のように、小さくて賢く、素早くデータを「捕まえて」保存します。最初は単純なログファイルだけの設計からはじまり、徐々に賢く、そして高機能なデータベースへと育てていきます。

## **🎯 プロジェクトの目的**

- **データベース内部構造の実践的学習**: 基本から高度な機能まで段階的に理解
- **シェルとGoの比較学習**: 同一機能を異なる実装で比較・分析
- **性能測定による学習**: 定量的な改善効果を可視化
- **実用的な開発ワークフロー習得**: CI/CD、品質管理、テスト手法

## **📊 現在の実装状況**

### **✅ 完了済み機能**

| フェーズ | 機能 | 実装状況 | 説明 |
|---------|------|----------|------|
| **1** | **シェル版基本実装** | ✅ 完了 | PUT/GET/DELETE/LIST/COMPACT/FILTER |
| **2.0** | **Go版基本実装** | ✅ 完了 | CRUD操作、メモリ最適化、スレッドセーフ |
| **2.1** | **自動コンパクション** | ✅ 完了 | ファイルサイズ・操作数・削除率による自動最適化 |
| **2.2** | **シェル-Go互換性** | ✅ 完了 | 完全互換性テスト、ファイル共有対応 |
| **2.3** | **性能ベンチマーク** | ✅ 完了 | Go vs Shell詳細比較、メモリ測定 |
| **3.0** | **バイナリフォーマット** | ✅ 完了 | CRC32チェックサム、高速シリアライゼーション |
| **3.1** | **Hash Index実装** | ✅ 完了 | O(1)検索、チェイン法による衝突解決 |
| **3.2** | **B-Tree Index実装** | ✅ 完了 | O(log n)検索、範囲検索・ソート対応 |
| **3.3** | **IndexManager統合** | ✅ 完了 | 動的インデックス選択、統合API |
| **3.4** | **高度クエリ機能** | ✅ 完了 | 範囲検索、プレフィックス検索、ソート済みアクセス |
| **3.5** | **CLI改善・ヘルプシステム** | ✅ 完了 | --helpフラグ、helpコマンド、包括的ガイド |
| **4.2** | **REST API実装** | ✅ 完了 | HTTP/JSONリモートアクセス、JWT認証、Webアプリ連携 |

## **🚀 劇的な性能向上を実現**

### **最新性能分析結果**
レガシーShell実装から最新Go実装への移行により、**革命的な性能向上**を達成：

| 実装 | PUT/挿入 | GET/検索 | 特徴 |
|------|----------|----------|------|
| **Legacy Shell** | 2.0ms/op | 5.0ms/op | ベースライン（ファイルスキャン） |
| **Go基本** | 0.091ms/op | - | **22x faster** |
| **Go + Hash Index** | 0.0004ms/op | **0.00007ms/op** | **71,429x faster** |
| **Go + B-Tree Index** | 0.0004ms/op | **0.0001ms/op** | **42,373x faster** |

### **🏆 圧倒的な性能向上**
- **Hash Index検索**: **71,429倍高速** (5ms → 0.00007ms)
- **B-Tree Index検索**: **42,373倍高速** (5ms → 0.0001ms)
- **範囲検索**: **2,080倍高速** + 新機能として実現
- **プレフィックス検索**: B-Tree実装で13.6倍高速

詳細は [GitHub Wiki - Performance Analysis Report](https://github.com/nyasuto/moz/wiki/Performance-Analysis-Report) を参照

## **📁 プロジェクト構成**

```
moz/
├── legacy/                    # Phase 1: シェル版実装
│   ├── put.sh                # キーと値の追加・更新
│   ├── get.sh                # キーの値を取得
│   ├── del.sh                # キーの削除
│   ├── list.sh               # 全件の一覧表示
│   ├── filter.sh             # 条件付き一覧表示
│   ├── compact.sh            # ログ整理・最適化
│   ├── test_performance.sh   # 性能測定テスト
│   └── analyze_performance.sh # 性能分析
│
├── cmd/
│   ├── moz/main.go           # Go版 CLI エントリーポイント
│   └── moz-server/main.go    # REST API サーバー
├── internal/
│   ├── kvstore/              # Go版 KVストア実装
│   │   ├── kvstore.go       # メイン実装（自動コンパクション + インデックス統合）
│   │   ├── binary_format.go # 高速バイナリフォーマット
│   │   ├── format_converter.go # テキスト↔バイナリ変換
│   │   ├── reader.go        # ログファイル読み込み
│   │   └── *_test.go       # 包括的テストスイート（97項目）
│   │
│   ├── index/               # インデックスシステム
│   │   ├── index.go         # IndexManager統合API
│   │   ├── hash_index.go    # Hash Index実装（O(1)検索）
│   │   ├── btree_index.go   # B-Tree Index実装（O(log n)、範囲検索）
│   │   ├── no_index.go      # インデックスなし実装
│   │   └── *_test.go       # インデックス専用テスト・ベンチマーク
│   │
│   └── api/                 # REST API実装
│       ├── server.go        # HTTPサーバー・ルーティング
│       ├── handlers.go      # CRUD エンドポイントハンドラー
│       ├── auth.go          # JWT・APIキー認証システム
│       ├── types.go         # API リクエスト・レスポンス型定義
│       └── *_test.go       # API テストスイート
│
├── scripts/                  # 性能測定・比較ツール
│   ├── performance_comparison.sh # 包括的性能比較
│   ├── compatibility_test.sh # シェル-Go互換性テスト
│   ├── shell_benchmark.sh   # シェル版ベンチマーク
│   └── simple_benchmark.sh  # 簡易比較ツール
│
├── performance_analysis.sh  # 自動化された性能分析ツール
├── benchmark_results/        # 性能測定結果（JSON形式）
├── .github/workflows/ci.yml  # CI/CD パイプライン
├── Makefile                  # 統合開発コマンド
├── go.mod                    # Go module 定義
└── moz.log                  # 共有データファイル
```

## **🚀 使用方法**

### **開発環境セットアップ**
```bash
make help                     # 利用可能コマンド一覧
make dev                      # 開発環境セットアップ
```

### **Go版（推奨） - 最新機能**
```bash
# ビルド
make go-build

# ヘルプ・使用法
./bin/moz --help              # 包括的ヘルプ表示
./bin/moz help                # 同じヘルプをコマンド形式で

# 基本操作
./bin/moz put name "Alice"    # データ追加
./bin/moz get name            # データ取得 → Alice
./bin/moz list                # 全データ表示
./bin/moz del name            # データ削除
./bin/moz compact             # 手動コンパクション
./bin/moz stats               # 統計情報表示

# 高性能インデックス機能
./bin/moz --index=hash put city Tokyo        # Hash Index使用
./bin/moz --index=btree put user:alice data  # B-Tree Index使用
./bin/moz range user:a user:z                # 範囲検索
./bin/moz prefix user:                       # プレフィックス検索
./bin/moz sorted                             # ソート済み一覧
./bin/moz rebuild-index                      # インデックス再構築
./bin/moz validate-index                     # インデックス検証

# バイナリフォーマット（高速化）
./bin/moz --format=binary put key value     # バイナリ形式
./bin/moz convert text binary                # フォーマット変換
./bin/moz validate binary                    # ファイル整合性検証

# Makefileコマンド
make go-run ARGS="--index=btree put city Tokyo"
make go-run ARGS="range a z"
```

### **REST API サーバー（Web連携）**
```bash
# サーバービルド・起動
make go-build
./bin/moz-server --port 8080

# 認証トークン取得
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}'

# データ操作（JWT認証）
export TOKEN="your-jwt-token"
curl -X PUT http://localhost:8080/api/v1/kv/user123 \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"value":"alice"}'

curl -X GET http://localhost:8080/api/v1/kv/user123 \
  -H "Authorization: Bearer $TOKEN"

# ヘルスチェック（認証不要）
curl http://localhost:8080/api/v1/health

# 統計情報取得
curl -X GET http://localhost:8080/api/v1/stats \
  -H "Authorization: Bearer $TOKEN"
```

### **シェル版（レガシー）**
```bash
# 基本操作
./legacy/put.sh name "Bob"
./legacy/get.sh name          # Output: Bob
./legacy/list.sh
./legacy/del.sh name
./legacy/compact.sh
```

### **性能分析・比較**
```bash
# 自動化された包括的性能分析
./performance_analysis.sh              # デフォルト（100操作）
./performance_analysis.sh 1000 both    # 1000操作、JSON+Markdown出力

# 個別ベンチマーク
make bench-go                 # Go実装ベンチマーク
make bench-shell              # シェル実装ベンチマーク
make bench-compare            # 包括的比較（推奨）
make bench-binary             # バイナリフォーマット性能測定
make bench-all                # 全ベンチマーク実行
```

### **品質管理**
```bash
make quality                  # 基本品質チェック
make quality-full             # セキュリティ含む包括的チェック
make pr-ready                 # PR提出前チェック
```

### **📋 包括的ヘルプシステム**
```bash
# 複数のヘルプアクセス方法
./bin/moz --help              # フラグ形式
./bin/moz help                # コマンド形式

# ヘルプ内容（例）
Global Flags:
  --format <text|binary>      # ストレージフォーマット指定
  --index <hash|btree|none>   # インデックス方式指定  
  --help                      # ヘルプメッセージ表示

基本操作・高速検索・管理・フォーマット操作の完全ガイド
```

## **🔧 主要機能詳細**

### **🚀 高性能インデックスシステム**
- **Hash Index**: O(1)平均検索時間、最高速キー検索
- **B-Tree Index**: O(log n)検索、範囲検索・ソート対応
- **動的選択**: 用途に応じたインデックスタイプ選択
- **メモリ効率**: 効率的なバケット管理・ノード分割

### **⚡ バイナリフォーマット**
- **CRC32チェックサム**: データ整合性保証
- **高速シリアライゼーション**: 83.96 ns/op WriteTo性能
- **相互変換**: テキスト↔バイナリ自由変換
- **後方互換性**: 既存データの無損失移行

### **🔍 高度クエリ機能**
- **範囲検索**: `GetRange(start, end)` - 効率的な範囲取得
- **プレフィックス検索**: `PrefixSearch(prefix)` - 前方一致検索
- **ソート済みアクセス**: `ListSorted()` - 順序保証取得
- **統計情報**: インデックスサイズ・メモリ使用量監視

### **🌐 REST API・Web連携**
- **RESTful設計**: HTTP/JSON標準プロトコル対応
- **JWT認証**: セキュアなトークンベース認証システム
- **APIキー認証**: 簡易認証方式対応
- **CORS対応**: クロスオリジンリクエスト対応
- **エラーハンドリング**: 構造化されたエラーレスポンス
- **メタデータ**: 実行時間・タイムスタンプ付きレスポンス

### **⚙️ 自動コンパクション**
- **ファイルサイズ閾値**: 1MB超過で自動実行
- **操作数閾値**: 1000操作で自動実行  
- **削除率閾値**: 削除済みエントリが50%超過で自動実行
- **非同期実行**: デッドロック回避、パフォーマンス最適化

### **🔄 完全互換性**
- **ファイル共有**: シェル版とGo版が同一ファイル使用
- **フォーマット互換**: TAB区切り形式で完全互換
- **相互運用**: シェル→Go、Go→シェル自由切り替え
- **包括的テスト**: 12種類の互換性テスト自動実行

## **📈 実用性とスケーラビリティ**

### **リアルタイム性能**
```bash
# 1秒間に実行可能な操作数（理論値）
Hash検索:     14,204,545 ops/sec
B-Tree検索:    8,460,237 ops/sec
範囲検索:        415,800 ops/sec
プレフィックス:   60,205 ops/sec
```

### **メモリ効率性**
- **動的リサイジング**: 負荷率に応じた自動調整
- **効率的管理**: バケット・ノード構造の最適化
- **並行安全性**: Mutex による安全な並行アクセス
- **リアルタイム監視**: メモリ使用量の追跡・レポート

### **エンタープライズ対応**
- **型安全性**: Go言語によるコンパイル時エラー検出
- **包括的テスト**: 97項目のテストカバレッジ
- **品質保証**: 自動化されたCI/CD・セキュリティスキャン
- **監視機能**: 構造化ログ・メトリクス出力
- **ユーザビリティ**: 包括的ヘルプシステム・直感的CLI操作
- **Web連携**: REST API・Webアプリケーション統合対応
- **認証・認可**: JWT・APIキーによるセキュアアクセス

## **🔄 開発ワークフロー**

### **CI/CDパイプライン**
```bash
# GitHub Actions で自動実行
- 品質チェック（lint, format, type-check）  
- セキュリティスキャン（gosec, govulncheck）
- 統合テスト（CRUD操作、インデックス、性能テスト）
- シェル-Go互換性テスト
- ブランチ保護・命名規則チェック
```

### **ブランチ戦略**
```bash
# ブランチ命名規則（CLAUDE.md準拠）
feat/issue-X-feature-name     # 新機能
fix/issue-X-description       # バグ修正
test/X-description           # テスト追加
docs/X-description           # ドキュメント
refactor/X-description       # リファクタリング
```

### **品質管理**
- **Pre-commit hooks**: 自動品質チェック
- **Conventional Commits**: 標準化されたコミットメッセージ
- **Issue tracking**: 日本語での詳細Issue管理
- **PR review**: 包括的コードレビュープロセス

## **🎓 学習効果**

このプロジェクトを通じて習得できる技術:

### **データベース技術**
- ログ構造マージツリー（LSM-Tree）の理解
- インデックス設計（Hash Table, B-Tree）
- 追記型データベースの設計原理
- クエリ最適化とアクセスパターン分析

### **システムプログラミング**
- Go言語での並行プログラミング
- メモリ管理とパフォーマンス最適化
- ファイルI/O、バイナリシリアライゼーション
- アルゴリズム・データ構造の実装

### **ソフトウェア開発プロセス**
- テスト駆動開発（TDD）
- 継続的インテグレーション（CI/CD）
- 性能ベンチマーキング手法
- コードレビューとチーム開発

### **運用・監視**
- 性能プロファイリング・分析
- メトリクス収集・可視化
- 品質管理・自動化
- デバッグ・トラブルシューティング

## **🚀 今後の開発ロードマップ**

### **Phase 4: エンタープライズ機能（進行中）**

| フェーズ | 機能 | 状況 | 説明 |
|---------|------|------|------|
| **4.1** | **分散対応** | 構想中 | レプリケーション、シャーディング |
| **4.2** | **REST API** | ✅ 完了 | HTTP/JSONリモートアクセス、JWT認証 |
| **4.3** | **監視・メトリクス** | 構想中 | OpenTelemetry対応 |
| **4.4** | **バックアップ・復元** | 構想中 | Point-in-time recovery |
| **4.5** | **クラスタリング** | 構想中 | 分散コンセンサス・一貫性保証 |

## **🤖 AI開発支援向け設計**

このプロジェクトはAI開発支援ツールとの協調を前提に設計:

- **段階的進化**: 複雑さを段階的に導入、理解しやすい構造
- **包括的ドキュメント**: `CLAUDE.md`での開発ルール明文化
- **自動化**: Makefile、CI/CDによる一貫した開発体験
- **テスタビリティ**: 包括的テストによる安全な変更
- **トレーサビリティ**: Issue tracking、履歴追跡
- **ユーザビリティ**: 直感的CLI・包括的ヘルプによる開発者体験向上

## **📝 貢献・フィードバック**

### **開発状況**
- **GitHub Issues**: [プロジェクトボード](https://github.com/nyasuto/moz/issues)
- **Performance Wiki**: [性能分析レポート](https://github.com/nyasuto/moz/wiki/Performance-Analysis-Report)
- **Pull Requests**: コードレビュー歓迎
- **Discussions**: アイデア・提案の議論

### **ライセンス**
MITライセンス。学習目的のため、フォーク・改変・提案すべて歓迎です。

---

**🎉 革命的な性能向上を実現！**  
シンプルなログファイルから本格的な高性能データベースまで、レガシー実装との比較で**最大71,429倍の性能向上**を達成。学習の旅はここから本格的なプロダクションレベルのシステムへと進化しました！