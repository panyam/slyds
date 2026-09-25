//go:build unix

package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func writeScript(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "analyzer.sh")
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCommandAnalyzer_PromptOnStdin(t *testing.T) {
	an, err := newCommandAnalyzer("cat", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	got, err := an.Analyze(t.Context(), "  hello from stdin \n")
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello from stdin" {
		t.Errorf("got %q", got)
	}
}

func TestCommandAnalyzer_PromptPlaceholder(t *testing.T) {
	an, err := newCommandAnalyzer("echo reviewed: {prompt}", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	// Shell metacharacters must arrive as literal text, not run.
	got, err := an.Analyze(t.Context(), "$(touch /tmp/x) ; rm -rf nothing")
	if err != nil {
		t.Fatal(err)
	}
	if got != "reviewed: $(touch /tmp/x) ; rm -rf nothing" {
		t.Errorf("got %q", got)
	}
}

func TestCommandAnalyzer_FailureIncludesStderr(t *testing.T) {
	an, err := newCommandAnalyzer(writeScript(t, "echo 'model unavailable' >&2\nexit 3\n"), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	_, err = an.Analyze(t.Context(), "x")
	if err == nil || !strings.Contains(err.Error(), "model unavailable") {
		t.Fatalf("err = %v, want stderr in message", err)
	}
}

func TestCommandAnalyzer_Timeout(t *testing.T) {
	an, err := newCommandAnalyzer(writeScript(t, "sleep 30\n"), 200*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = an.Analyze(t.Context(), "x")
	if err == nil || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("err = %v, want deadline exceeded", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Errorf("timeout took %s", time.Since(start))
	}
}

// TestCommandAnalyzer_CancelKillsChildren is the orphan check: cancelling
// must also kill processes the analyzer started, not just the analyzer.
func TestCommandAnalyzer_CancelKillsChildren(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "child.pid")
	an, err := newCommandAnalyzer(writeScript(t, "sleep 30 &\necho $! > "+pidFile+"\nwait\n"), time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		_, err := an.Analyze(ctx, "x")
		done <- err
	}()

	var childPid int
	deadline := time.Now().Add(5 * time.Second)
	for childPid == 0 && time.Now().Before(deadline) {
		if data, err := os.ReadFile(pidFile); err == nil {
			childPid, _ = strconv.Atoi(strings.TrimSpace(string(data)))
		}
		time.Sleep(20 * time.Millisecond)
	}
	if childPid == 0 {
		t.Fatal("analyzer never started its child")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Analyze did not return after cancel")
	}

	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if syscall.Kill(childPid, 0) != nil {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	syscall.Kill(childPid, syscall.SIGKILL)
	t.Fatalf("child %d survived cancellation", childPid)
}
