# pull-request-review-context

Pull request review comment と、そのコメントが指しているコード行だけを抽出するCLIです。
LLMへ渡すことを想定し、差分全体やpull request本文などの重複情報は出力しません。

## 前提

- Go 1.26.1以上
- GitHub CLI (`gh`)
- `gh auth login` 済みであること

## 使い方

カレントディレクトリのリポジトリを対象にする場合:

```bash
go run ./cmd/pull-request-review-context 123
```

リポジトリを明示する場合:

```bash
go run ./cmd/pull-request-review-context --repository OWNER/REPOSITORY 123
```

標準出力は、ファイル名・行番号・対象コード行・レビュー本文だけです。

```text
file: internal/example.go
line: 42

code:
return value, nil

review:
エラー処理を追加してください
```

機械処理用にはJSONを指定できます。

```bash
go run ./cmd/pull-request-review-context --format json 123
```

ファイル単位のコメントや対象行を持たないコメントは、コード行を抽出できないため出力しません。

## Codex skill

このCLIには、レビューコンテキストを取得するskillが埋め込まれています。
配置先を確認するだけなら、次を実行します。

```bash
go run ./cmd/pull-request-review-context skills install
```

実際にCodexのskillとして配置する場合は、`--apply`を明示します。

```bash
go run ./cmd/pull-request-review-context skills install --apply
```

既定の配置先は、`CODEX_HOME`が設定されていれば`$CODEX_HOME/skills`、未設定なら`~/.codex/skills`です。
配置先を変える場合は`--prefix`を指定してください。

```bash
go run ./cmd/pull-request-review-context skills install --prefix /path/to/skills --apply
```

更新状況の確認や更新もできます。

```bash
go run ./cmd/pull-request-review-context skills status
go run ./cmd/pull-request-review-context skills update --apply
```

実在するOSSのレビューコメントを使う統合テストも用意しています。`gh auth login` 済みの環境で実行してください。

```bash
go test -tags integration ./...
```
