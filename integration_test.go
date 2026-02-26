package main

import (
	"bytes"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/quick"
)

func TestDeterministicHarness(t *testing.T) {
	outA, codeA := runAndNormalize(t, []string{"needle", filepath.Join("testdata", "small")})
	outB, codeB := runAndNormalize(t, []string{"needle", filepath.Join("testdata", "small")})

	if codeA != 0 || codeB != 0 {
		t.Fatalf("expected zero exit codes, got %d and %d", codeA, codeB)
	}
	if strings.Join(outA, "\n") != strings.Join(outB, "\n") {
		t.Fatalf("deterministic harness mismatch\nA=%v\nB=%v", outA, outB)
	}
}

func TestIgnoreNegationProperty(t *testing.T) {
	property := func(name string) bool {
		name = strings.TrimSpace(name)
		if name == "" || strings.Contains(name, "/") {
			name = "x.txt"
		}

		cfg := Config{}
		base := `C:\tmp`
		pathText := filepath.Join(base, name)
		plainRule := ignoreRule{baseDir: base, pattern: name, negate: false, hasPath: false}
		negRule := ignoreRule{baseDir: base, pattern: name, negate: true, hasPath: false}

		ignoredByPlain := shouldIgnorePath(cfg, []ignoreRule{plainRule}, pathText, false)
		ignoredByNeg := shouldIgnorePath(cfg, []ignoreRule{negRule}, pathText, false)

		return ignoredByPlain != ignoredByNeg
	}

	if err := quick.Check(property, nil); err != nil {
		t.Fatalf("property check failed: %v", err)
	}
}

func TestDebugAndTraceLogging(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-debug", "-trace", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "trace:") {
		t.Fatalf("expected trace logs in stderr, got %s", stderr.String())
	}
}

func TestMetricsIncludePhaseTimings(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-metrics", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit 0, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "timings walk=") {
		t.Fatalf("expected phase timings in stderr, got %s", stderr.String())
	}
}

func runAndNormalize(t *testing.T, args []string) ([]string, int) {
	t.Helper()
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run(args, &stdout, &stderr)

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, trimmed)
	}
	sort.Strings(filtered)
	return filtered, exitCode
}
