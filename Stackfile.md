# Stackfile

> Tracks which stack components this project uses and at what version.
> Updated automatically by `/stack-update` or manually.

## Stack Components

| Component | Module | Version | Updated |
|-----------|--------|---------|---------|
| templar | github.com/panyam/templar | v0.1.2 | 2026-08-20 |
| mcpkit | github.com/panyam/mcpkit | v0.5.1 | 2026-08-20 |
| mcpkit/ext/auth | github.com/panyam/mcpkit/ext/auth | v0.5.1 | 2026-08-20 |
| mcpkit/ext/ui | github.com/panyam/mcpkit/ext/ui | v0.5.1 | 2026-08-20 |
| servicekit | github.com/panyam/servicekit | v0.1.4 | 2026-08-20 |
| oneauth | github.com/panyam/oneauth | v0.1.36 | 2026-08-20 |
| goutils | github.com/panyam/goutils | v0.1.13 | 2026-04-01 |

`oneauth` is an indirect dependency, pulled in by `mcpkit/ext/auth` for JWT
validation and OAuth discovery. It is listed because auth behavior tracks it.

`mcpkit/experimental/ext/protogen` was dropped in the v0.5.1 upgrade. It is
still pinned to the mcpkit v0.2.x API, so depending on it would have held the
module three minor versions back. See [proto/README.md](proto/README.md).

## Third-Party Dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| goquery | github.com/PuerkitoBio/goquery | v1.12.0 | CSS selector-based HTML DOM access for `slyds query` |
| cobra | github.com/spf13/cobra | v1.10.2 | CLI framework |
| yaml.v3 | gopkg.in/yaml.v3 | v3.0.1 | YAML parsing for `.slyds.yaml` manifest |

## Project Conventions

- **grpc**: none
- **go-toolchain-floor**: 1.26.6 (raised above mcpkit's 1.26.5 floor to clear six reachable stdlib advisories)
- **replace-pattern**: locallinks
- **frontend**: vanilla
- **proto-build**: none in the Go build. `proto/` is parked design source; buf config remains for a future regen (see [proto/README.md](proto/README.md))
- **wasm**: no
