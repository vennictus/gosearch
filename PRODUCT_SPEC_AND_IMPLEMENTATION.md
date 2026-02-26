# gosearch — Product Spec and Implementation Record

## 1) Product Definition

gosearch is a concurrent CLI tool that recursively searches files for text matches with deterministic shutdown behavior, bounded concurrency, and developer-focused observability/tooling.

## 2) Original PRD Intent

### Core goals
- Recursive search across a directory tree
- Bounded concurrency with worker pools
- Streaming output (no full-output buffering)
- Safe Ctrl+C cancellation
- Deterministic, testable behavior

### Original non-goals in initial scope
- No regex
- No flags/options
- No `.gitignore`
- No advanced output customizations

Note: Later stages intentionally expanded beyond initial non-goals as planned maturity steps.

## 3) Final Implemented Architecture

### Runtime pipeline
- Traversal engine
- path jobs channel
- IO workers (file access, binary checks, line extraction)
- line jobs channel
- CPU workers (match strategy evaluation)
- result channel
- single printer goroutine

### Design properties
- Single output owner (printer)
- Context-driven cancellation propagation
- Configurable worker bounds and channel backpressure
- Optional dynamic CPU worker scaling

## 4) Matching Model

- Strategy abstraction isolates matching logic.
- Substring strategy and regex strategy are separate implementations.
- Regex patterns are compiled once at startup.
- Highlighting consumes matcher-provided ranges only.

## 5) Filesystem Semantics

- Per-directory ignore parsing with inheritance:
  - `.gitignore`
  - `.gosearchignore`
- Default ignore directory rules (`.git`, `vendor`, `node_modules`)
- Ignore pruning before enqueue
- Configurable symlink policy and loop prevention via resolved path tracking
- Depth-limited traversal support

## 6) CLI and Config Contract

### Command
- `gosearch [flags] <pattern> <path>`

### Config defaults
- Optional `.gosearchrc` (JSON)
- CLI flags override config values

### Exit codes
- `0` match found
- `1` no matches
- `2` usage/fatal setup/runtime error

## 7) Performance, Observability, and Tooling

### Performance work
- Benchmarks for scanner vs reader, worker scaling, and stress workloads
- Runtime CPU/heap profile output options

### Observability
- Debug/trace modes
- Goroutine count monitoring
- Per-phase timings (walk, scan, print, total)
- Worker lifecycle metrics (started/stopped/active/idle/max active)

### Tooling ecosystem
- Man page
- Shell completions (bash/zsh/fish)
- Version injection via ldflags
- Cross-platform release scripts and checksums

## 8) Testing Record

- Unit tests for matching/binary behavior and config semantics
- Integration CLI execution tests
- Cancellation and concurrency safety checks
- Symlink and ignore-rule edge-case tests
- Fuzz tests for parser/matcher/ignore matching safety
- Property-based ignore-rule checks
- Race detector clean

## 9) Current Status

Project is in a release-ready state for its defined scope:
- architecture is explicit
- behavior is test-backed
- tooling/docs are integrated
- build/release workflow is reproducible
