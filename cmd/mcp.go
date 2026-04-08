package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/slyds/core"
	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Model Context Protocol server with semantic deck tools and resources",
	Long: `mcp starts an MCP server exposing slyds deck operations as semantic tools
and deck content as browsable resources.

  slyds mcp                          Streamable HTTP on 127.0.0.1:6274
  slyds mcp --sse                    Legacy HTTP+SSE transport
  slyds mcp --deck-root ./decks      Serve decks from a specific directory`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPServer()
	},
}

var (
	mcpListen    string
	mcpToken     string
	mcpPublicURL string
	mcpUseSSE    bool
	mcpUseStdio  bool
	mcpDeckRoot  string
)

func init() {
	mcpCmd.Flags().StringVar(&mcpListen, "listen", "127.0.0.1:6274", "Listen address")
	mcpCmd.Flags().StringVar(&mcpToken, "token", "", "Bearer token for authentication")
	mcpCmd.Flags().StringVar(&mcpPublicURL, "public-url", "", "Public URL for reverse proxy")
	mcpCmd.Flags().BoolVar(&mcpUseSSE, "sse", false, "Use legacy HTTP+SSE transport")
	mcpCmd.Flags().BoolVar(&mcpUseStdio, "stdio", false, "Use stdio transport (Content-Length framed JSON-RPC on stdin/stdout)")
	mcpCmd.Flags().StringVar(&mcpDeckRoot, "deck-root", ".", "Root directory for deck discovery")
	rootCmd.AddCommand(mcpCmd)
}

func runMCPServer() error {
	// Resolve deck root to absolute path
	root, err := filepath.Abs(mcpDeckRoot)
	if err != nil {
		return fmt.Errorf("invalid deck-root: %w", err)
	}

	// Token from env if flag not set
	mcpToken = resolveMCPToken(mcpToken)

	// Build server
	var serverOpts []server.Option
	serverOpts = append(serverOpts, server.WithListen(mcpListen))
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

	// Register resources and tools
	registerResources(srv, root)
	registerTools(srv, root)

	// Transport selection
	if mcpUseStdio {
		fmt.Fprintf(os.Stderr, "MCP server (stdio) — deck root: %s\n", root)
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()
		return srv.RunStdio(ctx)
	}

	var transportOpts []server.TransportOption
	if mcpPublicURL != "" {
		transportOpts = append(transportOpts, server.WithPublicURL(mcpPublicURL))
	}
	if mcpUseSSE {
		transportOpts = append(transportOpts, server.WithSSE(true), server.WithStreamableHTTP(false))
		fmt.Fprintf(os.Stderr, "MCP server (SSE) on %s — deck root: %s\n", mcpListen, root)
	} else {
		transportOpts = append(transportOpts, server.WithStreamableHTTP(true), server.WithSSE(false))
		fmt.Fprintf(os.Stderr, "MCP server (Streamable HTTP) on %s — deck root: %s\n", mcpListen, root)
	}

	return srv.ListenAndServe(transportOpts...)
}

// discoverDecks finds all deck directories under root.
// A deck is a directory containing index.html.
func discoverDecks(root string) []string {
	var decks []string

	// Check if root itself is a deck
	if _, err := os.Stat(filepath.Join(root, "index.html")); err == nil {
		decks = append(decks, ".")
	}

	// Check subdirectories
	entries, err := os.ReadDir(root)
	if err != nil {
		return decks
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, e.Name(), "index.html")); err == nil {
			decks = append(decks, e.Name())
		}
	}
	return decks
}

// resolveMCPToken returns the token from the flag value, falling back to the
// MCP_TOKEN environment variable if the flag is empty.
func resolveMCPToken(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("SLYDS_MCP_TOKEN")
}

// openDeck resolves a deck name to a Deck instance.
func openDeck(root, name string) (*core.Deck, error) {
	var dir string
	if name == "." || name == "" {
		dir = root
	} else {
		dir = filepath.Join(root, name)
	}
	return core.OpenDeckDir(dir)
}
