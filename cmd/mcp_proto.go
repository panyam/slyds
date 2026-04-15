package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/ext/ui"
	"github.com/panyam/mcpkit/server"
	gohttp "github.com/panyam/servicekit/http"
	"github.com/spf13/cobra"

	slydsv1 "github.com/panyam/slyds/gen/go/slyds/v1"
)

var mcpProtoCmd = &cobra.Command{
	Use:   "mcp-proto",
	Short: "MCP server using proto-generated tool and resource handlers (experimental)",
	Long: `mcp-proto starts an MCP server using the proto-generated SlydsService
implementation instead of the hand-written tool handlers. This is the
experimental path for validating protoc-gen-go-mcp code generation.

Same flags and behavior as 'slyds mcp' — only the tool/resource
registration path differs.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPProtoServer()
	},
}

func init() {
	// Reuse the same flags as mcp command
	mcpProtoCmd.Flags().StringVar(&mcpListen, "listen", "127.0.0.1:8274", "Listen address")
	mcpProtoCmd.Flags().StringVar(&mcpToken, "token", "", "Bearer token for authentication")
	mcpProtoCmd.Flags().StringVar(&mcpPublicURL, "public-url", "", "Public URL for reverse proxy")
	mcpProtoCmd.Flags().BoolVar(&mcpUseSSE, "sse", false, "Use legacy HTTP+SSE transport")
	mcpProtoCmd.Flags().BoolVar(&mcpUseStdio, "stdio", false, "Use stdio transport")
	mcpProtoCmd.Flags().StringVar(&mcpDeckRoot, "deck-root", "", "Root directory for deck discovery")
	mcpProtoCmd.Flags().StringSliceVar(&mcpAllowOrigins, "allow-origin", nil, "Allowed Origin headers. Use '*' for all.")
	rootCmd.AddCommand(mcpProtoCmd)
}

func runMCPProtoServer() error {
	ws, err := NewLocalWorkspace(resolveDeckRoot(mcpDeckRoot))
	if err != nil {
		return fmt.Errorf("invalid deck-root: %w", err)
	}
	root := ws.Root()
	mcpToken = resolveMCPToken(mcpToken)

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
			Name:    "slyds-proto",
			Version: Version,
		},
		serverOpts...,
	)

	// Proto-generated registration — this is the only difference from runMCPServer.
	impl := &SlydsServiceImpl{}
	slydsv1.RegisterSlydsServiceMCP(srv, impl)
	slydsv1.RegisterSlydsServiceMCPResources(srv, impl)

	// MCP Apps stays hand-written (outside proto scope).
	registerAppTools(srv)

	// Completions: proto-generated from completable_fields annotations.
	slydsv1.RegisterSlydsServiceMCPCompletions(srv, impl)

	// Proto-generated prompts.
	slydsv1.RegisterSlydsServiceMCPPrompts(srv, impl)

	if mcpUseStdio {
		fmt.Fprintf(os.Stderr, "MCP proto server (stdio) — deck root: %s\n", root)
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

	mcpHandler := srv.Handler(transportOpts...)
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mcpHandler.ServeHTTP(w, r)
	})

	fmt.Fprintf(os.Stderr, "MCP proto server (%s) on %s — deck root: %s\n", transport, mcpListen, root)
	if mcpToken != "" {
		fmt.Fprintf(os.Stderr, "  Auth: bearer token (%s)\n", maskToken(mcpToken))
	}
	fmt.Fprintf(os.Stderr, "  http://%s/\n", mcpListen)

	httpSrv := &http.Server{
		Addr:         mcpListen,
		Handler:      handler,
		WriteTimeout: 0,
	}
	return listenAndServeGraceful(httpSrv)
}
