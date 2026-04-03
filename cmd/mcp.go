package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/panyam/mcpkit"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Model Context Protocol server (HTTP+SSE or Streamable HTTP)",
	Long: `mcp exposes one tool, "slyds", which runs the slyds binary with the given
arguments in the given working directory.

  slyds mcp          Start MCP server (Streamable HTTP on 127.0.0.1:6274)
  slyds mcp --sse    Use legacy HTTP+SSE transport instead

Set min_version in tool arguments to fail fast if the installed slyds is too old.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPServer()
	},
}

var (
	mcpListen         string
	mcpToken          string
	mcpPublicURL      string
	mcpUseSSE         bool
)

func init() {
	mcpCmd.Flags().StringVar(&mcpListen, "listen", "127.0.0.1:6274", "Listen address")
	mcpCmd.Flags().StringVar(&mcpToken, "token", "", "Bearer token for authentication")
	mcpCmd.Flags().StringVar(&mcpPublicURL, "public-url", "", "Public URL for reverse proxy (endpoint event URL)")
	mcpCmd.Flags().BoolVar(&mcpUseSSE, "sse", false, "Use legacy HTTP+SSE transport instead of Streamable HTTP")
	rootCmd.AddCommand(mcpCmd)
}

func runMCPServer() error {
	// Build server options
	var serverOpts []mcpkit.Option
	serverOpts = append(serverOpts, mcpkit.WithListen(mcpListen))
	if mcpToken != "" {
		serverOpts = append(serverOpts, mcpkit.WithBearerToken(mcpToken))
	}

	srv := mcpkit.NewServer(
		mcpkit.ServerInfo{
			Name:    "slyds",
			Version: Version,
		},
		serverOpts...,
	)

	// Register the slyds tool
	srv.RegisterTool(slydsTool())

	// Transport options
	var transportOpts []mcpkit.TransportOption
	if mcpPublicURL != "" {
		transportOpts = append(transportOpts, mcpkit.WithPublicURL(mcpPublicURL))
	}
	if mcpUseSSE {
		transportOpts = append(transportOpts, mcpkit.WithSSE(true), mcpkit.WithStreamableHTTP(false))
		fmt.Fprintf(os.Stderr, "MCP server (SSE) on %s\n", mcpListen)
	} else {
		transportOpts = append(transportOpts, mcpkit.WithStreamableHTTP(true), mcpkit.WithSSE(false))
		fmt.Fprintf(os.Stderr, "MCP server (Streamable HTTP) on %s\n", mcpListen)
	}

	return srv.ListenAndServe(transportOpts...)
}

// slydsTool returns the ToolDef and handler for the "slyds" tool.
func slydsTool() (mcpkit.ToolDef, mcpkit.ToolHandler) {
	def := mcpkit.ToolDef{
		Name:        "slyds",
		Description: "Run slyds CLI with args in cwd; stdout is returned as text content.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cwd": map[string]any{
					"type":        "string",
					"description": "Working directory for the subprocess (deck root or subdirectory).",
				},
				"args": map[string]any{
					"type":        "array",
					"items":       map[string]any{"type": "string"},
					"description": "Arguments after `slyds`, e.g. [\"introspect\"] or [\"query\",\"1\",\"h1\",\"--count\"]",
				},
				"min_version": map[string]any{
					"type":        "string",
					"description": "Optional minimum slyds version (e.g. 0.0.10). Tool fails if current binary is older.",
				},
			},
			"required": []string{"cwd", "args"},
		},
	}

	handler := func(ctx context.Context, req mcpkit.ToolRequest) (mcpkit.ToolResult, error) {
		var p struct {
			Cwd        string   `json:"cwd"`
			Args       []string `json:"args"`
			MinVersion string   `json:"min_version,omitempty"`
		}
		if err := req.Bind(&p); err != nil {
			return mcpkit.ErrorResult(err.Error()), nil
		}
		if p.Cwd == "" {
			return mcpkit.ErrorResult("cwd is required"), nil
		}
		if p.MinVersion != "" && !versionAtLeast(Version, p.MinVersion) {
			return mcpkit.ToolResult{}, fmt.Errorf("slyds version %s is older than min_version %s", Version, p.MinVersion)
		}

		self, err := os.Executable()
		if err != nil {
			return mcpkit.ToolResult{}, fmt.Errorf("os.Executable: %w", err)
		}

		cmd := exec.CommandContext(ctx, self, p.Args...)
		cmd.Dir = p.Cwd
		out, err := cmd.CombinedOutput()
		text := string(out)
		if err != nil {
			if text != "" {
				text = text + "\n" + err.Error()
			} else {
				text = err.Error()
			}
			return mcpkit.ErrorResult(text), nil
		}
		return mcpkit.TextResult(text), nil
	}

	return def, handler
}

// versionAtLeast returns true if current >= min, using dotted numeric segments.
func versionAtLeast(current, min string) bool {
	c := parseVersionParts(current)
	m := parseVersionParts(min)
	for i := 0; i < len(m); i++ {
		var cv int
		if i < len(c) {
			cv = c[i]
		}
		if cv > m[i] {
			return true
		}
		if cv < m[i] {
			return false
		}
	}
	return true
}

func parseVersionParts(s string) []int {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.Split(s, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n := 0
		for _, r := range p {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return []int{0}
	}
	return out
}
