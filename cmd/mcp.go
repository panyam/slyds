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
	"time"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/ext/ui"
	"github.com/panyam/mcpkit/server"
	gohttp "github.com/panyam/servicekit/http"
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
)

func init() {
	mcpCmd.Flags().StringVar(&mcpListen, "listen", "127.0.0.1:8274", "Listen address")
	mcpCmd.Flags().StringVar(&mcpToken, "token", "", "Bearer token for authentication")
	mcpCmd.Flags().StringVar(&mcpPublicURL, "public-url", "", "Public URL for reverse proxy")
	mcpCmd.Flags().BoolVar(&mcpUseSSE, "sse", false, "Use legacy HTTP+SSE transport")
	mcpCmd.Flags().BoolVar(&mcpUseStdio, "stdio", false, "Use stdio transport (Content-Length framed JSON-RPC on stdin/stdout)")
	mcpCmd.Flags().StringVar(&mcpDeckRoot, "deck-root", "", "Root directory for deck discovery (default: $SLYDS_DECK_ROOT, or current directory)")
	mcpCmd.Flags().StringSliceVar(&mcpAllowOrigins, "allow-origin", nil, "Allowed Origin headers (default: localhost only). Use '*' to allow all origins (e.g. behind a tunnel)")
	rootCmd.AddCommand(mcpCmd)
}

func runMCPServer() error {
	// Build a LocalWorkspace rooted at --deck-root (falling back to
	// SLYDS_DECK_ROOT env var, then "."). In a future hosted deployment,
	// the workspace is built per request from auth; here it's a single
	// constant installed via middleware below.
	ws, err := NewLocalWorkspace(resolveDeckRoot(mcpDeckRoot))
	if err != nil {
		return fmt.Errorf("invalid deck-root: %w", err)
	}
	root := ws.Root()

	// Token from env if flag not set
	mcpToken = resolveMCPToken(mcpToken)

	// Build server
	var serverOpts []server.Option
	serverOpts = append(serverOpts, server.WithListen(mcpListen))
	serverOpts = append(serverOpts, server.WithExtension(ui.UIExtension{}))
	serverOpts = append(serverOpts, server.WithErrorHandler(&slydsMCPErrorHandler{}))
	serverOpts = append(serverOpts, server.WithMiddleware(workspaceMiddleware(ws)))
	if mcpToken != "" {
		serverOpts = append(serverOpts, server.WithBearerToken(mcpToken))
	}

	srv := server.NewServer(
		mcpcore.ServerInfo{
			Name:    "slyds",
			Version: Version,
		},
		serverOpts...,
	)

	// Register resources, tools, and app previews. None of these capture
	// the workspace directly — handlers resolve it from request context.
	registerResources(srv)
	registerTools(srv)
	registerAppTools(srv)
	registerCompletions(srv)
	registerPrompts(srv)

	// Transport selection
	if mcpUseStdio {
		fmt.Fprintf(os.Stderr, "MCP server (stdio) — deck root: %s\n", root)
		if mcpToken != "" {
			fmt.Fprintf(os.Stderr, "  Auth: bearer token (%s)\n", maskToken(mcpToken))
		}
		printStdioConfig(root, mcpToken)
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		return srv.RunStdio(ctx)
	}

	var transportOpts []server.TransportOption
	if mcpPublicURL != "" {
		transportOpts = append(transportOpts, server.WithPublicURL(mcpPublicURL))
	}
	if len(mcpAllowOrigins) > 0 {
		transportOpts = append(transportOpts, server.WithAllowedOrigins(mcpAllowOrigins...))
	}

	var transport string
	if mcpUseSSE {
		transportOpts = append(transportOpts,
			server.WithSSE(true),
			server.WithStreamableHTTP(false),
			server.WithSSEGracePeriod(30*time.Second),
		)
		transport = "SSE"
	} else {
		transportOpts = append(transportOpts,
			server.WithStreamableHTTP(true),
			server.WithSSE(false),
			server.WithEventStore(gohttp.NewMemoryEventStore(1000)),
		)
		transport = "Streamable HTTP"
	}

	// Build MCP handler and wrap with landing page at /. The landing page
	// needs deck names up front; we call ws.ListDecks() once at startup.
	mcpHandler := srv.Handler(transportOpts...)
	deckRefs, _ := ws.ListDecks()
	deckNames := make([]string, 0, len(deckRefs))
	for _, r := range deckRefs {
		deckNames = append(deckNames, r.Name)
	}
	handler := mcpWithLanding(mcpHandler, transport, mcpListen, deckNames, mcpToken != "")

	fmt.Fprintf(os.Stderr, "MCP server (%s) on %s — deck root: %s\n", transport, mcpListen, root)
	if mcpToken != "" {
		fmt.Fprintf(os.Stderr, "  Auth: bearer token (%s)\n", maskToken(mcpToken))
	}
	fmt.Fprintf(os.Stderr, "  http://%s/\n", mcpListen)
	printHTTPConfig(mcpListen, mcpToken)

	httpSrv := &http.Server{
		Addr:         mcpListen,
		Handler:      handler,
		WriteTimeout: 0, // SSE requires no write timeout
	}
	return listenAndServeGraceful(httpSrv)
}

// resolveMCPToken returns the token from the flag value, falling back to the
// SLYDS_MCP_TOKEN environment variable if the flag is empty.
func resolveMCPToken(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("SLYDS_MCP_TOKEN")
}

// resolveDeckRoot returns the workspace root in precedence order:
// explicit --deck-root flag → SLYDS_DECK_ROOT environment variable →
// current working directory ("."). Shared by `slyds mcp` and `slyds ws`
// so the fallback behavior is identical across commands.
func resolveDeckRoot(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if env := os.Getenv("SLYDS_DECK_ROOT"); env != "" {
		return env
	}
	return "."
}

// maskToken returns a redacted version of a token, showing only the first 2
// and last 2 characters with asterisks in between. Short tokens are fully masked.
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

// landingTmpl is parsed once from the embedded template.
var landingTmpl = func() *template.Template {
	tmplFS, _ := fs.Sub(assets.TemplatesFS, "templates")
	return template.Must(template.ParseFS(tmplFS, "mcp-landing.html.tmpl"))
}()

// mcpWithLanding wraps an MCP handler with a landing page at GET /.
// Non-root requests and non-GET requests pass through to the MCP handler.
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
// Embeds server.BaseErrorHandler for default no-op on unimplemented methods.
type slydsMCPErrorHandler struct {
	server.BaseErrorHandler
}

func (h *slydsMCPErrorHandler) OnSessionExpire(sessionID string, reason error) {
	fmt.Fprintf(os.Stderr, "MCP session expired: %s (%v)\n", sessionID, reason)
}

func (h *slydsMCPErrorHandler) OnKeepaliveFailure(sessionID string, consecutiveFailures int) {
	fmt.Fprintf(os.Stderr, "MCP keepalive failure: session=%s failures=%d\n", sessionID, consecutiveFailures)
}

