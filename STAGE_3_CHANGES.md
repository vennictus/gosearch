# Stage 3 of 5 — This Is No Longer a Toy

This document records additions and changes made in Stage 3.

## Scope Implemented

### Performance

- Added benchmark suite using `testing.B` in `benchmark_test.go`.
- Added scanner vs reader comparison benchmark.
- Added worker scaling benchmarks across multiple worker counts.
- Added large directory stress benchmark.
- Added runtime profiling support:
  - `-cpuprofile` for CPU profile output
  - `-memprofile` for heap profile output

### Correctness

- Added fuzz tests in `fuzz_test.go` for:
  - size parser robustness
  - matcher range generation safety
  - ignore rule match function robustness
- Added deterministic test harness helper for stable assertions under concurrent output.
- Added property-based test for ignore-rule negation semantics (`testing/quick`).

### Observability

- Added debug logging mode (`-debug`).
- Added verbose execution trace (`-trace`).
- Added goroutine count monitoring (`-monitor-goroutines`, `-monitor-interval-ms`).
- Added per-phase timing metrics (`walk`, `scan`, `print`, `total`) when `-metrics` is enabled.
- Extended worker metrics to include active/idle and max active observability.

## Files Changed in Stage 3

- `main.go`
- `README.md`
- `benchmark_test.go`
- `fuzz_test.go`
- `stage3_test.go`
- `STAGE_3_CHANGES.md`

## Validation

- `go test ./...`
- `go test -race ./...`
