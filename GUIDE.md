# gosearch - Guide

This guide explains how gosearch works from the ground up. No prior experience with Go or command-line tools is required.

---

## Table of Contents

1. [What problem does this solve?](#1-what-problem-does-this-solve)
2. [Core concepts](#2-core-concepts)
3. [The pipeline](#3-the-pipeline)
4. [Ignore rules](#4-ignore-rules)
5. [Symlinks and loop prevention](#5-symlinks-and-loop-prevention)
6. [Cancellation](#6-cancellation)
7. [Backpressure and flow control](#7-backpressure-and-flow-control)
8. [Dynamic worker scaling](#8-dynamic-worker-scaling)
9. [Exit codes and scripting](#9-exit-codes-and-scripting)
10. [Configuration file](#10-configuration-file)
11. [Testing](#11-testing)
12. [Glossary](#12-glossary)

---

## 1. What problem does this solve?

Real codebases are large. A typical project might have hundreds of files and tens of thousands of lines. Developers search them constantly - tracking down where a function is defined, where an error originates, which files reference a config key.

Basic tools like bare `grep` work, but they have no awareness of project structure. They return results from dependency folders with thousands of files you don't own, they can't produce structured output for pipelines, and they give you no tuning knobs for large trees.

gosearch is built specifically for this use case:

| Problem | gosearch's answer |
|---------|-------------------|
| Searching files one at a time is slow | Concurrent pipeline with separate IO and CPU workers |
| Results flooded with dependency files | Respects `.gitignore` and `.gosearchignore` |
| No context about where a match is | Reports file path, line number, and matched text |
| Hard to integrate into scripts | JSON output and deterministic exit codes |

---

## 2. Core concepts

### The file system as a tree

Your project's files are arranged in a tree. gosearch starts at a root directory you specify and walks every branch, deciding which files to search.

```
my-project/
├── main.go
├── internal/
│   ├── config.go
│   └── search.go
└── testdata/
    └── sample.txt
```

Each file has a **path** (its address in the tree) and **contents** (lines of text). Searching means walking the tree, opening each file, and checking each line against your pattern.

### Concurrency

gosearch searches multiple files at the same time by running work in parallel across multiple goroutines - Go's lightweight concurrent workers.

Without concurrency, files are searched one after another:

```
Worker: file1 → file2 → file3   (3 seconds total)
```

With concurrency, multiple files are searched simultaneously:

```
Worker 1: file1
Worker 2: file2                  (1 second total)
Worker 3: file3
```

Your CPU has multiple cores. Concurrency puts all of them to work.

### Goroutines and channels

A **goroutine** is a lightweight function that runs independently and concurrently. You can have thousands of them active at once without the overhead of traditional threads.

**Channels** are how goroutines pass data to each other safely - like a conveyor belt between workers. One goroutine puts an item on; another picks it up.

```
[Producer goroutine] --> channel --> [Consumer goroutine]
```

gosearch's entire pipeline is built from goroutines connected by channels.

### Matching strategies

gosearch supports two ways to match a pattern against a line of text.

**Substring** (default) checks whether the line contains the literal string. It is fast and requires no overhead.

**Regex** (`-regex`) checks whether the line matches a Go regular expression pattern. More flexible, slightly more expensive. The pattern is compiled once at startup and shared safely across all workers.

Both strategies support two modifiers:

| Modifier | Flag | Effect |
|----------|------|--------|
| Case-insensitive | `-i` | `"Error"` matches `"error"`, `"ERROR"`, etc. |
| Whole-word | `-w` | `"log"` does not match `"logger"` or `"catalog"` |

---

## 3. The pipeline

gosearch processes files through four sequential stages. Each stage runs concurrently with the others and hands work to the next stage through a channel.

```
Traversal → IO Workers → CPU Workers → Printer
```

### Stage 1: Traversal

Traversal walks the file system starting from the path you provide. For each entry it finds, it checks whether the file should be searched - applying ignore rules, depth limits, extension filters, and symlink policy. Files that pass are sent as path jobs to the IO workers.

Directories like `.git`, `vendor`, and `node_modules` are pruned here, before they ever reach a worker. Ignored paths consume no IO or CPU budget at all.

### Stage 2: IO workers

IO workers receive path jobs from traversal. Each worker opens the file, checks whether it is binary (and skips it if so), reads it line by line, and sends each line as a line job to the CPU workers.

IO and CPU work are handled by separate worker pools because their bottlenecks differ. IO workers are limited by disk read speed. CPU workers are limited by pattern evaluation speed. Separating them means each pool can be sized and tuned independently.

### Stage 3: CPU workers

CPU workers receive line jobs. Each worker runs the match strategy against the line and, if it matches, sends a result to the printer.

CPU workers can optionally scale dynamically. If the line job queue backs up, gosearch can spawn additional CPU workers up to a configurable ceiling. See [Dynamic worker scaling](#8-dynamic-worker-scaling).

### Stage 4: Printer

The printer is a single goroutine that owns all writes to stdout. It receives results from the CPU workers, formats them, and writes them out.

Having exactly one printer is important. If multiple workers wrote to stdout simultaneously, their output would interleave into garbled text. The single printer serializes output cleanly and maintains an accurate match count.

Output is either plain text or JSON, controlled by `-format`:

```
# Plain (default)
path/to/file.go:42: matching line text here

# JSON
{"path":"path/to/file.go","line":42,"text":"matching line text here"}
```

---

## 4. Ignore rules

### Why ignore files?

Projects contain folders you almost never want to search: `.git` holds Git's internal state, `node_modules` can contain 50,000+ files from JavaScript dependencies, `vendor` holds copied Go dependencies. Searching them produces noise and wastes time.

gosearch handles this by reading ignore files and pruning matched paths during traversal.

### Supported formats

gosearch reads two ignore file formats:

| File | Purpose |
|------|---------|
| `.gitignore` | Standard Git ignore rules - gosearch respects these automatically |
| `.gosearchignore` | Search-specific overrides using the same syntax |

Both files use the same pattern syntax:

```gitignore
# Ignore all log files
*.log

# Ignore the build output folder
build/

# Re-include one specific file despite the rule above
!important.log
```

### Inheritance

Ignore files apply to their directory and all subdirectories. A rule in `src/.gitignore` applies only under `src/`. A rule at the project root applies everywhere. Deeper rules can override parent rules using negation (`!`).

```
project/
├── .gitignore          ← "ignore *.log" applies everywhere below
├── logs/
│   └── debug.log       ← ignored
└── src/
    ├── .gitignore      ← "!important.log" re-includes it locally
    ├── important.log   ← not ignored (local rule wins)
    └── other.log       ← still ignored (parent rule applies)
```

---

## 5. Symlinks and loop prevention

A **symbolic link** (symlink) is a file that points to another file or directory - similar to a shortcut. Symlinks are skipped by default. You can enable following them with `-follow-symlinks`.

The risk with symlinks is infinite loops. If two directories each contain a symlink pointing to the other, a naive traversal would descend forever:

```
folder-a/ → symlink to folder-b
folder-b/ → symlink to folder-a   ← this sends traversal back to folder-a
```

gosearch prevents this by tracking every real (resolved) path it has visited. Before entering a symlinked directory, it checks whether that resolved path has already been seen. If it has, the directory is skipped.

---

## 6. Cancellation

gosearch supports clean cancellation via `Ctrl+C`. When the signal is received, a cancellation is propagated through a shared context to all active goroutines - traversal, all IO workers, all CPU workers, and the printer - and each stops as soon as it finishes its current unit of work.

This means no zombie search process continues consuming CPU and disk after you interrupt it.

The same mechanism works programmatically. Any code that holds the context can cancel the entire pipeline cleanly.

---

## 7. Backpressure and flow control

Traversal can find files faster than IO workers can read them, and IO workers can produce lines faster than CPU workers can match them. Without control, a fast producer would pile up millions of pending jobs in memory.

gosearch controls this with **buffered channels**. Each channel between pipeline stages has a fixed buffer. When the buffer is full, the producer blocks and waits until the consumer catches up. This naturally paces the pipeline without explicit coordination logic.

```
Traversal fills buffer → buffer full → traversal pauses
                                     → IO workers drain buffer
                                     → traversal resumes
```

The buffer size is configurable via `-backpressure`. A larger buffer allows more queued work at the cost of higher memory use. A smaller buffer keeps memory tighter at the cost of more frequent pauses. The default is auto-scaled to the worker counts.

---

## 8. Dynamic worker scaling

Fixed worker counts work well for consistent workloads but can underperform on mixed ones. A search through many small files is IO-bound. A search with a complex regex across large files is CPU-bound. The optimal CPU worker count differs between these cases.

With `-dynamic-workers`, gosearch monitors the line job queue. If jobs are accumulating faster than CPU workers can process them, it spawns additional CPU workers up to the ceiling set by `-max-workers`.

```bash
gosearch -dynamic-workers -max-workers 16 "pattern" .
```

Dynamic scaling is off by default. For most searches on modern hardware, the auto-sized default worker counts are sufficient.

---

## 9. Exit codes and scripting

When gosearch finishes, it returns a numeric exit code to the shell. This makes it composable in scripts and CI pipelines without parsing output.

| Code | Meaning | Notes |
|------|---------|-------|
| `0` | Matches found | The pattern was found at least once |
| `1` | No matches | Not an error - the standard "not found" signal |
| `2` | Fatal error | Bad arguments, invalid regex, or runtime failure |

Example: fail a CI build if hardcoded passwords are detected:

```bash
if gosearch -quiet "password" ./src; then
    echo "Hardcoded password found - failing build"
    exit 1
fi
```

Example: collect all TODOs into a JSON file:

```bash
gosearch -format json "TODO" . > todos.json
```

The `-quiet` flag suppresses all output so only the exit code matters. The `-count` flag prints just the total match count without individual results.

---

## 10. Configuration file

If you use the same flags on every search, you can set them as defaults in `.gosearchrc` at your project root. The file is JSON and all keys are optional.

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

CLI flags always override config file values. If `.gosearchrc` sets `"format": "json"` but you pass `-format plain`, plain wins.

A custom config path can be specified with `-config path/to/file`.

---

## 11. Testing

gosearch's test suite covers four categories:

| Type | Purpose |
|------|---------|
| Unit tests | Test individual components (matcher, ignore parser, config) in isolation |
| Integration tests | Test full CLI behavior, ignore rule evaluation, symlink edge cases |
| Race detector runs | Verify no data races exist under concurrent execution |
| Fuzz and property tests | Feed random inputs to the matcher to surface crashes and edge cases |

```bash
# Run all tests
go test -count=1 ./...

# Run with race detector
go test -count=1 -race ./...

# Run benchmarks
go test -bench=. -benchmem ./...

# Run fuzz tests (runs until interrupted)
go test -fuzz=Fuzz -run=^$ ./...
```

The race detector is the most important flag during development. It catches concurrency bugs - like two goroutines writing to the same variable - that are otherwise invisible and intermittent.

---

## 12. Glossary

| Term | Definition |
|------|------------|
| Backpressure | The mechanism by which a slow consumer slows down a fast producer to prevent memory overflow |
| Binary file | A file containing non-text data such as images or compiled executables; gosearch skips these automatically |
| Channel | A typed conduit for passing data between goroutines in Go |
| Concurrency | Running multiple tasks in overlapping time periods |
| Context | Go's mechanism for propagating cancellation and deadlines across goroutines |
| Exit code | An integer returned by a program on exit; `0` conventionally means success |
| Goroutine | A lightweight concurrent function in Go, cheaper than an OS thread |
| IO | Input/Output - reading from and writing to disk, network, etc. |
| Pipeline | A series of processing stages connected in sequence, each feeding the next |
| Regex | Regular expression - a pattern language for matching text |
| Symlink | A file that points to another file or directory |
| Traversal | Walking through a directory tree to enumerate its contents |
