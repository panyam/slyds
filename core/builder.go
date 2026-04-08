package core

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/panyam/templar"
)

// Result holds the build output.
type Result struct {
	HTML     string   `json:"html"`
	Warnings []string `json:"warnings"`
}

// Build reads index.html from the deck's FS, resolves all templar includes,
// inlines CSS/JS/images, and returns a self-contained HTML string.
// All I/O goes through the deck's WritableFS.
func (d *Deck) Build() (*Result, error) {
	// Set up templar loader backed by the deck's FS
	group := templar.NewTemplateGroup()
	group.Loader = &templar.FileSystemLoader{
		Folders:    []templar.FSFolder{{FS: d.FS, Path: "."}},
		Extensions: []string{"html"},
	}

	templates, err := group.Loader.Load("index.html", "")
	if err != nil {
		return nil, fmt.Errorf("failed to load index.html: %w", err)
	}
	if len(templates) == 0 {
		return nil, fmt.Errorf("no templates loaded from index.html")
	}

	// Render the template (resolves all includes)
	var buf bytes.Buffer
	err = group.RenderHtmlTemplate(&buf, templates[0], "", map[string]any{}, nil)
	if err != nil {
		return nil, fmt.Errorf("template rendering failed: %w", err)
	}

	html := buf.String()

	// Inline assets via FS
	result, err := d.inlineAssets(html)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Build is the package-level function for backward compatibility.
// Prefers Deck.Build() for new code.
func Build(root string) (*Result, error) {
	d, err := OpenDeck(templar.NewLocalFS(root))
	if err != nil {
		return nil, err
	}
	return d.Build()
}

// FlattenIncludes manually resolves {{# include "path" #}} directives
// using the deck's FS. Fallback if templar rendering doesn't work.
func (d *Deck) FlattenIncludes(html string) (string, error) {
	for {
		idx := strings.Index(html, "{{#")
		if idx == -1 {
			break
		}
		end := strings.Index(html[idx:], "#}}")
		if end == -1 {
			break
		}
		end += idx + 3

		directive := html[idx:end]
		directive = strings.TrimPrefix(directive, "{{#")
		directive = strings.TrimSuffix(directive, "#}}")
		directive = strings.TrimSpace(directive)

		if !strings.HasPrefix(directive, "include") {
			html = html[:idx] + html[end:]
			continue
		}

		parts := strings.Fields(directive)
		if len(parts) < 2 {
			return "", fmt.Errorf("malformed include directive: %s", html[idx:end])
		}
		path := strings.Trim(parts[1], `"'`)

		content, err := d.FS.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("failed to include %q: %w", path, err)
		}

		html = html[:idx] + string(content) + html[end:]
	}
	return html, nil
}

// FlattenIncludes is the package-level function for backward compatibility.
func FlattenIncludes(html string, root string) (string, error) {
	d := &Deck{FS: templar.NewLocalFS(root)}
	return d.FlattenIncludes(html)
}
