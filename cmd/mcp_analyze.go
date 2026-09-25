package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/ext/tasks"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/slyds/core"
)

// analyze_deck runs an external command once per slide and collects what it
// prints. It is registered as a SEP-2663 task-capable tool: a client that
// declares the tasks extension gets a task back immediately and watches the
// per-slide status messages; any other client gets the same analysis
// synchronously, with notifications/progress instead.
//
// The tool only exists when the server is started with --analyze-cmd (or
// SLYDS_ANALYZE_CMD), because it runs a local program.

// promptPlaceholder in an analyze command is replaced by the prompt as a
// single argv element. Without it, the prompt goes to the command's stdin.
const promptPlaceholder = "{prompt}"

const defaultAnalyzeTimeout = 2 * time.Minute

var (
	mcpAnalyzeCmd     string
	mcpAnalyzeTimeout time.Duration
)

// slideAnalyzer turns one slide's prompt into an analysis.
type slideAnalyzer interface {
	Analyze(ctx context.Context, prompt string) (string, error)
}

// commandAnalyzer runs argv once per slide. It never goes through a shell, so
// slide content can't be interpreted as shell syntax.
type commandAnalyzer struct {
	argv    []string
	timeout time.Duration
	running sync.WaitGroup
}

// newCommandAnalyzer parses a whitespace-separated command line. Quoting is
// not supported; a command that needs it should be wrapped in a script.
func newCommandAnalyzer(cmdline string, timeout time.Duration) (*commandAnalyzer, error) {
	argv := strings.Fields(cmdline)
	if len(argv) == 0 {
		return nil, errors.New("analyze command is empty")
	}
	if timeout <= 0 {
		timeout = defaultAnalyzeTimeout
	}
	return &commandAnalyzer{argv: argv, timeout: timeout}, nil
}

func (a *commandAnalyzer) Analyze(ctx context.Context, prompt string) (string, error) {
	a.running.Add(1)
	defer a.running.Done()
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	args := make([]string, 0, len(a.argv)-1)
	usesPlaceholder := false
	for _, arg := range a.argv[1:] {
		if arg == promptPlaceholder {
			arg = prompt
			usesPlaceholder = true
		}
		args = append(args, arg)
	}
	cmd := exec.CommandContext(ctx, a.argv[0], args...)
	isolateProcessGroup(cmd)
	if !usesPlaceholder {
		cmd.Stdin = strings.NewReader(prompt)
	}
	// Without WaitDelay, a killed command whose children still hold stdout
	// open would keep Wait blocked until they exit.
	cmd.WaitDelay = 2 * time.Second
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("%s: %w", a.argv[0], ctx.Err())
		}
		msg := strings.TrimSpace(stderr.String())
		if len(msg) > 500 {
			msg = msg[:500] + "…"
		}
		if msg != "" {
			return "", fmt.Errorf("%s: %w: %s", a.argv[0], err, msg)
		}
		return "", fmt.Errorf("%s: %w", a.argv[0], err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

// drain waits up to timeout for in-flight analyzer commands to exit. A
// short-lived process (slyds ws analyze) calls it before exiting so a
// cancelled task's command is killed rather than orphaned.
func (a *commandAnalyzer) drain(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		a.running.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

// resolveAnalyzeCmd applies the flag > env precedence used by the other
// SLYDS_* settings.
func resolveAnalyzeCmd(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("SLYDS_ANALYZE_CMD")
}

// registerAnalyzeTask installs the tasks extension and analyze_deck. It is a
// no-op when an is nil, so a server without --analyze-cmd advertises neither.
func registerAnalyzeTask(srv *server.Server, an slideAnalyzer) {
	if an == nil {
		return
	}
	tasks.Register(tasks.Config{Server: srv})
	srv.Register(analyzeDeckTool(an))
}

type analyzeDeckInput struct {
	Deck        string   `json:"deck"                  jsonschema:"required,description=Deck name"`
	Slides      []string `json:"slides,omitempty"      jsonschema:"description=Slides to analyze (position\\, slug\\, slide_id or filename). Omit to analyze every slide."`
	Instruction string   `json:"instruction,omitempty" jsonschema:"description=What to look for (default: a general review of clarity\\, structure and visual density)"`
}

type slideAnalysis struct {
	Position int    `json:"position"`
	SlideID  string `json:"slide_id"`
	Slug     string `json:"slug"`
	Title    string `json:"title"`
	Analysis string `json:"analysis,omitempty"`
	Error    string `json:"error,omitempty"`
}

type deckAnalysis struct {
	Deck     string          `json:"deck"`
	Analyzed int             `json:"analyzed"`
	Failed   int             `json:"failed"`
	Slides   []slideAnalysis `json:"slides"`
}

const defaultAnalyzeInstruction = "Review this slide for clarity, structure and visual density. " +
	"Point out anything confusing, overloaded or inconsistent with the rest of the deck, " +
	"and suggest concrete fixes. Be brief."

func analyzeDeckTool(an slideAnalyzer) mcpcore.TypedToolResult {
	return mcpcore.TypedTool[analyzeDeckInput, mcpcore.ToolResponse](
		"analyze_deck",
		"Analyze each slide of a deck with the server's configured analyzer (one call per slide). "+
			"Long-running: clients that support the MCP tasks extension get a task with per-slide status messages; "+
			"other clients wait for the full result.",
		func(ctx mcpcore.ToolContext, p analyzeDeckInput) (mcpcore.ToolResponse, error) {
			d, errResult := openDeckFromContext(ctx, p.Deck)
			if errResult != nil {
				return *errResult, nil
			}
			targets, err := analysisTargets(d, p.Slides)
			if err != nil {
				return mcpcore.ErrorResult(err.Error()), nil
			}

			// Bad input is rejected above, before any task exists. The tasks
			// middleware then re-runs this handler in a goroutine with a
			// TaskContext attached, which is the branch that does the work.
			tc := tasks.GetTaskContext(ctx)
			if tc == nil && mcpcore.ClientSupportsTasks(ctx) {
				return mcpcore.GoAsyncResult{}, nil
			}

			report := func(done, total int, msg string) {
				if tc != nil {
					tc.SetStatusMessage(msg)
				} else {
					ctx.Progress(float64(done), float64(total), msg)
				}
			}
			result, err := runDeckAnalysis(ctx, an, p.Deck, d.Title(), targets, p.Instruction, report)
			if err != nil {
				// Cancelled: the task is already terminal, so this result is
				// discarded. A sync caller sees the error.
				return mcpcore.ErrorResult(err.Error()), nil
			}
			return jsonResult(result)
		},
		mcpcore.WithToolExecution(&mcpcore.ToolExecution{TaskSupport: mcpcore.TaskSupportOptional}),
	)
}

// analysisTarget is one slide queued for analysis, with its HTML read up
// front so the deck isn't reopened from the task goroutine.
type analysisTarget struct {
	desc    core.SlideDescription
	content string
}

func analysisTargets(d *core.Deck, refs []string) ([]analysisTarget, error) {
	desc, err := d.Describe()
	if err != nil {
		return nil, err
	}
	positions := make([]int, 0, len(desc.Slides))
	if len(refs) == 0 {
		for _, s := range desc.Slides {
			positions = append(positions, s.Position)
		}
	} else {
		for _, ref := range refs {
			pos, err := resolveSlidePosition(d, ref, 0)
			if err != nil {
				return nil, err
			}
			positions = append(positions, pos)
		}
	}
	if len(positions) == 0 {
		return nil, errors.New("deck has no slides to analyze")
	}

	targets := make([]analysisTarget, 0, len(positions))
	for _, pos := range positions {
		if pos < 1 || pos > len(desc.Slides) {
			return nil, fmt.Errorf("slide position %d out of range (1-%d)", pos, len(desc.Slides))
		}
		content, err := d.GetSlideContent(pos)
		if err != nil {
			return nil, err
		}
		targets = append(targets, analysisTarget{desc: desc.Slides[pos-1], content: content})
	}
	return targets, nil
}

// runDeckAnalysis analyzes targets in order. A slide whose analyzer call
// fails is recorded and skipped; only cancellation stops the loop.
func runDeckAnalysis(
	ctx context.Context,
	an slideAnalyzer,
	deckName, deckTitle string,
	targets []analysisTarget,
	instruction string,
	report func(done, total int, msg string),
) (deckAnalysis, error) {
	if instruction == "" {
		instruction = defaultAnalyzeInstruction
	}
	out := deckAnalysis{Deck: deckName, Slides: make([]slideAnalysis, 0, len(targets))}
	total := len(targets)
	for i, t := range targets {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		label := t.desc.Slug
		if label == "" {
			label = t.desc.File
		}
		report(i, total, fmt.Sprintf("Analyzing slide %d/%d: %s", i+1, total, label))

		entry := slideAnalysis{
			Position: t.desc.Position,
			SlideID:  t.desc.SlideID,
			Slug:     t.desc.Slug,
			Title:    t.desc.Title,
		}
		prompt := fmt.Sprintf(
			"You are reviewing slide %d of %d in the presentation %q.\n\nInstruction: %s\n\nSlide HTML:\n\n%s\n",
			t.desc.Position, total, deckTitle, instruction, t.content,
		)
		analysis, err := an.Analyze(ctx, prompt)
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return out, ctxErr
			}
			entry.Error = err.Error()
			out.Failed++
		} else {
			entry.Analysis = analysis
			out.Analyzed++
		}
		out.Slides = append(out.Slides, entry)
	}
	report(total, total, fmt.Sprintf("Analyzed %d/%d slides", out.Analyzed, total))
	return out, nil
}
