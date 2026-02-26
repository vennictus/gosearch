# Design Tradeoff Log

## 1) Split IO and CPU Workers

- Chosen for clearer bottleneck isolation and scaling control.
- Tradeoff: more channels and lifecycle complexity.

## 2) Ignore Rules During Traversal

- Chosen to prune early and avoid wasted worker work.
- Tradeoff: more traversal logic and rule-state propagation.

## 3) Strategy Interface for Matching

- Chosen to isolate substring/regex logic from worker pipeline.
- Tradeoff: one more abstraction layer.

## 4) Explicit Backpressure Buffers

- Chosen for predictable memory and flow control.
- Tradeoff: queue tuning required for best performance.

## 5) Runtime Profiling Flags

- Chosen to simplify local diagnostics without external wrappers.
- Tradeoff: optional code paths increase config surface.
