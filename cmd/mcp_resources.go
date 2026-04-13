package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	mcpcore "github.com/panyam/mcpkit/core"
	"github.com/panyam/mcpkit/server"
	"github.com/panyam/slyds/core"
)

// registerResources registers all MCP resources on the server using
// single-struct registration (mcpkit v0.1.15). Every handler resolves the
// active Workspace from request context, matching the tool-handler pattern.
// This keeps localhost and future hosted deployments on the same code path.
func registerResources(srv *server.Server) {
	srv.Register(
		// Static: server info
		server.Resource{
			ResourceDef: mcpcore.ResourceDef{
				URI:         "slyds://server/info",
				Name:        "Server Info",
				Description: "slyds MCP server version, capabilities, and workspace root",
				MimeType:    "application/json",
			},
			Handler: func(ctx mcpcore.ResourceContext, req mcpcore.ResourceRequest) (mcpcore.ResourceResult, error) {
				ws := workspaceFromContext(ctx)
				info := map[string]any{
					"name":    "slyds",
					"version": Version,
					"themes":  core.AvailableThemeNames(),
				}
				if lw, ok := ws.(*LocalWorkspace); ok {
					info["deck_root"] = lw.Root()
				}
				layouts, _ := core.ListLayouts()
				info["layouts"] = layouts
				data, _ := json.Marshal(info)
				return mcpcore.ResourceResult{
					Contents: []mcpcore.ResourceReadContent{{
						URI:      "slyds://server/info",
						MimeType: "application/json",
						Text:     string(data),
					}},
				}, nil
			},
		},

		// Template: list decks
		server.ResourceTemplate{
			ResourceTemplate: mcpcore.ResourceTemplate{
				URITemplate: "slyds://decks",
				Name:        "Deck List",
				Description: "List all presentation decks visible to the current workspace",
				MimeType:    "application/json",
			},
			Handler: func(ctx mcpcore.ResourceContext, uri string, params map[string]string) (mcpcore.ResourceResult, error) {
				ws := workspaceFromContext(ctx)
				if ws == nil {
					return mcpcore.ResourceResult{}, fmt.Errorf("internal: no workspace on context")
				}
				refs, err := ws.ListDecks()
				if err != nil {
					return mcpcore.ResourceResult{}, err
				}
				var decks []deckSummary
				for _, ref := range refs {
					d, err := ws.OpenDeck(ref.Name)
					if err != nil {
						continue
					}
					count, _ := d.SlideCount()
					decks = append(decks, deckSummary{
						Name:   ref.Name,
						Title:  d.Title(),
						Theme:  d.Theme(),
						Slides: count,
					})
				}
				data, _ := json.Marshal(decks)
				return mcpcore.ResourceResult{
					Contents: []mcpcore.ResourceReadContent{{
						URI:      uri,
						MimeType: "application/json",
						Text:     string(data),
					}},
				}, nil
			},
		},

		// Template: deck metadata
		server.ResourceTemplate{
			ResourceTemplate: mcpcore.ResourceTemplate{
				URITemplate: "slyds://decks/{name}",
				Name:        "Deck Metadata",
				Description: "Structured description of a deck: title, theme, slides with metadata",
				MimeType:    "application/json",
			},
			Handler: func(ctx mcpcore.ResourceContext, uri string, params map[string]string) (mcpcore.ResourceResult, error) {
				d, err := openDeckForResource(ctx, params["name"])
				if err != nil {
					return mcpcore.ResourceResult{}, err
				}
				desc, err := d.Describe()
				if err != nil {
					return mcpcore.ResourceResult{}, err
				}
				data, _ := json.Marshal(desc)
				return mcpcore.ResourceResult{
					Contents: []mcpcore.ResourceReadContent{{
						URI:      uri,
						MimeType: "application/json",
						Text:     string(data),
					}},
				}, nil
			},
		},

		// Template: slide list
		server.ResourceTemplate{
			ResourceTemplate: mcpcore.ResourceTemplate{
				URITemplate: "slyds://decks/{name}/slides",
				Name:        "Slide List",
				Description: "List all slides in a deck with position, filename, layout, title, and word count",
				MimeType:    "application/json",
			},
			Handler: func(ctx mcpcore.ResourceContext, uri string, params map[string]string) (mcpcore.ResourceResult, error) {
				d, err := openDeckForResource(ctx, params["name"])
				if err != nil {
					return mcpcore.ResourceResult{}, err
				}
				desc, err := d.Describe()
				if err != nil {
					return mcpcore.ResourceResult{}, err
				}
				data, _ := json.Marshal(desc.Slides)
				return mcpcore.ResourceResult{
					Contents: []mcpcore.ResourceReadContent{{
						URI:      uri,
						MimeType: "application/json",
						Text:     string(data),
					}},
				}, nil
			},
		},

		// Template: individual slide content
		server.ResourceTemplate{
			ResourceTemplate: mcpcore.ResourceTemplate{
				URITemplate: "slyds://decks/{name}/slides/{n}",
				Name:        "Slide Content",
				Description: "Raw HTML content of a specific slide by position (1-based)",
				MimeType:    "text/html",
			},
			Handler: func(ctx mcpcore.ResourceContext, uri string, params map[string]string) (mcpcore.ResourceResult, error) {
				d, err := openDeckForResource(ctx, params["name"])
				if err != nil {
					return mcpcore.ResourceResult{}, err
				}
				n, err := strconv.Atoi(params["n"])
				if err != nil {
					return mcpcore.ResourceResult{}, fmt.Errorf("invalid slide number %q", params["n"])
				}
				content, err := d.GetSlideContent(n)
				if err != nil {
					return mcpcore.ResourceResult{}, err
				}
				return mcpcore.ResourceResult{
					Contents: []mcpcore.ResourceReadContent{{
						URI:      uri,
						MimeType: "text/html",
						Text:     content,
					}},
				}, nil
			},
		},

		// Template: deck config (.slyds.yaml)
		server.ResourceTemplate{
			ResourceTemplate: mcpcore.ResourceTemplate{
				URITemplate: "slyds://decks/{name}/config",
				Name:        "Deck Configuration",
				Description: "Raw .slyds.yaml manifest content",
				MimeType:    "text/yaml",
			},
			Handler: func(ctx mcpcore.ResourceContext, uri string, params map[string]string) (mcpcore.ResourceResult, error) {
				d, err := openDeckForResource(ctx, params["name"])
				if err != nil {
					return mcpcore.ResourceResult{}, err
				}
				data, err := d.FS.ReadFile(".slyds.yaml")
				if err != nil {
					return mcpcore.ResourceResult{}, fmt.Errorf("no .slyds.yaml in deck %q", params["name"])
				}
				return mcpcore.ResourceResult{
					Contents: []mcpcore.ResourceReadContent{{
						URI:      uri,
						MimeType: "text/yaml",
						Text:     string(data),
					}},
				}, nil
			},
		},

		// Template: AGENT.md
		server.ResourceTemplate{
			ResourceTemplate: mcpcore.ResourceTemplate{
				URITemplate: "slyds://decks/{name}/agent",
				Name:        "Agent Guide",
				Description: "AGENT.md content for the deck — commands, layouts, hooks, and conventions",
				MimeType:    "text/markdown",
			},
			Handler: func(ctx mcpcore.ResourceContext, uri string, params map[string]string) (mcpcore.ResourceResult, error) {
				d, err := openDeckForResource(ctx, params["name"])
				if err != nil {
					return mcpcore.ResourceResult{}, err
				}
				data, err := d.FS.ReadFile("AGENT.md")
				if err != nil {
					return mcpcore.ResourceResult{}, fmt.Errorf("no AGENT.md in deck %q", params["name"])
				}
				return mcpcore.ResourceResult{
					Contents: []mcpcore.ResourceReadContent{{
						URI:      uri,
						MimeType: "text/markdown",
						Text:     string(data),
					}},
				}, nil
			},
		},
	)
}

// openDeckForResource is the resource-handler twin of openDeckFromContext.
// It returns a plain error (not a ToolResult) so resource handlers can use
// their standard (ResourceResult, error) return signature.
func openDeckForResource(ctx context.Context, name string) (*core.Deck, error) {
	ws := workspaceFromContext(ctx)
	if ws == nil {
		return nil, fmt.Errorf("internal: no workspace on context")
	}
	d, err := ws.OpenDeck(name)
	if err != nil {
		return nil, fmt.Errorf("deck %q not found: %w", name, err)
	}
	return d, nil
}
