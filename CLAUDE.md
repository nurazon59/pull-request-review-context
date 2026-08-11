# Go Template

## アーキテクチャ

- **エントリーポイント**: `cmd/gotemplate/main.go` → `run.go` の `Run()` を呼び出す
- **CLI**: `kong` でパース、`--config` と `--version` フラグ
- **設定**: `internal/config` でYAML設定のロード（デフォルト値あり、ファイル不存在は許容）
- **XDG**: 設定ファイルのデフォルトパスは `xdg.ConfigFile("go-template/config.yaml")`

```
cmd/gotemplate/  ─ CLIエントリーポイント
run.go           ─ アプリケーションロジック（CLIパース→設定ロード）
internal/config/ ─ 設定ファイルの読み込み
```

## テストフィクスチャパターン

### 設計方針
- `test/fixture/` パッケージにテスト用fixtureを配置
- ボイラープレート(TempDir, error handling, file I/O)をfixtureに封じ込める
- テスト本体は「データを組み立ててアサーションするだけ」に集中
- フラット構造を採用し、メソッド名でドメインを明示する（例: `ConfigYAML`, `SaveReport`）

### Fixture struct
```
Fixture struct
├ t *testing.T      → TempDir管理を隠蔽
└ (lazy fields)     → 必要に応じてドメイン固有リソースをlazy生成
Methods
├ New(t)            → エントリポイント
├ ConfigYAML(content string) string → TempDirにconfig.yaml生成、パスを返す
└ (今後拡張)        → ドメイン固有メソッドを追加
```

### 使用例
```go
func TestConfig(t *testing.T) {
	f := fixture.New(t)

	path := f.ConfigYAML("version: 2\n")
	cfg, err := config.Load(path)

	assert.NoError(t, err)
	assert.Equal(t, 2, cfg.Version)
}
```

## テスト

- **Table driven test** を採用する
- `*_test.go` では `map[string]struct{}` パターンを使用
- **AAAパターン**（Arrange-Act-Assert）に従う
- `testify` の `assert` を使用する（fatalなエラーには `require`）
