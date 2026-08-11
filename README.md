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

実在するOSSのレビューコメントを使う統合テストも用意しています。`gh auth login` 済みの環境で実行してください。

```bash
go test -tags integration ./...
```
