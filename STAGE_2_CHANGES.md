# Stage 2 of 5 — Tier 2 Serious Engineering

This document records the architectural additions and changes made in Stage 2.

## Scope Implemented

### Search Logic

- Added regex search mode via `-regex`.
- Added precompiled matcher abstraction:
  - `MatchStrategy` interface
  - `Matcher` strategy (substring)
  - `RegexStrategy` (compiled once in startup path)
- Added strategy selection at startup so matching logic is shared safely across workers.

### Filesystem

- Added `.gitignore` support.
- Added support for multiple ignore files per directory:
  - `.gitignore`
  - `.gosearchignore`
- Added default ignore rules for common directories:
  - `.git`
  - `vendor`
  - `node_modules`
- Added symlink behavior flag:
  - `-follow-symlinks` (default off)

### Traversal

- Replaced plain `WalkDir` traversal with cancellation-aware recursive traversal.
- Added directory pruning before enqueue using:
  - ignore rules
  - default ignore directories
  - extension and size filters
- Added depth limiting via `-max-depth`.
- Added more frequent context cancellation checks for early stop responsiveness.

### Concurrency

- Introduced split worker pipeline:
  - IO workers: file open, binary check, line extraction
  - CPU workers: matching strategy evaluation and result emission
- Added dynamic CPU worker scaling via `-dynamic-workers` + `-max-workers`.
- Added configurable backpressure with buffered channels via `-backpressure`.
- Added worker lifecycle + throughput metrics via `-metrics`.

## New/Updated Flags (Stage 2)

- `-regex`
- `-follow-symlinks`
- `-max-depth`
- `-dynamic-workers`
- `-io-workers`
- `-cpu-workers`
- `-max-workers`
- `-backpressure`
- `-metrics`

## Tests Added in Stage 2

- Regex mode behavior.
- Regex negative-case behavior.
- Regex vs substring parity behavior.
- `.gitignore` filtering behavior.
- Nested ignore precedence behavior.
- Depth-limited traversal behavior.
- Dynamic worker config parsing behavior.
- Symlink follow/non-follow behavior.
- Symlink loop and dangling symlink behavior.
- Cancellation with regex + ignore enabled.
- Metrics output observability behavior.

## Stage-2 Completion Status

- Matcher remains strategy-based with regex precompiled once at startup.
- Ignore rules are applied before enqueue; ignored paths do not reach workers.
- Symlink follow policy is explicit (`off` by default, opt-in by flag) with loop prevention.
- Traversal cancellation checks are early and frequent.
- Backpressure and split IO/CPU worker pipelines remain explicit and test-covered.
- Worker lifecycle metrics include started/stopped plus active/idle observability.
- Stage-1 contracts (output modes and exit code semantics) are preserved.

## Files Changed in Stage 2

- `main.go`
- `main_test.go`
- `STAGE_2_CHANGES.md`
