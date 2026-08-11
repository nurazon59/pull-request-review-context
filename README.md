# go-template

`go-template` is a small Go CLI template with a thin `kong`-based entrypoint, XDG-backed config loading, and release automation for GitHub Releases.

## Usage

```bash
go run ./main.go --help
go run ./main.go --version
```

### Config

The CLI loads config from one of these locations:

1. `--config /path/to/config.yaml`
2. `GO_TEMPLATE_CONFIG=/path/to/config.yaml`
3. `~/.config/go-template/config.yaml`

Config format:

```yaml
version: 1
```

If the file is missing, the CLI starts with defaults.

## Development

```bash
task fmt
task test
task lint
task vet
task ci
task snapshot
```
