package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type Result struct {
	Path string
	Line int
	Text string
}

func main() {
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) != 2 || strings.TrimSpace(args[0]) == "" || strings.TrimSpace(args[1]) == "" {
		fmt.Fprintln(stderr, "Usage: gosearch <pattern> <path>")
		return 1
	}

	pattern := args[0]
	rootPath := args[1]

	info, err := os.Stat(rootPath)
	if err != nil || !info.IsDir() {
		fmt.Fprintln(stderr, "Usage: gosearch <pattern> <path>")
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	jobs := make(chan string)
	results := make(chan Result)

	var workers sync.WaitGroup
	workerCount := runtime.NumCPU()
	for i := 0; i < workerCount; i++ {
		workers.Add(1)
		go worker(ctx, pattern, jobs, results, stderr, &workers)
	}

	printerDone := make(chan struct{})
	go printer(results, stdout, printerDone)

	walkErr := walkFiles(ctx, rootPath, jobs, stderr)
	close(jobs)

	workers.Wait()
	close(results)
	<-printerDone

	if walkErr != nil && !errors.Is(walkErr, context.Canceled) {
		fmt.Fprintln(stderr, walkErr)
		return 1
	}

	return 0
}

func walkFiles(ctx context.Context, rootPath string, jobs chan<- string, stderr io.Writer) error {
	return filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			fmt.Fprintln(stderr, walkErr)
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			return nil
		}

		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case jobs <- path:
			return nil
		}
	})
}

func worker(ctx context.Context, pattern string, jobs <-chan string, results chan<- Result, stderr io.Writer, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case path, ok := <-jobs:
			if !ok {
				return
			}

			matches, err := scanFile(path, pattern)
			if err != nil {
				fmt.Fprintln(stderr, err)
				continue
			}

			for _, match := range matches {
				select {
				case <-ctx.Done():
					return
				case results <- match:
				}
			}
		}
	}
}

func scanFile(path string, pattern string) ([]Result, error) {
	binary, err := isBinaryFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if binary {
		return nil, nil
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
		if strings.Contains(line, pattern) {
			matches = append(matches, Result{Path: path, Line: lineNumber, Text: line})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	return matches, nil
}

func isBinaryFile(path string) (bool, error) {
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

func printer(results <-chan Result, stdout io.Writer, done chan<- struct{}) {
	defer close(done)
	for result := range results {
		fmt.Fprintf(stdout, "%s:%d: %s\n", result.Path, result.Line, result.Text)
	}
}
