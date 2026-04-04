# Code Quality & Linting

This project uses industry-standard linting and formatting tools for both Go and Rust.

## Quick Commands

```bash
make fmt     # Format all code (gofumpt + rustfmt)
make lint    # Lint all code (golangci-lint + clippy)
make tidy    # Tidy Go module dependencies
```

## Go Tooling

### gofumpt (Formatting)
A stricter formatter than `go fmt` with better consistency.

**Install:**
```bash
go install mvdan.cc/gofumpt@latest
```

**Features:**
- Stricter whitespace rules
- Consistent import grouping
- Better alignment

**Fallback:** If gofumpt is not installed, `go fmt` is used automatically.

### golangci-lint (Linting)
A fast Go linters aggregator that runs multiple linters in parallel.

**Install:**
```bash
# macOS
brew install golangci-lint

# Linux/Windows
curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin
```

**Enabled Linters:**
- `errcheck` - Check for unchecked errors
- `gosimple` - Simplify code
- `govet` - Reports suspicious constructs
- `ineffassign` - Detect ineffectual assignments
- `staticcheck` - Static analysis
- `unused` - Detect unused code
- `gofumpt` - Stricter gofmt
- `revive` - Drop-in replacement for golint
- `gosec` - Security issues
- `misspell` - Spelling mistakes
- `unconvert` - Unnecessary type conversions
- `unparam` - Unused function parameters
- `bodyclose` - Unclosed HTTP response bodies
- `gocritic` - Opinionated code checks

**Configuration:** `.golangci.yml`

## Rust Tooling

### rustfmt (Formatting)
Official Rust code formatter.

**Install:**
Comes with Rust installation via rustup.

**Features:**
- Consistent code style
- Configurable via `rustfmt.toml`
- Part of standard Rust toolchain

### Clippy (Linting)
Official Rust linter that catches common mistakes and improves code.

**Install:**
```bash
rustup component add clippy
```

**Features:**
- Over 600 lints
- Performance improvements
- Idiomatic code suggestions
- Security checks

**Configuration:** `services/cursor-udp/clippy.toml`

**Lint Level:** Set to deny warnings (`-D warnings`) to treat warnings as errors.

## CI/CD Integration

Add these commands to your CI pipeline:

```yaml
# Example GitHub Actions
- name: Format Check
  run: make fmt

- name: Lint
  run: make lint

- name: Build
  run: make build

- name: Test
  run: make test
```

## IDE Integration

### VSCode

**Go:**
```json
{
  "go.formatTool": "gofumpt",
  "go.lintTool": "golangci-lint",
  "go.lintOnSave": "workspace"
}
```

**Rust:**
```json
{
  "rust-analyzer.checkOnSave.command": "clippy",
  "editor.formatOnSave": true
}
```

### GoLand/IntelliJ

- Go to Preferences → Tools → File Watchers
- Add gofumpt and golangci-lint watchers

## Troubleshooting

### gofumpt not found
```bash
go install mvdan.cc/gofumpt@latest
```

### golangci-lint not found
```bash
brew install golangci-lint  # macOS
# or
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
```

### Clippy warnings
Fix automatically when possible:
```bash
cd services/cursor-udp
cargo clippy --fix
```

## Best Practices

1. **Run `make fmt` before committing**
2. **Run `make lint` regularly during development**
3. **Fix linting issues immediately** - Don't accumulate technical debt
4. **Configure your IDE** to format on save
5. **Use pre-commit hooks** for team consistency
