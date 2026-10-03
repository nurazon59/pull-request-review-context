# pull-request-review-context

Pull request review comment と、そのコメントが指しているコード行だけを抽出するCLIです。
LLMへ渡すことを想定し、差分全体やpull request本文などの重複情報は出力しません。

## 前提

- Go 1.26.1以上
- GitHub CLI (`gh`)
- `gh auth login` 済みであること

## 使い方

カレントブランチに紐づく open PR の未解決レビューコメントを取得します。

```bash
go run ./cmd/pull-request-review-context
```

PR 番号または PR URL を指定できます。リポジトリを明示する場合は `--repository` を指定します。

```bash
go run ./cmd/pull-request-review-context 123
go run ./cmd/pull-request-review-context https://github.com/OWNER/REPOSITORY/pull/123
go run ./cmd/pull-request-review-context --repository OWNER/REPOSITORY 123
```

review comment URL を指定すると、resolved / outdated 状態に関係なく、そのコメントだけを取得します。thread へのリンクも、アンカーが指すコメントだけを取得します。`--all` は PR 全体から resolved / outdated の thread も含めます。

```bash
go run ./cmd/pull-request-review-context https://github.com/OWNER/REPOSITORY/pull/123#discussion_r456
go run ./cmd/pull-request-review-context --all
```

標準出力は、ファイル名・行番号・対象コード行・レビュー本文です。PR 全体の取得では、既定で resolved / outdated thread を除外します。行番号とコードはコメント作成時の diff hunk に基づきます。

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
go run ./cmd/pull-request-review-context --format json
```

JSON の各項目には `comments` として comment ID、thread ID、URL、パス、行、本文、resolved / outdated 状態も含まれます。
対象コード行を特定できないコメントは出力しません。

未解決コメントがない場合は LLM 形式では空出力、JSON 形式では `[]` を出力して終了コード `0` です。current branch に open PR がない、指定した comment が見つからない、または GitHub CLI/API に失敗した場合は標準エラーにエラーを出し、終了コード `1` になります。

`--repository` 付きの自動解決では、現在のブランチ名で open PR を検索します。複数の fork が同じブランチ名を使っていて候補が複数ある場合や detached HEAD の場合は、PR 番号または URL を指定してください。URL のホストは `github.com` / `api.github.com` に対応しています。

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
