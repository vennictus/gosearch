package main

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func BenchmarkScannerVsReader(b *testing.B) {
	filePath := createBenchmarkFile(b)
	matcher := newMatcher("needle", false, false)

	b.Run("scanner", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := scanWithScanner(filePath, matcher); err != nil {
				b.Fatalf("scanWithScanner failed: %v", err)
			}
		}
	})

	b.Run("reader", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := scanWithReader(filePath, matcher); err != nil {
				b.Fatalf("scanWithReader failed: %v", err)
			}
		}
	})
}

func BenchmarkWorkerScaling(b *testing.B) {
	root := createBenchmarkDir(b)
	for _, workers := range []int{1, 2, 4, 8} {
		workers := workers
		b.Run("workers_"+strconv.Itoa(workers), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				exitCode := run([]string{"-workers", strconv.Itoa(workers), "needle", root}, ioDiscard{}, ioDiscard{})
				if exitCode != 0 {
					b.Fatalf("expected exit code 0, got %d", exitCode)
				}
			}
		})
	}
}

func BenchmarkLargeDirectoryStress(b *testing.B) {
	root := createBenchmarkDir(b)
	for i := 0; i < b.N; i++ {
		exitCode := run([]string{"-workers", "4", "needle", root}, ioDiscard{}, ioDiscard{})
		if exitCode != 0 {
			b.Fatalf("expected exit code 0, got %d", exitCode)
		}
	}
}

func scanWithScanner(path string, matcher Matcher) (int, error) {
	matches, err := scanFileWithMatcher(path, matcher, 0)
	if err != nil {
		return 0, err
	}
	return len(matches), nil
}

func scanWithReader(path string, matcher Matcher) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	count := 0
	for {
		line, readErr := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimSuffix(line, "\n")
			if len(matcher.FindRanges(line)) > 0 {
				count++
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return 0, readErr
		}
	}
	return count, nil
}

func createBenchmarkFile(tb testing.TB) string {
	tb.Helper()
	dir := tb.TempDir()
	filePath := filepath.Join(dir, "bench.txt")
	var builder strings.Builder
	for i := 0; i < 8000; i++ {
		if i%7 == 0 {
			builder.WriteString("this line has needle token\n")
		} else {
			builder.WriteString("this line has no token\n")
		}
	}
	if err := os.WriteFile(filePath, []byte(builder.String()), 0o644); err != nil {
		tb.Fatalf("failed to write benchmark file: %v", err)
	}
	return filePath
}

func createBenchmarkDir(tb testing.TB) string {
	tb.Helper()
	dir := tb.TempDir()
	for i := 0; i < 80; i++ {
		filePath := filepath.Join(dir, "f_"+strconv.Itoa(i)+".txt")
		var builder strings.Builder
		for line := 0; line < 400; line++ {
			if line%23 == 0 {
				builder.WriteString("needle benchmark line\n")
			} else {
				builder.WriteString("regular benchmark line\n")
			}
		}
		if err := os.WriteFile(filePath, []byte(builder.String()), 0o644); err != nil {
			tb.Fatalf("failed to write benchmark fixture: %v", err)
		}
	}
	return dir
}

type ioDiscard struct{}

func (ioDiscard) Write(data []byte) (int, error) {
	return len(data), nil
}
