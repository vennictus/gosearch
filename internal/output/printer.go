// Package output handles result printing and formatting.
package output

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/vennictus/gosearch/internal/config"
	"github.com/vennictus/gosearch/internal/search"
)

// PrintSummary contains the final match count.
type PrintSummary struct {
	MatchCount int
}

type jsonResult struct {
	Path string `json:"path"`
	Line *int   `json:"line,omitempty"`
	Text string `json:"text"`
}

// Printer reads results and prints them to stdout.
func Printer(
	ctx context.Context,
	results <-chan search.Result,
	stdout io.Writer,
	cfg config.Config,
	cancel context.CancelFunc,
	done chan<- PrintSummary,
) {
	count := 0
	jsonEncoder := json.NewEncoder(stdout)
	cancelledOnce := false

	for {
		select {
		case <-ctx.Done():
			for result := range results {
				count++
				_ = result
			}
			finalizePrint(count, cfg, jsonEncoder, stdout)
			done <- PrintSummary{MatchCount: count}
			close(done)
			return
		case result, ok := <-results:
			if !ok {
				finalizePrint(count, cfg, jsonEncoder, stdout)
				done <- PrintSummary{MatchCount: count}
				close(done)
				return
			}

			count++
			if cfg.Quiet {
				if !cfg.CountOnly && !cancelledOnce {
					cancel()
					cancelledOnce = true
				}
				continue
			}
			if cfg.CountOnly {
				continue
			}

			pathText := formatPath(result.Path, cfg.AbsPath)
			switch cfg.OutputFormat {
			case "json":
				out := jsonResult{Path: pathText, Text: result.Text}
				if cfg.ShowLineNumbers {
					line := result.Line
					out.Line = &line
				}
				_ = jsonEncoder.Encode(out)
			default:
				text := result.Text
				if cfg.Color {
					text = highlightRanges(text, result.Ranges)
				}
				if cfg.ShowLineNumbers {
					fmt.Fprintf(stdout, "%s:%d: %s\n", pathText, result.Line, text)
				} else {
					fmt.Fprintf(stdout, "%s: %s\n", pathText, text)
				}
			}
		}
	}
}

func finalizePrint(count int, cfg config.Config, jsonEncoder *json.Encoder, stdout io.Writer) {
	if cfg.CountOnly && !cfg.Quiet {
		if cfg.OutputFormat == "json" {
			_ = jsonEncoder.Encode(map[string]int{"count": count})
		} else {
			fmt.Fprintln(stdout, count)
		}
	}
}

func formatPath(pathText string, absolute bool) string {
	if !absolute {
		return pathText
	}
	abs, err := filepath.Abs(pathText)
	if err != nil {
		return pathText
	}
	return abs
}

func highlightRanges(line string, ranges []search.MatchRange) string {
	if len(ranges) == 0 {
		return line
	}

	var builder strings.Builder
	last := 0
	for _, match := range ranges {
		if match.Start < last || match.Start > len(line) || match.End > len(line) {
			continue
		}
		builder.WriteString(line[last:match.Start])
		builder.WriteString("\x1b[31m")
		builder.WriteString(line[match.Start:match.End])
		builder.WriteString("\x1b[0m")
		last = match.End
	}
	builder.WriteString(line[last:])
	return builder.String()
}

// PrintMetrics prints worker lifecycle metrics.
func PrintMetrics(stderr io.Writer, metrics *search.Metrics) {
	ioLive := metrics.IOWorkersStarted.Load() - metrics.IOWorkersStopped.Load()
	cpuLive := metrics.CPUWorkersStarted.Load() - metrics.CPUWorkersStopped.Load()
	ioIdle := ioLive - metrics.IOActiveWorkers.Load()
	cpuIdle := cpuLive - metrics.CPUActiveWorkers.Load()

	fmt.Fprintf(
		stderr,
		"metrics io(started=%d,stopped=%d,active=%d,idle=%d,max_active=%d) cpu(started=%d,stopped=%d,active=%d,idle=%d,max_active=%d,scaleups=%d) files(enqueued=%d,scanned=%d) lines(enqueued=%d,processed=%d) matches=%d\n",
		metrics.IOWorkersStarted.Load(),
		metrics.IOWorkersStopped.Load(),
		metrics.IOActiveWorkers.Load(),
		search.MaxInt64(0, ioIdle),
		metrics.IOMaxActive.Load(),
		metrics.CPUWorkersStarted.Load(),
		metrics.CPUWorkersStopped.Load(),
		metrics.CPUActiveWorkers.Load(),
		search.MaxInt64(0, cpuIdle),
		metrics.CPUMaxActive.Load(),
		metrics.ScaleUps.Load(),
		metrics.FilesEnqueued.Load(),
		metrics.FilesScanned.Load(),
		metrics.LinesEnqueued.Load(),
		metrics.LinesProcessed.Load(),
		metrics.MatchesProduced.Load(),
	)
}

// PrintPhaseTimings prints timing information for each phase.
func PrintPhaseTimings(stderr io.Writer, timings search.PhaseTimings) {
	fmt.Fprintf(
		stderr,
		"timings walk=%s scan=%s print=%s total=%s\n",
		timings.Walk,
		timings.Scan,
		timings.Print,
		timings.Total,
	)
}
