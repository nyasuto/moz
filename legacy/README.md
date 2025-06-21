# Legacy Phase 1: シェルベースKVストア

このディレクトリには、mozプロジェクトのフェーズ1で実装されたシェルベースのKVストアが含まれています。

## 📁 ファイル構成

### 基本操作スクリプト
- `put.sh` - キーと値の追加・更新
- `get.sh` - キーの値取得（最新値）
- `del.sh` - キーの削除（__DELETED__マーカー）
- `list.sh` - 全件一覧表示
- `filter.sh` - 条件付き一覧表示
- `compact.sh` - ログ整理・最新状態まとめ

### 性能測定
- `test_performance.sh` - 包括的な性能測定
- `analyze_performance.sh` - 性能分析レポート

## 🚀 使用方法

### 基本操作
```bash
# キー値ペアの追加
./legacy/put.sh name Alice

# 値の取得
./legacy/get.sh name

# 全件表示
./legacy/list.sh

# 条件検索
./legacy/filter.sh "na"

# 削除
./legacy/del.sh name

# ログの整理
./legacy/compact.sh
```

### 性能測定
```bash
# 1000件でテスト実行
./legacy/test_performance.sh 1000

# 結果分析
./legacy/analyze_performance.sh
```

## ⚙️ データフォーマット

- フォーマット: `key<TAB>value`
- ファイル: `moz.log`（追記専用）
- 削除: `__DELETED__`マーカー

## 📊 実装特徴

- 純粋なシェルスクリプト実装
- macOS bash 3.2対応
- awk使用による堅牢な処理
- 追記型ログ設計
- 性能測定・分析機能

## 🔄 次のフェーズ

このレガシー実装は、フェーズ2のGo実装の基盤となります。性能比較とアーキテクチャ学習のため保存されています。