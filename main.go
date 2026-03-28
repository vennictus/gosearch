// Package main is the entry point for gosearch CLI.
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"

	"gosearch/internal/config"
	"gosearch/internal/output"
	"gosearch/internal/search"
)

const (
	exitCodeMatchFound = 0
	exitCodeNoMatches  = 1
	exitCodeUsageError = 2
)

func main() {
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	startTotal := time.Now()
	cfg, err := config.Parse(args)
	if err != nil {
		fmt.Fprintln(stderr, config.UsageText)
		fmt.Fprintln(stderr, err)
		return exitCodeUsageError
	}

	if cfg.ShowVersion {
		fmt.Fprintln(stdout, cfg.VersionLabel)
		return exitCodeMatchFound
	}

	if cfg.CompletionTarget != "" {
		if !config.ValidCompletionTarget(cfg.CompletionTarget) {
			fmt.Fprintln(stderr, config.UsageText)
			fmt.Fprintln(stderr, "completion must be one of: bash, zsh, fish")
			return exitCodeUsageError
		}
		script, _ := config.CompletionScript(cfg.CompletionTarget)
		fmt.Fprint(stdout, script)
		return exitCodeMatchFound
	}

	cleanupProfile, profileErr := setupProfiling(cfg)
	if profileErr != nil {
		fmt.Fprintln(stderr, config.UsageText)
		fmt.Fprintln(stderr, profileErr)
		return exitCodeUsageError
	}
	defer cleanupProfile()

	strategy, err := search.BuildStrategy(cfg.Pattern, cfg.Regex, cfg.IgnoreCase, cfg.WholeWord)
	if err != nil {
		fmt.Fprintln(stderr, config.UsageText)
		fmt.Fprintln(stderr, "invalid regex pattern:", err)
		return exitCodeUsageError
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()

	metrics := &search.Metrics{}
	timings := search.PhaseTimings{}

	tracef(cfg, stderr, "runtime start")

	monitorDone := make(chan struct{})
	if cfg.MonitorGoroutine {
		go monitorGoroutines(ctx, cfg, stderr, monitorDone)
	} else {
		close(monitorDone)
	}

	pathJobs := make(chan string, cfg.Backpressure)
	lineJobs := make(chan search.LineItem, cfg.Backpressure)
	results := make(chan search.Result, cfg.Backpressure)

	printerDone := make(chan output.PrintSummary)
	go output.Printer(ctx, results, stdout, cfg, cancel, printerDone)

	var cpuWG sync.WaitGroup
	startCPUWorker := func() {
		cpuWG.Add(1)
		go search.CPUWorker(ctx, strategy, lineJobs, results, &cpuWG, metrics)
	}

	for i := 0; i < cfg.CPUWorkers; i++ {
		startCPUWorker()
	}

	scaleStop := make(chan struct{})
	scaleDone := make(chan struct{})
	if cfg.DynamicWorkers {
		go search.CPUScaler(ctx, lineJobs, scaleStop, cfg.CPUWorkers, cfg.MaxWorkers, startCPUWorker, metrics, scaleDone)
	} else {
		close(scaleDone)
	}

	var ioWG sync.WaitGroup
	for i := 0; i < cfg.IOWorkers; i++ {
		ioWG.Add(1)
		go search.IOWorker(ctx, cfg, pathJobs, lineJobs, stderr, &ioWG, metrics)
	}

	startWalk := time.Now()
	walkErr := search.WalkFiles(ctx, cfg, pathJobs, stderr, metrics)
	timings.Walk = time.Since(startWalk)
	tracef(cfg, stderr, "phase walk finished in %s", timings.Walk)
	close(pathJobs)

	startScan := time.Now()
	ioWG.Wait()
	close(lineJobs)
	close(scaleStop)
	<-scaleDone

	cpuWG.Wait()
	timings.Scan = time.Since(startScan)
	tracef(cfg, stderr, "phase scan finished in %s", timings.Scan)

	startPrint := time.Now()
	close(results)
	summary := <-printerDone
	timings.Print = time.Since(startPrint)
	timings.Total = time.Since(startTotal)
	tracef(cfg, stderr, "phase print finished in %s", timings.Print)
	<-monitorDone

	if walkErr != nil && !errors.Is(walkErr, context.Canceled) {
		fmt.Fprintln(stderr, walkErr)
		return exitCodeUsageError
	}

	if cfg.Metrics {
		output.PrintMetrics(stderr, metrics)
		output.PrintPhaseTimings(stderr, timings)
	}

	if summary.MatchCount > 0 {
		return exitCodeMatchFound
	}
	return exitCodeNoMatches
}

func setupProfiling(cfg config.Config) (func(), error) {
	cleanup := func() {}

	if cfg.CPUProfilePath == "" && cfg.MemProfilePath == "" {
		return cleanup, nil
	}

	var cpuFile *os.File
	if cfg.CPUProfilePath != "" {
		file, err := os.Create(cfg.CPUProfilePath)
		if err != nil {
			return cleanup, fmt.Errorf("cpuprofile: %w", err)
		}
		if err := pprof.StartCPUProfile(file); err != nil {
			_ = file.Close()
			return cleanup, fmt.Errorf("cpuprofile start: %w", err)
		}
		cpuFile = file
	}

	cleanup = func() {
		if cpuFile != nil {
			pprof.StopCPUProfile()
			_ = cpuFile.Close()
		}
		if cfg.MemProfilePath != "" {
			file, err := os.Create(cfg.MemProfilePath)
			if err == nil {
				_ = pprof.WriteHeapProfile(file)
				_ = file.Close()
			}
		}
	}

	return cleanup, nil
}

func monitorGoroutines(ctx context.Context, cfg config.Config, stderr io.Writer, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(cfg.MonitorInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fmt.Fprintf(stderr, "goroutines count=%d\n", runtime.NumGoroutine())
		}
	}
}

func tracef(cfg config.Config, stderr io.Writer, format string, args ...any) {
	if !cfg.Trace && !cfg.Debug {
		return
	}
	prefix := "debug"
	if cfg.Trace {
		prefix = "trace"
	}
	fmt.Fprintf(stderr, "%s: %s\n", prefix, fmt.Sprintf(format, args...))
}

// Test helper functions - wrappers around search package
func scanFile(path string, pattern string) ([]search.Result, error) {
	return search.ScanFile(path, pattern)
}

func isBinaryFile(path string) (bool, error) {
	return search.IsBinaryFile(path)
}
