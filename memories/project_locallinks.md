---
name: locallinks-pattern
description: Project uses locallinks/ symlinks for local Go module dependencies (make resymlink)
type: project
---

The project uses a `locallinks/` directory with symlinks to local dependencies.
`make resymlink` creates `locallinks/newstack` pointing to `~/newstack`.

**Why:** Enables local development of slyds with a local copy of templar without publishing.

**How to apply:** go.mod uses `replace github.com/panyam/templar => ./locallinks/newstack/templar/main`. After cloning, run `make resymlink` before `go mod tidy`. The linter auto-maintains this pattern.
