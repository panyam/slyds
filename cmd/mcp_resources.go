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

// registerResources registers all MCP resources on the server.
func registerResources(srv *server.Server, root string) {
	// Static: server info
	srv.RegisterResource(
		mcpcore.ResourceDef{
			URI:         "slyds://server/info",
			Name:        "Server Info",
			Description: "slyds MCP server version, capabilities, and deck root",
			MimeType:    "application/json",
		},
		func(ctx context.Context, req mcpcore.ResourceRequest) (mcpcore.ResourceResult, error) {
			info := map[string]any{
				"name":      "slyds",
				"version":   Version,
				"deck_root": root,
				"themes":    core.AvailableThemeNames(),
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
	)

	// Template: list decks
	srv.RegisterResourceTemplate(
		mcpcore.ResourceTemplate{
			URITemplate: "slyds://decks",
			Name:        "Deck List",
			Description: "List all presentation decks found under the deck root",
			MimeType:    "application/json",
		},
		func(ctx context.Context, uri string, params map[string]string) (mcpcore.ResourceResult, error) {
			names := discoverDecks(root)
			var decks []map[string]any
			for _, name := range names {
				d, err := openDeck(root, name)
				if err != nil {
					continue
				}
				count, _ := d.SlideCount()
				decks = append(decks, map[string]any{
					"name":   name,
					"title":  d.Title(),
					"theme":  d.Theme(),
					"slides": count,
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
	)

	// Template: deck metadata
	srv.RegisterResourceTemplate(
		mcpcore.ResourceTemplate{
			URITemplate: "slyds://decks/{name}",
			Name:        "Deck Metadata",
			Description: "Structured description of a deck: title, theme, slides with metadata",
			MimeType:    "application/json",
		},
		func(ctx context.Context, uri string, params map[string]string) (mcpcore.ResourceResult, error) {
			d, err := openDeck(root, params["name"])
			if err != nil {
				return mcpcore.ResourceResult{}, fmt.Errorf("deck %q not found: %w", params["name"], err)
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
	)

	// Template: slide list
	srv.RegisterResourceTemplate(
		mcpcore.ResourceTemplate{
			URITemplate: "slyds://decks/{name}/slides",
			Name:        "Slide List",
			Description: "List all slides in a deck with position, filename, layout, title, and word count",
			MimeType:    "application/json",
		},
		func(ctx context.Context, uri string, params map[string]string) (mcpcore.ResourceResult, error) {
			d, err := openDeck(root, params["name"])
			if err != nil {
				return mcpcore.ResourceResult{}, fmt.Errorf("deck %q not found: %w", params["name"], err)
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
	)

	// Template: individual slide content
	srv.RegisterResourceTemplate(
		mcpcore.ResourceTemplate{
			URITemplate: "slyds://decks/{name}/slides/{n}",
			Name:        "Slide Content",
			Description: "Raw HTML content of a specific slide by position (1-based)",
			MimeType:    "text/html",
		},
		func(ctx context.Context, uri string, params map[string]string) (mcpcore.ResourceResult, error) {
			d, err := openDeck(root, params["name"])
			if err != nil {
				return mcpcore.ResourceResult{}, fmt.Errorf("deck %q not found: %w", params["name"], err)
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
	)

	// Template: deck config (.slyds.yaml)
	srv.RegisterResourceTemplate(
		mcpcore.ResourceTemplate{
			URITemplate: "slyds://decks/{name}/config",
			Name:        "Deck Configuration",
			Description: "Raw .slyds.yaml manifest content",
			MimeType:    "text/yaml",
		},
		func(ctx context.Context, uri string, params map[string]string) (mcpcore.ResourceResult, error) {
			d, err := openDeck(root, params["name"])
			if err != nil {
				return mcpcore.ResourceResult{}, fmt.Errorf("deck %q not found: %w", params["name"], err)
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
	)

	// Template: AGENT.md
	srv.RegisterResourceTemplate(
		mcpcore.ResourceTemplate{
			URITemplate: "slyds://decks/{name}/agent",
			Name:        "Agent Guide",
			Description: "AGENT.md content for the deck — commands, layouts, hooks, and conventions",
			MimeType:    "text/markdown",
		},
		func(ctx context.Context, uri string, params map[string]string) (mcpcore.ResourceResult, error) {
			d, err := openDeck(root, params["name"])
			if err != nil {
				return mcpcore.ResourceResult{}, fmt.Errorf("deck %q not found: %w", params["name"], err)
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
	)
}
