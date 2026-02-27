# slyds

Lightweight HTML presentation toolkit. Zero dependencies in output. Works from `file://`.

## Quick Start

```bash
npx slyds init "My Talk Title"
cd my-talk-title
open index.html
```

## Commands

### `slyds init "Talk Title" [-n slides] [--local]`

Scaffolds a new presentation directory:

- `index.html` — Your presentation (default 3 slides, or specify with `-n`)
- `theme.css` — Color/style overrides (commented-out examples)

By default, `slyds.css` and `slyds.js` are loaded from the unpkg CDN. Use `--local` to copy them into the project directory instead (useful for offline work or `file://` usage).

```bash
slyds init "My Talk"            # CDN mode (2 files)
slyds init "My Talk" --local    # local mode (4 files)
slyds init "My Talk" -n 8       # 8 slides, CDN mode
```

### `slyds serve [dir]`

Starts a local dev server. Useful when `file://` restrictions block features.

```bash
slyds serve my-talk       # default port 3000
slyds serve my-talk -p 8080
```

### `slyds build [dir]`

Inlines all CSS and JS into a single self-contained `dist/index.html`. Fetches CDN assets automatically if needed.

```bash
slyds build my-talk
# → my-talk/dist/index.html
```

## Writing Slides

Each slide is a `<div class="slide">`:

```html
<div class="slide">
    <h1>Slide Title</h1>
    <p>Content here.</p>
    <div class="speaker-notes">
        <p>Notes only visible in presenter mode.</p>
    </div>
</div>
```

The first slide gets `class="slide active"`. Add as many slides as you need.

## Keyboard Shortcuts

| Key | Action |
|-----|--------|
| `→` | Next slide |
| `←` | Previous slide |
| `N` | Open speaker notes window |
| `Esc` | Close notes window |

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

## Theming

Edit `theme.css` to override base styles. Common overrides:

- `body` background gradient
- `.title-slide` / `.conclusion-slide` backgrounds
- `.slide h1` border color
- `.nav-button` colors
- `.stat-number` color

## License

MIT
