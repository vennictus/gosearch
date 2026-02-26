# gosearch

```text
   ____  ____  _________  _____  __________  ________
  / __ \/ __ \/ ___/ __ \/ ___/ / ____/ __ \/ ____/ /
 / /_/ / / / /\__ \/ /_/ /\__ \ / __/ / /_/ / /   / / 
/ _, _/ /_/ /___/ / ____/___/ // /___/ _, _/ /___/ /___
/_/ |_|\____//____/_/    /____//_____/_/ |_|\____/_____/
```

Fast, concurrent recursive search for codebases and text-heavy directories.

## ✨ Why it’s useful

- ⚡ Searches large trees quickly with bounded concurrency.
- 🧠 Supports substring and regex matching.
- 🧹 Honors `.gitignore` and `.gosearchignore` rules.
- 🛡️ Handles cancellation, symlink safety, and binary-file skipping.
- 🔍 Offers plain output, JSON output, quiet/count modes, and diagnostics.

## 🚀 Quick Start

Build and run:

```bash
go build -o gosearch .
./gosearch "needle" ./testdata/small
```

Common commands:

```bash
# case-insensitive
./gosearch -i "todo" .

# regex search
./gosearch -regex "func\\s+main" .

# JSON output
./gosearch -format json "error" .

# only count matches
./gosearch -count "needle" ./testdata
```

Command shape:

```text
gosearch [flags] <pattern> <path>
```

## 🧩 Core Features

- Concurrent pipeline with tunable worker controls.
- Ignore-rule inheritance with negation support.
- Optional symlink following with loop prevention.
- Config defaults from `.gosearchrc` (JSON).
- Metrics, debug/trace logging, and profile output flags.
- Shell completions and man page support.

## 🏁 Output & Exit Codes

Plain output (default):

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
- `2` invalid usage or fatal runtime/setup error

## ⚙️ Configuration

`gosearch` can load defaults from `.gosearchrc`:

```json
{
  "ignore_case": true,
  "workers": 8,
  "format": "json"
}
```

CLI flags always override config values.

## 🧪 Testing

One-command Windows validation:

```powershell
./scripts/test.ps1
```

Optional fuzz mode:

```powershell
./scripts/test.ps1 -IncludeFuzz
```

Manual test commands:

```bash
go test -count=1 ./...
go test -count=1 -race ./...
go test -bench=. -benchmem ./...
```

## 📦 Release

```bash
make cross VERSION=vX.Y.Z
make release VERSION=vX.Y.Z
```

Version injection example:

```bash
go build -ldflags "-X main.version=vX.Y.Z" -o gosearch .
```

Release helpers:

- `scripts/release.sh`
- `scripts/release.ps1`

## 🧭 Documentation

- Project design: `DESIGN.md`
- Project changelog: `CHANGELOG.md`
- Architecture notes: `docs/architecture.md`
- Concurrency notes: `docs/concurrency.md`
- Performance report: `docs/performance-report.md`
- Tradeoffs: `docs/design-tradeoffs.md`
- Why-not-X: `docs/why-not-x.md`

## 📚 CLI Extras

- Man page: `man/gosearch.1`
- Completions:
  - `completions/bash/gosearch.bash`
  - `completions/zsh/_gosearch`
  - `completions/fish/gosearch.fish`

Generate completion scripts dynamically:

```bash
gosearch -completion bash
gosearch -completion zsh
gosearch -completion fish
```

## ⚠️ Known limitations

- Output order is not deterministic (concurrent streaming).
- SIGINT test behavior differs on Windows.
