
# gosearch - Product Specification
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
Version: 1.0.0  
Status: Stable  
</div>
 
---
 
## Overview
 
gosearch is a concurrent, recursive file-search CLI built in Go. It is designed for developers who search real codebases daily and need something faster, more scriptable, and more composable than basic grep - without the complexity of a daemon or indexing system.
 
It is intentionally CLI-first: bounded execution, predictable exit semantics, structured output, and zero runtime dependencies.
 
---
 
## Goals
 
- Search recursively across large directory trees with bounded, tunable concurrency.
- Stream output as results arrive rather than buffering everything first.
- Respect `.gitignore` and project-local ignore rules with no extra configuration.
- Produce machine-readable output suitable for pipelines.
- Handle cancellation, symlinks, and binary files cleanly and predictably.
- Stay testable, benchmarkable, and release-ready without external tooling.
## Non-Goals
 
- No daemon or background indexing mode.
- No file-watching or incremental re-scan.
- No built-in paging or interactive TUI.
- No support for structured query languages beyond substring and regex.
---
 
## Command Interface
 
```
gosearch [flags] <pattern> <path>
```
 
`<pattern>` is a literal string by default. Use `-regex` to treat it as a Go `regexp` expression.  
`<path>` is the root directory to search. Use `.` for the current directory.
 
### Exit codes
 
| Code | Meaning | Notes |
|------|---------|-------|
| `0` | One or more matches found | Success |
| `1` | No matches found | Not an error - standard "not found" signal |
| `2` | Invalid usage, bad regex, or fatal runtime error | Check stderr for details |
 
Exit code `1` is not an error - it is the standard "not found" signal for scripting.
 
---
 
## Flags
 
### Search behavior
 
| Flag | Default | Description |
|------|---------|-------------|
| `-i` | false | Case-insensitive matching |
| `-w` | false | Whole-word matching (boundary-aware) |
| `-regex` | false | Treat pattern as a Go regexp |
| `-n` | true | Show line numbers; set `-n=false` to suppress |
 
### Scope and filtering
 
| Flag | Default | Description |
|------|---------|-------------|
| `-extensions` | (all) | Comma-separated list of extensions to include, e.g. `.go,.ts` |
| `-exclude-dir` | (none) | Directory names to skip, e.g. `vendor,node_modules` |
| `-max-size` | (none) | Skip files above this size. Accepts `10KB`, `2MB`, `1GB` |
| `-max-depth` | `-1` (unlimited) | Cap traversal depth |
| `-follow-symlinks` | false | Follow symlinked files and directories; loops are prevented |
 
### Output
 
| Flag | Default | Description |
|------|---------|-------------|
| `-format` | `plain` | Output format: `plain` or `json` |
| `-count` | false | Print only the total match count |
| `-quiet` | false | Suppress all output; use exit code only |
| `-color` | false | ANSI color highlighting in plain mode |
| `-abs` | false | Print absolute file paths |
 
### Concurrency
 
| Flag | Default | Description |
|------|---------|-------------|
| `-workers` | auto | Base worker count |
| `-io-workers` | auto | Workers dedicated to file reads |
| `-cpu-workers` | auto | Workers dedicated to pattern matching |
| `-dynamic-workers` | false | Enable auto-scaling of CPU workers under load |
| `-max-workers` | auto | Cap on dynamic CPU worker count |
| `-backpressure` | auto | Channel buffer depth |
 
### Diagnostics
 
| Flag | Default | Description |
|------|---------|-------------|
| `-metrics` | false | Print worker lifecycle and throughput summary after run |
| `-debug` | false | Enable debug logging |
| `-trace` | false | Enable verbose trace logging |
| `-monitor-goroutines` | false | Log goroutine count at regular intervals |
| `-monitor-interval-ms` | 250 | Interval for goroutine monitoring in ms (min 10) |
| `-cpuprofile <file>` | (none) | Write CPU profile to file |
| `-memprofile <file>` | (none) | Write heap profile to file on exit |
 
### Utility
 
| Flag | Default | Description |
|------|---------|-------------|
| `-config <path>` | `.gosearchrc` | Load JSON defaults from file |
| `-completion bash\|zsh\|fish` | (none) | Print shell completion script to stdout |
| `-version` | - | Print build version and exit |
 
---
 
## Output Format
 
### Plain (default)
 
```
path/to/file.go:42: matching line text here
```
 
### JSON (one object per line)
 
```json
{"path":"path/to/file.go","line":42,"text":"matching line text here"}
```
 
JSON output is newline-delimited, making it compatible with `jq`, `xargs`, and standard Unix pipelines.
 
---
 
## Ignore Rules
 
gosearch respects two ignore file formats:
 
- `.gitignore` - standard Git ignore syntax
- `.gosearchignore` - project-specific overrides with the same syntax
Both formats support:
- Glob patterns (`*.log`, `build/`)
- Negation patterns (`!important.log`)
- Directory-scoped inheritance (a rule in `src/.gitignore` applies only under `src/`)
Default ignored directories (always skipped unless explicitly negated): `.git`, `vendor`, `node_modules`.
 
Ignore evaluation happens at traversal time. Files that match ignore rules are pruned before they reach any worker - they never consume IO or CPU budget.
 
---
 
## Symlink Handling
 
Symlinks are skipped by default.
 
When `-follow-symlinks` is enabled:
- Both file and directory symlinks are followed.
- A visited-path set (using resolved real paths) prevents infinite loops from circular symlinks.
- The depth limit from `-max-depth` still applies.
---
 
## Architecture
 
gosearch processes files through a four-stage concurrent pipeline:
 
```
Traversal → IO Workers → CPU Workers → Printer
```
 
**Traversal** walks the filesystem, applies ignore rules, depth limits, and symlink policy, and emits path jobs onto a buffered channel.
 
**IO Workers** consume path jobs. Each worker opens the file, detects binary content (and skips it), reads lines, and emits line jobs.
 
**CPU Workers** consume line jobs. Each worker runs the match strategy (substring or regex) against each line and emits results.
 
**Printer** is a single goroutine that owns all writes to stdout. It serializes output, counts matches, and emits the final summary when the pipeline drains.
 
### Key properties
 
- Context cancellation (`SIGINT` / programmatic) propagates through all pipeline stages. No stage blocks indefinitely on shutdown.
- The single-printer design prevents interleaved output under any concurrency setting.
- Channel depths (backpressure) are configurable and default to values auto-scaled to worker counts.
- Worker counts for IO and CPU stages are independently configurable because their bottlenecks differ (disk throughput vs. regex evaluation).
### Matching strategies
 
Two strategies are available, selected at startup:
 
- **Substring** (default): uses `strings.Contains` or `bytes.Contains`. Fast, no allocation per match.
- **Regex**: compiles the pattern once at startup using Go's `regexp` package. Worker goroutines share the compiled `*regexp.Regexp` (which is safe for concurrent use).
Both strategies support case-insensitive and whole-word modifiers applied as preprocessing steps.
 
---
 
## Configuration File
 
gosearch loads defaults from `.gosearchrc` in the working directory, or from a path specified via `-config`.
 
Format is JSON. All keys are optional. CLI flags override config values.
 
```json
{
  "ignore_case": true,
  "workers": 8,
  "format": "json",
  "dynamic_workers": true,
  "extensions": [".go", ".ts"],
  "exclude_dirs": ["vendor", "node_modules"]
}
```
 
---
 
## Performance Characteristics
 
Benchmarks run on Windows, Intel Core Ultra 5 125H, Go benchmark mode (`-count=3 -benchmem`).
 
### IO strategy: Scanner vs. Reader
 
The `bufio.Reader` path uses approximately half the memory of the `bufio.Scanner` path for the same workload (~224 KB/op vs ~470 KB/op), with comparable latency. The Reader strategy is preferred.
 
### Worker scaling
 
| Workers | Latency range | Note |
|---------|---------------|------|
| 1 | 20–26 ms | Baseline, no parallelism |
| 2 | 28–35 ms | Overhead exceeds gain |
| 4 | 20–22 ms | Best observed throughput |
| 8 | 23–29 ms | Over-provisioned for this fixture |
 
4 workers produced the best throughput for the benchmark fixture. Over-provisioning to 8 workers added synchronization overhead without throughput gain. The right count is workload-dependent - the default is auto-scaled to `GOMAXPROCS`.
 
### Large-directory stress
 
10,000-file synthetic fixture: 20–24 ms per run, ~1.5 MB/op, ~38K allocs/op. No pathological growth observed across three passes.
 
### Profiling
 
CPU and heap profiles can be captured at runtime:
 
```bash
gosearch -cpuprofile cpu.out -memprofile mem.out "needle" ./src
go tool pprof cpu.out
```
 
---
 
## Testing and Quality
 
```bash
# All tests
go test -count=1 ./...
 
# Race detector
go test -count=1 -race ./...
 
# Benchmarks
go test -bench=. -benchmem ./...
 
# Fuzz (runs until interrupted)
go test -fuzz=Fuzz -run=^$ ./...
```
 
Test coverage includes:
- Unit tests for each pipeline stage and matching strategy
- Integration tests for CLI behavior, ignore rules, and symlink edge cases
- Race-detector clean on all test runs
- Property-based and fuzz tests for pattern matching robustness
---
 
## Release
 
Version is injected at build time:
 
```bash
go build -ldflags "-X main.version=v1.0.0" -o gosearch .
```
 
Cross-platform builds and checksums:
 
```bash
make cross VERSION=v1.0.0
make release VERSION=v1.0.0
```
 
Release scripts: `scripts/release.sh` (Unix), `scripts/release.ps1` (Windows).
 
---
 
## Shell Completions and Man Page
 
Completions are generated from the binary:
 
```bash
gosearch -completion bash > ~/.local/share/bash-completion/completions/gosearch
gosearch -completion zsh  > ~/.zfunc/_gosearch
gosearch -completion fish > ~/.config/fish/completions/gosearch.fish
```
 
Static assets are also committed to the repo under `completions/` for package maintainers.
 
Man page: `man/gosearch.1`
 
---
 
## Design Decisions
 
### Split IO and CPU worker pools
 
IO and CPU work have different bottlenecks. IO workers are gated by disk throughput; CPU workers by regex evaluation speed. Separating the pools lets each be tuned independently. The cost is more channel infrastructure and a slightly more complex lifecycle.
 
### Ignore rules evaluated at traversal, not at workers
 
Pruning ignored paths before they enter the pipeline means workers never waste time on files that will be discarded. The cost is that traversal carries rule-parsing state. This is preferable to a global cache because per-directory inheritance keeps rule precedence local and easy to reason about.
 
### Strategy interface for matching
 
Substring and regex logic is isolated behind a `MatchStrategy` interface rather than scattered through worker code. This makes the strategies independently testable and the workers agnostic to matching implementation. It adds one abstraction layer.
 
### Single printer goroutine
 
Output correctness under concurrency requires exactly one writer. A mutex would work but adds contention; a dedicated goroutine with a result channel is cleaner and naturally serializes the final match count.
 
### No daemon mode
 
gosearch is designed for bounded, on-demand execution. A persistent daemon would add operational complexity (lifecycle management, stale index invalidation, IPC) that is out of scope for a CLI-first tool. Users who need persistent search should use a dedicated indexing tool.
 
---
 
## Known Limitations
 
- Output order is non-deterministic under concurrency. Results are printed as they arrive from CPU workers, not in filesystem order.
- `SIGINT` handling in tests behaves differently on Windows vs. Unix due to OS signal delivery differences. Tests that rely on cancellation behavior are Unix-only.
