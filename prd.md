# Product Requirements Document (PRD)

## Project Metadata

- **Project Name:** gosearch
- **Category:** Systems / CLI / Concurrency
- **Language:** Go (Golang)
- **Platform:** Linux (via WSL2)
- **Complexity Level:** Entry → Intermediate Systems Programming

## 1. Purpose & Context

This project is designed as a first serious Go project with the goal of mastering:

- Idiomatic Go structure
- Goroutines and channels with intent
- Worker pool concurrency
- OS interaction (filesystem, signals)
- Deterministic behavior under concurrency
- Testing I/O-heavy systems code

The output must be simple, correct, and boring — boring is good. Reliability beats cleverness.

## 2. Problem Statement

Searching for a text pattern across a directory tree is a common developer task. Existing tools are optimized and complex, but obscure their internal mechanics.

This project builds a clear, understandable, concurrent file-search CLI tool that demonstrates how such tools work internally, not how many features they can support.

## 3. Product Goals

### Primary Goals

- Recursively search files for a substring
- Use bounded concurrency (worker pool)
- Stream results without buffering entire output
- Handle Ctrl+C cleanly
- Be testable and deterministic

### Explicit Non-Goals (Hard Constraints)

The following must **not** be implemented:

- Regex search
- Flags or options
- `.gitignore` handling
- Sorting or result ordering
- Windows native support
- Colorized output
- Parallel directory walking
- Performance micro-optimizations
- Background jobs

Scope violations = failed project.

## 4. User Profile

Target user:

- Developer comfortable with terminal usage
- Wants fast feedback from a CLI tool
- Does not require advanced filtering or formatting

## 5. CLI Specification

### Command Format

```text
gosearch <pattern> <path>
```

### Arguments

| Name | Type | Required | Description |
|---|---|---|---|
| `pattern` | string | Yes | Case-sensitive substring to search |
| `path` | string | Yes | Root directory for recursive search |

### Invalid Usage Handling

If arguments are missing or invalid:

- Print to stderr:

  ```text
  Usage: gosearch <pattern> <path>
  ```

- Exit with non-zero status

## 6. Functional Requirements

### 6.1 Directory Traversal

- Use `filepath.WalkDir`
- Traverse recursively from `<path>`
- Skip directories automatically
- Ignore symlink loops
- Stop traversal immediately on cancellation

### 6.2 File Filtering

- Only process text files
- Binary file detection:
  - Read first 512 bytes
  - If any `\x00` byte is present → skip file
- Skip unreadable files without crashing

### 6.3 File Scanning

- Read files line-by-line using `bufio.Scanner`
- Maintain line number counter
- Match logic:

  ```go
  strings.Contains(line, pattern)
  ```

### 6.4 Concurrency Model (Mandatory)

#### Worker Pool

- Fixed number of workers
- Worker count = `runtime.NumCPU()`
- No dynamic scaling
- No goroutine-per-file model

#### Channels

- `jobs chan string` → file paths
- `results chan Result` → matches
- Directional channels must be used where applicable

#### Result Type

```go
type Result struct {
    Path string
    Line int
    Text string
}
```

### 6.5 Output Handling

- Exactly one printer goroutine
- Print results as soon as received
- Output format:

  ```text
  path/to/file:line_number: line_text
  ```

- Order is not guaranteed
- Output must never interleave or corrupt

### 6.6 Cancellation & Signals

- Capture `SIGINT` (Ctrl+C)

On signal:

- Cancel root context
- Stop directory walking
- Allow workers to exit
- Exit without panic or deadlock

Cancellation is normal behavior, not an error.

## 7. Error Handling Rules

- Panics are forbidden
- Errors must be returned or logged

File errors:

- Print to stderr
- Continue execution

Argument errors:

- Exit immediately

## 8. Performance Constraints

- Files must not be loaded fully into memory
- Memory usage must scale with worker count, not file count
- Tool must remain responsive under large directories

## 9. Architecture Overview

### Execution Flow

```text
main
 ├─ parse CLI args
 ├─ create cancellable context
 ├─ setup signal handler
 ├─ create jobs channel
 ├─ create results channel
 ├─ start worker pool
 ├─ start printer goroutine
 ├─ walk filesystem → send jobs
 ├─ close jobs channel
 ├─ wait for workers
 ├─ close results channel
 └─ exit
```

### Design Constraints

- No global state
- No shared mutable state
- No package-level variables
- Single responsibility per function

## 10. File Structure

```text
gosearch/
 ├─ main.go
 ├─ go.mod
 ├─ README.md
 └─ testdata/
     ├─ small/
     ├─ binary/
     └─ nested/
```

## 11. Testing Strategy (Critical Section)

This project must include tests. Not optional.

### 11.1 Testing Philosophy

- Focus on behavior, not implementation
- Prefer small controlled test directories
- Do not mock the filesystem — use real files
- Deterministic assertions only

### 11.2 Test Data Layout

```text
testdata/small/
  a.txt   → contains pattern once
  b.txt   → contains pattern multiple times
  c.txt   → no matches

testdata/binary/
  binary.dat → contains null bytes

testdata/nested/
  dir1/
    d.txt
  dir2/
    e.txt
```

### 11.3 Unit Tests

#### Test: File Matching

Input: known file + pattern

Assert:

- Correct number of matches
- Correct line numbers
- Correct content

#### Test: Binary Detection

Input: binary file

Assert:

- File is skipped
- No results returned

### 11.4 Integration Tests

#### End-to-End CLI Test

- Execute `gosearch` via `os/exec`
- Capture stdout/stderr
- Run against `testdata/small`

Assert:

- Output contains expected lines
- Output does not contain unexpected files

### 11.5 Concurrency Safety Test

- Run search multiple times in a loop

Ensure:

- No deadlocks
- No race conditions

Run with:

```bash
go test -race
```

Race detector must pass.

### 11.6 Cancellation Test

- Start search on large directory
- Send SIGINT programmatically

Assert:

- Process exits cleanly
- No panic
- No goroutine leaks

## 12. README Requirements

README must include:

- Clear problem statement
- Usage example
- Architecture explanation
- Concurrency model explanation
- Testing instructions
- Known limitations
- Why Go was chosen

Tone: confident, precise, not verbose.

## 13. Acceptance Criteria

The project is complete when:

- All tests pass
- `go test -race` passes
- Ctrl+C works
- Output format is correct
- Code is readable and idiomatic
- Scope strictly followed

## 14. Future Work (Not Implemented)

- Regex support
- Flags
- `.gitignore`
- Colored output
- Performance benchmarking

Mentioned only in README.

## Final Instruction for Code Generation

Implement exactly what is specified.
Do not invent features.
Do not simplify concurrency.
Do not ignore tests.
Favor clarity over cleverness.