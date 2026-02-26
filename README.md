# gosearch

`gosearch` is a small concurrent CLI tool that recursively searches files for a case-sensitive substring.

## Problem Statement

Developers often need to search large directory trees quickly. This project demonstrates how a clear, bounded-concurrency search tool works internally using idiomatic Go.

## Usage

```bash
gosearch [flags] <pattern> <path>
```

Example:

```bash
gosearch -i -w -workers 4 needle ./testdata/small
```

### Flags

- `-i` case-insensitive matching
- `-n` show line numbers (default `true`, disable with `-n=false`)
- `-w` whole-word matching
- `-workers N` worker pool size
- `-max-size` max file size (bytes or `KB`/`MB`/`GB` suffix)
- `-extensions .go,.txt` include-only file extensions
- `-exclude-dir vendor,node_modules` directory names to skip
- `-count` print only total match count
- `-quiet` suppress output and rely on exit code only
- `-color` ANSI highlight matches in plain output
- `-abs` print absolute file paths
- `-format plain|json` output mode switch
- `-regex` treat pattern as regex
- `-follow-symlinks` follow symlinked files/directories
- `-max-depth` traversal depth limit (`-1` for unlimited)
- `-dynamic-workers` enable dynamic CPU worker scaling
- `-io-workers` number of IO workers (`0` auto)
- `-cpu-workers` number of CPU workers (`0` auto)
- `-max-workers` dynamic mode CPU worker cap (`0` auto)
- `-backpressure` buffered channel size (`0` auto)
- `-metrics` print worker lifecycle and throughput metrics
- `-debug` enable debug logging
- `-trace` enable verbose execution trace
- `-monitor-goroutines` periodically report goroutine count
- `-monitor-interval-ms` goroutine monitor interval in milliseconds
- `-cpuprofile file.out` write CPU profile
- `-memprofile file.out` write heap profile on exit
- `-config file` load defaults from `.gosearchrc` JSON
- `-completion bash|zsh|fish` print completion script
- `-version` print build version

Config file support:

- Default config file path is `.gosearchrc`.
- Config file format is JSON.
- CLI flags override config file defaults.

Example `.gosearchrc`:

```json
{
	"ignore_case": true,
	"workers": 8,
	"format": "json"
}
```

### Output format

Plain mode (`-format plain`, default):

```text
path/to/file:line_number: line_text
```

JSON mode (`-format json`):

```json
{"path":"...","line":12,"text":"..."}
```

### Exit codes

- `0` one or more matches found
- `1` no matches found
- `2` invalid usage or fatal setup/runtime error

## Ignore & Symlink Semantics

- `.gitignore` and `.gosearchignore` are parsed during traversal.
- Ignore rules are evaluated per directory and inherited by child directories.
- Nested ignore files can override parent rules using negation patterns (`!pattern`).
- Default ignored directories include `.git`, `vendor`, and `node_modules`.
- Ignore pruning happens before file enqueue; ignored paths are never scanned by workers.
- Symlinks are skipped by default.
- Enable symlink following with `-follow-symlinks`.
- Symlink directory loops are prevented using resolved-path tracking.

## Performance Notes

- Regex mode has higher CPU cost than plain substring mode.
- Ignore-rule parsing adds traversal overhead, especially on deeply nested trees with many ignore files.
- Dynamic worker scaling improves throughput under bursty loads but may increase scheduling overhead.

## Stage-3 Engineering Tooling

- Benchmarks are available for scanner vs reader performance, worker scaling, and large-directory stress.
- Fuzz tests cover parser and matcher robustness.
- Property-based tests cover ignore-rule semantics.
- Metrics mode now includes phase timings (`walk`, `scan`, `print`, `total`).

## CLI Ecosystem

- Man page: `man/gosearch.1`
- Completion assets:
	- `completions/bash/gosearch.bash`
	- `completions/zsh/_gosearch`
	- `completions/fish/gosearch.fish`

Install man page locally example:

```bash
sudo cp man/gosearch.1 /usr/local/share/man/man1/
man gosearch
```

Generate completion scripts from CLI:

```bash
gosearch -completion bash
gosearch -completion zsh
gosearch -completion fish
```

## Packaging & Releases

- Cross-compile builds: `make cross VERSION=vX.Y.Z`
- Release + checksums: `make release VERSION=vX.Y.Z`
- Release automation scripts:
	- `scripts/release.sh`
	- `scripts/release.ps1`
- Version injection:

```bash
go build -ldflags "-X main.version=vX.Y.Z" -o gosearch .
```

## Documentation Artifacts

- Architecture diagram: `docs/architecture.md`
- Concurrency diagram: `docs/concurrency.md`
- Performance report: `docs/performance-report.md`
- Design tradeoff log: `docs/design-tradeoffs.md`
- Why-not-X section: `docs/why-not-x.md`

## Architecture

Execution flow:

1. Parse CLI args
2. Create cancellable context with SIGINT handling
3. Resolve config defaults (including optional `.gosearchrc`)
4. Build precompiled match strategy (substring or regex)
5. Start single printer goroutine
6. Start IO and CPU worker pools (with optional dynamic CPU scaling)
7. Traverse filesystem with ignore-rule pruning and depth/symlink controls
8. IO workers read files and emit line jobs
9. CPU workers apply match strategy and emit results
10. Printer streams results and finalizes exit outcome

## Concurrency Model

- Split pipeline: traversal → file jobs → IO workers → line jobs → CPU workers → results → single printer
- Match strategy is precompiled once and shared safely across CPU workers
- Dynamic CPU scaling available (`-dynamic-workers` + `-max-workers`)
- Backpressure is tunable via buffered channels (`-backpressure`)
- One printer goroutine serializes output to avoid interleaving
- Cancellation propagates through context to walker and workers

## Stage-1 Contract Stability

Stage-2 features are additive and preserve Stage-1 behavior:

- Output ordering remains non-deterministic under concurrency.
- `plain`, `json`, `count`, and `quiet` modes keep the same semantics.
- Exit codes remain `0` (match found), `1` (no matches), `2` (usage/fatal setup/runtime).

## Testing

Run tests:

```bash
go test ./...
```

Run with race detector:

```bash
go test -race ./...
```

The suite includes:

- File matching behavior
- Binary file detection and skipping
- End-to-end CLI execution through `os/exec`
- Concurrency safety loop
- SIGINT cancellation behavior (skipped on Windows)
- Stage-1 flag behavior tests (`-i`, `-w`, `-count`, `-quiet`, `-format`)

## Known Limitations

- No output ordering guarantees (concurrent streaming)
- No Windows-native signal semantics for cancellation test

## Why Go

Go provides simple primitives for concurrency (goroutines, channels, context), strong standard library support for filesystem and process handling, and straightforward tooling for testing concurrent systems.

## Future Work (Not Implemented)

- Stage 5 scope
