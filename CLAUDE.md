# pull-request-review-context

## アーキテクチャ

- **エントリーポイント**: `cmd/pull-request-review-context/main.go` → `run.go` の `Run()` を呼び出す
- **GitHub API**: `internal/github` で GitHub CLI の認証済みAPI呼び出しとレビューコメント取得を行う
- **行抽出**: `internal/diff` で `diff_hunk` と左右の行番号から対象コード行を再構成する
- **出力**: `internal/output` でLLM向けの簡潔な形式とJSON形式を生成する
- **skill配布**: `skillsmith`で埋め込んだCodex skillを`skills`サブコマンドから配置する

```
cmd/pull-request-review-context/ ─ CLIエントリーポイント
run.go                           ─ CLIパースと処理の組み立て
internal/github/                 ─ GitHub CLI経由のデータ取得
internal/diff/                   ─ 対象コード行の抽出
internal/output/                 ─ LLM/JSON出力
skills/                          ─ 埋め込みskill本体
skills.go                        ─ skill配布サブコマンドの接続
```

## テスト

- 通常のテストは外部サービスを使わず、テーブル駆動で実行する
- 実在するOSSのレビューコメントを使うテストは `go test -tags integration ./...` で実行する
