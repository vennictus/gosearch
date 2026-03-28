# gosearch

[![CI](https://github.com/vennictus/gosearch/actions/workflows/ci.yml/badge.svg)](https://github.com/vennictus/gosearch/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/vennictus/gosearch)](https://goreportcard.com/report/github.com/vennictus/gosearch)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

A fast, concurrent code search tool written in Go.

## What It Does

Search through files recursively with support for:
- **Regex and substring matching**
- **Ignore rules** (`.gitignore` + `.gosearchignore`)
- **Concurrent processing** with configurable worker pools
- **JSON output** for tooling integration
- **Symlink handling** with loop detection

## Install

```bash
go install github.com/vennictus/gosearch@latest
```

Or build from source:

```bash
git clone https://github.com/vennictus/gosearch
cd gosearch
go build -o gosearch .
```

## Quick Start

```bash
# Basic search
gosearch "TODO" ./src

# Case-insensitive
gosearch -i "error" .

# Regex mode
gosearch -regex "func\s+\w+" .

# JSON output
gosearch -format json "config" .

# Filter by extension
gosearch -extensions .go,.md "test" .

# Count matches only
gosearch -count "needle" ./project
```

## Common Flags

| Flag | Description |
|------|-------------|
| `-i` | Case-insensitive search |
| `-w` | Whole-word matching |
| `-regex` | Treat pattern as regex |
| `-n` | Show line numbers (default: true) |
| `-extensions .go,.ts` | Filter by file extension |
| `-exclude-dir vendor` | Skip directories |
| `-max-size 10MB` | Skip large files |
| `-format json` | JSON output |
| `-count` | Print match count only |
| `-quiet` | No output, exit code only |
| `-color` | Syntax highlighting |

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Matches found |
| `1` | No matches |
| `2` | Error (bad args, etc.) |

## Config File

Create `.gosearchrc` in your project:

```json
{
  "ignore_case": true,
  "format": "json",
  "extensions": [".go", ".ts", ".js"]
}
```

CLI flags override config values.

## Performance Tuning

```bash
# Custom worker counts
gosearch -io-workers 8 -cpu-workers 16 "pattern" .

# Dynamic scaling
gosearch -dynamic-workers -max-workers 32 "pattern" .

# View metrics
gosearch -metrics "pattern" .
```

## Shell Completions

```bash
# Bash
gosearch -completion bash > ~/.local/share/bash-completion/completions/gosearch

# Zsh
gosearch -completion zsh > ~/.zfunc/_gosearch

# Fish
gosearch -completion fish > ~/.config/fish/completions/gosearch.fish
```

## Documentation

- **[GUIDE.md](GUIDE.md)** — Deep dive for beginners (how everything works)
- **[PRODUCT_SPEC.md](PRODUCT_SPEC.md)** — Full specification and design docs

## Development

```bash
# Run tests
go test ./...

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Build release binaries
make release VERSION=v1.0.0
```

## Project Structure

```
gosearch/
├── main.go                 # Entry point
├── internal/
│   ├── config/             # CLI parsing, config loading
│   ├── search/             # Matching, workers, traversal
│   ├── ignore/             # .gitignore handling
│   └── output/             # Result printing
├── completions/            # Shell completions
├── man/                    # Man page
├── scripts/                # Release scripts
└── testdata/               # Test fixtures
```

## License

MIT
