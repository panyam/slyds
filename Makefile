.PHONY: build setup-tools install test clean version examples examples-serve gh-pages \
       demo demo-smoke dev-http dev-sse dev-stdio dev-http-auth dev-sse-auth tunnel

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X github.com/panyam/slyds/cmd.Version=$(VERSION)

# Build the slyds binary
build:
	go build -ldflags="$(LDFLAGS)" -o slyds .

# Install required Go tools
setup-tools:
	go install github.com/spf13/cobra-cli@latest

# Install slyds into $GOBIN
install:
	go build -ldflags="$(LDFLAGS)" -o ${GOBIN}/slyds .

# Run tests
test:
	go test ./...

# Run MCP e2e tests only (full agent workflow via httptest)
e2e:
	go test ./cmd/... -run E2E -v

# Print the version that would be injected
version:
	@echo $(VERSION)

# Clean build artifacts
clean:
	rm -f slyds

resymlink:
	mkdir -p locallinks
	rm -Rf locallinks/*
	cd locallinks && ln -s ~/newstack

EXAMPLES := examples/slyds-intro examples/rich-content examples/hacker-showcase

# Build all example presentations into examples/dist/
examples: build
	@mkdir -p examples/dist/examples
	@for dir in $(EXAMPLES); do \
		./slyds build "$$dir"; \
		name=$$(basename "$$dir"); \
		mkdir -p "examples/dist/examples/$$name"; \
		cp "$$dir/dist/index.html" "examples/dist/examples/$$name/"; \
	done
	@cp examples/landing/index.html examples/dist/
	@echo "Examples built to examples/dist/"

# Serve built examples locally
examples-serve: examples
	@echo "Serving examples at http://localhost:8080"
	@cd examples/dist && python3 -m http.server 8080

# Deploy examples to gh-pages branch
gh-pages: examples
	@echo "Deploying to gh-pages branch..."
	@if [ -d examples/dist/.git ]; then \
		echo "Error: examples/dist appears to be a git repo. Please remove it first."; \
		exit 1; \
	fi
	@cd examples/dist && \
		touch .nojekyll && \
		git init && \
		git add -A && \
		git commit -m "Deploy examples to GitHub Pages" && \
		git branch -M gh-pages && \
		git remote add origin git@panyam-github:panyam/slyds.git && \
		git push -f origin gh-pages
	@echo "Deployed! Enable GitHub Pages in repo settings to serve from gh-pages branch."
	@rm -rf examples/dist/.git

# =============================================================================
# Demo decks + dev servers
# =============================================================================

DEMO_DIR := /tmp/slyds-demo
SLYDS_MCP_PORT ?= 6274

# Scaffold 3 demo decks for testing all transports and LLM integrations.
demo: build
	@rm -rf $(DEMO_DIR)
	@mkdir -p $(DEMO_DIR)
	@cd $(DEMO_DIR) && $(CURDIR)/slyds init "Getting Started" --theme default -n 3
	@cd $(DEMO_DIR) && $(CURDIR)/slyds init "Dark Mode Talk" --theme dark -n 5
	@cd $(DEMO_DIR) && $(CURDIR)/slyds init "Corporate Review" --theme corporate -n 4
	@echo ""
	@echo "Demo decks scaffolded in $(DEMO_DIR)/"
	@echo "  getting-started/   (3 slides, default theme)"
	@echo "  dark-mode-talk/    (5 slides, dark theme)"
	@echo "  corporate-review/  (4 slides, corporate theme)"

# Smoke test: exercise the Workspace layer end-to-end without starting a
# server. First checks `slyds ws info` and `slyds ws list`, then starts a
# real MCP server in the background, calls list_decks via curl/JSON-RPC,
# and tears everything down. Verifies both the CLI workspace path and the
# MCP middleware path resolve the same decks.
demo-smoke: demo
	@set -e; \
	echo "== slyds ws info =="; \
	$(CURDIR)/slyds ws info --deck-root $(DEMO_DIR); \
	echo ""; \
	echo "== slyds ws list =="; \
	$(CURDIR)/slyds ws list --deck-root $(DEMO_DIR); \
	echo ""; \
	echo "== slyds ws list --json =="; \
	$(CURDIR)/slyds ws list --deck-root $(DEMO_DIR) --json; \
	echo ""; \
	echo "== starting slyds mcp (http) =="; \
	$(CURDIR)/slyds mcp --listen 127.0.0.1:6275 --deck-root $(DEMO_DIR) > /tmp/slyds-smoke.log 2>&1 & \
	SRV_PID=$$!; \
	trap "kill $$SRV_PID 2>/dev/null; rm -f /tmp/slyds-smoke.log" EXIT; \
	sleep 1; \
	echo "== initialize session =="; \
	RESP=$$(curl -s -i -X POST http://127.0.0.1:6275/mcp \
	  -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
	  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"smoke","version":"1"}}}'); \
	SESSION=$$(printf '%s' "$$RESP" | awk -F': ' 'tolower($$1)=="mcp-session-id"{gsub(/[\r\n]/,"",$$2); print $$2}'); \
	if [ -z "$$SESSION" ]; then echo "FAIL: no session id from initialize"; echo "$$RESP"; exit 1; fi; \
	echo "session: $$SESSION"; \
	curl -s -X POST http://127.0.0.1:6275/mcp \
	  -H "Mcp-Session-Id: $$SESSION" -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
	  -d '{"jsonrpc":"2.0","method":"notifications/initialized"}' > /dev/null; \
	echo "== tools/call list_decks =="; \
	CALL=$$(curl -s -X POST http://127.0.0.1:6275/mcp \
	  -H "Mcp-Session-Id: $$SESSION" -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
	  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_decks","arguments":{}}}'); \
	echo "$$CALL"; \
	echo "$$CALL" | grep -q 'getting-started' || { echo "FAIL: list_decks missing getting-started"; exit 1; }; \
	echo ""; \
	echo "OK — CLI ws and MCP list_decks agree on deck set."

# Dev: Streamable HTTP on :$(SLYDS_MCP_PORT)
dev-http: demo
	@echo ""
	@echo "  MCP endpoint: http://127.0.0.1:$(SLYDS_MCP_PORT)/mcp"
	@echo "  Landing page: http://127.0.0.1:$(SLYDS_MCP_PORT)/"
	@echo ""
	./slyds mcp --listen 127.0.0.1:$(SLYDS_MCP_PORT) --deck-root $(DEMO_DIR)

# Dev: SSE on :$(SLYDS_MCP_PORT)
dev-sse: demo
	@echo ""
	@echo "  MCP endpoint: http://127.0.0.1:$(SLYDS_MCP_PORT)/sse"
	@echo "  Landing page: http://127.0.0.1:$(SLYDS_MCP_PORT)/"
	@echo ""
	./slyds mcp --sse --listen 127.0.0.1:$(SLYDS_MCP_PORT) --deck-root $(DEMO_DIR)

# Dev: stdio (for pipe testing or manual JSON-RPC)
dev-stdio: demo
	./slyds mcp --stdio --deck-root $(DEMO_DIR)

# Dev: HTTP with bearer auth
dev-http-auth: demo
	@echo ""
	@echo "  MCP endpoint: http://127.0.0.1:$(SLYDS_MCP_PORT)/mcp"
	@echo "  Landing page: http://127.0.0.1:$(SLYDS_MCP_PORT)/"
	@echo "  Token: dev-secret"
	@echo ""
	./slyds mcp --listen 127.0.0.1:$(SLYDS_MCP_PORT) --deck-root $(DEMO_DIR) --token dev-secret

# Dev: SSE with bearer auth
dev-sse-auth: demo
	@echo ""
	@echo "  MCP endpoint: http://127.0.0.1:$(SLYDS_MCP_PORT)/sse"
	@echo "  Landing page: http://127.0.0.1:$(SLYDS_MCP_PORT)/"
	@echo "  Token: dev-secret"
	@echo ""
	./slyds mcp --sse --listen 127.0.0.1:$(SLYDS_MCP_PORT) --deck-root $(DEMO_DIR) --token dev-secret

# Start a localhost tunnel (requires ngrok or cloudflared)
tunnel:
	@bash scripts/tunnel.sh

# =============================================================================
# Security audit
# =============================================================================

# Full security audit: dependency vulns + code patterns + secrets
audit:
	@echo "=== govulncheck ==="
	govulncheck ./...
	@echo ""
	@echo "=== gosec ==="
	gosec -quiet -severity=medium ./... || true
	@echo ""
	@echo "=== gitleaks ==="
	gitleaks detect --source . -v 2>/dev/null || echo "gitleaks not installed (go install github.com/gitleaks/gitleaks/v8@latest)"
	@echo ""
	@echo "=== Audit complete ==="

.PHONY: audit
