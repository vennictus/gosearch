# Stage 1 of 5 — Tier 1 Polished Tool

This document records the additions and changes made in Stage 1.

## Scope Implemented

### CLI & UX

- Added flags using Go `flag` package:
  - `-i` case-insensitive search
  - `-n` line number toggle (default: `true`, can disable with `-n=false`)
  - `-w` whole-word matching
  - `-workers N` configurable worker pool size
  - `-max-size` file size limit (supports raw bytes and `KB`, `MB`, `GB` suffixes)
  - `-extensions .go,.txt` include-only extension filter
  - `-exclude-dir vendor,node_modules` directory exclude list
  - `-count` print only total match count
  - `-quiet` suppress all output and rely on exit code

### Output

- Added ANSI color highlighting support in plain output via `-color`.
- Added matched-substring highlighting in plain output.
- Added optional absolute paths via `-abs` (default remains relative/as-walked path).
- Added format switch via `-format plain|json`.

## Behavioral Changes

- CLI usage is now:

```text
gosearch [flags] <pattern> <path>
```

- Exit code behavior:
  - `0` when at least one match is found
  - `1` when no matches are found
  - `2` for invalid usage or fatal runtime setup errors

## Internal Implementation Notes

- Introduced `Config` struct for parsed runtime options.
- Introduced `Matcher` with:
  - case-insensitive matching support
  - whole-word boundary checking
  - substring range capture for highlighting
- Kept worker-pool concurrency model and single printer goroutine.
- Added traversal-time filters for extensions, excluded directories, and max file size.
- Added printer support for:
  - plain output
  - JSON output
  - count-only mode
  - quiet mode

## Tests Added/Updated

- Updated invalid-args usage assertion for new usage string.
- Added tests for:
  - case-insensitive flag behavior
  - whole-word matching behavior
  - count-only output
  - JSON output format
  - quiet mode exit-code semantics

## Files Changed in Stage 1

- `main.go`
- `main_test.go`
- `README.md`
- `STAGE_1_CHANGES.md`

## Stage-1 Closure Checklist (Completed)

All readiness items identified at the end of Stage 1 have been completed:

- README synced with actual CLI behavior:
  - Update usage to `gosearch [flags] <pattern> <path>`.
  - Document Stage-1 flags accurately.
  - Document exit code semantics (`0` match found, `1` no matches, `2` usage/fatal setup error).
  - Remove outdated limitations that conflict with Stage-1 implementation (for example, no flags/options, no colorized output).
- Expanded Stage-1 test coverage for previously missing implemented flags:
  - `-workers`
  - `-max-size`
  - `-extensions`
  - `-exclude-dir`
  - `-n`
  - `-abs`
  - `-color`
- Tightened invalid-argument test assertion to require exact exit code `2` (not only non-zero).

Validation status:

- `go test ./...` passing
- `go test -race ./...` passing
