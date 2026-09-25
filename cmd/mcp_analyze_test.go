package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/panyam/mcpkit/client"
	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/mcpkit/testutil"
	"github.com/panyam/slyds/core"
)

// funcAnalyzer adapts a func to slideAnalyzer so tests can script the
// analyzer without a subprocess.
type funcAnalyzer func(ctx context.Context, prompt string) (string, error)

func (f funcAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	return f(ctx, prompt)
}

// newAnalyzeTestClient serves a deck root with only the analyze task
// registered, which keeps these tests independent of the other tools.
func newAnalyzeTestClient(t *testing.T, root string, an slideAnalyzer, opts ...client.ClientOption) *testutil.TestClient {
	t.Helper()
	ws, err := NewLocalWorkspace(root)
	if err != nil {
		t.Fatalf("NewLocalWorkspace: %v", err)
	}
	srv := server.NewServer(
		mcpcore.ServerInfo{Name: "slyds-test", Version: "0.0.1"},
		server.WithMiddleware(workspaceMiddleware(ws)),
	)
	registerAnalyzeTask(srv, an)
	return testutil.NewTestClient(t, srv, opts...)
}

func newAnalyzeDeck(t *testing.T, slides int) string {
	t.Helper()
	root := t.TempDir()
	if _, err := core.CreateInDir("Analyze Me", slides, "default", filepath.Join(root, "talk"), true); err != nil {
		t.Fatalf("CreateInDir: %v", err)
	}
	return root
}

func decodeAnalysis(t *testing.T, r *mcpcore.ToolResult) deckAnalysis {
	t.Helper()
	if r == nil {
		t.Fatal("nil tool result")
	}
	if r.IsError {
		t.Fatalf("tool returned error: %+v", r.Content)
	}
	raw, _ := json.Marshal(r.StructuredContent)
	var a deckAnalysis
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("decode analysis: %v\nraw: %s", err, raw)
	}
	return a
}

// slideNumber pulls N out of the "slide N of M" line runDeckAnalysis puts
// at the top of every prompt.
func slideNumber(prompt string) int {
	var n, total int
	fmt.Sscanf(prompt[strings.Index(prompt, "slide "):], "slide %d of %d", &n, &total)
	return n
}

func waitForStatus(t *testing.T, c *client.Client, taskID string, match func(*mcpcore.DetailedTask) bool) *mcpcore.DetailedTask {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		dt, err := client.GetTask(t.Context(), c, taskID)
		if err != nil {
			t.Fatalf("GetTask: %v", err)
		}
		if match(dt) {
			return dt
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("task %s never reached the expected state", taskID)
	return nil
}

// TestAnalyzeDeck_TaskReportsPerSlideStatus is the demo path: a client that
// declares the tasks extension gets a task, sees a status message per slide,
// and reads one analysis per slide from the completed task.
func TestAnalyzeDeck_TaskReportsPerSlideStatus(t *testing.T) {
	root := newAnalyzeDeck(t, 3)
	release := make(chan struct{})
	an := funcAnalyzer(func(ctx context.Context, prompt string) (string, error) {
		if slideNumber(prompt) == 2 {
			<-release
		}
		return fmt.Sprintf("looks fine (slide %d)", slideNumber(prompt)), nil
	})
	tc := newAnalyzeTestClient(t, root, an, client.WithTasksExtension())

	res, err := client.ToolCall(t.Context(), tc.Client, "analyze_deck", map[string]any{"deck": "talk"})
	if err != nil {
		t.Fatalf("ToolCall: %v", err)
	}
	if !res.IsTask() {
		t.Fatalf("expected a task, got sync result %+v", res.Sync)
	}
	taskID := res.Task.TaskID

	// The analyzer is parked on slide 2, so the status message must say so.
	dt := waitForStatus(t, tc.Client, taskID, func(dt *mcpcore.DetailedTask) bool {
		return strings.HasPrefix(dt.StatusMessage, "Analyzing slide 2/3")
	})
	if dt.Status != mcpcore.TaskWorking {
		t.Errorf("status = %s, want working", dt.Status)
	}
	close(release)

	final, err := client.WaitForTask(t.Context(), tc.Client, taskID, client.WaitOptions{PollInterval: 20 * time.Millisecond})
	if err != nil {
		t.Fatalf("WaitForTask: %v", err)
	}
	if final.Status != mcpcore.TaskCompleted {
		t.Fatalf("status = %s, want completed", final.Status)
	}
	a := decodeAnalysis(t, final.Result)
	if a.Analyzed != 3 || a.Failed != 0 || len(a.Slides) != 3 {
		t.Fatalf("analysis = %+v, want 3 analyzed", a)
	}
	for i, s := range a.Slides {
		if s.Position != i+1 || s.SlideID == "" || s.Slug == "" {
			t.Errorf("slide %d identity incomplete: %+v", i+1, s)
		}
		if want := fmt.Sprintf("looks fine (slide %d)", i+1); s.Analysis != want {
			t.Errorf("slide %d analysis = %q, want %q", i+1, s.Analysis, want)
		}
	}
}

// TestAnalyzeDeck_SyncWithoutTasksExtension checks the fallback: a client
// that never declared tasks gets the finished analysis from tools/call.
func TestAnalyzeDeck_SyncWithoutTasksExtension(t *testing.T) {
	root := newAnalyzeDeck(t, 2)
	an := funcAnalyzer(func(ctx context.Context, prompt string) (string, error) {
		return "ok", nil
	})
	tc := newAnalyzeTestClient(t, root, an)

	res, err := client.ToolCall(t.Context(), tc.Client, "analyze_deck", map[string]any{"deck": "talk"})
	if err != nil {
		t.Fatalf("ToolCall: %v", err)
	}
	if res.IsTask() {
		t.Fatal("client without the tasks extension must not get a task")
	}
	if a := decodeAnalysis(t, res.Sync); a.Analyzed != 2 {
		t.Errorf("analyzed = %d, want 2", a.Analyzed)
	}
}

// TestAnalyzeDeck_FailingSlideDoesNotFailTask checks that one analyzer error
// is recorded against its slide and the rest of the deck still runs.
func TestAnalyzeDeck_FailingSlideDoesNotFailTask(t *testing.T) {
	root := newAnalyzeDeck(t, 3)
	an := funcAnalyzer(func(ctx context.Context, prompt string) (string, error) {
		if slideNumber(prompt) == 2 {
			return "", errors.New("analyzer crashed")
		}
		return "ok", nil
	})
	tc := newAnalyzeTestClient(t, root, an, client.WithTasksExtension())

	res, err := client.ToolCall(t.Context(), tc.Client, "analyze_deck", map[string]any{"deck": "talk"})
	if err != nil || !res.IsTask() {
		t.Fatalf("ToolCall: err=%v task=%v", err, res != nil && res.IsTask())
	}
	final, err := client.WaitForTask(t.Context(), tc.Client, res.Task.TaskID, client.WaitOptions{PollInterval: 20 * time.Millisecond})
	if err != nil {
		t.Fatalf("WaitForTask: %v", err)
	}
	if final.Status != mcpcore.TaskCompleted {
		t.Fatalf("status = %s, want completed", final.Status)
	}
	a := decodeAnalysis(t, final.Result)
	if a.Analyzed != 2 || a.Failed != 1 {
		t.Fatalf("analyzed/failed = %d/%d, want 2/1", a.Analyzed, a.Failed)
	}
	if !strings.Contains(a.Slides[1].Error, "analyzer crashed") {
		t.Errorf("slide 2 error = %q", a.Slides[1].Error)
	}
}

// TestAnalyzeDeck_CancelStopsAnalyzer checks that tasks/cancel reaches the
// in-flight analyzer call and no later slide starts.
func TestAnalyzeDeck_CancelStopsAnalyzer(t *testing.T) {
	root := newAnalyzeDeck(t, 3)
	started := make(chan struct{})
	stopped := make(chan struct{})
	var calls sync.Map
	an := funcAnalyzer(func(ctx context.Context, prompt string) (string, error) {
		calls.Store(slideNumber(prompt), true)
		close(started)
		<-ctx.Done()
		close(stopped)
		return "", ctx.Err()
	})
	tc := newAnalyzeTestClient(t, root, an, client.WithTasksExtension())

	res, err := client.ToolCall(t.Context(), tc.Client, "analyze_deck", map[string]any{"deck": "talk"})
	if err != nil || !res.IsTask() {
		t.Fatalf("ToolCall: err=%v", err)
	}
	<-started
	if err := client.CancelTask(t.Context(), tc.Client, res.Task.TaskID); err != nil {
		t.Fatalf("CancelTask: %v", err)
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("analyzer never saw the cancellation")
	}
	dt := waitForStatus(t, tc.Client, res.Task.TaskID, func(dt *mcpcore.DetailedTask) bool {
		return dt.Status.IsTerminal()
	})
	if dt.Status != mcpcore.TaskCancelled {
		t.Errorf("status = %s, want cancelled", dt.Status)
	}
	// Give the goroutine a moment in case it wrongly carried on to slide 2.
	time.Sleep(100 * time.Millisecond)
	if _, ok := calls.Load(2); ok {
		t.Error("analysis continued past the cancelled slide")
	}
}

// TestAnalyzeDeck_BadInputFailsBeforeTask checks that an unknown deck or
// slide is reported on tools/call itself, with no task created.
func TestAnalyzeDeck_BadInputFailsBeforeTask(t *testing.T) {
	root := newAnalyzeDeck(t, 2)
	an := funcAnalyzer(func(ctx context.Context, prompt string) (string, error) {
		t.Error("analyzer must not run for bad input")
		return "", nil
	})
	tc := newAnalyzeTestClient(t, root, an, client.WithTasksExtension())

	for name, args := range map[string]map[string]any{
		"unknown deck":  {"deck": "nope"},
		"unknown slide": {"deck": "talk", "slides": []string{"no-such-slide"}},
	} {
		t.Run(name, func(t *testing.T) {
			res, err := client.ToolCall(t.Context(), tc.Client, "analyze_deck", args)
			if err != nil {
				t.Fatalf("ToolCall: %v", err)
			}
			// The tasks middleware wraps any sync reply in a task born
			// completed, so the error arrives as that task's result.
			var r *mcpcore.ToolResult
			if res.IsTask() {
				if res.Task.Status != mcpcore.TaskCompleted {
					t.Fatalf("bad input started a %s task", res.Task.Status)
				}
				dt, err := client.GetTask(t.Context(), tc.Client, res.Task.TaskID)
				if err != nil {
					t.Fatalf("GetTask: %v", err)
				}
				r = dt.Result
			} else {
				r = res.Sync
			}
			if r == nil || !r.IsError {
				t.Fatalf("expected an error result, got %+v", r)
			}
		})
	}
}

// TestAnalyzeDeck_NotRegisteredWithoutAnalyzer checks the off switch: with
// no --analyze-cmd the tool and the tasks extension are both absent.
func TestAnalyzeDeck_NotRegisteredWithoutAnalyzer(t *testing.T) {
	tc := newAnalyzeTestClient(t, newAnalyzeDeck(t, 1), nil)
	for _, tool := range tc.ListTools() {
		if tool.Name == "analyze_deck" {
			t.Fatal("analyze_deck registered without an analyzer")
		}
	}
}

func TestSetTaskStatusMessage_DoesNotReviveTerminalTask(t *testing.T) {
	store := server.NewInMemoryStore()
	info := mcpcore.TaskInfo{TaskID: "task-1", Status: mcpcore.TaskWorking}
	if err := store.Create(info, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Cancel("task-1", ""); err != nil {
		t.Fatal(err)
	}
	setTaskStatusMessage(context.Background(), store, "task-1", "Analyzing slide 9/9")
	got, _ := store.Get("task-1", "")
	if got.Status != mcpcore.TaskCancelled || got.StatusMessage == "Analyzing slide 9/9" {
		t.Errorf("cancelled task changed: status=%s message=%q", got.Status, got.StatusMessage)
	}
}
