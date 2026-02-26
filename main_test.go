package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestScanFileMatching(t *testing.T) {
	path := filepath.Join("testdata", "small", "b.txt")
	matches, err := scanFile(path, "needle")
	if err != nil {
		t.Fatalf("scanFile returned error: %v", err)
	}

	if len(matches) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(matches))
	}

	if matches[0].Line != 1 || matches[1].Line != 2 || matches[2].Line != 4 {
		t.Fatalf("unexpected line numbers: %+v", matches)
	}

	if !strings.Contains(matches[0].Text, "needle") {
		t.Fatalf("first match does not contain pattern: %q", matches[0].Text)
	}
}

func TestBinaryDetection(t *testing.T) {
	path := filepath.Join("testdata", "binary", "binary.dat")
	isBinary, err := isBinaryFile(path)
	if err != nil {
		t.Fatalf("isBinaryFile returned error: %v", err)
	}

	if !isBinary {
		t.Fatalf("expected binary file to be detected")
	}

	matches, err := scanFile(path, "needle")
	if err != nil {
		t.Fatalf("scanFile returned error for binary file: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("expected zero matches for binary file, got %d", len(matches))
	}
}

func TestCLIEndToEnd(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "needle", filepath.Join("testdata", "small"))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("command failed: %v, stderr: %s", err, stderr.String())
	}

	output := stdout.String()
	expectedA := filepath.Join("testdata", "small", "a.txt") + ":1: alpha needle"
	expectedB := filepath.Join("testdata", "small", "b.txt") + ":1: needle first"

	if !strings.Contains(output, expectedA) {
		t.Fatalf("output missing expected line %q\nfull output:\n%s", expectedA, output)
	}
	if !strings.Contains(output, expectedB) {
		t.Fatalf("output missing expected line %q\nfull output:\n%s", expectedB, output)
	}
	if strings.Contains(output, filepath.Join("testdata", "small", "c.txt")) {
		t.Fatalf("output unexpectedly contains c.txt\nfull output:\n%s", output)
	}
}

func TestConcurrencySafety(t *testing.T) {
	bin := buildBinary(t)

	for i := 0; i < 20; i++ {
		cmd := exec.Command(bin, "needle", filepath.Join("testdata", "nested"))
		var stdout bytes.Buffer
		var stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			t.Fatalf("iteration %d failed: %v, stderr: %s", i, err, stderr.String())
		}
	}
}

func TestCancellationWithSIGINT(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal behavior for os.Interrupt differs on Windows")
	}

	bin := buildBinary(t)
	largeDir := createLargeTestDir(t)

	cmd := exec.Command(bin, "needle", largeDir)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start command: %v", err)
	}

	time.Sleep(150 * time.Millisecond)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("failed to send interrupt: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("process did not exit cleanly: %v, stderr: %s", err, stderr.String())
		}
		if strings.Contains(strings.ToLower(stderr.String()), "panic") {
			t.Fatalf("stderr contains panic:\n%s", stderr.String())
		}
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("process did not exit after interrupt")
	}
}

func TestUsageMessageOnInvalidArgs(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"only-pattern"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2 for invalid args, got %d", exitCode)
	}

	if !strings.Contains(stderr.String(), "Usage: gosearch [flags] <pattern> <path>") {
		t.Fatalf("usage message missing, stderr: %q", stderr.String())
	}
}

func TestWorkersFlagAffectsConfig(t *testing.T) {
	cfg, err := parseConfig([]string{"-workers", "1", "needle", filepath.Join("testdata", "small")})
	if err != nil {
		t.Fatalf("parseConfig returned error: %v", err)
	}

	if cfg.workers != 1 {
		t.Fatalf("expected workers to be 1, got %d", cfg.workers)
	}
}

func TestInvalidWorkersReturnsExitCodeTwo(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-workers", "0", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2 for invalid workers, got %d", exitCode)
	}
}

func TestCaseInsensitiveFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-i", "NEEDLE", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected zero exit code, got %d, stderr: %s", exitCode, stderr.String())
	}

	if !strings.Contains(stdout.String(), "needle") {
		t.Fatalf("expected case-insensitive matches, got output:\n%s", stdout.String())
	}
}

func TestWholeWordMatching(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "words.txt")
	content := "needle needles needled\nneedle only\n"
	if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}

	matches, err := scanFileWithMatcher(filePath, newMatcher("needle", false, true), 0)
	if err != nil {
		t.Fatalf("scanFileWithMatcher returned error: %v", err)
	}

	if len(matches) != 2 {
		t.Fatalf("expected 2 whole-word matches, got %d", len(matches))
	}
}

func TestCountOnlyOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-count", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected zero exit code, got %d, stderr: %s", exitCode, stderr.String())
	}

	if strings.TrimSpace(stdout.String()) != "4" {
		t.Fatalf("expected count output 4, got %q", stdout.String())
	}
}

func TestMaxSizeFiltersFiles(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-max-size", "1B", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 when all files are filtered out, got %d", exitCode)
	}
	if strings.TrimSpace(stdout.String()) != "" {
		t.Fatalf("expected no output when files are size-filtered, got %q", stdout.String())
	}
}

func TestExtensionsFilter(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-extensions", ".txt", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches with .txt extension filter, got exit %d", exitCode)
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"-extensions", ".md", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected no matches with .md extension filter, got exit %d", exitCode)
	}
}

func TestExcludeDirFilter(t *testing.T) {
	root := t.TempDir()
	keepDir := filepath.Join(root, "keep")
	vendorDir := filepath.Join(root, "vendor")

	if err := os.MkdirAll(keepDir, 0o755); err != nil {
		t.Fatalf("failed to create keep dir: %v", err)
	}
	if err := os.MkdirAll(vendorDir, 0o755); err != nil {
		t.Fatalf("failed to create vendor dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(keepDir, "a.txt"), []byte("needle in keep\n"), 0o644); err != nil {
		t.Fatalf("failed to write keep file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "b.txt"), []byte("needle in vendor\n"), 0o644); err != nil {
		t.Fatalf("failed to write vendor file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"-exclude-dir", "vendor", "needle", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected match in keep dir, got exit %d (stderr: %s)", exitCode, stderr.String())
	}

	output := stdout.String()
	if strings.Contains(output, "vendor") {
		t.Fatalf("expected vendor directory to be excluded, got output: %s", output)
	}
	if !strings.Contains(output, "keep") {
		t.Fatalf("expected keep directory match in output: %s", output)
	}
}

func TestDisableLineNumbers(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-n=false", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches, got exit %d", exitCode)
	}

	output := stdout.String()
	if strings.Contains(output, ":1:") {
		t.Fatalf("expected output without line numbers, got: %s", output)
	}
	if !strings.Contains(output, "a.txt: alpha needle") {
		t.Fatalf("expected plain line without line number, got: %s", output)
	}
}

func TestAbsolutePathOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-abs", "-n=false", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches, got exit %d", exitCode)
	}

	firstLine := strings.Split(strings.TrimSpace(stdout.String()), "\n")[0]
	parts := strings.SplitN(firstLine, ": ", 2)
	if len(parts) != 2 {
		t.Fatalf("unexpected output format: %q", firstLine)
	}
	if !filepath.IsAbs(parts[0]) {
		t.Fatalf("expected absolute path, got %q", parts[0])
	}
}

func TestColorHighlightOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-color", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches, got exit %d", exitCode)
	}

	output := stdout.String()
	if !strings.Contains(output, "\x1b[31mneedle\x1b[0m") {
		t.Fatalf("expected highlighted match in output, got: %q", output)
	}
}

func TestJSONOutputFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-format", "json", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected zero exit code, got %d, stderr: %s", exitCode, stderr.String())
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) == 0 {
		t.Fatalf("expected json output lines")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &parsed); err != nil {
		t.Fatalf("expected valid json line, got error: %v", err)
	}
	if _, ok := parsed["path"]; !ok {
		t.Fatalf("expected json line to include path field: %v", parsed)
	}
}

func TestQuietModeUsesExitCodeOnly(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-quiet", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected zero exit code for found matches, got %d", exitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("expected no output in quiet mode, got %q", stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"-quiet", "missing-token", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1 when no matches in quiet mode, got %d", exitCode)
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()

	binPath := filepath.Join(t.TempDir(), "gosearch")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = "."
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, string(output))
	}

	return binPath
}

func createLargeTestDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for i := 0; i < 12; i++ {
		filePath := filepath.Join(dir, "large_"+string(rune('a'+i))+".txt")
		var builder strings.Builder
		for line := 0; line < 60000; line++ {
			builder.WriteString("this line does not include the token\n")
		}
		if err := os.WriteFile(filePath, []byte(builder.String()), 0o644); err != nil {
			t.Fatalf("failed to create file %s: %v", filePath, err)
		}
	}

	return dir
}
