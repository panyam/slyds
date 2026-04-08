.PHONY: build setup-tools install test clean version examples examples-serve gh-pages \
       demo dev-http dev-sse dev-stdio dev-http-auth dev-sse-auth tunnel

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
MCP_PORT ?= 6274

# Find a free port: try MCP_PORT first, fall back to OS-assigned.
define find_free_port
$(shell python3 -c "import socket; s=socket.socket(); s.bind(('127.0.0.1',$(MCP_PORT))); print($(MCP_PORT)); s.close()" 2>/dev/null || python3 -c "import socket; s=socket.socket(); s.bind(('127.0.0.1',0)); print(s.getsockname()[1]); s.close()")
endef

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

# Dev: Streamable HTTP (auto-detect free port)
dev-http: demo
	$(eval PORT := $(find_free_port))
	@echo ""
	@echo "  MCP endpoint: http://127.0.0.1:$(PORT)/mcp"
	@echo ""
	./slyds mcp --listen 127.0.0.1:$(PORT) --deck-root $(DEMO_DIR)

# Dev: SSE (auto-detect free port)
dev-sse: demo
	$(eval PORT := $(find_free_port))
	@echo ""
	@echo "  MCP endpoint: http://127.0.0.1:$(PORT)/sse"
	@echo ""
	./slyds mcp --sse --listen 127.0.0.1:$(PORT) --deck-root $(DEMO_DIR)

# Dev: stdio (for pipe testing or manual JSON-RPC)
dev-stdio: demo
	./slyds mcp --stdio --deck-root $(DEMO_DIR)

# Dev: HTTP with bearer auth (auto-detect free port)
dev-http-auth: demo
	$(eval PORT := $(find_free_port))
	@echo ""
	@echo "  MCP endpoint: http://127.0.0.1:$(PORT)/mcp"
	@echo "  Token: dev-secret"
	@echo ""
	./slyds mcp --listen 127.0.0.1:$(PORT) --deck-root $(DEMO_DIR) --token dev-secret

# Dev: SSE with bearer auth (auto-detect free port)
dev-sse-auth: demo
	$(eval PORT := $(find_free_port))
	@echo ""
	@echo "  MCP endpoint: http://127.0.0.1:$(PORT)/sse"
	@echo "  Token: dev-secret"
	@echo ""
	./slyds mcp --sse --listen 127.0.0.1:$(PORT) --deck-root $(DEMO_DIR) --token dev-secret

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
