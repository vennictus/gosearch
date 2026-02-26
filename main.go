package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
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

	regex          bool
	followSymlinks bool
	maxDepth       int

	dynamicWorkers   bool
	ioWorkers        int
	cpuWorkers       int
	maxWorkers       int
	backpressure     int
	metrics          bool
	debug            bool
	trace            bool
	monitorGoroutine bool
	monitorInterval  time.Duration
	cpuProfilePath   string
	memProfilePath   string

	defaultIgnoreDirs map[string]struct{}
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

type MatchStrategy interface {
	FindRanges(line string) []MatchRange
}

type RegexStrategy struct {
	expression *regexp.Regexp
}

type jsonResult struct {
	Path string `json:"path"`
	Line *int   `json:"line,omitempty"`
	Text string `json:"text"`
}

type lineItem struct {
	Path string
	Line int
	Text string
}

type workerMetrics struct {
	ioWorkersStarted  atomic.Int64
	ioWorkersStopped  atomic.Int64
	cpuWorkersStarted atomic.Int64
	cpuWorkersStopped atomic.Int64
	ioActiveWorkers   atomic.Int64
	cpuActiveWorkers  atomic.Int64
	ioMaxActive       atomic.Int64
	cpuMaxActive      atomic.Int64
	filesEnqueued     atomic.Int64
	filesScanned      atomic.Int64
	linesEnqueued     atomic.Int64
	linesProcessed    atomic.Int64
	matchesProduced   atomic.Int64
	scaleUps          atomic.Int64
}

type phaseTimings struct {
	walk  time.Duration
	scan  time.Duration
	print time.Duration
	total time.Duration
}

type ignoreRule struct {
	baseDir string
	pattern string
	negate  bool
	dirOnly bool
	hasPath bool
}

const usageText = "Usage: gosearch [flags] <pattern> <path>"

func main() {
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	startTotal := time.Now()
	cfg, err := parseConfig(args)
	if err != nil {
		fmt.Fprintln(stderr, usageText)
		fmt.Fprintln(stderr, err)
		return 2
	}

	cleanupProfile, profileErr := setupProfiling(cfg)
	if profileErr != nil {
		fmt.Fprintln(stderr, usageText)
		fmt.Fprintln(stderr, profileErr)
		return 2
	}
	defer cleanupProfile()

	strategy, err := buildStrategy(cfg)
	if err != nil {
		fmt.Fprintln(stderr, usageText)
		fmt.Fprintln(stderr, err)
		return 2
	}

	signalCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithCancel(signalCtx)
	defer cancel()

	metrics := &workerMetrics{}
	timings := phaseTimings{}

	tracef(cfg, stderr, "runtime start")

	monitorDone := make(chan struct{})
	if cfg.monitorGoroutine {
		go monitorGoroutines(ctx, cfg, stderr, monitorDone)
	} else {
		close(monitorDone)
	}

	pathJobs := make(chan string, cfg.backpressure)
	lineJobs := make(chan lineItem, cfg.backpressure)
	results := make(chan Result, cfg.backpressure)

	printerDone := make(chan PrintSummary)
	go printer(ctx, results, stdout, cfg, cancel, printerDone)

	var cpuWG sync.WaitGroup
	startCPUWorker := func() {
		cpuWG.Add(1)
		go cpuWorker(ctx, strategy, lineJobs, results, &cpuWG, metrics)
	}

	for i := 0; i < cfg.cpuWorkers; i++ {
		startCPUWorker()
	}

	scaleStop := make(chan struct{})
	scaleDone := make(chan struct{})
	if cfg.dynamicWorkers {
		go cpuScaler(ctx, lineJobs, scaleStop, cfg, startCPUWorker, metrics, scaleDone)
	} else {
		close(scaleDone)
	}

	var ioWG sync.WaitGroup
	for i := 0; i < cfg.ioWorkers; i++ {
		ioWG.Add(1)
		go ioWorker(ctx, cfg, pathJobs, lineJobs, stderr, &ioWG, metrics)
	}

	startWalk := time.Now()
	walkErr := walkFiles(ctx, cfg, pathJobs, stderr, metrics)
	timings.walk = time.Since(startWalk)
	tracef(cfg, stderr, "phase walk finished in %s", timings.walk)
	close(pathJobs)

	startScan := time.Now()
	ioWG.Wait()
	close(lineJobs)
	close(scaleStop)
	<-scaleDone

	cpuWG.Wait()
	timings.scan = time.Since(startScan)
	tracef(cfg, stderr, "phase scan finished in %s", timings.scan)

	startPrint := time.Now()
	close(results)
	summary := <-printerDone
	timings.print = time.Since(startPrint)
	timings.total = time.Since(startTotal)
	tracef(cfg, stderr, "phase print finished in %s", timings.print)
	<-monitorDone

	if walkErr != nil && !errors.Is(walkErr, context.Canceled) {
		fmt.Fprintln(stderr, walkErr)
		return 2
	}

	if cfg.metrics {
		printMetrics(stderr, metrics)
		printPhaseTimings(stderr, timings)
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
	workers := fs.Int("workers", runtime.NumCPU(), "base worker count")
	maxSize := fs.String("max-size", "", "max file size in bytes, KB, MB, or GB")
	extensions := fs.String("extensions", "", "comma-separated extensions, e.g. .go,.txt")
	excludeDir := fs.String("exclude-dir", "", "comma-separated directory names to skip")
	countOnly := fs.Bool("count", false, "print only total match count")
	quiet := fs.Bool("quiet", false, "suppress output, use exit code only")
	color := fs.Bool("color", false, "enable ANSI color and highlighting in plain output")
	absPath := fs.Bool("abs", false, "print absolute paths")
	outputFormat := fs.String("format", "plain", "output format: plain|json")

	regexMode := fs.Bool("regex", false, "treat pattern as regex")
	followSymlinks := fs.Bool("follow-symlinks", false, "follow symlinked files/directories")
	maxDepth := fs.Int("max-depth", -1, "max traversal depth (-1 for unlimited)")

	dynamicWorkers := fs.Bool("dynamic-workers", false, "dynamically scale CPU workers")
	ioWorkers := fs.Int("io-workers", 0, "number of IO workers (0=auto)")
	cpuWorkers := fs.Int("cpu-workers", 0, "number of CPU workers (0=auto)")
	maxWorkers := fs.Int("max-workers", 0, "max CPU workers when dynamic scaling is enabled (0=auto)")
	backpressure := fs.Int("backpressure", 0, "channel buffer size (0=auto)")
	metrics := fs.Bool("metrics", false, "print worker lifecycle metrics")
	debug := fs.Bool("debug", false, "enable debug logging")
	trace := fs.Bool("trace", false, "enable verbose execution trace")
	monitorGoroutines := fs.Bool("monitor-goroutines", false, "periodically log goroutine count")
	monitorIntervalMs := fs.Int("monitor-interval-ms", 250, "goroutine monitor interval in milliseconds")
	cpuProfile := fs.String("cpuprofile", "", "write CPU profile to file")
	memProfile := fs.String("memprofile", "", "write heap profile to file on exit")

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

	if *maxDepth < -1 {
		return Config{}, errors.New("max-depth must be -1 or greater")
	}

	maxSizeBytes, err := parseSize(*maxSize)
	if err != nil {
		return Config{}, err
	}

	format := strings.ToLower(strings.TrimSpace(*outputFormat))
	if format != "plain" && format != "json" {
		return Config{}, errors.New("format must be plain or json")
	}

	resolvedIOWorkers := *ioWorkers
	if resolvedIOWorkers == 0 {
		resolvedIOWorkers = maxInt(1, *workers/2)
	}
	if resolvedIOWorkers < 1 {
		return Config{}, errors.New("io-workers must be at least 1")
	}

	resolvedCPUWorkers := *cpuWorkers
	if resolvedCPUWorkers == 0 {
		resolvedCPUWorkers = maxInt(1, *workers)
	}
	if resolvedCPUWorkers < 1 {
		return Config{}, errors.New("cpu-workers must be at least 1")
	}

	resolvedMaxWorkers := *maxWorkers
	if resolvedMaxWorkers == 0 {
		resolvedMaxWorkers = maxInt(resolvedCPUWorkers, resolvedCPUWorkers*2)
	}
	if resolvedMaxWorkers < resolvedCPUWorkers {
		return Config{}, errors.New("max-workers must be >= cpu-workers")
	}

	resolvedBackpressure := *backpressure
	if resolvedBackpressure == 0 {
		resolvedBackpressure = maxInt(1, (*workers)*8)
	}
	if resolvedBackpressure < 1 {
		return Config{}, errors.New("backpressure must be at least 1")
	}

	if *monitorIntervalMs < 10 {
		return Config{}, errors.New("monitor-interval-ms must be at least 10")
	}

	excluded := parseCSVSet(*excludeDir, false)
	defaults := map[string]struct{}{
		".git":         {},
		"node_modules": {},
		"vendor":       {},
	}
	for item := range excluded {
		defaults[item] = struct{}{}
	}

	cfg := Config{
		pattern:           pattern,
		rootPath:          rootPath,
		ignoreCase:        *ignoreCase,
		showLineNumbers:   *showLineNumbers,
		wholeWord:         *wholeWord,
		workers:           *workers,
		maxSizeBytes:      maxSizeBytes,
		extensions:        parseCSVSet(*extensions, true),
		excludeDirs:       excluded,
		countOnly:         *countOnly,
		quiet:             *quiet,
		color:             *color,
		absPath:           *absPath,
		outputFormat:      format,
		regex:             *regexMode,
		followSymlinks:    *followSymlinks,
		maxDepth:          *maxDepth,
		dynamicWorkers:    *dynamicWorkers,
		ioWorkers:         resolvedIOWorkers,
		cpuWorkers:        resolvedCPUWorkers,
		maxWorkers:        resolvedMaxWorkers,
		backpressure:      resolvedBackpressure,
		metrics:           *metrics,
		debug:             *debug,
		trace:             *trace,
		monitorGoroutine:  *monitorGoroutines,
		monitorInterval:   time.Duration(*monitorIntervalMs) * time.Millisecond,
		cpuProfilePath:    strings.TrimSpace(*cpuProfile),
		memProfilePath:    strings.TrimSpace(*memProfile),
		defaultIgnoreDirs: defaults,
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

func buildStrategy(cfg Config) (MatchStrategy, error) {
	if !cfg.regex {
		return newMatcher(cfg.pattern, cfg.ignoreCase, cfg.wholeWord), nil
	}

	pattern := cfg.pattern
	if cfg.wholeWord {
		pattern = "\\b(?:" + pattern + ")\\b"
	}
	if cfg.ignoreCase {
		pattern = "(?i)" + pattern
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid regex pattern: %w", err)
	}
	return RegexStrategy{expression: re}, nil
}

func walkFiles(ctx context.Context, cfg Config, jobs chan<- string, stderr io.Writer, metrics *workerMetrics) error {
	visited := make(map[string]struct{})
	rootAbs, _ := filepath.Abs(cfg.rootPath)
	if cfg.followSymlinks {
		if resolved, err := filepath.EvalSymlinks(rootAbs); err == nil {
			visited[resolved] = struct{}{}
		}
	}
	return walkDirectory(ctx, cfg, cfg.rootPath, 0, nil, visited, jobs, stderr, metrics)
}

func walkDirectory(
	ctx context.Context,
	cfg Config,
	currentDir string,
	depth int,
	inheritedRules []ignoreRule,
	visited map[string]struct{},
	jobs chan<- string,
	stderr io.Writer,
	metrics *workerMetrics,
) error {
	if cfg.maxDepth >= 0 && depth > cfg.maxDepth {
		return nil
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	rules, err := loadIgnoreRules(currentDir, inheritedRules)
	if err != nil {
		fmt.Fprintln(stderr, err)
	}

	entries, err := os.ReadDir(currentDir)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return nil
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fullPath := filepath.Join(currentDir, entry.Name())
		entryType := entry.Type()
		isSymlink := entryType&os.ModeSymlink != 0
		isDir := entry.IsDir()

		if shouldIgnorePath(cfg, rules, fullPath, isDir) {
			continue
		}

		if isSymlink {
			if !cfg.followSymlinks {
				continue
			}
			targetInfo, statErr := os.Stat(fullPath)
			if statErr != nil {
				fmt.Fprintln(stderr, statErr)
				continue
			}
			isDir = targetInfo.IsDir()

			if shouldIgnorePath(cfg, rules, fullPath, isDir) {
				continue
			}
		}

		if isDir {
			if _, blocked := cfg.defaultIgnoreDirs[strings.ToLower(entry.Name())]; blocked {
				continue
			}
			if isSymlink {
				resolved, resolveErr := filepath.EvalSymlinks(fullPath)
				if resolveErr != nil {
					fmt.Fprintln(stderr, resolveErr)
					continue
				}
				if _, seen := visited[resolved]; seen {
					continue
				}
				visited[resolved] = struct{}{}
			}
			if err := walkDirectory(ctx, cfg, fullPath, depth+1, rules, visited, jobs, stderr, metrics); err != nil {
				if errors.Is(err, context.Canceled) {
					return err
				}
			}
			continue
		}

		if len(cfg.extensions) > 0 {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if _, ok := cfg.extensions[ext]; !ok {
				continue
			}
		}

		if cfg.maxSizeBytes > 0 {
			entryInfo, infoErr := entry.Info()
			if infoErr != nil {
				fmt.Fprintln(stderr, infoErr)
				continue
			}
			if entryInfo.Size() > cfg.maxSizeBytes {
				continue
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case jobs <- fullPath:
			metrics.filesEnqueued.Add(1)
		}
	}

	return nil
}

func loadIgnoreRules(currentDir string, inherited []ignoreRule) ([]ignoreRule, error) {
	rules := make([]ignoreRule, 0, len(inherited)+8)
	rules = append(rules, inherited...)
	for _, fileName := range []string{".gitignore", ".gosearchignore"} {
		pathToIgnore := filepath.Join(currentDir, fileName)
		file, err := os.Open(pathToIgnore)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return rules, fmt.Errorf("%s: %w", pathToIgnore, err)
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			negate := strings.HasPrefix(line, "!")
			if negate {
				line = strings.TrimSpace(strings.TrimPrefix(line, "!"))
			}
			if line == "" {
				continue
			}

			dirOnly := strings.HasSuffix(line, "/")
			line = strings.TrimSuffix(line, "/")
			if line == "" {
				continue
			}

			rules = append(rules, ignoreRule{
				baseDir: currentDir,
				pattern: line,
				negate:  negate,
				dirOnly: dirOnly,
				hasPath: strings.Contains(line, "/"),
			})
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return rules, fmt.Errorf("%s: %w", pathToIgnore, err)
		}
		_ = file.Close()
	}
	return rules, nil
}

func shouldIgnorePath(cfg Config, rules []ignoreRule, fullPath string, isDir bool) bool {
	name := strings.ToLower(filepath.Base(fullPath))
	if isDir {
		if _, blocked := cfg.defaultIgnoreDirs[name]; blocked {
			return true
		}
	}

	ignored := false
	for _, rule := range rules {
		if rule.dirOnly && !isDir {
			continue
		}

		rel, err := filepath.Rel(rule.baseDir, fullPath)
		if err != nil {
			continue
		}
		relSlash := filepath.ToSlash(rel)
		if relSlash == "." || strings.HasPrefix(relSlash, "../") {
			continue
		}

		if ruleMatch(rule, relSlash) {
			ignored = !rule.negate
		}
	}
	return ignored
}

func ruleMatch(rule ignoreRule, relSlash string) bool {
	patternText := strings.ReplaceAll(rule.pattern, "**", "*")
	if rule.hasPath {
		if globMatch(patternText, relSlash) {
			return true
		}
		prefix := strings.TrimSuffix(patternText, "/") + "/"
		return strings.HasPrefix(relSlash, prefix)
	}

	for _, segment := range strings.Split(relSlash, "/") {
		if globMatch(patternText, segment) {
			return true
		}
	}
	return false
}

func globMatch(patternText string, value string) bool {
	matched, err := path.Match(patternText, value)
	if err != nil {
		return false
	}
	return matched
}

func cpuScaler(
	ctx context.Context,
	lineJobs <-chan lineItem,
	stop <-chan struct{},
	cfg Config,
	spawn func(),
	metrics *workerMetrics,
	done chan<- struct{},
) {
	defer close(done)
	active := cfg.cpuWorkers
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
			if pending > active*2 && active < cfg.maxWorkers {
				spawn()
				active++
				metrics.scaleUps.Add(1)
			}
		}
	}
}

func ioWorker(
	ctx context.Context,
	cfg Config,
	pathJobs <-chan string,
	lineJobs chan<- lineItem,
	stderr io.Writer,
	wg *sync.WaitGroup,
	metrics *workerMetrics,
) {
	metrics.ioWorkersStarted.Add(1)
	defer func() {
		metrics.ioWorkersStopped.Add(1)
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

			metrics.ioActiveWorkers.Add(1)
			updateMaxActive(&metrics.ioMaxActive, metrics.ioActiveWorkers.Load())

			func() {
				defer metrics.ioActiveWorkers.Add(-1)

				if cfg.maxSizeBytes > 0 {
					info, statErr := os.Stat(filePath)
					if statErr != nil {
						fmt.Fprintln(stderr, statErr)
						return
					}
					if info.Size() > cfg.maxSizeBytes {
						return
					}
				}

				binary, err := isBinaryFile(filePath)
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
					case lineJobs <- lineItem{Path: filePath, Line: lineNumber, Text: lineText}:
						metrics.linesEnqueued.Add(1)
					}
				}

				if err := scanner.Err(); err != nil {
					fmt.Fprintln(stderr, fmt.Errorf("%s: %w", filePath, err))
				}
				_ = file.Close()
				metrics.filesScanned.Add(1)
			}()
		}
	}
}

func cpuWorker(
	ctx context.Context,
	strategy MatchStrategy,
	lineJobs <-chan lineItem,
	results chan<- Result,
	wg *sync.WaitGroup,
	metrics *workerMetrics,
) {
	metrics.cpuWorkersStarted.Add(1)
	defer func() {
		metrics.cpuWorkersStopped.Add(1)
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
			metrics.cpuActiveWorkers.Add(1)
			updateMaxActive(&metrics.cpuMaxActive, metrics.cpuActiveWorkers.Load())

			func() {
				defer metrics.cpuActiveWorkers.Add(-1)
				metrics.linesProcessed.Add(1)

				ranges := strategy.FindRanges(item.Text)
				if len(ranges) == 0 {
					return
				}

				result := Result{Path: item.Path, Line: item.Line, Text: item.Text, Ranges: ranges}
				select {
				case <-ctx.Done():
					return
				case results <- result:
					metrics.matchesProduced.Add(1)
				}
			}()
		}
	}
}

func newMatcher(pattern string, ignoreCase bool, wholeWord bool) Matcher {
	matcher := Matcher{pattern: pattern, ignoreCase: ignoreCase, wholeWord: wholeWord}
	if ignoreCase {
		matcher.patternFold = strings.ToLower(pattern)
	}
	return matcher
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

func (strategy RegexStrategy) FindRanges(line string) []MatchRange {
	indices := strategy.expression.FindAllStringIndex(line, -1)
	if len(indices) == 0 {
		return nil
	}
	ranges := make([]MatchRange, 0, len(indices))
	for _, match := range indices {
		ranges = append(ranges, MatchRange{Start: match[0], End: match[1]})
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

func printer(
	ctx context.Context,
	results <-chan Result,
	stdout io.Writer,
	cfg Config,
	cancel context.CancelFunc,
	done chan<- PrintSummary,
) {
	count := 0
	jsonEncoder := json.NewEncoder(stdout)
	cancelledOnce := false

	for {
		select {
		case <-ctx.Done():
			// keep draining until channel is closed to avoid losing in-flight results.
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
			if cfg.quiet {
				if !cfg.countOnly && !cancelledOnce {
					cancel()
					cancelledOnce = true
				}
				continue
			}
			if cfg.countOnly {
				continue
			}

			pathText := formatPath(result.Path, cfg.absPath)
			switch cfg.outputFormat {
			case "json":
				out := jsonResult{Path: pathText, Text: result.Text}
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
					fmt.Fprintf(stdout, "%s:%d: %s\n", pathText, result.Line, text)
				} else {
					fmt.Fprintf(stdout, "%s: %s\n", pathText, text)
				}
			}
		}
	}
}

func finalizePrint(count int, cfg Config, jsonEncoder *json.Encoder, stdout io.Writer) {
	if cfg.countOnly && !cfg.quiet {
		if cfg.outputFormat == "json" {
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

func printMetrics(stderr io.Writer, metrics *workerMetrics) {
	ioLive := metrics.ioWorkersStarted.Load() - metrics.ioWorkersStopped.Load()
	cpuLive := metrics.cpuWorkersStarted.Load() - metrics.cpuWorkersStopped.Load()
	ioIdle := ioLive - metrics.ioActiveWorkers.Load()
	cpuIdle := cpuLive - metrics.cpuActiveWorkers.Load()

	fmt.Fprintf(
		stderr,
		"metrics io(started=%d,stopped=%d,active=%d,idle=%d,max_active=%d) cpu(started=%d,stopped=%d,active=%d,idle=%d,max_active=%d,scaleups=%d) files(enqueued=%d,scanned=%d) lines(enqueued=%d,processed=%d) matches=%d\n",
		metrics.ioWorkersStarted.Load(),
		metrics.ioWorkersStopped.Load(),
		metrics.ioActiveWorkers.Load(),
		maxInt64(0, ioIdle),
		metrics.ioMaxActive.Load(),
		metrics.cpuWorkersStarted.Load(),
		metrics.cpuWorkersStopped.Load(),
		metrics.cpuActiveWorkers.Load(),
		maxInt64(0, cpuIdle),
		metrics.cpuMaxActive.Load(),
		metrics.scaleUps.Load(),
		metrics.filesEnqueued.Load(),
		metrics.filesScanned.Load(),
		metrics.linesEnqueued.Load(),
		metrics.linesProcessed.Load(),
		metrics.matchesProduced.Load(),
	)
}

func printPhaseTimings(stderr io.Writer, timings phaseTimings) {
	fmt.Fprintf(
		stderr,
		"timings walk=%s scan=%s print=%s total=%s\n",
		timings.walk,
		timings.scan,
		timings.print,
		timings.total,
	)
}

func setupProfiling(cfg Config) (func(), error) {
	cleanup := func() {}

	if cfg.cpuProfilePath == "" && cfg.memProfilePath == "" {
		return cleanup, nil
	}

	var cpuFile *os.File
	if cfg.cpuProfilePath != "" {
		file, err := os.Create(cfg.cpuProfilePath)
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
		if cfg.memProfilePath != "" {
			file, err := os.Create(cfg.memProfilePath)
			if err == nil {
				_ = pprof.WriteHeapProfile(file)
				_ = file.Close()
			}
		}
	}

	return cleanup, nil
}

func monitorGoroutines(ctx context.Context, cfg Config, stderr io.Writer, done chan<- struct{}) {
	defer close(done)
	ticker := time.NewTicker(cfg.monitorInterval)
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

func tracef(cfg Config, stderr io.Writer, format string, args ...any) {
	if !cfg.trace && !cfg.debug {
		return
	}
	prefix := "debug"
	if cfg.trace {
		prefix = "trace"
	}
	fmt.Fprintf(stderr, "%s: %s\n", prefix, fmt.Sprintf(format, args...))
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func updateMaxActive(target *atomic.Int64, current int64) {
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
