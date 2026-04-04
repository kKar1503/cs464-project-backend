# Migrating from GVM to asdf

Guide to migrate from GVM to asdf for Go version management.

## Current Setup Summary

### Custom Go Settings (PRESERVE THESE)
```bash
export GOPRIVATE="github.com/Limbo-Labs-Org/*"
export GONOPROXY="github.com/Limbo-Labs-Org/*"
export GONOSUMDB="github.com/Limbo-Labs-Org/*"
export CGO_ENABLED="0"
```

### Current GVM Setup
- Active: Go 1.23.1
- Installed: Go 1.25.3
- Location: `~/.gvm/`
- Issue: Auto-switching `cd` overrides

## Step 1: Install asdf

```bash
# Install asdf via Homebrew
brew install asdf

# Add to ~/.zshrc (DO THIS MANUALLY AFTER BACKUP)
# Add this line BEFORE the gvm line:
echo -e '\n. $(brew --prefix asdf)/libexec/asdf.sh' >> ~/.zshrc
```

## Step 2: Install Go Plugin

```bash
# Install Go plugin
asdf plugin add golang https://github.com/asdf-community/asdf-golang.git

# Install your Go versions
asdf install golang 1.23.1
asdf install golang 1.25.3

# Set global default
asdf global golang 1.23.1
```

## Step 3: Preserve Custom Go Settings

Create `~/.config/go/env` or add to `~/.zshrc`:

```bash
# Add to ~/.zshrc (after asdf sourcing)
export GOPRIVATE="github.com/Limbo-Labs-Org/*"
export GONOPROXY="github.com/Limbo-Labs-Org/*"
export GONOSUMDB="github.com/Limbo-Labs-Org/*"
export CGO_ENABLED="0"
```

Or set permanently:

```bash
go env -w GOPRIVATE="github.com/Limbo-Labs-Org/*"
go env -w GONOPROXY="github.com/Limbo-Labs-Org/*"
go env -w GONOSUMDB="github.com/Limbo-Labs-Org/*"
```

## Step 4: Configure Project

Create `.tool-versions` in your project:

```bash
# In /Users/kar/school/smu/cs464-project/backend/
echo "golang 1.23.1" > .tool-versions
```

This pins the Go version for this project only.

## Step 5: Update ~/.zshrc

### Before (Current):
```bash
[[ -s "/Users/kar/.gvm/scripts/gvm" ]] && source "/Users/kar/.gvm/scripts/gvm"
```

### After (New):
```bash
# asdf version manager
. $(brew --prefix asdf)/libexec/asdf.sh

# Go custom settings
export GOPRIVATE="github.com/Limbo-Labs-Org/*"
export GONOPROXY="github.com/Limbo-Labs-Org/*"
export GONOSUMDB="github.com/Limbo-Labs-Org/*"
export CGO_ENABLED="0"

# OLD GVM (comment out or remove)
# [[ -s "/Users/kar/.gvm/scripts/gvm" ]] && source "/Users/kar/.gvm/scripts/gvm"
```

## Step 6: Verify Installation

```bash
# Reload shell
source ~/.zshrc

# Check Go version
go version
# Should show: go version go1.23.1 darwin/arm64

# Check which Go
which go
# Should show: /Users/kar/.asdf/shims/go

# Check Go env
go env GOPRIVATE
# Should show: github.com/Limbo-Labs-Org/*

# Test project switching
cd /Users/kar/school/smu/cs464-project/backend/
asdf current
# Should show: golang 1.23.1 (set by /Users/kar/school/smu/cs464-project/backend/.tool-versions)
```

## Step 7: Cleanup GVM (AFTER VERIFICATION)

```bash
# ONLY do this after confirming asdf works!

# Remove GVM from ~/.zshrc (already done in Step 5)

# Remove GVM directory (CAREFUL!)
rm -rf ~/.gvm

# Remove GVM-related env vars from shell configs
# (Already handled by removing the source line)
```

## Step 8: Update Go Module Cache

```bash
# Clear old module cache if needed
go clean -modcache

# Download modules again
cd /Users/kar/school/smu/cs464-project/backend/
go mod download
```

## asdf Usage Guide

### Basic Commands

```bash
# List installed versions
asdf list golang

# List all available versions
asdf list all golang

# Install specific version
asdf install golang 1.23.2

# Set global default
asdf global golang 1.23.1

# Set project-specific version
asdf local golang 1.23.1

# See current version
asdf current golang

# Uninstall version
asdf uninstall golang 1.25.3
```

### Project Version Management

Create `.tool-versions` in any project:

```
# .tool-versions
golang 1.23.1
nodejs 20.10.0
python 3.11.0
```

asdf automatically switches when you `cd` into the directory, but it doesn't override `cd` - it uses shell hooks properly.

### Global vs Local Versions

```bash
# Global (applies everywhere)
~/.tool-versions
golang 1.23.1

# Project-specific (overrides global)
~/projects/my-app/.tool-versions
golang 1.22.0

# When you cd into ~/projects/my-app, it uses 1.22.0
# When you cd elsewhere, it uses 1.23.1
```

## Advantages Over GVM

| Feature | GVM | asdf |
|---------|-----|------|
| Multi-language | ❌ Go only | ✅ Go, Node, Python, Rust, etc. |
| `cd` override | ❌ Yes (breaks things) | ✅ Clean shell hooks |
| Per-project versions | ✅ Yes | ✅ Yes (.tool-versions) |
| Community | ⚠️ Less active | ✅ Very active |
| Integration | ⚠️ Go-specific | ✅ Works with many tools |
| Maintenance | ⚠️ Occasional issues | ✅ Well-maintained |

## Consolidate Version Managers (Optional)

You currently have:
- **gvm** (Go) → migrate to asdf
- **nvm** (Node) → can migrate to asdf
- **pyenv** (Python) → can migrate to asdf
- **sdkman** (Java, Maven, Gradle) → can migrate to asdf

### Migrate All to asdf:

```bash
# Install plugins
asdf plugin add nodejs
asdf plugin add python
asdf plugin add java
asdf plugin add maven
asdf plugin add gradle

# Install your current versions
asdf install nodejs 20.10.0
asdf install python 3.11.0
# ... etc

# Set globals
asdf global nodejs 20.10.0
asdf global python 3.11.0
```

Then remove:
- `~/.nvm/`
- `~/.pyenv/`
- `~/.sdkman/`

And clean up `~/.zshrc` to only have:
```bash
. $(brew --prefix asdf)/libexec/asdf.sh
```

## Troubleshooting

### "go: command not found"

```bash
# Reload shell
source ~/.zshrc

# Check asdf is loaded
asdf --version

# Reinstall shims
asdf reshim golang
```

### Wrong Go version

```bash
# Check current
asdf current golang

# Check .tool-versions
cat .tool-versions

# Force install
asdf install golang 1.23.1
asdf global golang 1.23.1
```

### GOPRIVATE not working

```bash
# Set permanently
go env -w GOPRIVATE="github.com/Limbo-Labs-Org/*"

# Verify
go env GOPRIVATE
```

### asdf not activating

```bash
# Check .zshrc has asdf sourced
grep asdf ~/.zshrc

# Should see:
# . $(brew --prefix asdf)/libexec/asdf.sh

# Reload
source ~/.zshrc
```

## Quick Migration Script

```bash
#!/bin/bash
# migrate-to-asdf.sh

echo "Installing asdf..."
brew install asdf

echo "Adding asdf to shell..."
echo -e '\n. $(brew --prefix asdf)/libexec/asdf.sh' >> ~/.zshrc

echo "Installing Go plugin..."
asdf plugin add golang

echo "Installing Go 1.23.1..."
asdf install golang 1.23.1

echo "Setting global version..."
asdf global golang 1.23.1

echo "Setting up project..."
cd /Users/kar/school/smu/cs464-project/backend/
echo "golang 1.23.1" > .tool-versions

echo "Preserving custom Go settings..."
go env -w GOPRIVATE="github.com/Limbo-Labs-Org/*"
go env -w GONOPROXY="github.com/Limbo-Labs-Org/*"
go env -w GONOSUMDB="github.com/Limbo-Labs-Org/*"

echo ""
echo "✅ Migration complete!"
echo ""
echo "Next steps:"
echo "1. Open a new terminal or run: source ~/.zshrc"
echo "2. Verify: go version"
echo "3. Comment out GVM line in ~/.zshrc"
echo "4. After testing, remove: rm -rf ~/.gvm"
```

## References

- [asdf Documentation](https://asdf-vm.com/)
- [asdf-golang Plugin](https://github.com/asdf-community/asdf-golang)
- [Go Environment Variables](https://pkg.go.dev/cmd/go#hdr-Environment_variables)
