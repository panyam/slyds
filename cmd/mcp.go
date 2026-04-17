package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"

	"github.com/panyam/mcpkit/server"
	"github.com/panyam/slyds/assets"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Model Context Protocol server with semantic deck tools and resources",
	Long: `mcp starts an MCP server exposing slyds deck operations as semantic tools
and deck content as browsable resources.

  slyds mcp                          Streamable HTTP on 127.0.0.1:8274
  slyds mcp --sse                    Legacy HTTP+SSE transport
  slyds mcp --deck-root ./decks      Serve decks from a specific directory`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPServer()
	},
}

var (
	mcpListen       string
	mcpToken        string
	mcpPublicURL    string
	mcpUseSSE       bool
	mcpUseStdio     bool
	mcpDeckRoot     string
	mcpAllowOrigins []string
	mcpAppBridge    bool
	mcpVerbose      bool
	mcpAuth         MCPAuthConfig
)

func init() {
	addCommonFlags(mcpCmd)
	rootCmd.AddCommand(mcpCmd)
}

// registerHandwrittenTools registers all hand-written tools, resources,
// prompts, completions, and app previews on the server.
func registerHandwrittenTools(srv *server.Server) {
	registerResources(srv)
	registerTools(srv)
	registerAppTools(srv)
	registerCompletions(srv)
	registerPrompts(srv)
}

func runMCPServer() error {
	ws, err := initWorkspace()
	if err != nil {
		return err
	}

	cfg := &mcpServerConfig{ServerName: "slyds", Workspace: ws}
	srv := cfg.buildServer()
	registerHandwrittenTools(srv)

	if mcpUseStdio {
		return cfg.runStdio(srv)
	}

	// Wrap MCP handler with landing page
	return cfg.runHTTP(srv, func(mcpHandler http.Handler) http.Handler {
		deckRefs, _ := ws.ListDecks()
		deckNames := make([]string, 0, len(deckRefs))
		for _, r := range deckRefs {
			deckNames = append(deckNames, r.Name)
		}
		authEnabled := mcpAuth.IsEnabled() || mcpToken != ""
		return mcpWithLanding(mcpHandler, "", mcpListen, deckNames, authEnabled)
	})
}

// --- Landing page + config printing (mcp-specific, not shared) ---

// resolveMCPToken returns the token from the flag value, falling back to the
// SLYDS_MCP_TOKEN environment variable if the flag is empty.
func resolveMCPToken(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("SLYDS_MCP_TOKEN")
}

func resolveDeckRoot(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := os.Getenv("SLYDS_DECK_ROOT"); env != "" {
		return env
	}
	return "."
}


// maskToken returns a redacted version of a token.
func maskToken(token string) string {
	if len(token) <= 4 {
		return "****"
	}
	return token[:2] + "****" + token[len(token)-2:]
}

// landingData is the template context for the MCP landing page.
type landingData struct {
	Transport   string
	Listen      string
	Decks       []string
	AuthEnabled bool
	ConfigJSON  string
}

var landingTmpl = func() *template.Template {
	tmplFS, _ := fs.Sub(assets.TemplatesFS, "templates")
	return template.Must(template.ParseFS(tmplFS, "mcp-landing.html.tmpl"))
}()

func mcpWithLanding(mcpHandler http.Handler, transport, listen string, decks []string, authEnabled bool) http.Handler {
	info := map[string]any{
		"server":    "slyds MCP",
		"version":   Version,
		"transport": transport,
		"listen":    listen,
		"decks":     decks,
		"auth":      authEnabled,
	}
	configJSON, _ := json.MarshalIndent(info, "", "  ")
	data := landingData{
		Transport:   transport,
		Listen:      listen,
		Decks:       decks,
		AuthEnabled: authEnabled,
		ConfigJSON:  string(configJSON),
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == http.MethodGet && strings.Contains(r.Header.Get("Accept"), "text/html") {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			landingTmpl.Execute(w, data)
			return
		}
		mcpHandler.ServeHTTP(w, r)
	})
}

// printHTTPConfig prints a ready-to-paste MCP config snippet for HTTP transports.
func printHTTPConfig(listen, token string) {
	url := fmt.Sprintf("http://%s/mcp", listen)
	fmt.Fprintf(os.Stderr, "\n  Add to Claude Desktop / Claude Code / Cursor config:\n\n")
	if token != "" {
		fmt.Fprintf(os.Stderr, `  {
    "mcpServers": {
      "slyds": {
        "url": "%s",
        "headers": {
          "Authorization": "Bearer %s"
        }
      }
    }
  }
`, url, token)
	} else {
		fmt.Fprintf(os.Stderr, `  {
    "mcpServers": {
      "slyds": {
        "url": "%s"
      }
    }
  }
`, url)
	}
	fmt.Fprintln(os.Stderr)
}

// printStdioConfig prints a ready-to-paste MCP config snippet for stdio transport.
func printStdioConfig(root, token string) {
	slydsPath, _ := os.Executable()
	if slydsPath == "" {
		slydsPath = "slyds"
	}
	args := fmt.Sprintf(`"mcp", "--stdio", "--deck-root", "%s"`, root)
	if token != "" {
		args += fmt.Sprintf(`, "--token", "%s"`, token)
	}
	fmt.Fprintf(os.Stderr, "\n  Add to Claude Desktop / Claude Code / Cursor config:\n\n")
	fmt.Fprintf(os.Stderr, `  {
    "mcpServers": {
      "slyds": {
        "command": "%s",
        "args": [%s]
      }
    }
  }
`, slydsPath, args)
	fmt.Fprintln(os.Stderr)
}

// slydsMCPErrorHandler logs MCP session lifecycle events to stderr.
type slydsMCPErrorHandler struct {
	server.BaseErrorHandler
}

func (h *slydsMCPErrorHandler) OnSessionExpire(sessionID string, reason error) {
	fmt.Fprintf(os.Stderr, "MCP session expired: %s (%v)\n", sessionID, reason)
}

func (h *slydsMCPErrorHandler) OnKeepaliveFailure(sessionID string, consecutiveFailures int) {
	fmt.Fprintf(os.Stderr, "MCP keepalive failure: session=%s failures=%d\n", sessionID, consecutiveFailures)
}

// listenAndServeGraceful starts the HTTP server with signal-based graceful shutdown.
func listenAndServeGraceful(srv *http.Server) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return srv.Close()
	}
}
