# Themes and packs for agents

This note complements per-deck `AGENT.md`. It documents how presentations reference themes and engine assets so tools can validate paths and avoid brittle guesses.

## Built-in themes

Built-in theme names come from the slyds binary (`slyds introspect` → `themes_builtin`). Scaffolds copy CSS into `themes/` under the deck; `.slyds.yaml` records the active `theme` name.

## Manifest (`.slyds.yaml`)

| Field | Role |
|-------|------|
| `theme` | Theme name used for scaffolding and `slyds update` (built-in name or custom label on disk). |
| `title` | Presentation title. |
| `sources` | Optional templar source entries (URL + path) for vendored engine or theme modules. |
| `modules_dir` | Where fetched modules live (default `.slyds-modules`). |

Agents should treat `sources` as the extension point for **theme packs** shared across decks: a Git URL plus `path` to the directory that contains `theme.css`, templates, or assets. Use `slyds install` / lockfiles as documented in the main README for reproducible fetches.

## Custom themes on disk

If a deck uses a theme that is not in `themes_builtin`, files may still live under `themes/` next to the deck. `slyds update` may not re-render that theme from embedded templates; engine files (`slyds.css`, `slyds.js`, etc.) can still be refreshed. See GitHub issue #48 for ongoing improvements (non-fatal `update`, optional `theme_dir` in the manifest).

## Validation hooks

- `slyds check` — sync between `index.html` and `slides/`, layouts, assets, notes.
- `slyds describe --json` — instance state (per-slide layout, titles).
- `slyds introspect` — global contract: layout names, `data-slot` keys, commands.

Prefer these over reading-only fragments of `AGENT.md` when building automation.

## Agent workflow

1. Run `slyds introspect [dir]` once per environment to learn layouts and slots.
2. Create or edit slides with `slyds add` / `insert` / `query`, or `slyds query --batch` for multiple writes.
3. Optionally pass `--slots-file slots.json` on `add`/`insert` with `{ "slotName": "<html fragment>" }` to fill `[data-slot="slotName"]` after the slide is created.
4. Run `slyds check` before handing off.

HTML content remains author/agent responsibility; slyds only places fragments into structural slots.

See **[MCP.md](MCP.md)** to expose the same CLI to remote MCP clients (HTTP+SSE) or local stdio.
