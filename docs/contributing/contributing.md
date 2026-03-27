# How to Contribute

Thank you for your interest in contributing to Prism. Contributions of all kinds are welcome -- bug reports, feature requests, documentation improvements, and code.

## Development Setup

### 1. Clone the Repository

```bash
git clone https://github.com/himattm/prism.git
cd prism
```

### 2. Install Go

Prism requires Go 1.21 or later. Download it from [go.dev/dl](https://go.dev/dl/).

### 3. Build

```bash
go build -o prism ./cmd/prism/
```

### 4. Run Tests

```bash
go test ./... -v
```

## Code Style

- All code must be formatted with `gofmt`. CI checks formatting and will fail if any files are unformatted.
- Run `gofmt -w .` before committing.
- Run `go vet ./...` to catch common issues before opening a pull request.

## Testing

- Write tests for new plugins and features.
- Test files go next to the code they test, following Go conventions (`*_test.go`).
- Run the full test suite before submitting a pull request.

## Adding a New Plugin

1. Create `internal/plugins/yourplugin.go`
2. Implement the `NativePlugin` interface
3. Optionally implement `Hookable` for event handling
4. Register the plugin in `internal/plugins/interface.go`
5. Add tests in `internal/plugins/yourplugin_test.go`
6. Document the plugin in `docs/plugins/built-in-plugins.md`
7. Update the default sections in `internal/config/config.go` if appropriate

See the existing plugins in `internal/plugins/` for reference implementations.

## Pull Request Process

1. Fork the repository and create a feature branch from `main`.
2. Make your changes with accompanying tests.
3. Ensure CI passes: `gofmt`, `go vet`, `go build`, and `go test` must all succeed.
4. Open a pull request against `main`.
5. Describe what you changed and why in the PR description.

## Project Structure

```
cmd/prism/           CLI entry point
internal/plugins/    Built-in plugin implementations
internal/plugin/     Plugin manager and types for community plugins
internal/config/     Configuration loading and merging
internal/statusline/ Status line rendering
internal/hooks/      Hook dispatch system
internal/cache/      Shared caching layer
internal/colors/     ANSI color codes
internal/version/    Version info
examples/            Example configs and script plugins
docs/                This documentation site
```

## Release Process

- Tags matching `v*` trigger automated builds via GitHub Actions.
- Binaries are built for macOS (arm64/amd64) and Linux (amd64/arm64).
- Release assets are uploaded to GitHub automatically.

## Reporting Issues

Please use [GitHub Issues](https://github.com/himattm/prism/issues) to report bugs or request features. Include as much detail as possible: your OS, Go version, Prism version, and the steps to reproduce.
