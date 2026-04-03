package cmd

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"

	"github.com/panyam/slyds/core"
	"github.com/panyam/templar"
	"github.com/spf13/cobra"
)

var servePort int

var serveCmd = &cobra.Command{
	Use:   "serve [dir]",
	Short: "Start a local dev server with live template rendering",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		root, err := core.FindDeckRoot(dir)
		if err != nil {
			return err
		}

		// Set up templar for on-the-fly include resolution.
		// Uses SourceLoader if .slyds.yaml declares sources, otherwise FileSystemLoader.
		group := templar.NewTemplateGroup()
		group.Loader = core.NewLoaderForDeck(root)

		mux := http.NewServeMux()

		// Serve static files (CSS, JS, images, slide assets)
		fileServer := http.FileServer(http.Dir(root))

		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if path == "/" {
				path = "/index.html"
			}

			// Only render .html files through templar (to resolve includes)
			if filepath.Ext(path) == ".html" {
				templateName := path[1:] // strip leading /
				templates, err := group.Loader.Load(templateName, "")
				if err != nil {
					log.Printf("Template load error: %s: %v", templateName, err)
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if err := group.RenderHtmlTemplate(w, templates[0], "", map[string]any{}, nil); err != nil {
					log.Printf("Template render error: %s: %v", templateName, err)
					http.Error(w, "Render error", http.StatusInternalServerError)
				}
				return
			}

			// Everything else: serve as static file
			fileServer.ServeHTTP(w, r)
		})

		addr := fmt.Sprintf(":%d", servePort)
		fmt.Printf("\nServing %s at http://localhost:%d\n", root, servePort)
		fmt.Println("Press Ctrl+C to stop.")

		server := &http.Server{
			Addr:        addr,
			Handler:     mux,
			BaseContext: func(_ net.Listener) context.Context { return context.Background() },
		}
		return server.ListenAndServe()
	},
}

func init() {
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 3000, "port to serve on")
	rootCmd.AddCommand(serveCmd)
}
