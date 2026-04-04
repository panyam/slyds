# Presentation: slyds - Multi-File HTML Presentations

This is a slyds presentation. Edit slides in the `slides/` directory.

## Quick Reference

```
slyds serve              # preview at localhost:3000
slyds build              # build dist/index.html (self-contained)
slyds ls                 # list slides with layouts
slyds describe           # structured deck summary (YAML)
slyds introspect         # machine JSON: layouts, slots, themes, commands
slyds check              # validate deck
```

## MCP (Model Context Protocol)

Agents can drive this deck through the **slyds** CLI via MCP (`slyds mcp` for stdio, `slyds mcp serve` for HTTP+SSE). Per-editor setup (**Cursor**, **Claude**, **Copilot**) and security details:

**https://github.com/panyam/slyds/blob/main/docs/MCP.md**

To omit this section in the future, set `agent_include_mcp: false` in `.slyds.yaml` and run `slyds update`.

## Editing Slides

```
slyds add "Name" --layout content     # add a slide (append)
slyds add "Name" --after 3            # add after slide 3
slyds insert 2 "Name" --layout title  # insert at position
slyds rm 4                            # remove slide 4
slyds mv 2 5                          # move slide 2 to position 5
```

## Reading/Writing Slide Content

```
slyds query 3 h1 --text                                # read heading
slyds query 3 h1 --set "New Title"                     # set heading
slyds query 3 '[data-slot="left"]' --set "<p>…</p>"    # set slot content
slyds query 3 '[data-slot="body"]' --append "<li>…</li>"
```

## Available Layouts

| Layout | Use for | Slots |
|--------|---------|-------|
| `blank` | Empty slide with no predefined structure | body |
| `closing` | Closing/thank-you slide | title, body, floater |
| `content` | Standard content slide with heading and body | title, body, floater |
| `section` | Section divider slide | title, subtitle |
| `title` | Full-screen title slide with subtitle | title, subtitle |
| `two-col` | Two-column side-by-side layout | title, left, right, floater |

## Available Themes

corporate, dark, default, hacker, minimal (current: default)

Switch at runtime via the theme button in the toolbar, or set in `.slyds.yaml`.

## Conventions

- One HTML file per slide in `slides/`
- Slides are plain HTML — no template syntax
- `index.html` controls slide order (don't edit manually — use slyds commands)
- Use `slyds query` for content edits, not regex/string manipulation
- Speaker notes go in `<div class="speaker-notes">` inside each slide
- Each slide has a `data-layout` attribute identifying its structural layout
- Use `[data-slot="name"]` CSS selectors to target named content regions

## Charts & Dynamic Content

Slides use `display:none` when inactive. Canvas-based libraries (Chart.js, D3)
need real dimensions to render. Use slide lifecycle hooks:

| Event | When | Target |
|-------|------|--------|
| `slideEnter` | After incoming slide gets `.active` (has dimensions) | slide element |
| `slideLeave` | Before outgoing slide loses `.active` (still has dimensions) | slide element |

Events bubble, so `document.addEventListener('slideEnter', ...)` works.

### Event Detail

```js
event.detail = {
  index:     0,           // 0-based
  slideNum:  1,           // 1-based (matches counter)
  title:     "Slide 1",   // from <h1> or fallback
  layout:    "content",   // data-layout value or null
  total:     10,          // total slides
  direction: "forward",   // "forward" | "backward" | "init"
  data:      {}           // all data-* attributes
}
```

### Recommended Pattern

Create charts on first `slideEnter`, cache at page level, resize on revisit.
Do **not** destroy and recreate on every navigation.

```js
document.addEventListener('slideEnter', function(e) {
  var ctx = window.slydsContext.state;
  var key = 'chart-' + e.detail.slideNum;
  var canvas = e.target.querySelector('canvas');
  if (!canvas) return;
  if (!ctx[key]) {
    ctx[key] = new Chart(canvas, config);  // first visit
  } else {
    ctx[key].resize();  // revisit — fix dimensions
  }
});
```

### window.slydsContext

Persistent object available throughout the presentation:

- `slydsContext.totalSlides` — total slide count
- `slydsContext.currentSlide` — current 1-based slide number
- `slydsContext.direction` — last navigation direction
- `slydsContext.state` — user/agent state bag (survives transitions)

## Floating Overlays

Layouts with a `floater` slot support pinned overlays (footers, watermarks, logos).
The element is `position: absolute` within the slide — it stays fixed while content flows.

```
slyds query 3 '[data-slot="floater"]' --set '<span style="bottom:0;left:0;right:0;padding:8px 24px;font-size:0.7em;opacity:0.5;">Confidential</span>'
```

Common patterns:
- **Footer**: `style="bottom:0; left:0; right:0; padding:8px 24px; font-size:0.7em; opacity:0.5;"`
- **Watermark**: `style="top:50%; left:50%; transform:translate(-50%,-50%) rotate(-30deg); opacity:0.08; font-size:4em;"`
- **Logo**: `style="top:12px; right:12px;"` with an `<img>` inside

The floater slot is empty by default. Not all layouts have it — title and section slides omit it.
Existing slides without a floater slot are unaffected.
