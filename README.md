# gosearch

```text
   ____  ____  _________  _____  __________  ________
  / __ \/ __ \/ ___/ __ \/ ___/ / ____/ __ \/ ____/ /
 / /_/ / / / /\__ \/ /_/ /\__ \ / __/ / /_/ / /   / /
/ _, _/ /_/ /___/ / ____/___/ // /___/ _, _/ /___/ /___
/_/ |_|\____//____/_/    /____//_____/_/ |_|\____/_____/
```

`gosearch` is a high-throughput, concurrency-first CLI search engine for real repositories.

If standard grep-style workflows feel too bare and heavy codebases need more control, this is the upgrade path: recursive traversal, ignore-rule awareness, structured output, deterministic exit semantics, and deep observability when you need to tune performance.

## What You Built

- Recursive search with streaming output.
- Concurrency pipeline split into traversal, IO, CPU matching, and single-owner printing.
- Substring and regex strategies with startup regex precompilation.
- Ignore-aware traversal via `.gitignore` and `.gosearchignore` (with inheritance and negation).
- Symlink controls with loop prevention.
- Production-grade CLI surface: filters, output modes, metrics, tracing, profiling, config defaults.
- Full engineering package: tests, race checks, fuzzing, benchmarks, completions, man page, release scripts.

## Fast Start

```bash
go build -o gosearch .
./gosearch "needle" ./testdata/small
```

Command contract:

```text
gosearch [flags] <pattern> <path>
```

## Practical Commands

```bash
# case-insensitive search
./gosearch -i "todo" .

# whole-word exact token matching
./gosearch -w "Config" .

# regex mode
./gosearch -regex "func\\s+main" .

# JSON output for tooling/pipelines
./gosearch -format json "error" .

# count-only mode (fast reporting)
./gosearch -count "needle" ./testdata

# quiet mode for scripting via exit code only
./gosearch -quiet "needle" ./testdata

# restrict by extension and max file size
./gosearch -extensions .go,.md -max-size 2MB "worker" .

# follow symlinks up to bounded depth
./gosearch -follow-symlinks -max-depth 6 "TODO" .
```

## Full Flag Reference

### Search behavior

- `-i` case-insensitive search
- `-n` show line numbers (default `true`, set `-n=false` to disable)
- `-w` whole-word matching
- `-regex` treat pattern as regex

### Input scope and filtering

- `-extensions .go,.txt` include only listed extensions
- `-exclude-dir vendor,node_modules` skip named directories
- `-max-size 10MB` skip files larger than threshold (bytes/KB/MB/GB)
- `-max-depth N` cap traversal depth (`-1` = unlimited)
- `-follow-symlinks` include symlinked files/dirs with loop prevention

### Output modes

- `-format plain|json` choose output format
- `-count` print total match count only
- `-quiet` suppress output; rely on exit code
- `-color` enable ANSI highlight in plain mode
- `-abs` print absolute paths

### Worker and throughput controls

- `-workers N` base worker count
- `-io-workers N` IO workers (`0` = auto)
- `-cpu-workers N` CPU workers (`0` = auto)
- `-dynamic-workers` enable dynamic CPU scaling
- `-max-workers N` cap dynamic CPU workers (`0` = auto)
- `-backpressure N` channel buffer size (`0` = auto)

### Diagnostics and profiling

- `-metrics` print worker lifecycle and throughput metrics
- `-debug` debug logging
- `-trace` verbose trace logging
- `-monitor-goroutines` periodic goroutine count logging
- `-monitor-interval-ms N` monitor interval (minimum `10`, default `250`)
- `-cpuprofile file.out` write CPU profile
- `-memprofile file.out` write heap profile on exit

### Config and CLI metadata

- `-config path/to/.gosearchrc` load JSON defaults
- `-completion bash|zsh|fish` print completion script
- `-version` print build version

## Output Contract

Plain output (`-format plain`, default):

```text
path/to/file:line_number: line_text
```

JSON output (`-format json`):

```json
{"path":"...","line":12,"text":"..."}
```

Exit codes:

- `0` one or more matches found
- `1` no matches found
- `2` invalid usage or fatal setup/runtime error

## Ignore and Symlink Semantics

- Traversal parses `.gitignore` and `.gosearchignore`.
- Ignore rules are inherited by child directories.
- Negation patterns (`!pattern`) can re-include paths at deeper levels.
- Default ignored directories include `.git`, `vendor`, and `node_modules`.
- Ignore pruning happens before work enqueue, so ignored paths never hit workers.
- Symlinks are skipped unless `-follow-symlinks` is enabled.
- Directory symlink loops are blocked using resolved-path tracking.

## Config File

Default path is `.gosearchrc`.

Example:

```json
{
  "ignore_case": true,
  "workers": 8,
  "format": "json",
  "dynamic_workers": true
}
```

Precedence rule: CLI flags always override config values.

## Architecture Snapshot

```text
walk filesystem
  -> path jobs
    -> IO workers (read + binary detection)
      -> line jobs
        -> CPU workers (match strategy)
          -> result channel
            -> single printer goroutine
```

Design priorities:

- no output interleaving
- bounded concurrency
- deterministic shutdown
- cancellation propagation via context

## Testing and Validation

Windows one-command validation:

```powershell
./scripts/test.ps1
```

Optional fuzz mode:

```powershell
./scripts/test.ps1 -IncludeFuzz
```

Cross-platform manual commands:

```bash
go test -count=1 ./...
go test -count=1 -race ./...
go test -bench=. -benchmem ./...
go test -fuzz=Fuzz -run=^$ ./...
```

## Release Workflow

```bash
make cross VERSION=vX.Y.Z
make release VERSION=vX.Y.Z
```

Version injection:

```bash
go build -ldflags "-X main.version=vX.Y.Z" -o gosearch .
```

Release helpers:

- `scripts/release.sh`
- `scripts/release.ps1`

## Documentation Map

- `DESIGN.md`
- `CHANGELOG.md`
- `docs/architecture.md`
- `docs/concurrency.md`
- `docs/performance-report.md`
- `docs/design-tradeoffs.md`
- `docs/why-not-x.md`

## Completions and Man Page

- Man page: `man/gosearch.1`
- Completion assets:
  - `completions/bash/gosearch.bash`
  - `completions/zsh/_gosearch`
  - `completions/fish/gosearch.fish`

Generate from CLI:

```bash
gosearch -completion bash
gosearch -completion zsh
gosearch -completion fish
```

## Known Limitations

- Output order is intentionally non-deterministic under concurrency.
- SIGINT behavior in tests differs on Windows compared to Unix-like environments.
