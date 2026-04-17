package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/ext/auth"
	"github.com/panyam/mcpkit/server"
	"github.com/spf13/cobra"
)

// MCPAuthConfig encapsulates MCP auth configuration — JWT validation, PRM
// endpoint, and scope enforcement. Designed for reuse across services;
// extract to mcpkit when a second consumer needs it.
type MCPAuthConfig struct {
	JWKSURL  string // JWKS endpoint for JWT validation
	Issuer   string // Expected JWT issuer
	Audience string // Expected JWT audience (defaults to server URL)
	Scopes   string // Comma-separated required scopes

	validator *auth.JWTValidator
}

// AddFlags registers auth-related CLI flags on a cobra command.
func (c *MCPAuthConfig) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&c.JWKSURL, "jwks-url", "", "JWKS endpoint for JWT validation (enables OAuth auth)")
	cmd.Flags().StringVar(&c.Issuer, "issuer", "", "Expected JWT issuer")
	cmd.Flags().StringVar(&c.Audience, "audience", "", "Expected JWT audience (defaults to server URL)")
	cmd.Flags().StringVar(&c.Scopes, "scopes", "", "Required scopes (comma-separated, e.g. 'slyds:read,slyds:write')")
}

// IsEnabled returns true when JWT auth is configured (--jwks-url is set).
func (c *MCPAuthConfig) IsEnabled() bool {
	return c.JWKSURL != ""
}

// ScopeList returns the parsed scopes as a string slice.
func (c *MCPAuthConfig) ScopeList() []string {
	if c.Scopes == "" {
		return nil
	}
	var scopes []string
	for _, s := range strings.Split(c.Scopes, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			scopes = append(scopes, s)
		}
	}
	return scopes
}

// ServerOptions returns mcpkit server options for auth. Returns nil if
// auth is not enabled, allowing callers to append unconditionally.
func (c *MCPAuthConfig) ServerOptions() []server.Option {
	if !c.IsEnabled() {
		return nil
	}
	c.validator = auth.NewJWTValidator(auth.JWTConfig{
		JWKSURL:        c.JWKSURL,
		Issuer:         c.Issuer,
		Audience:       c.Audience,
		RequiredScopes: c.ScopeList(),
	})
	return []server.Option{
		server.WithAuth(c.validator),
		server.WithExtension(auth.AuthExtension{}),
	}
}

// MountPRM registers the OAuth Protected Resource Metadata endpoint (RFC 9728)
// on the given mux. Also sets ResourceMetadataURL on the validator so 401
// responses include it in WWW-Authenticate — this is how clients discover
// the PRM endpoint and find the authorization server. No-op if auth is not enabled.
func (c *MCPAuthConfig) MountPRM(mux *http.ServeMux, resourceURI, mcpPath string) {
	if !c.IsEnabled() {
		return
	}
	// Wire the PRM URL into the validator so 401 WWW-Authenticate headers
	// include resource_metadata="..." for client discovery.
	if c.validator != nil {
		c.validator.ResourceMetadataURL = resourceURI + "/.well-known/oauth-protected-resource" + mcpPath
	}
	authServers := []string{}
	if c.Issuer != "" {
		authServers = append(authServers, c.Issuer)
	}
	auth.MountAuth(mux, auth.AuthConfig{
		ResourceURI:          resourceURI,
		AuthorizationServers: authServers,
		ScopesSupported:      c.ScopeList(),
		MCPPath:              mcpPath,
		Validator:            c.validator,
	})
}

// PrintInfo logs auth configuration to stderr.
func (c *MCPAuthConfig) PrintInfo() {
	if !c.IsEnabled() {
		return
	}
	fmt.Fprintf(os.Stderr, "  Auth: JWT (JWKS: %s, issuer: %s)\n", c.JWKSURL, c.Issuer)
	if c.Audience != "" {
		fmt.Fprintf(os.Stderr, "  Audience: %s\n", c.Audience)
	}
	if c.Scopes != "" {
		fmt.Fprintf(os.Stderr, "  Required scopes: %s\n", c.Scopes)
	}
}

// WriteScope is the scope required for mutation tools. Matches the
// scope name in Keycloak realm config (tests/keycloak/realm.json).
const WriteScope = "slyds-write"

// RequireWriteScope checks that the request has the slyds-write scope.
// Returns nil if auth is not configured (backward compatible).
func RequireWriteScope(ctx mcpcore.ToolContext) error {
	if mcpcore.AuthClaims(ctx) == nil {
		return nil
	}
	return auth.RequireScope(ctx, WriteScope)
}

// BuildMCPMux creates an http.ServeMux with the MCP handler at / and
// optionally mounts PRM endpoints for OAuth discovery. Shared by
// both slyds mcp and slyds mcp-proto.
func BuildMCPMux(mcpHandler http.Handler, authCfg *MCPAuthConfig) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/", mcpHandler)
	if authCfg != nil && authCfg.IsEnabled() {
		resourceURI := fmt.Sprintf("http://%s", mcpListen)
		if mcpPublicURL != "" {
			resourceURI = mcpPublicURL
		}
		authCfg.MountPRM(mux, resourceURI, "/mcp")
	}
	return mux
}

// PrintAuthInfo logs auth configuration to stderr. Shared by both
// slyds mcp and slyds mcp-proto.
func PrintAuthInfo(authCfg *MCPAuthConfig) {
	if authCfg != nil && authCfg.IsEnabled() {
		authCfg.PrintInfo()
	} else if mcpToken != "" {
		fmt.Fprintf(os.Stderr, "  Auth: bearer token (%s)\n", maskToken(mcpToken))
	}
}

// AuthServerOptions returns server options for the configured auth mode.
// JWT auth takes precedence over static bearer token.
func AuthServerOptions(authCfg *MCPAuthConfig) []server.Option {
	if authCfg != nil && authCfg.IsEnabled() {
		return authCfg.ServerOptions()
	}
	if mcpToken != "" {
		return []server.Option{server.WithBearerToken(mcpToken)}
	}
	return nil
}
