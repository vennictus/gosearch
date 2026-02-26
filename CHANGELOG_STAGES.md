# gosearch Stage Changelog

## Stage 1 — Polished Tool

### Delivered
- CLI flags for search behavior, worker count, file filters, and output controls.
- Plain and JSON output modes, highlighting support, and stable exit semantics.
- Baseline Stage 1 tests and readiness closure checks.

### Key outcomes
- Usage moved to `gosearch [flags] <pattern> <path>`.
- Exit codes standardized:
  - 0: matches found
  - 1: no matches
  - 2: usage/fatal setup/runtime error

## Stage 2 — Serious Engineering

### Delivered
- Match strategy abstraction with substring and regex implementations.
- Regex precompile at startup, shared safely across workers.
- Ignore-rule aware traversal with `.gitignore` and `.gosearchignore` support.
- Symlink policy controls and loop prevention.
- Split IO/CPU worker pipeline, dynamic CPU scaling, and backpressure controls.

### Key outcomes
- Ignore pruning occurs before enqueue.
- Worker lifecycle metrics added without changing core output ownership.
- Stage 1 contracts preserved.

## Stage 3 — Systems-Level Validation & Observability

### Delivered
- Benchmarks for scanner vs reader, worker scaling, and stress workloads.
- Fuzz tests for parser/matcher/ignore-rule safety.
- Deterministic harness and property-based checks.
- Debug/trace modes, goroutine monitoring, and phase timings.
- Runtime profile outputs via CPU and heap profile flags.

### Key outcomes
- Performance and diagnostics became measurable and repeatable.

## Stage 4 — Tooling & Ecosystem

### Delivered
- Config file defaults via `.gosearchrc` (JSON) with CLI override precedence.
- Version output and ldflags version injection.
- Runtime completion output and shell completion assets (bash/zsh/fish).
- Man page, cross-build/release automation, and checksum generation.
- Documentation set: architecture, concurrency, performance report, tradeoffs, why-not-x.

### Key outcomes
- Project became release-ready and portfolio-grade with reproducible tooling.

## Validation Snapshot

- `go test ./...` passes
- `go test -race ./...` passes
- Cross-platform release artifacts and checksum generation validated via scripts
