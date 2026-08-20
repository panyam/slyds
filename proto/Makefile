buf: ensureenv clean
	buf dep update
	buf generate
	goimports -w `find ../gen | grep "\.go"`

clean:
	rm -Rf ../gen
	rm -f buf.lock

cleanall: clean remove-proto-symlinks
	rm -f buf.yaml
	rm -f buf.gen.yaml

# MCPKIT_DIR points at a local mcpkit checkout. Override it if your worktree
# lives elsewhere: make setupdev MCPKIT_DIR=~/newstack/mcpkit/some-worktree
MCPKIT_DIR ?= $(HOME)/newstack/mcpkit/main
PROTOGEN_PROTO_DIR := $(MCPKIT_DIR)/experimental/ext/protogen/proto/mcp

# ── Plugin ──
# protoc-gen-go-mcp is not on the BSR, and it cannot be `go install`ed at a
# version either: protogen's go.mod carries `replace github.com/panyam/mcpkit
# => ../../../`, and Go refuses to install a versioned module whose go.mod has
# replace directives. So the plugin is built from a local mcpkit checkout.
# That is the only thing MCPKIT_DIR is still needed for — the annotation protos
# come from the BSR (see setupprod).
setup-plugin:
	@test -d "$(MCPKIT_DIR)" || { \
	  echo "mcpkit checkout not found at $(MCPKIT_DIR)"; \
	  echo "Override with: make setup-plugin MCPKIT_DIR=~/newstack/mcpkit/some-worktree"; \
	  exit 1; }
	cd $(MCPKIT_DIR)/experimental/ext/protogen && go build -o $${GOBIN:-$$(go env GOPATH)/bin}/protoc-gen-go-mcp ./cmd/protoc-gen-go-mcp
	@echo "Built protoc-gen-go-mcp from $(MCPKIT_DIR)"

# ── Dev / Prod environment switching ──
# Prod (preferred): protogen annotations come from the buf.build/mcpkit/protogen
#   BSR dep, pinned by digest in buf.lock. Needs no local mcpkit checkout.
# Dev: annotations come from a symlink into a local mcpkit worktree. Use this
#   only when co-developing slyds and protogen together.

# In dev mode the protogen annotations come from a local symlink, so mcp/ must
# exist. In prod mode they come from the buf.build/mcpkit/protogen BSR dep and
# mcp/ is deliberately absent — requiring it unconditionally broke `make buf`
# for anyone on the prod config.
ensureenv:
	@test -f buf.yaml && test -f buf.gen.yaml || { echo "buf.yaml not found. Run 'make setupdev' or 'make setupprod'."; exit 1; }
	@if [ "$$(readlink buf.yaml)" = "buf.yaml.dev" ]; then \
	  test -L mcp || test -d mcp || { echo "mcp/ not found. Run 'make setupdev' to symlink local mcpkit protos."; exit 1; }; \
	fi

setupdev: cleanall symlink-protos
	ln -s buf.yaml.dev buf.yaml
	ln -s buf.gen.yaml.dev buf.gen.yaml
	@echo "Dev environment ready. Run 'make buf' to generate."

setupprod: cleanall remove-proto-symlinks
	ln -s buf.yaml.prod buf.yaml
	ln -s buf.gen.yaml.prod buf.gen.yaml
	@echo "Prod environment ready. Run 'make buf' to generate."

symlink-protos: remove-proto-symlinks
	@test -d "$(PROTOGEN_PROTO_DIR)" || { \
	  echo "protogen protos not found at $(PROTOGEN_PROTO_DIR)"; \
	  echo "Set MCPKIT_DIR to a local mcpkit checkout, e.g."; \
	  echo "  make setupdev MCPKIT_DIR=~/newstack/mcpkit/some-worktree"; \
	  exit 1; }
	ln -s "$(PROTOGEN_PROTO_DIR)" mcp
	@echo "Symlinked mcp/ → $(PROTOGEN_PROTO_DIR)"

remove-proto-symlinks:
	rm -f mcp
