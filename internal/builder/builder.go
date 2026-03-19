package builder

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/panyam/templar"
)

// Result holds the build output.
type Result struct {
	HTML     string
	Warnings []string
}

// Build reads index.html from root, resolves all templar includes,
// inlines CSS/JS/images, and returns a self-contained HTML string.
func Build(root string) (*Result, error) {
	// Use templar to resolve includes
	group := templar.NewTemplateGroup()
	group.Loader = (&templar.LoaderList{}).AddLoader(templar.NewFileSystemLoader(root))

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

	// Inline assets
	result, err := InlineAssets(html, root)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// FlattenIncludes manually resolves {{# include "path" #}} directives.
// This is a fallback if templar rendering doesn't work as expected.
func FlattenIncludes(html string, root string) (string, error) {
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
		// Parse: {{# include "path" #}}
		directive = strings.TrimPrefix(directive, "{{#")
		directive = strings.TrimSuffix(directive, "#}}")
		directive = strings.TrimSpace(directive)

		if !strings.HasPrefix(directive, "include") {
			// Skip non-include directives
			html = html[:idx] + html[end:]
			continue
		}

		// Extract path
		parts := strings.Fields(directive)
		if len(parts) < 2 {
			return "", fmt.Errorf("malformed include directive: %s", html[idx:end])
		}
		path := strings.Trim(parts[1], `"'`)
		fullPath := filepath.Join(root, path)

		content, err := os.ReadFile(fullPath)
		if err != nil {
			return "", fmt.Errorf("failed to include %q: %w", path, err)
		}

		html = html[:idx] + string(content) + html[end:]
	}
	return html, nil
}
