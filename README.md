# slyds

Lightweight HTML presentation toolkit. Each slide lives in its own file. A single Go binary handles scaffolding, serving, building, and slide management. Zero dependencies in output — works from `file://`.

## Install

```bash
go install github.com/panyam/slyds@latest
```

Or build from source:

```bash
git clone https://github.com/panyam/slyds.git
cd slyds
make resymlink   # set up local templar dependency
make build       # produces ./slyds binary
```

## Quick Start

```bash
slyds init "Scaling at the Edge" -n 5
slyds serve scaling-at-the-edge
# → http://localhost:3000
```

## For AI Agents

slyds works with Claude, Cursor, Copilot, and other AI agents in two modes:

- **CLI-direct** — agent runs `slyds` commands via shell (no MCP server needed)
- **MCP** — full protocol server with 10 tools + 7 resources (stdio, HTTP, SSE)

See [AGENT-SETUP.md](AGENT-SETUP.md) for the agent-readable setup guide, or [docs/SETUP.md](docs/SETUP.md) for the human-readable version.

## Examples

**[Live demos](https://panyam.github.io/slyds/)** — or browse the source in `examples/`:

| Deck | Theme | Showcases |
|------|-------|-----------|
| [slyds-intro](https://panyam.github.io/slyds/examples/slyds-intro/) | default | Meta-presentation about slyds itself — workflow, commands, theming |
| [rich-content](https://panyam.github.io/slyds/examples/rich-content/) | dark | CSS components: code blocks, callouts, stats grids, tables, phase boxes |
| [hacker-showcase](https://panyam.github.io/slyds/examples/hacker-showcase/) | hacker | Background images, JetBrains Mono, progress-aware styling |

Build and preview locally:

```bash
make examples          # builds to examples/dist/
make examples-serve    # serves at localhost:8080
make gh-pages          # deploy to GitHub Pages
```

## How It Works

`slyds init` creates a presentation with each slide in its own file:

```
scaling-at-the-edge/
  index.html              # Organizer — composes slides via templar includes
  slyds.css               # Base presentation styles
  slyds.js                # Client-side slide engine
  theme.css               # Your color/style overrides
  slides/
    01-title.html          # Title slide
    02-slide.html          # Content slides
    03-slide.html
    04-slide.html
    05-closing.html        # Closing slide
```

`index.html` uses [templar](https://github.com/panyam/templar) `{{# include #}}` directives to compose slides. `slyds serve` resolves these on the fly; `slyds build` flattens everything into a single self-contained HTML file.

## Commands

### `slyds init "Title" [-n count] [--theme name] [--mcp]`

Scaffolds a new presentation directory with the given number of slides (default 3, min 2). By default, **`AGENT.md` includes a short MCP section** with a link to **[docs/MCP.md](docs/MCP.md#coding-agents-cursor)**. Use **`--mcp=false`** to omit it (stored in `.slyds.yaml` as `agent_include_mcp`).

```bash
slyds init "Why Go is Secretly Fun"                  # 3 slides, default theme
slyds init "Distributed Systems 101" -n 8             # 8 slides
slyds init "Zero to Production" --theme dark           # dark theme
slyds init "The Art of Simplicity" --theme minimal     # clean, no gradients
slyds init "Hacking the Mainframe" --theme hacker      # terminal vibes, nerdy fun
slyds init "No MCP block" --mcp=false                 # AGENT.md without MCP section
```

Available themes: `default`, `minimal`, `dark`, `corporate`, `hacker`.

### `slyds serve [dir] [-p port]`

Dev server with live template resolution. Edits to slide files are reflected on browser refresh.

```bash
slyds serve zero-to-production           # default port 3000
slyds serve zero-to-production -p 8080
```

### `slyds build [dir]`

Flattens all includes and inlines CSS, JS, and images into a single `dist/index.html`. The result works offline from `file://` with zero external dependencies.

```bash
slyds build zero-to-production
# → zero-to-production/dist/index.html
```

### `slyds add "name" [--after N] [--type title|content|closing]`

Adds a new slide, creates the file, and updates `index.html`.

```bash
slyds add "demo"                          # append content slide
slyds add "intro" --after 1 --type title
slyds add "overview" --type two-column    # left/right split layout
slyds add "part-2" --type section         # section divider
```

Available slide types: `title`, `content`, `closing`, `two-column`, `section`.

### `slyds rm <name-or-number>`

Removes a slide file and its include line from `index.html`.

```bash
slyds rm 3          # by position
slyds rm demo       # by name match
```

### `slyds mv <from> <to>`

Reorders slides. Renumbers files and updates `index.html`.

```bash
slyds mv 5 2        # move slide 5 to position 2
```

### `slyds ls`

Lists slides in order with filenames and first heading.

### Agents & MCP

For automation and hosted agents: **`slyds introspect`**, **`slyds describe --json`**, **`slyds query --batch`**, and **`slyds mcp`** / **`slyds mcp serve`**. **MCP setup (Cursor, Claude, Copilot, protocol, security):** **[docs/MCP.md](docs/MCP.md)**. Contributor conventions: **[CLAUDE.md](CLAUDE.md)**.

## Writing Slides

Each slide is a plain HTML file with a `<div class="slide">`:

```html
<div class="slide">
  <h1>Slide Title</h1>
  <p>Content here.</p>
  <div class="speaker-notes">
    <p>Notes only visible in presenter mode.</p>
  </div>
</div>
```

No template syntax needed in slide files. The first slide gets `class="slide active"` automatically.

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `→` | Next slide |
| `←` | Previous slide |
| `N` | Open speaker notes window |
| `T` | Start/pause presentation timer |
| `Esc` | Close notes window |

The speaker notes window includes an elapsed timer, per-slide reading time estimate (~200 WPM), and remaining deck time. Timer state persists across notes window close/reopen.

## Built-in CSS Classes

| Class | Use |
|-------|-----|
| `.title-slide` | Full-color title slide |
| `.conclusion-slide` | Dark closing slide |
| `.highlight` | Gradient callout box |
| `.stats-grid` + `.stat-box` | Grid of stat cards |
| `.phase-box` | Bordered content block |
| `.tier-table` | Styled table |
| `.controls-bar` | Fixed top navigation bar |

## Themes

slyds ships with five built-in themes:

| Theme | Description |
|-------|-------------|
| `default` | Purple gradient, white cards — conference talks |
| `minimal` | White background, serif font — academic, maximum readability |
| `dark` | Dark backgrounds, cyan accents — code talks, demos |
| `corporate` | Navy blues, clean grays — business presentations |
| `hacker` | Terminal aesthetics, nerdy but fun — backend engineers, tech demos |

Each theme includes position-aware CSS: slides automatically get `--slide-index` and `--slide-progress` CSS custom properties, enabling effects like alternating backgrounds and progress-based color shifts.

### Customizing

Edit `theme.css` to override base styles. Common overrides:

- `body` background gradient
- `.title-slide` / `.conclusion-slide` backgrounds
- `.slide h1` border color
- `.nav-button` colors
- `.stat-number` color

### Custom slide types

Each theme defines its slide types in `theme.yaml`. To add a custom type, create a `.html.tmpl` file in your theme's `slides/` directory and register it in `theme.yaml`.

## License

MIT
