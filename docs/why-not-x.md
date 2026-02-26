# Why Not X?

## Why not goroutine-per-file?

Unbounded goroutine creation harms predictability under very large trees.

## Why not regex-only mode?

Substring matching is faster and sufficient for many workflows.

## Why not global ignore cache?

Per-directory inheritance is easier to reason about and keeps rule precedence local.

## Why not parallel printers?

Single printer preserves output integrity without interleaving.

## Why not full daemon/service mode?

This project is intentionally CLI-first with bounded execution and simple operational model.
