package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/ext/ui"
	"github.com/panyam/mcpkit/server"
	gohttp "github.com/panyam/servicekit/http"
	"github.com/spf13/cobra"
)

// mcpServerConfig holds the shared configuration for both slyds mcp and
// slyds mcp-proto. Extracted to eliminate duplication — the only difference
// between the two paths is how tools/resources are registered on the server.
type mcpServerConfig struct {
	ServerName string // "slyds" or "slyds-proto"
	Workspace  Workspace
}

// buildServer creates a configured mcpkit server with workspace middleware,
// auth, verbose logging, and UI extension. Callers register their own
// tools/resources on the returned server.
func (c *mcpServerConfig) buildServer() *server.Server {
	var opts []server.Option
	opts = append(opts, server.WithListen(mcpListen))
	opts = append(opts, server.WithExtension(ui.UIExtension{}))
	opts = append(opts, server.WithErrorHandler(&slydsMCPErrorHandler{}))
	opts = append(opts, server.WithMiddleware(workspaceMiddleware(c.Workspace)))
	opts = append(opts, AuthServerOptions(&mcpAuth)...)
	if mcpVerbose {
		opts = append(opts, server.WithRequestLogging(log.New(os.Stderr, fmt.Sprintf("[%s] ", c.ServerName), log.LstdFlags)))
	}

	return server.NewServer(
		mcpcore.ServerInfo{
			Name:    c.ServerName,
			Version: Version,
		},
		opts...,
	)
}

// runStdio runs the server in stdio mode (for editor-spawned processes).
func (c *mcpServerConfig) runStdio(srv *server.Server) error {
	root := ""
	if lw, ok := c.Workspace.(*LocalWorkspace); ok {
		root = lw.Root()
	}
	fmt.Fprintf(os.Stderr, "MCP %s server (stdio) — deck root: %s\n", c.ServerName, root)
	PrintAuthInfo(&mcpAuth)
	if !mcpAuth.IsEnabled() && mcpToken != "" {
		printStdioConfig(root, mcpToken)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return srv.RunStdio(ctx)
}

// runHTTP runs the server over HTTP (Streamable HTTP or SSE).
// The handler parameter wraps the MCP handler (e.g., with a landing page).
// If handler is nil, the raw MCP handler is used.
func (c *mcpServerConfig) runHTTP(srv *server.Server, wrapHandler func(http.Handler) http.Handler) error {
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

	mcpHandler := srv.Handler(transportOpts...)
	handler := http.Handler(mcpHandler)
	if wrapHandler != nil {
		handler = wrapHandler(mcpHandler)
	}
	mux := BuildMCPMux(handler, &mcpAuth)

	root := ""
	if lw, ok := c.Workspace.(*LocalWorkspace); ok {
		root = lw.Root()
	}
	fmt.Fprintf(os.Stderr, "MCP %s server (%s) on %s — deck root: %s\n", c.ServerName, transport, mcpListen, root)
	PrintAuthInfo(&mcpAuth)
	fmt.Fprintf(os.Stderr, "  http://%s/\n", mcpListen)

	httpSrv := &http.Server{
		Addr:         mcpListen,
		Handler:      mux,
		WriteTimeout: 0, // SSE requires no write timeout
	}
	return listenAndServeGraceful(httpSrv)
}

// initWorkspace creates a LocalWorkspace from flags/env and resolves the token.
func initWorkspace() (Workspace, error) {
	ws, err := NewLocalWorkspace(resolveDeckRoot(mcpDeckRoot))
	if err != nil {
		return nil, fmt.Errorf("invalid deck-root: %w", err)
	}
	mcpToken = resolveMCPToken(mcpToken)
	return ws, nil
}

// addCommonFlags registers flags shared by both mcp and mcp-proto commands.
func addCommonFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&mcpListen, "listen", "127.0.0.1:8274", "Listen address")
	cmd.Flags().StringVar(&mcpToken, "token", "", "Bearer token for authentication")
	cmd.Flags().StringVar(&mcpPublicURL, "public-url", "", "Public URL for reverse proxy")
	cmd.Flags().BoolVar(&mcpUseSSE, "sse", false, "Use legacy HTTP+SSE transport")
	cmd.Flags().BoolVar(&mcpUseStdio, "stdio", false, "Use stdio transport")
	cmd.Flags().StringVar(&mcpDeckRoot, "deck-root", "", "Root directory for deck discovery (default: $SLYDS_DECK_ROOT, or current directory)")
	cmd.Flags().StringSliceVar(&mcpAllowOrigins, "allow-origin", nil, "Allowed Origin headers (default: localhost only). Use '*' to allow all origins")
	cmd.Flags().BoolVar(&mcpAppBridge, "app-bridge", true, "Inject MCP App Bridge into previews")
	cmd.Flags().BoolVar(&mcpVerbose, "verbose", false, "Log HTTP requests and auth events to stderr")
	mcpAuth.AddFlags(cmd)
}
