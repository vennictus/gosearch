# Performance Report (Stage 4)

## Environment

- OS: Windows
- CPU: Intel Core Ultra 5 125H
- Go benchmark mode: `go test -run ^$ -bench . -benchmem -count=3 ./...`

## Benchmark Results

### Scanner vs Reader (`BenchmarkScannerVsReader`)

- Scanner latency range: ~586,708 to ~1,090,551 ns/op
- Reader latency range: ~941,451 to ~1,012,973 ns/op
- Scanner allocations: ~469,976 B/op, 9,163 allocs/op
- Reader allocations: ~224,352 B/op, 9,147 allocs/op

Conclusion:

- Reader path materially reduced memory allocation in this workload.
- Latency differences were mixed across runs, but memory profile clearly favored reader mode.

### Worker Scaling (`BenchmarkWorkerScaling`)

- workers=1: ~20.4ms to ~25.9ms per run
- workers=2: ~27.7ms to ~34.5ms per run
- workers=4: ~19.9ms to ~21.8ms per run
- workers=8: ~22.6ms to ~28.8ms per run

Conclusion:

- Best observed range was at 4 workers for this benchmark fixture.
- Over-scaling to 8 workers did not improve consistently.

### Large Directory Stress (`BenchmarkLargeDirectoryStress`)

- Latency range: ~20.2ms to ~23.7ms per run
- Allocation range: ~1,513,286 to ~1,524,581 B/op
- Allocations: ~37,950 allocs/op

Conclusion:

- Stress benchmark remained stable across three passes.
- No pathological blow-up observed for the synthetic large fixture.

## Runtime Profiling Capture

Command used:

```powershell
.\gosearch.exe -cpuprofile cpu-stage4.out -memprofile mem-stage4.out needle .\testdata\small
```

Artifacts produced in project root:

- `cpu-stage4.out`
- `mem-stage4.out`

## Key Takeaways

- Worker count should be tuned per workload instead of maximized blindly.
- Allocation pressure is an important optimization axis alongside raw latency.
- Runtime profile artifacts are now part of reproducible diagnostics for regressions.
