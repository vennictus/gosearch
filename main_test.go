package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/vennictus/gosearch/internal/config"
	"github.com/vennictus/gosearch/internal/search"
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

func TestVersionFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-version"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0 for version, got %d", exitCode)
	}
	if strings.TrimSpace(stdout.String()) == "" {
		t.Fatalf("expected version output, got empty stdout")
	}
}

func TestCompletionFlag(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-completion", "bash"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0 for completion, got %d", exitCode)
	}
	if !strings.Contains(stdout.String(), "complete -F _gosearch_completion") {
		t.Fatalf("expected bash completion script output, got: %s", stdout.String())
	}
}

func TestConfigFileDefaults(t *testing.T) {
	root := t.TempDir()
	configPath := filepath.Join(root, ".gosearchrc")
	fixturePath := filepath.Join(root, "a.txt")

	if err := os.WriteFile(fixturePath, []byte("needle here\n"), 0o644); err != nil {
		t.Fatalf("failed to write fixture: %v", err)
	}
	config := `{"ignore_case":true,"workers":1}`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"-config", configPath, "NEEDLE", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected config-driven case-insensitive match, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "needle here") {
		t.Fatalf("expected matched output, got %s", stdout.String())
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
	cfg, err := config.Parse([]string{"-workers", "1", "needle", filepath.Join("testdata", "small")})
	if err != nil {
		t.Fatalf("config.Parse returned error: %v", err)
	}

	if cfg.Workers != 1 {
		t.Fatalf("expected workers to be 1, got %d", cfg.Workers)
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

	matches, err := search.ScanFileWithMatcher(filePath, search.NewMatcher("needle", false, true), 0)
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

func TestRegexMode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-regex", "needle\\s+first", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected regex match, got exit %d, stderr: %s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "needle first") {
		t.Fatalf("expected regex-matched output, got: %s", stdout.String())
	}
}

func TestRegexModeNoMatch(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-regex", "nomatch\\d+", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected no-match exit code 1, got %d (stderr: %s)", exitCode, stderr.String())
	}
}

func TestRegexAndSubstringParityForEquivalentPattern(t *testing.T) {
	var substringOut bytes.Buffer
	var substringErr bytes.Buffer
	subExit := run([]string{"needle", filepath.Join("testdata", "small")}, &substringOut, &substringErr)
	if subExit != 0 {
		t.Fatalf("substring run failed with exit %d, stderr: %s", subExit, substringErr.String())
	}

	var regexOut bytes.Buffer
	var regexErr bytes.Buffer
	rexExit := run([]string{"-regex", "needle", filepath.Join("testdata", "small")}, &regexOut, &regexErr)
	if rexExit != 0 {
		t.Fatalf("regex run failed with exit %d, stderr: %s", rexExit, regexErr.String())
	}

	countLines := func(text string) int {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			return 0
		}
		return len(strings.Split(trimmed, "\n"))
	}

	if countLines(substringOut.String()) != countLines(regexOut.String()) {
		t.Fatalf("expected equivalent match counts, substring=%d regex=%d", countLines(substringOut.String()), countLines(regexOut.String()))
	}
}

func TestGitignoreSupport(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored.txt"), []byte("needle hidden\n"), 0o644); err != nil {
		t.Fatalf("failed to write ignored file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), []byte("needle visible\n"), 0o644); err != nil {
		t.Fatalf("failed to write visible file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"needle", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected match in visible file, got exit %d, stderr: %s", exitCode, stderr.String())
	}

	output := stdout.String()
	if strings.Contains(output, "ignored.txt") {
		t.Fatalf("expected ignored file to be skipped, got output: %s", output)
	}
	if !strings.Contains(output, "visible.txt") {
		t.Fatalf("expected visible file in output: %s", output)
	}
}

func TestNestedIgnorePrecedence(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("failed to create nested dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("nested/*.txt\n"), 0o644); err != nil {
		t.Fatalf("failed to write root .gitignore: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, ".gitignore"), []byte("!keep.txt\n"), 0o644); err != nil {
		t.Fatalf("failed to write nested .gitignore: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "drop.txt"), []byte("needle drop\n"), 0o644); err != nil {
		t.Fatalf("failed to write drop file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "keep.txt"), []byte("needle keep\n"), 0o644); err != nil {
		t.Fatalf("failed to write keep file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"needle", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected keep.txt match, got exit %d stderr=%s", exitCode, stderr.String())
	}

	output := stdout.String()
	if strings.Contains(output, "drop.txt") {
		t.Fatalf("drop.txt should be ignored by parent rule, output: %s", output)
	}
	if !strings.Contains(output, "keep.txt") {
		t.Fatalf("keep.txt should be restored by nested negate rule, output: %s", output)
	}
}

func TestMaxDepth(t *testing.T) {
	root := t.TempDir()
	level1 := filepath.Join(root, "level1")
	level2 := filepath.Join(level1, "level2")
	if err := os.MkdirAll(level2, 0o755); err != nil {
		t.Fatalf("failed to create directories: %v", err)
	}
	if err := os.WriteFile(filepath.Join(level1, "top.txt"), []byte("needle top\n"), 0o644); err != nil {
		t.Fatalf("failed to write top file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(level2, "deep.txt"), []byte("needle deep\n"), 0o644); err != nil {
		t.Fatalf("failed to write deep file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"-max-depth", "1", "needle", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected at least top-level match, got exit %d", exitCode)
	}
	if strings.Contains(stdout.String(), "deep.txt") {
		t.Fatalf("expected deep file to be excluded by max-depth, got: %s", stdout.String())
	}
}

func TestDynamicWorkersConfig(t *testing.T) {
	cfg, err := config.Parse([]string{"-dynamic-workers", "-cpu-workers", "2", "-max-workers", "4", "needle", filepath.Join("testdata", "small")})
	if err != nil {
		t.Fatalf("config.Parse returned error: %v", err)
	}
	if !cfg.DynamicWorkers {
		t.Fatalf("expected dynamic-workers to be enabled")
	}
	if cfg.CPUWorkers != 2 || cfg.MaxWorkers != 4 {
		t.Fatalf("unexpected worker config: cpu=%d max=%d", cfg.CPUWorkers, cfg.MaxWorkers)
	}
}

func TestFollowSymlinkFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation typically requires elevated privileges on Windows")
	}

	root := t.TempDir()
	realFile := filepath.Join(root, "real.txt")
	linkFile := filepath.Join(root, "link.txt")
	if err := os.WriteFile(realFile, []byte("needle in symlink target\n"), 0o644); err != nil {
		t.Fatalf("failed to write real file: %v", err)
	}
	if err := os.Symlink(realFile, linkFile); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"needle", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected match from real file, got %d", exitCode)
	}
	withoutFollow := stdout.String()

	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"-follow-symlinks", "needle", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches with symlink following, got %d", exitCode)
	}
	withFollow := stdout.String()

	if strings.Count(withFollow, "needle in symlink target") <= strings.Count(withoutFollow, "needle in symlink target") {
		t.Fatalf("expected more matches when following symlinks; without=%q with=%q", withoutFollow, withFollow)
	}
}

func TestSymlinkLoopDoesNotHang(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation typically requires elevated privileges on Windows")
	}

	root := t.TempDir()
	realDir := filepath.Join(root, "real")
	if err := os.MkdirAll(realDir, 0o755); err != nil {
		t.Fatalf("failed to create real dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(realDir, "a.txt"), []byte("needle once\n"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	loopLink := filepath.Join(realDir, "loop")
	if err := os.Symlink(realDir, loopLink); err != nil {
		t.Fatalf("failed to create looping symlink: %v", err)
	}

	bin := buildBinary(t)
	cmd := exec.Command(bin, "-follow-symlinks", "needle", root)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("command failed: %v stderr=%s", err, stderr.String())
		}
	case <-time.After(3 * time.Second):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		t.Fatal("symlink loop traversal appears to hang")
	}
}

func TestDanglingSymlinkIsHandled(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation typically requires elevated privileges on Windows")
	}

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ok.txt"), []byte("needle ok\n"), 0o644); err != nil {
		t.Fatalf("failed to write ok file: %v", err)
	}
	dangling := filepath.Join(root, "dangling.txt")
	if err := os.Symlink(filepath.Join(root, "missing.txt"), dangling); err != nil {
		t.Fatalf("failed to create dangling symlink: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := run([]string{"-follow-symlinks", "needle", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected successful run despite dangling symlink, got %d stderr=%s", exitCode, stderr.String())
	}
	if !strings.Contains(stdout.String(), "ok.txt") {
		t.Fatalf("expected regular file match in output: %s", stdout.String())
	}
}

func TestCancellationWithIgnoreAndRegex(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signal behavior for os.Interrupt differs on Windows")
	}

	root := createLargeTestDir(t)
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("large_a.txt\nlarge_b.txt\n"), 0o644); err != nil {
		t.Fatalf("failed to write .gitignore: %v", err)
	}

	bin := buildBinary(t)
	cmd := exec.Command(bin, "-regex", "needle.*not", "-follow-symlinks", "needle", root)
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
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected clean cancellation exit, got %v stderr=%s", err, stderr.String())
		}
	case <-time.After(5 * time.Second):
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		t.Fatal("process did not exit after interrupt")
	}
}

func TestMetricsOutputIncludesWorkerState(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-metrics", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected successful run, got %d", exitCode)
	}
	metricsText := stderr.String()
	if !strings.Contains(metricsText, "active=") || !strings.Contains(metricsText, "idle=") {
		t.Fatalf("expected active/idle metrics output, got: %s", metricsText)
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

// ============================================================================
// UNICODE AND MULTIBYTE TESTS
// ============================================================================

func TestUnicodeEmojiMatching(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"needle", filepath.Join("testdata", "unicode")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches in unicode dir, got exit %d, stderr: %s", exitCode, stderr.String())
	}

	output := stdout.String()
	// Should match multiple lines with needle across emoji.txt and multibyte.txt
	if !strings.Contains(output, "emoji.txt") {
		t.Fatalf("expected emoji.txt in output:\n%s", output)
	}
}

func TestUnicodeMultibyteCharacters(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"needle", filepath.Join("testdata", "unicode")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches in unicode dir, got exit %d, stderr: %s", exitCode, stderr.String())
	}

	output := stdout.String()
	// Verify matches work alongside Japanese, Greek, Korean, Thai, Hindi, Hebrew
	expectedLanguages := []string{"日本語", "Ελληνικά", "한국어", "ไทย", "हिन्दी", "עברית"}
	for _, lang := range expectedLanguages {
		if !strings.Contains(output, lang) {
			t.Fatalf("expected output to contain %s language text, got:\n%s", lang, output)
		}
	}
}

func TestCaseInsensitiveWithUnicode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-i", "NEEDLE", filepath.Join("testdata", "unicode")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected case-insensitive unicode matches, got exit %d", exitCode)
	}
}

// ============================================================================
// EDGE CASE TESTS
// ============================================================================

func TestEmptyLinesHandling(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-count", "needle", filepath.Join("testdata", "edge-cases")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches in edge-cases, got exit %d, stderr: %s", exitCode, stderr.String())
	}
}

func TestWhitespacePreservation(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"needle", filepath.Join("testdata", "edge-cases")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches with whitespace, got exit %d", exitCode)
	}

	output := stdout.String()
	// Verify leading spaces are preserved in whitespace.txt output
	if !strings.Contains(output, "   needle with leading spaces") {
		t.Fatalf("leading spaces not preserved in output:\n%s", output)
	}
}

func TestLongLineMatching(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"needle", filepath.Join("testdata", "edge-cases")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches in edge-cases, got exit %d", exitCode)
	}

	output := stdout.String()
	// Should find needle in long-lines.txt
	if !strings.Contains(output, "long-lines.txt") {
		t.Fatalf("expected long-lines.txt in output:\n%s", output)
	}
}

func TestSpecialRegexCharactersInSubstring(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Search for literal dot pattern without regex mode
	exitCode := run([]string{"needle.with.dots", filepath.Join("testdata", "edge-cases")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected match for literal dots, got exit %d", exitCode)
	}
}

func TestSpecialRegexCharactersInRegexMode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// In regex mode, dots should match any character
	exitCode := run([]string{"-regex", "needle.with.dots", filepath.Join("testdata", "edge-cases")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected regex match, got exit %d", exitCode)
	}

	// Test escaped regex special chars
	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"-regex", `needle\*with\*asterisks`, filepath.Join("testdata", "edge-cases")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected escaped regex match, got exit %d, stderr: %s", exitCode, stderr.String())
	}
}

func TestCaseVariations(t *testing.T) {
	testCases := []struct {
		name        string
		flags       []string
		minExpected int
	}{
		{"case-sensitive", []string{"-count", "needle"}, 1},
		{"case-insensitive", []string{"-count", "-i", "needle"}, 5},
		{"exact-upper", []string{"-count", "NEEDLE"}, 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			args := append(tc.flags, filepath.Join("testdata", "edge-cases"))
			exitCode := run(args, &stdout, &stderr)
			if exitCode != 0 && tc.minExpected > 0 {
				t.Fatalf("expected matches, got exit %d, stderr: %s", exitCode, stderr.String())
			}

			count := strings.TrimSpace(stdout.String())
			countInt := 0
			_, _ = fmt.Sscanf(count, "%d", &countInt)
			if countInt < tc.minExpected {
				t.Fatalf("expected at least %d matches, got %d", tc.minExpected, countInt)
			}
		})
	}
}

func TestWholeWordBoundaries(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Without whole-word flag, should match all
	exitCode := run([]string{"-count", "needle", filepath.Join("testdata", "edge-cases")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches, got exit %d", exitCode)
	}
	allCount := strings.TrimSpace(stdout.String())

	// With whole-word flag, should match fewer
	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"-count", "-w", "needle", filepath.Join("testdata", "edge-cases")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected whole-word matches, got exit %d", exitCode)
	}
	wholeWordCount := strings.TrimSpace(stdout.String())

	allCountInt := 0
	wholeWordCountInt := 0
	_, _ = fmt.Sscanf(allCount, "%d", &allCountInt)
	_, _ = fmt.Sscanf(wholeWordCount, "%d", &wholeWordCountInt)

	if wholeWordCountInt >= allCountInt {
		t.Fatalf("whole-word should find fewer matches: all=%d wholeWord=%d", allCountInt, wholeWordCountInt)
	}
}

func TestWindowsLineEndings(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"needle", filepath.Join("testdata", "edge-cases")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches, got exit %d", exitCode)
	}

	output := stdout.String()
	if !strings.Contains(output, "windows-endings.txt") {
		t.Fatalf("expected windows-endings.txt in output:\n%s", output)
	}
}

func TestUnixLineEndings(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"needle", filepath.Join("testdata", "edge-cases")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches, got exit %d", exitCode)
	}

	output := stdout.String()
	if !strings.Contains(output, "unix-endings.txt") {
		t.Fatalf("expected unix-endings.txt in output:\n%s", output)
	}
}

// ============================================================================
// CODE SAMPLE TESTS
// ============================================================================

func TestSearchInGoCode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-extensions", ".go", "needle", filepath.Join("testdata", "code-samples")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches in Go code, got exit %d", exitCode)
	}

	output := stdout.String()
	if !strings.Contains(output, "SearchForNeedle") {
		t.Fatalf("expected to find function name, got:\n%s", output)
	}
}

func TestSearchInPythonCode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-extensions", ".py", "needle", filepath.Join("testdata", "code-samples")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches in Python code, got exit %d", exitCode)
	}

	output := stdout.String()
	if !strings.Contains(output, "find_needle") {
		t.Fatalf("expected to find function name, got:\n%s", output)
	}
}

func TestSearchInJavaScriptCode(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-extensions", ".js", "needle", filepath.Join("testdata", "code-samples")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches in JavaScript code, got exit %d", exitCode)
	}

	output := stdout.String()
	// Should find needle in JS comments and strings
	if !strings.Contains(output, "sample.js") {
		t.Fatalf("expected sample.js in output, got:\n%s", output)
	}
}

func TestSearchInJSON(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-extensions", ".json", "needle", filepath.Join("testdata", "code-samples")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches in JSON, got exit %d", exitCode)
	}
}

func TestSearchInTOML(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-extensions", ".toml", "needle", filepath.Join("testdata", "code-samples")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches in TOML, got exit %d", exitCode)
	}
}

func TestMultipleExtensions(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-extensions", ".go,.py,.js", "-count", "needle", filepath.Join("testdata", "code-samples")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches with multiple extensions, got exit %d", exitCode)
	}

	count := strings.TrimSpace(stdout.String())
	countInt := 0
	_, _ = fmt.Sscanf(count, "%d", &countInt)
	if countInt < 10 {
		t.Fatalf("expected at least 10 matches across Go/Py/JS files, got %s", count)
	}
}

// ============================================================================
// REGEX PATTERN TESTS
// ============================================================================

func TestRegexFunctionPattern(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Find Go function declarations
	exitCode := run([]string{"-regex", "-extensions", ".go", `func\s+\w+`, filepath.Join("testdata", "code-samples")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected regex matches for functions, got exit %d, stderr: %s", exitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "func SearchForNeedle") && !strings.Contains(output, "func main") {
		t.Fatalf("expected to match function declarations, got:\n%s", output)
	}
}

func TestRegexClassPattern(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Find Python class declarations
	exitCode := run([]string{"-regex", "-extensions", ".py", `class\s+\w+`, filepath.Join("testdata", "code-samples")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected regex matches for class, got exit %d, stderr: %s", exitCode, stderr.String())
	}

	output := stdout.String()
	if !strings.Contains(output, "class NeedleFinder") {
		t.Fatalf("expected to match class declaration, got:\n%s", output)
	}
}

func TestRegexCommentPattern(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Find single-line comments in Go files
	exitCode := run([]string{"-regex", "-i", "-extensions", ".go", `//.*needle`, filepath.Join("testdata", "code-samples")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected regex matches for comments, got exit %d, stderr: %s", exitCode, stderr.String())
	}
}

func TestInvalidRegexReturnsError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Invalid regex should return exit code 2
	exitCode := run([]string{"-regex", "[invalid(regex", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2 for invalid regex, got %d", exitCode)
	}
}

// ============================================================================
// COMBINED FLAG TESTS
// ============================================================================

func TestCombinedFlags(t *testing.T) {
	testCases := []struct {
		name     string
		flags    []string
		wantExit int
	}{
		{"case-insensitive-whole-word", []string{"-i", "-w", "NEEDLE", filepath.Join("testdata", "small")}, 0},
		{"regex-case-insensitive", []string{"-regex", "-i", "NEEDLE.*first", filepath.Join("testdata", "small")}, 0},
		{"count-quiet-conflict", []string{"-count", "-quiet", "needle", filepath.Join("testdata", "small")}, 0},
		{"json-color", []string{"-format", "json", "-color", "needle", filepath.Join("testdata", "small")}, 0},
		{"extensions-exclude-dir", []string{"-extensions", ".txt", "-exclude-dir", "nested", "needle", filepath.Join("testdata")}, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer

			exitCode := run(tc.flags, &stdout, &stderr)
			if exitCode != tc.wantExit {
				t.Fatalf("expected exit %d, got %d, stderr: %s", tc.wantExit, exitCode, stderr.String())
			}
		})
	}
}

// ============================================================================
// ERROR HANDLING TESTS
// ============================================================================

func TestNonExistentPath(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"needle", filepath.Join("testdata", "nonexistent-directory-12345")}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2 for nonexistent path, got %d", exitCode)
	}
}

func TestEmptyPattern(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2 for empty pattern, got %d", exitCode)
	}
}

func TestNoArguments(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2 for no arguments, got %d", exitCode)
	}
}

func TestInvalidMaxSize(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-max-size", "invalid", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2 for invalid max-size, got %d", exitCode)
	}
}

func TestInvalidMaxDepth(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-max-depth", "abc", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2 for invalid max-depth, got %d", exitCode)
	}
}

// ============================================================================
// PERFORMANCE AND STRESS TESTS
// ============================================================================

func TestManySmallFiles(t *testing.T) {
	root := t.TempDir()
	
	// Create 100 small files
	for i := 0; i < 100; i++ {
		content := fmt.Sprintf("file %d content\n", i)
		if i%10 == 0 {
			content += "needle match here\n"
		}
		filePath := filepath.Join(root, fmt.Sprintf("file_%03d.txt", i))
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-count", "needle", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches, got exit %d", exitCode)
	}

	count := strings.TrimSpace(stdout.String())
	if count != "10" {
		t.Fatalf("expected 10 matches, got %s", count)
	}
}

func TestDeepDirectoryStructure(t *testing.T) {
	root := t.TempDir()
	
	// Create 10 levels deep
	current := root
	for i := 0; i < 10; i++ {
		current = filepath.Join(current, fmt.Sprintf("level%d", i))
		if err := os.MkdirAll(current, 0o755); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}
		content := fmt.Sprintf("depth %d needle here\n", i)
		filePath := filepath.Join(current, "file.txt")
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to create file: %v", err)
		}
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	// Without depth limit
	exitCode := run([]string{"-count", "needle", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches, got exit %d", exitCode)
	}
	allCount := strings.TrimSpace(stdout.String())
	if allCount != "10" {
		t.Fatalf("expected 10 matches, got %s", allCount)
	}

	// With depth limit
	stdout.Reset()
	stderr.Reset()
	exitCode = run([]string{"-count", "-max-depth", "3", "needle", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected matches with depth limit, got exit %d", exitCode)
	}
	limitedCount := strings.TrimSpace(stdout.String())
	limitedCountInt := 0
	_, _ = fmt.Sscanf(limitedCount, "%d", &limitedCountInt)
	if limitedCountInt >= 10 {
		t.Fatalf("depth limit should reduce matches: got %s", limitedCount)
	}
}

func TestLargeFileHandling(t *testing.T) {
	root := t.TempDir()
	
	// Create a 5MB file
	var builder strings.Builder
	for i := 0; i < 100000; i++ {
		builder.WriteString("this is a regular line without matches\n")
		if i == 50000 {
			builder.WriteString("needle appears in the middle of large file\n")
		}
	}
	
	largePath := filepath.Join(root, "large.txt")
	if err := os.WriteFile(largePath, []byte(builder.String()), 0o644); err != nil {
		t.Fatalf("failed to create large file: %v", err)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"needle", root}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected match in large file, got exit %d", exitCode)
	}

	if !strings.Contains(stdout.String(), "needle appears in the middle") {
		t.Fatalf("expected match output, got:\n%s", stdout.String())
	}
}

// ============================================================================
// OUTPUT FORMAT TESTS  
// ============================================================================

func TestJSONOutputStructure(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"-format", "json", "needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected match, got exit %d, stderr: %s", exitCode, stderr.String())
	}

	var result struct {
		Path string `json:"path"`
		Line int    `json:"line"`
		Text string `json:"text"`
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) == 0 {
		t.Fatal("expected JSON output")
	}

	if err := json.Unmarshal([]byte(lines[0]), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if result.Path == "" {
		t.Fatal("JSON missing path field")
	}
	if result.Line == 0 {
		t.Fatal("JSON missing line field")
	}
	if result.Text == "" {
		t.Fatal("JSON missing text field")
	}
}

func TestPlainOutputFormat(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := run([]string{"needle", filepath.Join("testdata", "small")}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected match, got exit %d, stderr: %s", exitCode, stderr.String())
	}

	output := stdout.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		t.Fatal("expected output lines")
	}
	
	// Plain format: path:line: text
	firstLine := lines[0]
	parts := strings.SplitN(firstLine, ":", 3)
	if len(parts) != 3 {
		t.Fatalf("expected plain format 'path:line: text', got: %s", firstLine)
	}

	// Line number should be numeric
	lineNum := 0
	_, err := fmt.Sscanf(parts[1], "%d", &lineNum)
	if err != nil || lineNum == 0 {
		t.Fatalf("expected numeric line number, got: %s", parts[1])
	}
}
