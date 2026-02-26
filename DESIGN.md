# gosearch Design

## What this project is

`gosearch` is a concurrent command-line search tool for recursively scanning files and reporting matching lines. It is designed to be fast, predictable, and practical for real repositories.

## Product goals

- Search recursively with bounded concurrency.
- Stream output instead of buffering all results.
- Handle cancellation cleanly.
- Provide practical CLI ergonomics for daily use.
- Keep behavior testable and release-ready.

## Runtime architecture

Pipeline:

1. Filesystem traversal with ignore-rule pruning
2. Path jobs sent to IO workers
3. IO workers open/read files and emit line jobs
4. CPU workers evaluate match strategy and emit results
5. Single printer goroutine owns output

Key properties:

- Context cancellation propagates across all stages
- Single output owner prevents interleaved writes
- Worker counts and channel backpressure are configurable
- Optional dynamic CPU worker scaling for bursty workloads

## Matching model

- Substring strategy (default)
- Regex strategy (`-regex`), compiled once at startup
- Optional whole-word and case-insensitive matching

## Filesystem semantics

- Supports `.gitignore` and `.gosearchignore`
- Ignore rules inherit by directory depth
- Negation rules (`!pattern`) are supported
- Default ignored directories: `.git`, `vendor`, `node_modules`
- Symlinks are skipped by default and can be enabled with loop prevention

## CLI contract

Command format:

```text
gosearch [flags] <pattern> <path>
```

Exit codes:

- `0`: one or more matches found
- `1`: no matches
- `2`: invalid usage or fatal runtime/setup error

Config:

- Optional JSON defaults from `.gosearchrc`
- CLI flags override config values

## Observability and performance tooling

- Metrics mode includes worker lifecycle and phase timings
- Debug/trace logging modes are available
- Optional CPU and heap profile outputs
- Benchmark and fuzz suites are included

## Quality strategy

- Unit and integration tests for search behavior and CLI flows
- Ignore/symlink edge-case tests
- Race-detector test runs
- Property-based and fuzz testing for robustness

## Release tooling

- Version injection via `-ldflags`
- Man page and shell completions (bash/zsh/fish)
- Cross-build and checksum workflows via Makefile and scripts
