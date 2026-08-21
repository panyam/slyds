# proto/ — parked

These definitions describe the slyds MCP surface as annotated proto RPCs:
tools as `mcp_tool` RPCs, resources as `mcp_resource` RPCs, prompts as
`mcp_prompt` RPCs, with `completable_fields` annotations for deck names and
slide positions. `protoc-gen-go-mcp` turns them into typed MCP registrations
plus sampling and elicitation helpers.

**Nothing in the Go build depends on this directory today.** The generated
tree (`gen/`) and the `slyds mcp-proto` subcommand were removed when slyds
moved to mcpkit v0.5.1. The reason is version drift, not a problem with the
generated code: `protoc-gen-go-mcp` still lives in mcpkit *experimental* and
is pinned to the mcpkit v0.2.x API, so keeping it would have held the whole
module back three minor versions. See the constraint in
[CONSTRAINTS.md](../CONSTRAINTS.md).

The definitions stay because they encode real API design work, and because
regenerating from them is cheap once the generator catches up.

## Reviving the proto path

When `protogen` is tagged against a current mcpkit release (mcpkit issue 1319):

1. Get `protoc-gen-go-mcp` onto your PATH. `make check-plugin` says whether it
   is there, and prints the workaround if not.
2. `cd proto && make buf` to regenerate `gen/`.
3. Restore the server wiring from git history. It was removed in the
   mcpkit v0.5.1 upgrade; the last commit that still carried it is `4f1e060`:
   ```
   git checkout 4f1e060 -- cmd/mcp_proto.go cmd/mcp_proto_impl.go cmd/mcp_proto_e2e_test.go
   ```
4. Expect to fix the handler ABI in `cmd/mcp_proto_impl.go`. mcpkit v0.3.0
   made tool and prompt handlers return the sealed `core.ToolResponse` /
   `core.PromptResponse` interfaces instead of the concrete result structs.
5. Rewrite the constraint in [CONSTRAINTS.md](../CONSTRAINTS.md).

The parity tests in `cmd/mcp_proto_e2e_test.go` are the thing worth
recovering carefully. They asserted that the proto path and the hand-written
path produce byte-identical output for the same call, which is the only
check that kept the two from drifting.

## Where the pieces come from

Everything here resolves from published artifacts. The annotation protos come
from `buf.build/mcpkit/protogen`, pinned by digest in the committed
`buf.lock`. There is no dev/prod split and no `MCPKIT_DIR`: slyds makes no
assumption about anyone's local filesystem layout.

The one exception is the `protoc-gen-go-mcp` plugin, and it is an upstream
gap rather than a choice. It is not on the BSR, and `go install ...@version`
fails because protogen's `go.mod` carries `replace github.com/panyam/mcpkit
=> ../../../` and Go refuses to install a versioned module with replace
directives. Both that and the tagging gap are tracked in mcpkit issue 1319.
Until one of them is fixed, building the plugin needs an mcpkit checkout;
`make check-plugin` prints the command.

## Gotchas worth keeping

Elicitation schema messages must live in `service.proto`, the same file as the
RPC that references them, not in `models.proto`. `protoc-gen-go-mcp` resolves
`schema_message` within `file.Messages` only.
