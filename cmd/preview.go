package cmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/template"

	"github.com/panyam/templar"
	"github.com/spf13/cobra"
	"github.com/panyam/slyds/core"
)

var previewPort int

var previewCmd = &cobra.Command{
	Use:   "preview <theme-dir>",
	Short: "Preview a theme by generating a sample deck and serving it",
	Long: `Preview a theme directory by scaffolding a temporary presentation
with one slide of each type defined in the theme's theme.yaml,
then serving it locally. The temp files are cleaned up on exit.

Works with any theme directory — built-in or external:

  slyds preview ./my-custom-theme
  slyds preview ~/themes/neon -p 8080`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		themeDir, err := filepath.Abs(args[0])
		if err != nil {
			return err
		}

		// Validate theme directory
		if _, err := os.Stat(filepath.Join(themeDir, "theme.yaml")); os.IsNotExist(err) {
			return fmt.Errorf("no theme.yaml found in %s — is this a slyds theme directory?", themeDir)
		}

		// Load theme config to discover all slide types
		cfg, err := core.LoadThemeConfigFromDir(themeDir)
		if err != nil {
			return err
		}

		// Create temp dir for the preview presentation
		tmpDir, err := os.MkdirTemp("", "slyds-preview-*")
		if err != nil {
			return fmt.Errorf("failed to create temp dir: %w", err)
		}

		// Clean up on exit
		defer os.RemoveAll(tmpDir)

		// Scaffold a sample presentation with standard slides (title + content + closing)
		title := fmt.Sprintf("%s Theme Preview", cfg.Name)
		if err := core.CreateFromDir(tmpDir, title, 3, themeDir); err != nil {
			return fmt.Errorf("failed to scaffold preview: %w", err)
		}

		// Add one slide for each non-standard type defined in theme.yaml
		standardTypes := map[string]bool{
			"title": true, "content": true, "closing": true,
		}
		slideNum := 4 // after title(1) + content(2) + closing(3)
		for typeName, tmplPath := range cfg.SlideTypes {
			if standardTypes[typeName] {
				continue
			}
			content, err := renderDiskSlide(themeDir, tmplPath, typeName, slideNum)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: skipping slide type %q: %v\n", typeName, err)
				continue
			}
			fileName := fmt.Sprintf("%02d-%s.html", slideNum, typeName)
			if err := os.WriteFile(filepath.Join(tmpDir, "slides", fileName), []byte(content), 0644); err != nil {
				return err
			}
			addIncludeToIndex(filepath.Join(tmpDir, "index.html"), fileName)
			slideNum++
		}

		// Serve the preview
		group := templar.NewTemplateGroup()
		group.Loader = (&templar.LoaderList{}).AddLoader(templar.NewFileSystemLoader(tmpDir))

		mux := http.NewServeMux()
		fileServer := http.FileServer(http.Dir(tmpDir))

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if path == "/" {
				path = "/index.html"
			}
			if filepath.Ext(path) == ".html" {
				templateName := path[1:]
				templates, err := group.Loader.Load(templateName, "")
				if err != nil {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if err := group.RenderHtmlTemplate(w, templates[0], "", map[string]any{}, nil); err != nil {
					log.Printf("Render error: %s: %v", templateName, err)
					http.Error(w, "Render error", http.StatusInternalServerError)
				}
				return
			}
			fileServer.ServeHTTP(w, r)
		})

		addr := fmt.Sprintf(":%d", previewPort)
		fmt.Printf("\nPreviewing theme %q from %s\n", cfg.Name, themeDir)
		fmt.Printf("Serving at http://localhost:%d\n", previewPort)
		fmt.Println("Press Ctrl+C to stop.")

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		server := &http.Server{
			Addr:        addr,
			Handler:     mux,
			BaseContext: func(_ net.Listener) context.Context { return context.Background() },
		}

		go func() {
			<-sigCh
			fmt.Println("\nCleaning up preview...")
			server.Shutdown(context.Background())
		}()

		err = server.ListenAndServe()
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	},
}

func init() {
	previewCmd.Flags().IntVarP(&previewPort, "port", "p", 3000, "port to serve on")
	rootCmd.AddCommand(previewCmd)
}

// renderDiskSlide renders a single slide template from a theme directory on disk.
func renderDiskSlide(themeDir, tmplPath, typeName string, number int) (string, error) {
	content, err := os.ReadFile(filepath.Join(themeDir, tmplPath))
	if err != nil {
		return "", fmt.Errorf("template %q not found: %w", tmplPath, err)
	}

	tmpl, err := template.New(tmplPath).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("failed to parse template %q: %w", tmplPath, err)
	}

	// Title-case the type name for display
	words := strings.Split(strings.ReplaceAll(typeName, "-", " "), " ")
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	displayName := strings.Join(words, " ")

	data := map[string]any{
		"Title":  displayName,
		"Number": number,
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// addIncludeToIndex inserts a templar include line for the given slide file
// into index.html, before the navigation div.
func addIncludeToIndex(indexPath, slideFileName string) error {
	content, err := os.ReadFile(indexPath)
	if err != nil {
		return err
	}

	includeLine := fmt.Sprintf(`    {{# include "slides/%s" #}}`, slideFileName)
	lines := strings.Split(string(content), "\n")
	var newLines []string
	for _, line := range lines {
		if strings.Contains(line, `<div class="navigation">`) {
			newLines = append(newLines, includeLine)
		}
		newLines = append(newLines, line)
	}

	return os.WriteFile(indexPath, []byte(strings.Join(newLines, "\n")), 0644)
}
