// Package search provides worker and metrics types.
package search

import (
	"sync/atomic"
	"time"
)

// LineItem represents a line to be processed by CPU workers.
type LineItem struct {
	Path string
	Line int
	Text string
}

// Metrics tracks worker lifecycle and throughput metrics.
type Metrics struct {
	IOWorkersStarted  atomic.Int64
	IOWorkersStopped  atomic.Int64
	CPUWorkersStarted atomic.Int64
	CPUWorkersStopped atomic.Int64
	IOActiveWorkers   atomic.Int64
	CPUActiveWorkers  atomic.Int64
	IOMaxActive       atomic.Int64
	CPUMaxActive      atomic.Int64
	FilesEnqueued     atomic.Int64
	FilesScanned      atomic.Int64
	LinesEnqueued     atomic.Int64
	LinesProcessed    atomic.Int64
	MatchesProduced   atomic.Int64
	ScaleUps          atomic.Int64
}

// PhaseTimings tracks timing for each phase of the search.
type PhaseTimings struct {
	Walk  time.Duration
	Scan  time.Duration
	Print time.Duration
	Total time.Duration
}

// UpdateMaxActive atomically updates the max active counter.
func UpdateMaxActive(target *atomic.Int64, current int64) {
	for {
		existing := target.Load()
		if current <= existing {
			return
		}
		if target.CompareAndSwap(existing, current) {
			return
		}
	}
}

// MaxInt64 returns the maximum of two int64 values.
func MaxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
