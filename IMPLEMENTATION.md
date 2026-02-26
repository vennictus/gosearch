# gosearch Implementation Summary

## What Was Built

A concurrent CLI search tool in Go that recursively scans a directory for a case-sensitive substring.

Command:

```bash
gosearch <pattern> <path>
```

Example:

```bash
./gosearch needle ./testdata/small
```

## Core Behavior Implemented

- Recursively walks directories using `filepath.WalkDir`.
- Sends file paths to a bounded worker pool (`runtime.NumCPU()` workers).
- Detects binary files by reading the first 512 bytes and skipping files containing `\x00`.
- Scans text files line-by-line using `bufio.Scanner`.
- Matches with `strings.Contains(line, pattern)`.
- Streams matches through a `results` channel.
- Uses exactly one printer goroutine so output never interleaves.
- Handles `Ctrl+C` with context cancellation and clean shutdown.
- Skips unreadable files by logging errors to stderr and continuing.

Output format:

```text
path/to/file:line_number: line_text
```

## Architecture (Implemented)

1. Parse CLI args and validate usage.
2. Create cancellable root context with SIGINT handling.
3. Start worker pool.
4. Start one printer goroutine.
5. Walk filesystem and enqueue jobs.
6. Close jobs channel when walk completes.
7. Wait for workers.
8. Close results channel.
9. Exit cleanly.

## Files Added

- `main.go` — CLI, traversal, workers, scanning, output, cancellation.
- `main_test.go` — unit and integration tests.
- `testdata/small/*` — deterministic text fixtures.
- `testdata/nested/*` — nested directory fixtures.
- `testdata/binary/binary.dat` — binary detection fixture.
- `go.mod` — module definition.
- `README.md` — project documentation.

## Tests Implemented

- File matching unit test (count, line numbers, contents).
- Binary detection unit test (binary is skipped).
- End-to-end CLI test via `os/exec`.
- Concurrency safety loop test (repeated runs).
- SIGINT cancellation test (clean exit; skipped on Windows in test).
- Invalid argument usage test.

## Validation Already Run

Build:

```bash
go build -o gosearch.exe .
```

Tests:

```bash
go test ./...
go test -race ./...
```

Smoke run:

```powershell
.\gosearch.exe needle .\testdata\small
```

Observed output:

```text
testdata\small\a.txt:1: alpha needle
testdata\small\b.txt:1: needle first
testdata\small\b.txt:2: needle second
testdata\small\b.txt:4: ending with needle
```

## Scope Kept Intentionally Out

Not implemented by design:

- Regex search
- Flags/options
- `.gitignore` handling
- Output ordering guarantees
- Colored output
- Parallel directory walking
