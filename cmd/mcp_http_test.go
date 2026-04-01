package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const mcpTestPrefix = "/mcp"

// mcpTestSaveGlobals restores MCP package flags after tests that mutate them.
func mcpTestSaveGlobals(t *testing.T) func() {
	t.Helper()
	oldToken := mcpToken
	oldOrigin := mcpAllowAnyOrigin
	oldPublic := mcpPublicURL
	return func() {
		mcpToken = oldToken
		mcpAllowAnyOrigin = oldOrigin
		mcpPublicURL = oldPublic
	}
}

func mcpTestMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(mcpTestPrefix+"/sse", mcpSSEHandler(mcpTestPrefix))
	mux.HandleFunc(mcpTestPrefix+"/message", mcpMessageHandler())
	return mux
}

// readSSEEvent reads one SSE event (until blank line after fields).
func readSSEEvent(br *bufio.Reader) (eventName string, data []byte, err error) {
	var ev string
	var dataBuf strings.Builder
	for {
		line, err := br.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", nil, err
		}
		line = strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if line == "" {
			if ev != "" || dataBuf.Len() > 0 {
				return ev, []byte(dataBuf.String()), nil
			}
			if err == io.EOF {
				return "", nil, io.EOF
			}
			continue
		}
		if strings.HasPrefix(line, "event:") {
			ev = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			if dataBuf.Len() > 0 {
				dataBuf.WriteByte('\n')
			}
			dataBuf.WriteString(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
		if err == io.EOF {
			if ev != "" || dataBuf.Len() > 0 {
				return ev, []byte(dataBuf.String()), nil
			}
			return "", nil, io.EOF
		}
	}
}

func TestMCPMessagePOSTPushesSSE(t *testing.T) {
	id := "deadbeefcafebabe"
	var pushed []byte
	mcpHubInst.register(id, func(event string, data []byte) error {
		if event != "message" {
			t.Errorf("event: %s", event)
		}
		pushed = append([]byte(nil), data...)
		return nil
	})
	defer mcpHubInst.unregister(id)

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`
	req := httptest.NewRequest(http.MethodPost, "/mcp/message?session="+id, strings.NewReader(body))
	req.Header.Set("Origin", "http://localhost")
	rr := httptest.NewRecorder()
	mcpMessageHandler()(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if len(pushed) == 0 {
		t.Fatal("expected SSE payload")
	}
	var resp jsonRPCResponse
	if err := json.Unmarshal(pushed, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Error != nil {
		t.Fatalf("rpc error: %+v", resp.Error)
	}
}

func TestMCPBuildMessageURL(t *testing.T) {
	cleanup := mcpTestSaveGlobals(t)
	defer cleanup()
	mcpPublicURL = ""

	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9999/mcp/sse", nil)
	r.Host = "127.0.0.1:9999"
	u := mcpBuildMessageURL(r, "/mcp", "abc123")
	if !strings.Contains(u, "session=abc123") || !strings.Contains(u, "/message") {
		t.Fatalf("unexpected url: %s", u)
	}
}

// TestMCPHTTPSSEFullFlow exercises GET /sse → endpoint event → POST /message → message event (real httptest server + Flusher).
func TestMCPHTTPSSEFullFlow(t *testing.T) {
	cleanup := mcpTestSaveGlobals(t)
	defer cleanup()
	mcpToken = ""
	mcpAllowAnyOrigin = false
	mcpPublicURL = ""

	srv := httptest.NewServer(mcpTestMux())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+mcpTestPrefix+"/sse", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /sse: %s", resp.Status)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("Content-Type: %q", ct)
	}

	br := bufio.NewReader(resp.Body)
	ev, data, err := readSSEEvent(br)
	if err != nil {
		t.Fatal(err)
	}
	if ev != "endpoint" {
		t.Fatalf("first event: %q want endpoint", ev)
	}
	var ep struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(data, &ep); err != nil {
		t.Fatalf("endpoint JSON: %v", err)
	}
	if ep.URL == "" || !strings.Contains(ep.URL, "/message") || !strings.Contains(ep.URL, "session=") {
		t.Fatalf("bad endpoint url: %q", ep.URL)
	}

	postBody := `{"jsonrpc":"2.0","id":42,"method":"tools/list"}`
	postReq, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, strings.NewReader(postBody))
	if err != nil {
		t.Fatal(err)
	}
	postReq.Header.Set("Content-Type", "application/json")
	postRes, err := http.DefaultClient.Do(postReq)
	if err != nil {
		t.Fatal(err)
	}
	defer postRes.Body.Close()
	if postRes.StatusCode != http.StatusAccepted {
		t.Fatalf("POST message: %s body %s", postRes.Status, readBodyString(postRes.Body))
	}

	ev2, data2, err := readSSEEvent(br)
	if err != nil {
		t.Fatal(err)
	}
	if ev2 != "message" {
		t.Fatalf("second event: %q want message", ev2)
	}
	var rpc jsonRPCResponse
	if err := json.Unmarshal(data2, &rpc); err != nil {
		t.Fatal(err)
	}
	if rpc.Error != nil {
		t.Fatalf("rpc error: %+v", rpc.Error)
	}
	if rpc.ID == nil {
		t.Fatal("missing id")
	}
	cancel()
}

func readBodyString(r io.Reader) string {
	b, _ := io.ReadAll(r)
	return string(b)
}

func TestMCPHTTPSSEUnauthorized(t *testing.T) {
	cleanup := mcpTestSaveGlobals(t)
	defer cleanup()
	mcpToken = "secret-token"
	mcpAllowAnyOrigin = true

	srv := httptest.NewServer(mcpTestMux())
	defer srv.Close()

	resp, err := http.Get(srv.URL + mcpTestPrefix + "/sse")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("want 401, got %s", resp.Status)
	}
}

func TestMCPHTTPSSEForbiddenOrigin(t *testing.T) {
	cleanup := mcpTestSaveGlobals(t)
	defer cleanup()
	mcpToken = ""
	mcpAllowAnyOrigin = false

	srv := httptest.NewServer(mcpTestMux())
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL+mcpTestPrefix+"/sse", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Origin", "https://evil.example")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %s", resp.Status)
	}
}

func TestMCPMessagePOSTMissingSession(t *testing.T) {
	cleanup := mcpTestSaveGlobals(t)
	defer cleanup()
	mcpToken = ""
	mcpAllowAnyOrigin = true

	req := httptest.NewRequest(http.MethodPost, "/mcp/message", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	rr := httptest.NewRecorder()
	mcpMessageHandler()(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", rr.Code)
	}
}

func TestMCPHTTPSSETokenAccepted(t *testing.T) {
	cleanup := mcpTestSaveGlobals(t)
	defer cleanup()
	mcpToken = "ok-token"
	mcpAllowAnyOrigin = true

	srv := httptest.NewServer(mcpTestMux())
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+mcpTestPrefix+"/sse", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer ok-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %s body %s", resp.Status, readBodyString(resp.Body))
	}
	cancel()
}
