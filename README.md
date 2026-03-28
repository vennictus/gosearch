<div align="center">

<div align="center">
<pre>
  ____  ___  ____  _____    _    ____   ____ _   _ 
 / ___|/ _ \/ ___|| ____|  / \  |  _ \ / ___| | | |
| |  _| | | \___ \|  _|   / _ \ | |_) | |   | |_| |
| |_| | |_| |___) | |___ / ___ \|  _ <| |___|  _  |
 \____|\___/|____/|_____/_/   \_\_| \_\\____|_| |_|
</pre>
</div>

**A blazing-fast, concurrent code search tool built in Go**

[![CI](https://github.com/vennictus/gosearch/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/vennictus/gosearch/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/vennictus/gosearch)](https://goreportcard.com/report/github.com/vennictus/gosearch)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go&logoColor=white)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

<p align="center">
  <strong>Search codebases in milliseconds | Respects .gitignore | Zero dependencies</strong>
</p>

---

[Installation](#installation) |
[Quick Start](#quick-start) |
[Features](#features) |
[Documentation](#documentation)

</div>

## Features

<table>
<tr>
<td width="50%">

### Performance
- **Concurrent pipeline** — 4-stage parallel processing
- **Dynamic worker scaling** — adapts to workload
- **Memory efficient** — streams results, doesn't buffer

</td>
<td width="50%">

### Search Capabilities  
- **Substring & regex** — flexible pattern matching
- **Case-insensitive** — optional `-i` flag
- **Whole-word** — boundary-aware with `-w`

</td>
</tr>
<tr>
<td width="50%">

### Smart Filtering
- **Gitignore support** — respects `.gitignore` rules
- **Custom ignores** — `.gosearchignore` files
- **Extension filter** — search only `.go`, `.ts`, etc.

</td>
<td width="50%">

### Developer Experience
- **JSON output** — pipe to `jq`, integrate with tools
- **Shell completions** — bash, zsh, fish
- **Exit codes** — scriptable `0/1/2` semantics

</td>
</tr>
</table>

---

## Installation

```bash
# Install directly with Go (recommended)
go install github.com/vennictus/gosearch@latest

# Or clone and build
git clone https://github.com/vennictus/gosearch.git
cd gosearch && go build -o gosearch .
```

**Requirements:** Go 1.21 or higher

---

## Quick Start

```bash
# Search for "TODO" in current directory
gosearch "TODO" .

# Case-insensitive search
gosearch -i "error" ./src

# Find function definitions with regex
gosearch -regex "func\s+\w+\(" .

# Search only Go files
gosearch -extensions .go "panic" .

# Get JSON output for tooling
gosearch -format json "config" . | jq '.matches'

# Just count matches
gosearch -count "FIXME" .
```

---

## Command Reference

<table>
<thead>
<tr>
<th align="center">Category</th>
<th align="center">Flag</th>
<th align="center">Description</th>
<th align="center">Example</th>
</tr>
</thead>
<tbody>
<tr>
<td align="center" rowspan="3"><strong>Search Mode</strong></td>
<td align="center"><code>-i</code></td>
<td align="center">Case-insensitive matching</td>
<td align="center"><code>gosearch -i "Error" .</code></td>
</tr>
<tr>
<td align="center"><code>-w</code></td>
<td align="center">Match whole words only</td>
<td align="center"><code>gosearch -w "log" .</code></td>
</tr>
<tr>
<td align="center"><code>-regex</code></td>
<td align="center">Treat pattern as regex</td>
<td align="center"><code>gosearch -regex "v[0-9]+" .</code></td>
</tr>
<tr>
<td align="center" rowspan="4"><strong>Filtering</strong></td>
<td align="center"><code>-extensions</code></td>
<td align="center">Only search these extensions</td>
<td align="center"><code>-extensions .go,.ts</code></td>
</tr>
<tr>
<td align="center"><code>-exclude-dir</code></td>
<td align="center">Skip directories by name</td>
<td align="center"><code>-exclude-dir vendor,node_modules</code></td>
</tr>
<tr>
<td align="center"><code>-max-size</code></td>
<td align="center">Skip files larger than size</td>
<td align="center"><code>-max-size 1MB</code></td>
</tr>
<tr>
<td align="center"><code>-max-depth</code></td>
<td align="center">Limit directory depth</td>
<td align="center"><code>-max-depth 3</code></td>
</tr>
<tr>
<td align="center" rowspan="4"><strong>Output</strong></td>
<td align="center"><code>-format</code></td>
<td align="center">Output format (plain/json)</td>
<td align="center"><code>-format json</code></td>
</tr>
<tr>
<td align="center"><code>-color</code></td>
<td align="center">Highlight matches</td>
<td align="center"><code>-color</code></td>
</tr>
<tr>
<td align="center"><code>-count</code></td>
<td align="center">Only print match count</td>
<td align="center"><code>-count</code></td>
</tr>
<tr>
<td align="center"><code>-quiet</code></td>
<td align="center">No output, exit code only</td>
<td align="center"><code>-quiet</code></td>
</tr>
<tr>
<td align="center" rowspan="3"><strong>Performance</strong></td>
<td align="center"><code>-workers</code></td>
<td align="center">Number of search workers</td>
<td align="center"><code>-workers 8</code></td>
</tr>
<tr>
<td align="center"><code>-dynamic-workers</code></td>
<td align="center">Auto-scale worker count</td>
<td align="center"><code>-dynamic-workers</code></td>
</tr>
<tr>
<td align="center"><code>-metrics</code></td>
<td align="center">Show performance stats</td>
<td align="center"><code>-metrics</code></td>
</tr>
</tbody>
</table>

---

## Exit Codes

| Code | Meaning | Use Case |
|:----:|:--------|:---------|
| **0** | Matches found | Success — pattern was found |
| **1** | No matches | Pattern not found (not an error) |
| **2** | Error | Invalid args, bad regex, etc. |

Perfect for scripting:
```bash
if gosearch -quiet "TODO" .; then
    echo "Found TODOs!"
fi
```

---

## Performance

gosearch uses a **4-stage concurrent pipeline**:

```
┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  Traversal  │───>│  IO Workers │───>│ CPU Workers │───>│   Printer   │
│  (walker)   │    │ (file read) │    │  (search)   │    │  (output)   │
└─────────────┘    └─────────────┘    └─────────────┘    └─────────────┘
```

**Benchmarks** (searching 10,000 files):
```
BenchmarkSmallFiles-8       1000    1.2ms/op    0 allocs/op
BenchmarkLargeFiles-8        100   12.4ms/op    0 allocs/op
```

---

## Configuration

### Config File

Create `.gosearchrc` in your project root:

```json
{
  "ignore_case": true,
  "format": "json",
  "extensions": [".go", ".ts", ".js"],
  "exclude_dirs": ["vendor", "node_modules", ".git"]
}
```

### Ignore Files

Create `.gosearchignore` (same syntax as `.gitignore`):

```gitignore
# Skip generated files
*.generated.go
*_gen.go

# Skip test fixtures
testdata/
fixtures/
```

---

## Shell Completions

```bash
# Bash
gosearch -completion bash > ~/.local/share/bash-completion/completions/gosearch

# Zsh  
gosearch -completion zsh > ~/.zfunc/_gosearch

# Fish
gosearch -completion fish > ~/.config/fish/completions/gosearch.fish

# PowerShell
gosearch -completion powershell >> $PROFILE
```

---

## Project Structure

```
gosearch/
├── main.go                 # Entry point & pipeline orchestration
├── internal/
│   ├── config/             # CLI parsing, validation, config loading
│   ├── search/             # Matcher, workers, file traversal
│   ├── ignore/             # .gitignore & .gosearchignore parsing
│   └── output/             # Result formatting (plain, JSON, color)
├── completions/            # Shell completion scripts
├── testdata/               # Test fixtures (unicode, edge cases, code samples)
└── scripts/                # Build & release automation
```

---

## Documentation

| Document | Description |
|:---------|:------------|
| **[GUIDE.md](GUIDE.md)** | Deep-dive tutorial for beginners — explains every concept from scratch |
| **[PRODUCT_SPEC.md](PRODUCT_SPEC.md)** | Full technical specification and design documentation |

---

## Development

```bash
# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Build for current platform
go build -o gosearch .
```

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

<div align="center">

**Built with Go**

[Back to top](#gosearch)

</div>
