package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
)

type Result struct {
	Path   string
	Line   int
	Text   string
	Ranges []MatchRange
}

type MatchRange struct {
	Start int
	End   int
}

type Config struct {
	pattern         string
	rootPath        string
	ignoreCase      bool
	showLineNumbers bool
	wholeWord       bool
	workers         int
	maxSizeBytes    int64
	extensions      map[string]struct{}
	excludeDirs     map[string]struct{}
	countOnly       bool
	quiet           bool
	color           bool
	absPath         bool
	outputFormat    string
}

type PrintSummary struct {
	MatchCount int
}

type Matcher struct {
	pattern     string
	patternFold string
	ignoreCase  bool
	wholeWord   bool
}

type jsonResult struct {
	Path string `json:"path"`
	Line *int   `json:"line,omitempty"`
	Text string `json:"text"`
}

const usageText = "Usage: gosearch [flags] <pattern> <path>"

func main() {
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	cfg, err := parseConfig(args)
	if err != nil {
		fmt.Fprintln(stderr, usageText)
		fmt.Fprintln(stderr, err)
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	jobs := make(chan string)
	results := make(chan Result)
	matcher := newMatcher(cfg.pattern, cfg.ignoreCase, cfg.wholeWord)

	var workers sync.WaitGroup
	for i := 0; i < cfg.workers; i++ {
		workers.Add(1)
		go worker(ctx, matcher, cfg.maxSizeBytes, jobs, results, stderr, &workers)
	}

	printerDone := make(chan PrintSummary)
	go printer(results, stdout, cfg, printerDone)

	walkErr := walkFiles(ctx, cfg, jobs, stderr)
	close(jobs)

	workers.Wait()
	close(results)
	summary := <-printerDone

	if walkErr != nil && !errors.Is(walkErr, context.Canceled) {
		fmt.Fprintln(stderr, walkErr)
		return 2
	}

	if summary.MatchCount > 0 {
		return 0
	}

	return 1
}

func parseConfig(args []string) (Config, error) {
	fs := flag.NewFlagSet("gosearch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	ignoreCase := fs.Bool("i", false, "case-insensitive search")
	showLineNumbers := fs.Bool("n", true, "show line numbers")
	wholeWord := fs.Bool("w", false, "whole-word matching")
	workers := fs.Int("workers", runtime.NumCPU(), "worker count")
	maxSize := fs.String("max-size", "", "max file size in bytes, KB, MB, or GB")
	extensions := fs.String("extensions", "", "comma-separated extensions, e.g. .go,.txt")
	excludeDir := fs.String("exclude-dir", "", "comma-separated directory names to skip")
	countOnly := fs.Bool("count", false, "print only total match count")
	quiet := fs.Bool("quiet", false, "suppress output, use exit code only")
	color := fs.Bool("color", false, "enable ANSI color and highlighting in plain output")
	absPath := fs.Bool("abs", false, "print absolute paths")
	outputFormat := fs.String("format", "plain", "output format: plain|json")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	remaining := fs.Args()
	if len(remaining) != 2 {
		return Config{}, errors.New("expected <pattern> and <path>")
	}

	pattern := strings.TrimSpace(remaining[0])
	rootPath := strings.TrimSpace(remaining[1])
	if pattern == "" || rootPath == "" {
		return Config{}, errors.New("pattern and path must be non-empty")
	}

	info, err := os.Stat(rootPath)
	if err != nil || !info.IsDir() {
		return Config{}, errors.New("path must be a readable directory")
	}

	if *workers < 1 {
		return Config{}, errors.New("workers must be at least 1")
	}

	maxSizeBytes, err := parseSize(*maxSize)
	if err != nil {
		return Config{}, err
	}

	format := strings.ToLower(strings.TrimSpace(*outputFormat))
	if format != "plain" && format != "json" {
		return Config{}, errors.New("format must be plain or json")
	}

	cfg := Config{
		pattern:         pattern,
		rootPath:        rootPath,
		ignoreCase:      *ignoreCase,
		showLineNumbers: *showLineNumbers,
		wholeWord:       *wholeWord,
		workers:         *workers,
		maxSizeBytes:    maxSizeBytes,
		extensions:      parseCSVSet(*extensions, true),
		excludeDirs:     parseCSVSet(*excludeDir, false),
		countOnly:       *countOnly,
		quiet:           *quiet,
		color:           *color,
		absPath:         *absPath,
		outputFormat:    format,
	}

	return cfg, nil
}

func parseSize(input string) (int64, error) {
	text := strings.TrimSpace(strings.ToUpper(input))
	if text == "" {
		return 0, nil
	}

	multiplier := int64(1)
	for _, suffix := range []struct {
		Token string
		Scale int64
	}{
		{Token: "GB", Scale: 1024 * 1024 * 1024},
		{Token: "MB", Scale: 1024 * 1024},
		{Token: "KB", Scale: 1024},
		{Token: "B", Scale: 1},
	} {
		if strings.HasSuffix(text, suffix.Token) {
			text = strings.TrimSpace(strings.TrimSuffix(text, suffix.Token))
			multiplier = suffix.Scale
			break
		}
	}

	value, err := strconv.ParseInt(text, 10, 64)
	if err != nil || value < 0 {
		return 0, errors.New("invalid -max-size value")
	}

	return value * multiplier, nil
}

func parseCSVSet(input string, normalizeExtension bool) map[string]struct{} {
	result := make(map[string]struct{})
	for _, item := range strings.Split(input, ",") {
		trimmed := strings.TrimSpace(strings.ToLower(item))
		if trimmed == "" {
			continue
		}
		if normalizeExtension && !strings.HasPrefix(trimmed, ".") {
			trimmed = "." + trimmed
		}
		result[trimmed] = struct{}{}
	}
	return result
}

func walkFiles(ctx context.Context, cfg Config, jobs chan<- string, stderr io.Writer) error {
	return filepath.WalkDir(cfg.rootPath, func(path string, d fs.DirEntry, walkErr error) error {
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
			if path != cfg.rootPath {
				if _, blocked := cfg.excludeDirs[strings.ToLower(d.Name())]; blocked {
					return fs.SkipDir
				}
			}
			return nil
		}

		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}

		if len(cfg.extensions) > 0 {
			ext := strings.ToLower(filepath.Ext(d.Name()))
			if _, ok := cfg.extensions[ext]; !ok {
				return nil
			}
		}

		if cfg.maxSizeBytes > 0 {
			info, infoErr := d.Info()
			if infoErr != nil {
				fmt.Fprintln(stderr, infoErr)
				return nil
			}
			if info.Size() > cfg.maxSizeBytes {
				return nil
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case jobs <- path:
			return nil
		}
	})
}

func newMatcher(pattern string, ignoreCase bool, wholeWord bool) Matcher {
	matcher := Matcher{pattern: pattern, ignoreCase: ignoreCase, wholeWord: wholeWord}
	if ignoreCase {
		matcher.patternFold = strings.ToLower(pattern)
	}
	return matcher
}

func worker(ctx context.Context, matcher Matcher, maxSizeBytes int64, jobs <-chan string, results chan<- Result, stderr io.Writer, wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-ctx.Done():
			return
		case path, ok := <-jobs:
			if !ok {
				return
			}

			matches, err := scanFileWithMatcher(path, matcher, maxSizeBytes)
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
	return scanFileWithMatcher(path, newMatcher(pattern, false, false), 0)
}

func scanFileWithMatcher(path string, matcher Matcher, maxSizeBytes int64) ([]Result, error) {
	binary, err := isBinaryFile(path)
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

func (matcher Matcher) FindRanges(line string) []MatchRange {
	needle := matcher.pattern
	haystack := line
	if matcher.ignoreCase {
		needle = matcher.patternFold
		haystack = strings.ToLower(line)
	}

	if needle == "" {
		return nil
	}

	ranges := make([]MatchRange, 0)
	searchFrom := 0
	for {
		index := strings.Index(haystack[searchFrom:], needle)
		if index < 0 {
			break
		}

		start := searchFrom + index
		end := start + len(needle)
		if !matcher.wholeWord || isWholeWordMatch(line, start, end) {
			ranges = append(ranges, MatchRange{Start: start, End: end})
			searchFrom = end
			continue
		}

		searchFrom = start + 1
	}

	return ranges
}

func isWholeWordMatch(line string, start int, end int) bool {
	leftBoundary := start == 0 || !isWordByte(line[start-1])
	rightBoundary := end == len(line) || !isWordByte(line[end])
	return leftBoundary && rightBoundary
}

func isWordByte(value byte) bool {
	return (value >= 'a' && value <= 'z') ||
		(value >= 'A' && value <= 'Z') ||
		(value >= '0' && value <= '9') ||
		value == '_'
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

func printer(results <-chan Result, stdout io.Writer, cfg Config, done chan<- PrintSummary) {
	count := 0
	jsonEncoder := json.NewEncoder(stdout)

	for result := range results {
		count++
		if cfg.quiet || cfg.countOnly {
			continue
		}

		path := formatPath(result.Path, cfg.absPath)
		switch cfg.outputFormat {
		case "json":
			out := jsonResult{Path: path, Text: result.Text}
			if cfg.showLineNumbers {
				line := result.Line
				out.Line = &line
			}
			_ = jsonEncoder.Encode(out)
		default:
			text := result.Text
			if cfg.color {
				text = highlightRanges(text, result.Ranges)
			}
			if cfg.showLineNumbers {
				fmt.Fprintf(stdout, "%s:%d: %s\n", path, result.Line, text)
			} else {
				fmt.Fprintf(stdout, "%s: %s\n", path, text)
			}
		}
	}

	if cfg.countOnly && !cfg.quiet {
		if cfg.outputFormat == "json" {
			_ = jsonEncoder.Encode(map[string]int{"count": count})
		} else {
			fmt.Fprintln(stdout, count)
		}
	}

	done <- PrintSummary{MatchCount: count}
	close(done)
}

func formatPath(path string, absolute bool) string {
	if !absolute {
		return path
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func highlightRanges(line string, ranges []MatchRange) string {
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
