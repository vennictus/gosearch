// Package search provides IO and CPU worker implementations.
// The search package implements the core scanning pipeline:
// filesystem traversal → IO workers (file reading) → CPU workers (pattern matching).
// All components are designed for concurrent execution with proper cancellation support.
package search

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/vennictus/gosearch/internal/config"
)

// IOWorker reads files and sends lines to CPU workers.
func IOWorker(
	ctx context.Context,
	cfg config.Config,
	pathJobs <-chan string,
	lineJobs chan<- LineItem,
	stderr io.Writer,
	wg *sync.WaitGroup,
	metrics *Metrics,
) {
	metrics.IOWorkersStarted.Add(1)
	defer func() {
		metrics.IOWorkersStopped.Add(1)
		wg.Done()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case filePath, ok := <-pathJobs:
			if !ok {
				return
			}

			metrics.IOActiveWorkers.Add(1)
			UpdateMaxActive(&metrics.IOMaxActive, metrics.IOActiveWorkers.Load())

			func() {
				defer metrics.IOActiveWorkers.Add(-1)

				if cfg.MaxSizeBytes > 0 {
					info, statErr := os.Stat(filePath)
					if statErr != nil {
						fmt.Fprintln(stderr, statErr)
						return
					}
					if info.Size() > cfg.MaxSizeBytes {
						return
					}
				}

				binary, err := IsBinaryFile(filePath)
				if err != nil {
					fmt.Fprintln(stderr, fmt.Errorf("%s: %w", filePath, err))
					return
				}
				if binary {
					return
				}

				file, err := os.Open(filePath)
				if err != nil {
					fmt.Fprintln(stderr, fmt.Errorf("%s: %w", filePath, err))
					return
				}

				scanner := bufio.NewScanner(file)
				lineNumber := 0
				for scanner.Scan() {
					lineNumber++
					lineText := scanner.Text()

					select {
					case <-ctx.Done():
						_ = file.Close()
						return
					case lineJobs <- LineItem{Path: filePath, Line: lineNumber, Text: lineText}:
						metrics.LinesEnqueued.Add(1)
					}
				}

				if err := scanner.Err(); err != nil {
					fmt.Fprintln(stderr, fmt.Errorf("%s: %w", filePath, err))
				}
				_ = file.Close()
				metrics.FilesScanned.Add(1)
			}()
		}
	}
}

// CPUWorker matches lines against the pattern and sends results.
func CPUWorker(
	ctx context.Context,
	strategy MatchStrategy,
	lineJobs <-chan LineItem,
	results chan<- Result,
	wg *sync.WaitGroup,
	metrics *Metrics,
) {
	metrics.CPUWorkersStarted.Add(1)
	defer func() {
		metrics.CPUWorkersStopped.Add(1)
		wg.Done()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case item, ok := <-lineJobs:
			if !ok {
				return
			}
			metrics.CPUActiveWorkers.Add(1)
			UpdateMaxActive(&metrics.CPUMaxActive, metrics.CPUActiveWorkers.Load())

			func() {
				defer metrics.CPUActiveWorkers.Add(-1)
				metrics.LinesProcessed.Add(1)

				ranges := strategy.FindRanges(item.Text)
				if len(ranges) == 0 {
					return
				}

				result := Result{Path: item.Path, Line: item.Line, Text: item.Text, Ranges: ranges}
				select {
				case <-ctx.Done():
					return
				case results <- result:
					metrics.MatchesProduced.Add(1)
				}
			}()
		}
	}
}

// CPUScaler dynamically scales CPU workers based on queue pressure.
func CPUScaler(
	ctx context.Context,
	lineJobs <-chan LineItem,
	stop <-chan struct{},
	cpuWorkers int,
	maxWorkers int,
	spawn func(),
	metrics *Metrics,
	done chan<- struct{},
) {
	defer close(done)
	active := cpuWorkers
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-stop:
			return
		case <-ticker.C:
			pending := len(lineJobs)
			if pending > active*2 && active < maxWorkers {
				spawn()
				active++
				metrics.ScaleUps.Add(1)
			}
		}
	}
}

// IsBinaryFile checks if a file contains binary content.
func IsBinaryFile(path string) (bool, error) {
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()

	buffer := make([]byte, 512)
	count, readErr := file.Read(buffer)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		return false, readErr
	}

	for _, b := range buffer[:count] {
		if b == 0 {
			return true, nil
		}
	}
	return false, nil
}

// ScanFile is a convenience function for scanning a single file.
func ScanFile(path string, pattern string) ([]Result, error) {
	return ScanFileWithMatcher(path, NewMatcher(pattern, false, false), 0)
}

// ScanFileWithMatcher scans a file with a specific matcher.
func ScanFileWithMatcher(path string, matcher Matcher, maxSizeBytes int64) ([]Result, error) {
	binary, err := IsBinaryFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if binary {
		return nil, nil
	}

	if maxSizeBytes > 0 {
		info, statErr := os.Stat(path)
		if statErr != nil {
			return nil, fmt.Errorf("%s: %w", path, statErr)
		}
		if info.Size() > maxSizeBytes {
			return nil, nil
		}
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	matches := make([]Result, 0)

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		ranges := matcher.FindRanges(line)
		if len(ranges) > 0 {
			matches = append(matches, Result{Path: path, Line: lineNumber, Text: line, Ranges: ranges})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return matches, nil
}
