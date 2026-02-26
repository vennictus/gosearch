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

## Architecture

Execution flow:

1. Parse CLI args
2. Create cancellable context with SIGINT handling
3. Start worker pool (`runtime.NumCPU()` workers)
4. Start a single printer goroutine
5. Walk filesystem with `filepath.WalkDir` and send file paths to workers
6. Workers scan files line-by-line and emit matches
7. Printer streams results as they arrive

## Concurrency Model

- Fixed-size worker pool (`runtime.NumCPU()`)
- `jobs` channel carries file paths
- `results` channel carries match records
- One printer goroutine serializes output to avoid interleaving
- Cancellation propagates through context to walker and workers

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

- No regex support
- No output ordering guarantees
- No Windows-native signal semantics for cancellation test

## Why Go

Go provides simple primitives for concurrency (goroutines, channels, context), strong standard library support for filesystem and process handling, and straightforward tooling for testing concurrent systems.

## Future Work (Not Implemented)

- Regex support
- `.gitignore` integration
- Performance benchmarking
