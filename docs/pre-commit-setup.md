# Pre-Commit Hooks Setup Guide

This guide explains how to set up and use the pre-commit hooks for the acp-relay project.

## Overview

Our pre-commit hooks ensure code quality by running automated checks before each commit:

### Go Checks
- **go fmt** - Format Go code
- **go vet** - Static analysis
- **golangci-lint** - Comprehensive linting (20+ linters enabled)
- **go mod tidy** - Clean up dependencies
- **go test** - Run all tests with race detection
- **go build** - Verify both binaries compile

### Python Checks
- **ruff** - Fast Python linting and formatting
- **pytest** - Run Python tests

### Security & Quality
- **detect-secrets** - Prevent committing secrets
- **hadolint** - Dockerfile linting
- **YAML validation**
- **Trailing whitespace, EOF fixes**
- **Large file detection**
- **Merge conflict detection**

## Installation

### Prerequisites

1. **Install pre-commit**
   ```bash
   # macOS
   brew install pre-commit

   # Or using pip
   pip install pre-commit

   # Or using uv
   uv tool install pre-commit
   ```

2. **Install golangci-lint**
   ```bash
   # macOS
   brew install golangci-lint

   # Or using go
   go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
   ```

3. **Install hadolint** (optional, for Dockerfile linting)
   ```bash
   # macOS
   brew install hadolint
   ```

4. **Install Python tools** (if working with Python files)
   ```bash
   uv tool install ruff
   # pytest will be installed via uv when running tests
   ```

### Enable Pre-Commit Hooks

1. **Install the git hooks**
   ```bash
   pre-commit install
   ```

2. **Verify installation**
   ```bash
   pre-commit --version
   ```

## Usage

### Automatic Execution

Once installed, the hooks run automatically on `git commit`:

```bash
git add .
git commit -m "your message"
# Hooks run automatically before commit completes
```

### Manual Execution

Run hooks on all files:
```bash
pre-commit run --all-files
```

Run hooks on staged files only:
```bash
pre-commit run
```

Run a specific hook:
```bash
pre-commit run golangci-lint --all-files
pre-commit run go-test --all-files
pre-commit run ruff --all-files
```

### Updating Hooks

Update to the latest hook versions:
```bash
pre-commit autoupdate
```

## Hook Failure Protocol

**IMPORTANT**: Never use `--no-verify` to bypass hooks!

When a hook fails, follow this process:

1. **Read the error output** - Understand what failed and why
2. **Identify the tool** - Which linter/test failed?
3. **Fix the root cause** - Don't work around the issue
4. **Re-run hooks** - Verify the fix works
5. **Only then commit** - After all hooks pass

Example workflow:
```bash
$ git commit -m "Add feature"
[INFO] Running pre-commit hooks...
golangci-lint............Failed
- hook id: golangci-lint
- exit code: 1

internal/websocket/server.go:45:2: Error return value is not checked (errcheck)

# Fix the error in the code
$ git add internal/websocket/server.go
$ git commit -m "Add feature"
[INFO] Running pre-commit hooks...
golangci-lint............Passed
go-test..................Passed
✓ All hooks passed!
```

## Configuration Files

- **`.pre-commit-config.yaml`** - Main pre-commit configuration
- **`.golangci.yml`** - golangci-lint rules and settings
- **`pyproject.toml`** - ruff and pytest configuration
- **`.secrets.baseline`** - detect-secrets baseline

## Common Issues

### Hook installation fails
```bash
# Clear cache and reinstall
pre-commit clean
pre-commit install --install-hooks
```

### golangci-lint timeout
If linting takes too long, increase timeout in `.golangci.yml`:
```yaml
run:
  timeout: 10m  # Increase from 5m
```

### Python tests fail
Ensure dependencies are installed:
```bash
uv sync
```

### Secrets detected
If secrets are detected but are false positives:
```bash
# Update baseline
detect-secrets scan > .secrets.baseline
```

## Skipping Hooks (Emergency Only)

In rare cases where you absolutely must commit without hooks:
```bash
# ONLY use this in emergencies with explicit approval
git commit --no-verify -m "message"
```

**Never do this without understanding why hooks are failing!**

## Performance Tips

### Speed up go test hook
To run only affected package tests:
```bash
# Modify the go-test hook to test only changed packages
# Edit .pre-commit-config.yaml
```

### Parallel execution
Pre-commit runs hooks in parallel by default for faster execution.

### Cache
Pre-commit caches hook environments. To clear:
```bash
pre-commit clean
```

## Integration with CI/CD

The same hooks should run in CI. Add to your GitHub Actions:

```yaml
- name: Run pre-commit
  run: |
    pip install pre-commit
    pre-commit run --all-files
```

## Troubleshooting

### Hook not running
```bash
# Reinstall hooks
pre-commit uninstall
pre-commit install
```

### Wrong version of tool
```bash
# Update hooks to latest versions
pre-commit autoupdate
```

### Need help?
Run with verbose output:
```bash
pre-commit run --all-files --verbose
```

## Quality Standards

Our pre-commit hooks enforce:

- ✅ All Go code is formatted with `go fmt`
- ✅ No unchecked errors
- ✅ No security vulnerabilities (gosec)
- ✅ All tests pass with race detection
- ✅ Code builds successfully
- ✅ Python code follows PEP 8
- ✅ No secrets in code
- ✅ Dockerfiles are well-formed

Remember: **Quality checks are guardrails that help us, not barriers that block us!**
