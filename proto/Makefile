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

# ── Dev / Prod environment switching ──
# Dev: symlinks to local mcpkit proto sources + local protoc-gen-go-mcp
# Prod: buf.build dependencies (once published)

ensureenv:
	@test -f buf.yaml && test -f buf.gen.yaml || { echo "buf.yaml not found. Run 'make setupdev' or 'make setupprod'."; exit 1; }
	@test -L mcp || test -d mcp || { echo "mcp/ not found. Run 'make setupdev' to symlink local mcpkit protos."; exit 1; }

setupdev: cleanall symlink-protos
	ln -s buf.yaml.dev buf.yaml
	ln -s buf.gen.yaml.dev buf.gen.yaml
	@echo "Dev environment ready. Run 'make buf' to generate."

setupprod: cleanall remove-proto-symlinks
	ln -s buf.yaml.prod buf.yaml
	ln -s buf.gen.yaml.prod buf.gen.yaml
	@echo "Prod environment ready. Run 'make buf' to generate."

symlink-protos: remove-proto-symlinks
	ln -s $(HOME)/newstack/mcpkit/main/experimental/ext/protogen/proto/mcp mcp
	@echo "Symlinked mcp/ → local mcpkit protogen protos"

remove-proto-symlinks:
	rm -f mcp
