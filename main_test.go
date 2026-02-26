package main

import (
	"bytes"
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
	if exitCode == 0 {
		t.Fatalf("expected non-zero exit code for invalid args")
	}

	if !strings.Contains(stderr.String(), "Usage: gosearch <pattern> <path>") {
		t.Fatalf("usage message missing, stderr: %q", stderr.String())
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
