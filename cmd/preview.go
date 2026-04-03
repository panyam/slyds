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
	"syscall"

	"github.com/panyam/slyds/core"
	"github.com/panyam/templar"
	"github.com/spf13/cobra"
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

		// Load theme from directory
		theme, err := core.LoadTheme(templar.NewLocalFS(themeDir))
		if err != nil {
			return fmt.Errorf("not a slyds theme directory: %w", err)
		}
		cfg := theme.Config

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

		// Open as Deck and add extra slides for non-standard theme types
		d, err := core.OpenDeckDir(tmpDir)
		if err != nil {
			return err
		}
		standardTypes := map[string]bool{
			"title": true, "content": true, "closing": true,
		}
		slideNum := 4
		for typeName := range cfg.SlideTypes {
			if standardTypes[typeName] {
				continue
			}
			content, err := theme.RenderSlideWithTitle(typeName, typeName, slideNum, "")
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: skipping slide type %q: %v\n", typeName, err)
				continue
			}
			fileName := fmt.Sprintf("%02d-%s.html", slideNum, typeName)
			if err := d.AddSlide(slideNum, fileName, content); err != nil {
				return err
			}
			slideNum++
		}
		_ = d // deck used for slide insertion above

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


