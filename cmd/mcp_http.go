package cmd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	mcpListen           string
	mcpPublicURL        string
	mcpPathPrefix       string
	mcpToken            string
	mcpAllowAnyOrigin   bool
)

// mcpServeCmd runs the MCP HTTP+SSE transport (MCP spec 2024-11-05).
var mcpServeCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run MCP server over HTTP with Server-Sent Events (remote clients)",
	Long: `Starts an HTTP server for the Model Context Protocol "HTTP with SSE" transport.

Clients open GET <prefix>/sse (text/event-stream). The server sends an "endpoint"
event with a POST URL for JSON-RPC requests; responses arrive as SSE "message" events.

Bind defaults to 127.0.0.1 — use --public-url when behind a reverse proxy so the
endpoint event advertises the correct absolute URL for clients (e.g. Glean).

Security: validate Origin unless --dangerous-allow-any-origin; use --token for
a shared secret (Authorization: Bearer).`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPServeHTTP(cmd.Context())
	},
}

func init() {
	mcpServeCmd.Flags().StringVar(&mcpListen, "listen", "127.0.0.1:8787", "TCP address to bind (use 127.0.0.1 for local-only)")
	mcpServeCmd.Flags().StringVar(&mcpPublicURL, "public-url", "", "Public base URL for the endpoint event (e.g. https://mcp.example.com/mcp); if empty, derived from each request")
	mcpServeCmd.Flags().StringVar(&mcpPathPrefix, "path-prefix", "/mcp", "URL prefix for /sse and /message routes")
	mcpServeCmd.Flags().StringVar(&mcpToken, "token", "", "If set, require Authorization: Bearer <token> for SSE and POST")
	mcpServeCmd.Flags().BoolVar(&mcpAllowAnyOrigin, "dangerous-allow-any-origin", false, "Disable Origin header checks (not for production)")
}

type mcpHub struct {
	mu       sync.Mutex
	sessions map[string]*mcpSSESession
}

type mcpSSESession struct {
	send func(eventName string, data []byte) error
}

func (h *mcpHub) register(id string, send func(string, []byte) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.sessions == nil {
		h.sessions = make(map[string]*mcpSSESession)
	}
	h.sessions[id] = &mcpSSESession{send: send}
}

func (h *mcpHub) unregister(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.sessions, id)
}

func (h *mcpHub) push(id string, eventName string, data []byte) error {
	h.mu.Lock()
	s := h.sessions[id]
	h.mu.Unlock()
	if s == nil || s.send == nil {
		return fmt.Errorf("unknown or closed session")
	}
	return s.send(eventName, data)
}

var mcpHubInst mcpHub

func runMCPServeHTTP(ctx context.Context) error {
	prefix := strings.TrimSuffix(mcpPathPrefix, "/")
	if prefix == "" {
		prefix = "/mcp"
	}

	mux := http.NewServeMux()
	mux.HandleFunc(prefix+"/sse", mcpSSEHandler(prefix))
	mux.HandleFunc(prefix+"/message", mcpMessageHandler())

	srv := &http.Server{
		Addr:    mcpListen,
		Handler: mux,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	shutdown := func() error {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}

	select {
	case <-sigCh:
		return shutdown()
	case <-ctx.Done():
		return shutdown()
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		return nil
	}
}

func mcpSSEHandler(prefix string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !mcpCheckAuth(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !mcpOriginOK(r) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}

		fl, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		id, err := randomSessionID()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		send := func(eventName string, data []byte) error {
			_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventName, string(data))
			if err != nil {
				return err
			}
			fl.Flush()
			return nil
		}

		mcpHubInst.register(id, send)
		defer mcpHubInst.unregister(id)

		postURL := mcpBuildMessageURL(r, prefix, id)
		endpointPayload, _ := json.Marshal(map[string]string{"url": postURL})
		if err := send("endpoint", endpointPayload); err != nil {
			return
		}

		<-r.Context().Done()
	}
}

func randomSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func mcpMessageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !mcpCheckAuth(r) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !mcpOriginOK(r) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}

		id := r.URL.Query().Get("session")
		if id == "" {
			http.Error(w, "missing session", http.StatusBadRequest)
			return
		}

		var req jsonRPCRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		resp := handleMCPRequest(&req)
		if resp == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		out, err := json.Marshal(resp)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if err := mcpHubInst.push(id, "message", out); err != nil {
			http.Error(w, "session gone", http.StatusGone)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}
}

func mcpBuildMessageURL(r *http.Request, prefix, sessionID string) string {
	base := strings.TrimSuffix(mcpPublicURL, "/")
	if base == "" {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		host := r.Host
		if host == "" {
			host = mcpListen
		}
		// If listen is :8787 without host, use localhost
		if strings.HasPrefix(host, ":") {
			host = "127.0.0.1" + host
		}
		base = scheme + "://" + host
	}
	u, err := url.Parse(base + prefix + "/message")
	if err != nil {
		return base + prefix + "/message?session=" + url.QueryEscape(sessionID)
	}
	q := u.Query()
	q.Set("session", sessionID)
	u.RawQuery = q.Encode()
	return u.String()
}

func mcpCheckAuth(r *http.Request) bool {
	if mcpToken == "" {
		return true
	}
	h := r.Header.Get("Authorization")
	const p = "Bearer "
	if !strings.HasPrefix(h, p) {
		return false
	}
	return strings.TrimPrefix(h, p) == mcpToken
}

func mcpOriginOK(r *http.Request) bool {
	if mcpAllowAnyOrigin {
		return true
	}
	o := r.Header.Get("Origin")
	if o == "" {
		return true
	}
	u, err := url.Parse(o)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if host == "127.0.0.1" || host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}
