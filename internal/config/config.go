// Package config handles CLI configuration parsing and loading.
package config

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for gosearch.
type Config struct {
	ConfigPath       string
	ShowVersion      bool
	CompletionTarget string
	VersionLabel     string

	Pattern         string
	RootPath        string
	IgnoreCase      bool
	ShowLineNumbers bool
	WholeWord       bool
	Workers         int
	MaxSizeBytes    int64
	Extensions      map[string]struct{}
	ExcludeDirs     map[string]struct{}
	CountOnly       bool
	Quiet           bool
	Color           bool
	AbsPath         bool
	OutputFormat    string

	Regex          bool
	FollowSymlinks bool
	MaxDepth       int

	DynamicWorkers   bool
	IOWorkers        int
	CPUWorkers       int
	MaxWorkers       int
	Backpressure     int
	Metrics          bool
	Debug            bool
	Trace            bool
	MonitorGoroutine bool
	MonitorInterval  time.Duration
	CPUProfilePath   string
	MemProfilePath   string

	DefaultIgnoreDirs map[string]struct{}
}

// RCConfig represents the JSON config file structure.
type RCConfig struct {
	IgnoreCase        *bool   `json:"ignore_case,omitempty"`
	ShowLineNumbers   *bool   `json:"show_line_numbers,omitempty"`
	WholeWord         *bool   `json:"whole_word,omitempty"`
	Workers           *int    `json:"workers,omitempty"`
	MaxSize           *string `json:"max_size,omitempty"`
	Extensions        *string `json:"extensions,omitempty"`
	ExcludeDir        *string `json:"exclude_dir,omitempty"`
	CountOnly         *bool   `json:"count,omitempty"`
	Quiet             *bool   `json:"quiet,omitempty"`
	Color             *bool   `json:"color,omitempty"`
	AbsPath           *bool   `json:"abs,omitempty"`
	OutputFormat      *string `json:"format,omitempty"`
	Regex             *bool   `json:"regex,omitempty"`
	FollowSymlinks    *bool   `json:"follow_symlinks,omitempty"`
	MaxDepth          *int    `json:"max_depth,omitempty"`
	DynamicWorkers    *bool   `json:"dynamic_workers,omitempty"`
	IOWorkers         *int    `json:"io_workers,omitempty"`
	CPUWorkers        *int    `json:"cpu_workers,omitempty"`
	MaxWorkers        *int    `json:"max_workers,omitempty"`
	Backpressure      *int    `json:"backpressure,omitempty"`
	Metrics           *bool   `json:"metrics,omitempty"`
	Debug             *bool   `json:"debug,omitempty"`
	Trace             *bool   `json:"trace,omitempty"`
	MonitorGoroutines *bool   `json:"monitor_goroutines,omitempty"`
	MonitorIntervalMs *int    `json:"monitor_interval_ms,omitempty"`
}

const UsageText = "Usage: gosearch [flags] <pattern> <path>"

var Version = "dev"

// Parse parses command line arguments and returns a Config.
func Parse(args []string) (Config, error) {
	rcPath := detectConfigPath(args)
	rcDefaults, rcErr := loadRCConfig(rcPath)
	if rcErr != nil {
		return Config{}, rcErr
	}

	fs := flag.NewFlagSet("gosearch", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	showVersion := fs.Bool("version", false, "print version")
	completion := fs.String("completion", "", "print shell completion script: bash|zsh|fish")
	configPath := fs.String("config", rcPath, "path to config file (.gosearchrc JSON)")

	ignoreCase := fs.Bool("i", boolWithDefault(rcDefaults.IgnoreCase, false), "case-insensitive search")
	showLineNumbers := fs.Bool("n", boolWithDefault(rcDefaults.ShowLineNumbers, true), "show line numbers")
	wholeWord := fs.Bool("w", boolWithDefault(rcDefaults.WholeWord, false), "whole-word matching")
	workers := fs.Int("workers", intWithDefault(rcDefaults.Workers, runtime.NumCPU()), "base worker count")
	maxSize := fs.String("max-size", stringWithDefault(rcDefaults.MaxSize, ""), "max file size in bytes, KB, MB, or GB")
	extensions := fs.String("extensions", stringWithDefault(rcDefaults.Extensions, ""), "comma-separated extensions, e.g. .go,.txt")
	excludeDir := fs.String("exclude-dir", stringWithDefault(rcDefaults.ExcludeDir, ""), "comma-separated directory names to skip")
	countOnly := fs.Bool("count", boolWithDefault(rcDefaults.CountOnly, false), "print only total match count")
	quiet := fs.Bool("quiet", boolWithDefault(rcDefaults.Quiet, false), "suppress output, use exit code only")
	color := fs.Bool("color", boolWithDefault(rcDefaults.Color, false), "enable ANSI color and highlighting in plain output")
	absPath := fs.Bool("abs", boolWithDefault(rcDefaults.AbsPath, false), "print absolute paths")
	outputFormat := fs.String("format", stringWithDefault(rcDefaults.OutputFormat, "plain"), "output format: plain|json")

	regexMode := fs.Bool("regex", boolWithDefault(rcDefaults.Regex, false), "treat pattern as regex")
	followSymlinks := fs.Bool("follow-symlinks", boolWithDefault(rcDefaults.FollowSymlinks, false), "follow symlinked files/directories")
	maxDepth := fs.Int("max-depth", intWithDefault(rcDefaults.MaxDepth, -1), "max traversal depth (-1 for unlimited)")

	dynamicWorkers := fs.Bool("dynamic-workers", boolWithDefault(rcDefaults.DynamicWorkers, false), "dynamically scale CPU workers")
	ioWorkers := fs.Int("io-workers", intWithDefault(rcDefaults.IOWorkers, 0), "number of IO workers (0=auto)")
	cpuWorkers := fs.Int("cpu-workers", intWithDefault(rcDefaults.CPUWorkers, 0), "number of CPU workers (0=auto)")
	maxWorkers := fs.Int("max-workers", intWithDefault(rcDefaults.MaxWorkers, 0), "max CPU workers when dynamic scaling is enabled (0=auto)")
	backpressure := fs.Int("backpressure", intWithDefault(rcDefaults.Backpressure, 0), "channel buffer size (0=auto)")
	metrics := fs.Bool("metrics", boolWithDefault(rcDefaults.Metrics, false), "print worker lifecycle metrics")
	debug := fs.Bool("debug", boolWithDefault(rcDefaults.Debug, false), "enable debug logging")
	trace := fs.Bool("trace", boolWithDefault(rcDefaults.Trace, false), "enable verbose execution trace")
	monitorGoroutines := fs.Bool("monitor-goroutines", boolWithDefault(rcDefaults.MonitorGoroutines, false), "periodically log goroutine count")
	monitorIntervalMs := fs.Int("monitor-interval-ms", intWithDefault(rcDefaults.MonitorIntervalMs, 250), "goroutine monitor interval in milliseconds")
	cpuProfile := fs.String("cpuprofile", "", "write CPU profile to file")
	memProfile := fs.String("memprofile", "", "write heap profile to file on exit")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	if *showVersion || strings.TrimSpace(*completion) != "" {
		return Config{
			ShowVersion:      *showVersion,
			CompletionTarget: strings.TrimSpace(*completion),
			ConfigPath:       strings.TrimSpace(*configPath),
			VersionLabel:     VersionString(),
		}, nil
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

	maxSizeBytes, err := ParseSize(*maxSize)
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

	excluded := ParseCSVSet(*excludeDir, false)
	defaults := map[string]struct{}{
		".git":         {},
		"node_modules": {},
		"vendor":       {},
	}
	for item := range excluded {
		defaults[item] = struct{}{}
	}

	cfg := Config{
		ConfigPath:        strings.TrimSpace(*configPath),
		ShowVersion:       *showVersion,
		CompletionTarget:  strings.TrimSpace(*completion),
		VersionLabel:      VersionString(),
		Pattern:           pattern,
		RootPath:          rootPath,
		IgnoreCase:        *ignoreCase,
		ShowLineNumbers:   *showLineNumbers,
		WholeWord:         *wholeWord,
		Workers:           *workers,
		MaxSizeBytes:      maxSizeBytes,
		Extensions:        ParseCSVSet(*extensions, true),
		ExcludeDirs:       excluded,
		CountOnly:         *countOnly,
		Quiet:             *quiet,
		Color:             *color,
		AbsPath:           *absPath,
		OutputFormat:      format,
		Regex:             *regexMode,
		FollowSymlinks:    *followSymlinks,
		MaxDepth:          *maxDepth,
		DynamicWorkers:    *dynamicWorkers,
		IOWorkers:         resolvedIOWorkers,
		CPUWorkers:        resolvedCPUWorkers,
		MaxWorkers:        resolvedMaxWorkers,
		Backpressure:      resolvedBackpressure,
		Metrics:           *metrics,
		Debug:             *debug,
		Trace:             *trace,
		MonitorGoroutine:  *monitorGoroutines,
		MonitorInterval:   time.Duration(*monitorIntervalMs) * time.Millisecond,
		CPUProfilePath:    strings.TrimSpace(*cpuProfile),
		MemProfilePath:    strings.TrimSpace(*memProfile),
		DefaultIgnoreDirs: defaults,
	}

	return cfg, nil
}

func detectConfigPath(args []string) string {
	defaultPath := ".gosearchrc"
	for i := 0; i < len(args); i++ {
		item := args[i]
		if item == "-config" && i+1 < len(args) {
			return strings.TrimSpace(args[i+1])
		}
		if strings.HasPrefix(item, "-config=") {
			return strings.TrimSpace(strings.TrimPrefix(item, "-config="))
		}
	}
	return defaultPath
}

func loadRCConfig(path string) (RCConfig, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return RCConfig{}, nil
	}

	content, err := os.ReadFile(trimmed)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return RCConfig{}, nil
		}
		return RCConfig{}, errors.New("config: " + err.Error())
	}

	var cfg RCConfig
	if err := json.Unmarshal(content, &cfg); err != nil {
		return RCConfig{}, errors.New("config parse: " + err.Error())
	}
	return cfg, nil
}

func boolWithDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func intWithDefault(value *int, fallback int) int {
	if value == nil {
		return fallback
	}
	return *value
}

func stringWithDefault(value *string, fallback string) string {
	if value == nil {
		return fallback
	}
	return strings.TrimSpace(*value)
}

// VersionString returns the version string.
func VersionString() string {
	if strings.TrimSpace(Version) == "" {
		return "dev"
	}
	return Version
}

// ParseSize parses a human-readable size string like "10MB" into bytes.
func ParseSize(input string) (int64, error) {
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

// ParseCSVSet parses a comma-separated string into a set.
func ParseCSVSet(input string, normalizeExtension bool) map[string]struct{} {
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

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}
