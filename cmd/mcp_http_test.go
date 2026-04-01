package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
	r := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:9999/mcp/sse", nil)
	r.Host = "127.0.0.1:9999"
	mcpPublicURL = ""
	u := mcpBuildMessageURL(r, "/mcp", "abc123")
	if !strings.Contains(u, "session=abc123") || !strings.Contains(u, "/message") {
		t.Fatalf("unexpected url: %s", u)
	}
}
