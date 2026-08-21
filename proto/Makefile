# Regeneration for the parked proto path. Nothing in the Go build depends on
# this; see README.md for what it is and when to run it.
#
# There is no dev/prod split and no local-checkout assumption. The protogen
# annotations come from buf.build/mcpkit/protogen, pinned by digest in the
# committed buf.lock.
#
# The one prerequisite is the protoc-gen-go-mcp plugin on PATH. It cannot be
# installed from the module proxy today (mcpkit issue 1319), so obtain it
# however you can and re-run. `make check-plugin` tells you if it is missing.

buf: check-plugin clean
	buf dep update
	buf generate
	goimports -w `find ../gen | grep "\.go"`

check-plugin:
	@command -v protoc-gen-go-mcp >/dev/null || { \
	  echo "protoc-gen-go-mcp not found on PATH."; \
	  echo ""; \
	  echo "It is not published to the BSR and cannot be 'go install'ed at a"; \
	  echo "version: protogen's go.mod carries a replace directive, and Go"; \
	  echo "refuses to install a versioned module that has one. Tracked as"; \
	  echo "mcpkit issue 1319."; \
	  echo ""; \
	  echo "Until that is fixed, build it from an mcpkit checkout:"; \
	  echo "  cd <mcpkit>/experimental/ext/protogen \\"; \
	  echo "    && go build -o \$$(go env GOPATH)/bin/protoc-gen-go-mcp ./cmd/protoc-gen-go-mcp"; \
	  exit 1; }

# Removes generated output only. buf.lock is committed and is the version pin,
# so it deliberately survives.
clean:
	rm -Rf ../gen

.PHONY: buf check-plugin clean
