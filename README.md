# slyds

Lightweight HTML presentation toolkit. Each slide lives in its own file. A single Go binary handles scaffolding, serving, building, and slide management. Zero dependencies in output — works from `file://`.

## Install

```bash
go install github.com/user/slyds@latest
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
slyds init "My Talk" -n 5
slyds serve my-talk
# → http://localhost:3000
```

## How It Works

`slyds init` creates a presentation with each slide in its own file:

```
my-talk/
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

### `slyds init "Title" [-n count]`

Scaffolds a new presentation directory with the given number of slides (default 3, min 2).

```bash
slyds init "My Talk"         # 3 slides
slyds init "My Talk" -n 8    # 8 slides
```

### `slyds serve [dir] [-p port]`

Dev server with live template resolution. Edits to slide files are reflected on browser refresh.

```bash
slyds serve my-talk           # default port 3000
slyds serve my-talk -p 8080
```

### `slyds build [dir]`

Flattens all includes and inlines CSS, JS, and images into a single `dist/index.html`. The result works offline from `file://` with zero external dependencies.

```bash
slyds build my-talk
# → my-talk/dist/index.html
```

### `slyds add "name" [--after N] [--type title|content|closing]`

Adds a new slide, creates the file, and updates `index.html`.

```bash
slyds add "demo"               # append content slide
slyds add "intro" --after 1 --type title
```

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
