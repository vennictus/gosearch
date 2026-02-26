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

## Architecture

Execution flow:

1. Parse CLI args
2. Create cancellable context with SIGINT handling
3. Start worker pool (`runtime.NumCPU()` workers)
4. Start a single printer goroutine
5. Traverse filesystem with ignore-rule pruning and depth/symlink controls
6. IO workers read files and emit line jobs
7. CPU workers apply match strategy and emit results
8. Printer streams results as they arrive

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

- Performance benchmarking
