package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Run the Model Context Protocol server (stdio, JSON-RPC)",
	Long: `mcp runs a thin MCP server on stdin/stdout. It exposes one tool, "slyds",
which executes the slyds binary with the given arguments in the given working
directory. No presentation logic lives in the MCP layer — it is a transport
wrapper for agents (e.g. Glean) that cannot shell out reliably.

Set MCP_MIN_VERSION in tool arguments to fail fast if the installed slyds is too old.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runMCPServer()
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

// jsonRPCRequest is a minimal JSON-RPC 2.0 request envelope.
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

// jsonRPCResponse is a minimal JSON-RPC 2.0 response.
type jsonRPCResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *jsonRPCError   `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func runMCPServer() error {
	r := bufio.NewReader(os.Stdin)
	for {
		msg, err := readMCPMessage(r)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		var req jsonRPCRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			continue
		}
		resp := handleMCPRequest(&req)
		if resp == nil {
			continue
		}
		out, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		if err := writeMCPMessage(os.Stdout, out); err != nil {
			return err
		}
	}
}

func readMCPMessage(r *bufio.Reader) ([]byte, error) {
	var line string
	for {
		l, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(l)
		if line != "" {
			break
		}
	}
	lower := strings.ToLower(line)
	if !strings.HasPrefix(lower, "content-length:") {
		return nil, fmt.Errorf("expected Content-Length header, got %q", line)
	}
	idx := strings.Index(line, ":")
	if idx < 0 {
		return nil, fmt.Errorf("invalid header %q", line)
	}
	n, err := strconv.Atoi(strings.TrimSpace(line[idx+1:]))
	if err != nil {
		return nil, fmt.Errorf("parse Content-Length from %q: %w", line, err)
	}
	if _, err := r.ReadString('\n'); err != nil {
		return nil, err
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func writeMCPMessage(w io.Writer, body []byte) error {
	if _, err := fmt.Fprintf(w, "Content-Length: %d\r\n\r\n", len(body)); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

func handleMCPRequest(req *jsonRPCRequest) *jsonRPCResponse {
	id := req.ID
	if id == nil {
		id = json.RawMessage("null")
	}

	switch req.Method {
	case "initialize":
		res := map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]string{
				"name":    "slyds",
				"version": Version,
			},
		}
		raw, _ := json.Marshal(res)
		return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Result: raw}

	case "notifications/initialized", "initialized":
		return nil

	case "tools/list":
		res := map[string]any{
			"tools": []map[string]any{
				{
					"name":        "slyds",
					"description": "Run slyds CLI with args in cwd; stdout is returned as text content.",
					"inputSchema": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"cwd": map[string]any{
								"type":        "string",
								"description": "Working directory for the subprocess (deck root or subdirectory).",
							},
							"args": map[string]any{
								"type":        "array",
								"items":       map[string]any{"type": "string"},
								"description": "Arguments after `slyds`, e.g. [\"introspect\"] or [\"query\",\"1\",\"h1\",\"--count\"]",
							},
							"min_version": map[string]any{
								"type":        "string",
								"description": "Optional minimum slyds version (e.g. 0.0.10). Tool fails if current binary is older.",
							},
						},
						"required": []string{"cwd", "args"},
					},
				},
			},
		}
		raw, _ := json.Marshal(res)
		return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Result: raw}

	case "tools/call":
		return handleToolsCall(id, req.Params)

	default:
		errObj := jsonRPCError{Code: -32601, Message: "method not found: " + req.Method}
		return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Error: &errObj}
	}
}

type slydsToolParams struct {
	Cwd        string   `json:"cwd"`
	Args       []string `json:"args"`
	MinVersion string   `json:"min_version,omitempty"`
}

func handleToolsCall(id json.RawMessage, params json.RawMessage) *jsonRPCResponse {
	var envelope struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(params, &envelope); err != nil {
		e := jsonRPCError{Code: -32602, Message: err.Error()}
		return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Error: &e}
	}
	if envelope.Name != "slyds" {
		e := jsonRPCError{Code: -32602, Message: "unknown tool: " + envelope.Name}
		return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Error: &e}
	}

	var p slydsToolParams
	if err := json.Unmarshal(envelope.Arguments, &p); err != nil {
		e := jsonRPCError{Code: -32602, Message: err.Error()}
		return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Error: &e}
	}
	if p.Cwd == "" {
		e := jsonRPCError{Code: -32602, Message: "cwd is required"}
		return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Error: &e}
	}
	if p.MinVersion != "" && !versionAtLeast(Version, p.MinVersion) {
		e := jsonRPCError{Code: -32603, Message: fmt.Sprintf("slyds version %s is older than min_version %s", Version, p.MinVersion)}
		return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Error: &e}
	}

	self, err := os.Executable()
	if err != nil {
		e := jsonRPCError{Code: -32603, Message: err.Error()}
		return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Error: &e}
	}
	cmd := exec.Command(self, p.Args...)
	cmd.Dir = p.Cwd
	out, err := cmd.CombinedOutput()
	text := string(out)
	if err != nil {
		if text != "" {
			text = text + "\n" + err.Error()
		} else {
			text = err.Error()
		}
		res := map[string]any{
			"content": []map[string]string{
				{"type": "text", "text": text},
			},
			"isError": true,
		}
		raw, _ := json.Marshal(res)
		return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Result: raw}
	}
	res := map[string]any{
		"content": []map[string]string{
			{"type": "text", "text": text},
		},
	}
	raw, _ := json.Marshal(res)
	return &jsonRPCResponse{JSONRPC: "2.0", ID: id, Result: raw}
}

// versionAtLeast returns true if current >= min, using dotted numeric segments (simple semver subset).
func versionAtLeast(current, min string) bool {
	c := parseVersionParts(current)
	m := parseVersionParts(min)
	for i := 0; i < len(m); i++ {
		var cv int
		if i < len(c) {
			cv = c[i]
		}
		mv := m[i]
		if cv > mv {
			return true
		}
		if cv < mv {
			return false
		}
	}
	return true
}

func parseVersionParts(s string) []int {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	parts := strings.Split(s, ".")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		n := 0
		for _, r := range p {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
		}
		out = append(out, n)
	}
	if len(out) == 0 {
		return []int{0}
	}
	return out
}
