# Stage 4 of 5 — Tooling & Ecosystem

This document records additions and changes made in Stage 4.

## CLI Ecosystem

- Added man page: `man/gosearch.1`
- Added shell completions:
  - `completions/bash/gosearch.bash`
  - `completions/zsh/_gosearch`
  - `completions/fish/gosearch.fish`
- Added explicit exit code constants for result types in runtime.
- Added config file support via `-config` (default `.gosearchrc`, JSON format).
- Added `-version` flag and version injection support (`main.version`).
- Added completion runtime flag `-completion` with bash/zsh/fish output.

## Packaging

- Added cross-compile and release targets in `Makefile`.
- Added release automation scripts:
  - `scripts/release.sh`
  - `scripts/release.ps1`
- Added checksum generation (`SHA256SUMS.txt` in `dist/`).
- Added ldflags-based version injection in build/release scripts.

## Documentation

- Added architecture diagram doc: `docs/architecture.md`
- Added concurrency diagram doc: `docs/concurrency.md`
- Added performance report template: `docs/performance-report.md`
- Added design tradeoff log: `docs/design-tradeoffs.md`
- Added “Why not X?” doc: `docs/why-not-x.md`

## Files Changed in Stage 4

- `main.go`
- `main_test.go`
- `README.md`
- `Makefile`
- `scripts/release.sh`
- `scripts/release.ps1`
- `man/gosearch.1`
- `completions/bash/gosearch.bash`
- `completions/zsh/_gosearch`
- `completions/fish/gosearch.fish`
- `docs/architecture.md`
- `docs/concurrency.md`
- `docs/performance-report.md`
- `docs/design-tradeoffs.md`
- `docs/why-not-x.md`
- `STAGE_4_CHANGES.md`
