package cmd

import (
	"github.com/panyam/mcpkit/server"
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
	addCommonFlags(mcpProtoCmd)
	rootCmd.AddCommand(mcpProtoCmd)
}

// registerProtoTools registers proto-generated tools, resources, prompts,
// and completions on the server. Also registers hand-written app tools
// (outside proto scope).
func registerProtoTools(srv *server.Server) {
	impl := &SlydsServiceImpl{}
	slydsv1.RegisterSlydsServiceMCP(srv, impl)
	slydsv1.RegisterSlydsServiceMCPResources(srv, impl)
	slydsv1.RegisterSlydsServiceMCPCompletions(srv, impl)
	slydsv1.RegisterSlydsServiceMCPPrompts(srv, impl)
	registerAppTools(srv) // MCP Apps stays hand-written
}

func runMCPProtoServer() error {
	ws, err := initWorkspace()
	if err != nil {
		return err
	}

	cfg := &mcpServerConfig{ServerName: "slyds-proto", Workspace: ws}
	srv := cfg.buildServer()
	registerProtoTools(srv)

	if mcpUseStdio {
		return cfg.runStdio(srv)
	}
	return cfg.runHTTP(srv, nil) // no landing page for proto
}
