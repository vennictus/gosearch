# Changelog

All notable improvements to `gosearch` are documented here.

## v1.0.0 (Project maturity release)

### Added

- Full CLI surface for search behavior, output modes, and worker controls.
- JSON output, quiet/count modes, and color highlighting support.
- Regex matching mode with startup precompilation.
- Config-file defaults via `.gosearchrc` (JSON).
- Shell completion support (bash, zsh, fish) and built-in completion generation.
- `-version` support with ldflags version injection.
- Man page and release automation scripts.
- Benchmarks, fuzz tests, and property-based correctness checks.
- Runtime observability flags: debug, trace, metrics, goroutine monitoring.

### Improved

- Search engine architecture split into traversal, IO, CPU, and output stages.
- Ignore-rule handling with inherited `.gitignore` and `.gosearchignore` semantics.
- Symlink policy with loop prevention.
- Dynamic CPU worker scaling and configurable backpressure.

### Reliability

- Expanded tests for edge cases and integration behavior.
- Stable exit code contract:
  - `0` match found
  - `1` no matches
  - `2` invalid usage/fatal runtime error
- Race-detector clean test runs.

## Notes

This release reflects progression from an MVP search utility to a production-style CLI project suitable for portfolio use.
