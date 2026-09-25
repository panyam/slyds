package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/panyam/mcpkit/client"
	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/spf13/cobra"
)

// wsAnalyzeCmd is a tasks-aware MCP client for analyze_deck. Editors don't
// declare the tasks extension yet, so this is how to watch the task run:
// status messages as each slide is analyzed, Ctrl-C to cancel.
var wsAnalyzeCmd = &cobra.Command{
	Use:   "analyze <deck>",
	Short: "Run analyze_deck as an MCP task and watch it slide by slide",
	Long: `analyze calls the analyze_deck tool as an MCP task (SEP-2663) and prints
each status update while the server works through the slides. Ctrl-C sends
tasks/cancel.

  slyds ws analyze talk --analyze-cmd 'agent -p --output-format text {prompt}'
      Starts an in-process server over --deck-root with that analyzer.

  slyds ws analyze talk --server http://127.0.0.1:8274/mcp
      Uses a running 'slyds mcp --analyze-cmd ...' instead.`,
	Args:         cobra.ExactArgs(1),
	SilenceUsage: true,
	RunE:         runWsAnalyze,
}

var (
	wsAnalyzeServer      string
	wsAnalyzeToken       string
	wsAnalyzeSlides      []string
	wsAnalyzeInstruction string
	wsAnalyzeCmdline     string
	wsAnalyzeTimeout     time.Duration
)

func init() {
	f := wsAnalyzeCmd.Flags()
	f.StringVar(&wsAnalyzeServer, "server", "", "URL of a running slyds MCP server (default: start one in-process)")
	f.StringVar(&wsAnalyzeToken, "token", "", "Bearer token for --server (default: $SLYDS_MCP_TOKEN)")
	f.StringSliceVar(&wsAnalyzeSlides, "slides", nil, "Slides to analyze (position, slug, slide_id or filename; default: all)")
	f.StringVar(&wsAnalyzeInstruction, "instruction", "", "What to look for in each slide")
	f.StringVar(&wsAnalyzeCmdline, "analyze-cmd", "", "Analyzer command for the in-process server (default: $SLYDS_ANALYZE_CMD)")
	f.DurationVar(&wsAnalyzeTimeout, "analyze-timeout", defaultAnalyzeTimeout, "Per-slide timeout for the in-process analyzer")
	wsCmd.AddCommand(wsAnalyzeCmd)
}

type staticTokenSource string

func (s staticTokenSource) Token() (string, error) { return string(s), nil }

func runWsAnalyze(cmd *cobra.Command, args []string) error {
	ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt)
	defer stop()

	status := newStatusPrinter(cmd.ErrOrStderr())
	opts := []client.ClientOption{
		client.WithTasksExtension(),
		client.WithNotificationCallback(func(method string, params any) {
			if method == "notifications/tasks" {
				status.fromNotification(params)
			}
		}),
	}

	c, cleanup, err := connectAnalyzeClient(ctx, opts)
	if err != nil {
		return err
	}
	defer cleanup()

	return analyzeViaClient(ctx, c, cmd.OutOrStdout(), status, analyzeDeckInput{
		Deck:        args[0],
		Slides:      wsAnalyzeSlides,
		Instruction: wsAnalyzeInstruction,
	})
}

// connectAnalyzeClient returns a connected client and a cleanup func that
// closes it and, for the in-process server, lets a cancelled analyzer command
// be killed before the process exits.
func connectAnalyzeClient(ctx context.Context, opts []client.ClientOption) (*client.Client, func(), error) {
	info := mcpcore.ClientInfo{Name: "slyds-ws-analyze", Version: Version}
	var c *client.Client
	var an *commandAnalyzer
	if wsAnalyzeServer != "" {
		if tok := resolveMCPToken(wsAnalyzeToken); tok != "" {
			opts = append(opts, client.WithTokenSource(staticTokenSource(tok)))
		}
		c = client.NewClient(wsAnalyzeServer, info, opts...)
	} else {
		cmdline := resolveAnalyzeCmd(wsAnalyzeCmdline)
		if cmdline == "" {
			return nil, nil, errors.New("no analyzer: pass --analyze-cmd (or set SLYDS_ANALYZE_CMD), or --server to use a running slyds mcp")
		}
		var err error
		an, err = newCommandAnalyzer(cmdline, wsAnalyzeTimeout)
		if err != nil {
			return nil, nil, err
		}
		ws, err := NewLocalWorkspace(resolveDeckRoot(wsDeckRoot))
		if err != nil {
			return nil, nil, err
		}
		srv := server.NewServer(
			mcpcore.ServerInfo{Name: "slyds", Version: Version},
			server.WithMiddleware(workspaceMiddleware(ws)),
		)
		registerAnalyzeTask(srv, an)
		opts = append(opts, client.WithTransport(server.NewInProcessTransport(srv)))
		c = client.NewClient("memory://", info, opts...)
	}
	if err := c.Connect(ctx); err != nil {
		return nil, nil, fmt.Errorf("connect: %w", err)
	}
	cleanup := func() {
		c.Close()
		if an != nil {
			an.drain(5 * time.Second)
		}
	}
	return c, cleanup, nil
}

// analyzeViaClient calls analyze_deck and follows the task to the end. It
// handles a sync reply too, for a server that answered without a task.
func analyzeViaClient(ctx context.Context, c *client.Client, out io.Writer, status *statusPrinter, in analyzeDeckInput) error {
	res, err := client.ToolCall(ctx, c, "analyze_deck", in)
	if err != nil {
		return err
	}
	if !res.IsTask() {
		if res.Sync == nil {
			return errors.New("analyze_deck: unexpected response shape")
		}
		return printAnalysis(out, res.Sync)
	}

	taskID := res.Task.TaskID
	status.printf("task %s started\n", taskID)
	dt, err := client.WaitForTask(ctx, c, taskID, client.WaitOptions{OnStatus: status.fromTask})
	if err != nil {
		if ctx.Err() != nil {
			cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if cerr := client.CancelTask(cctx, c, taskID); cerr != nil {
				return fmt.Errorf("interrupted; tasks/cancel failed: %w", cerr)
			}
			status.printf("task %s cancelled\n", taskID)
			return errors.New("cancelled")
		}
		return err
	}
	switch dt.Status {
	case mcpcore.TaskCompleted:
		if dt.Result == nil {
			return errors.New("task completed without a result")
		}
		return printAnalysis(out, dt.Result)
	case mcpcore.TaskFailed:
		if dt.Error != nil {
			return fmt.Errorf("task failed: %s", dt.Error.Message)
		}
		return errors.New("task failed")
	default:
		return fmt.Errorf("task ended %s", dt.Status)
	}
}

func printAnalysis(out io.Writer, r *mcpcore.ToolResult) error {
	if r.IsError {
		var msgs []string
		for _, c := range r.Content {
			msgs = append(msgs, c.Text)
		}
		return errors.New(strings.Join(msgs, "; "))
	}
	raw, err := json.Marshal(r.StructuredContent)
	if err != nil {
		return err
	}
	var a deckAnalysis
	if err := json.Unmarshal(raw, &a); err != nil {
		return fmt.Errorf("decode analysis: %w", err)
	}
	if wsJSON {
		data, _ := json.MarshalIndent(a, "", "  ")
		fmt.Fprintln(out, string(data))
		return nil
	}
	for _, s := range a.Slides {
		name := s.Title
		if name == "" {
			name = s.Slug
		}
		fmt.Fprintf(out, "## %d. %s\n\n", s.Position, name)
		if s.Error != "" {
			fmt.Fprintf(out, "(analysis failed: %s)\n\n", s.Error)
			continue
		}
		fmt.Fprintf(out, "%s\n\n", s.Analysis)
	}
	fmt.Fprintf(out, "%d analyzed, %d failed\n", a.Analyzed, a.Failed)
	return nil
}

// statusPrinter prints each distinct task status line once. Updates arrive
// from both notifications/tasks and tasks/get polling, so most show up twice.
type statusPrinter struct {
	mu   sync.Mutex
	w    io.Writer
	last string
}

func newStatusPrinter(w io.Writer) *statusPrinter { return &statusPrinter{w: w} }

func (p *statusPrinter) printf(format string, args ...any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	fmt.Fprintf(p.w, format, args...)
}

func (p *statusPrinter) fromTask(dt *mcpcore.DetailedTask) {
	p.show(dt.Status, dt.StatusMessage)
}

func (p *statusPrinter) fromNotification(params any) {
	raw, err := json.Marshal(params)
	if err != nil {
		return
	}
	var dt mcpcore.DetailedTask
	if json.Unmarshal(raw, &dt) == nil {
		p.show(dt.Status, dt.StatusMessage)
	}
}

func (p *statusPrinter) show(st mcpcore.TaskStatus, msg string) {
	line := fmt.Sprintf("[%s] %s", st, msg)
	p.mu.Lock()
	defer p.mu.Unlock()
	if line == p.last {
		return
	}
	p.last = line
	fmt.Fprintln(p.w, strings.TrimSpace(line))
}
