# MCP Features Demo Cheatsheet

Step-by-step walkthrough showcasing all MCP capabilities in slyds. Uses MCPJam as the client.

## Setup

```bash
# 1. Build slyds
make build

# 2. Scaffold demo decks
make demo
# Creates: /tmp/slyds-demo/ with getting-started, dark-mode-talk, corporate-review

# 3. Start the MCP server
slyds mcp --deck-root /tmp/slyds-demo/ --listen 127.0.0.1:8274
```

Connect MCPJam to `http://127.0.0.1:8274/mcp`.

---

## 1. Tools (14 tools)

### List decks
Call `list_decks` — returns all 3 demo decks with titles, themes, slide counts.

### Create a new deck
Call `create_deck`:
```json
{ "name": "demo-talk", "title": "MCP Features Demo", "slides": 4 }
```
**Note:** Don't pass `theme` — this triggers elicitation (see section 6).

### Read + edit a slide
```json
// read_slide
{ "deck": "demo-talk", "slide": "1" }

// edit_slide — use the version from read_slide
{ "deck": "demo-talk", "slide": "1",
  "content": "<div class=\"slide\" data-layout=\"title\"><h1>MCP Features</h1><p>A tour of prompts, sampling, and elicitation</p></div>",
  "expected_version": "<version from read>" }
```

### Check + build
```json
// check_deck
{ "deck": "demo-talk" }

// build_deck
{ "deck": "demo-talk" }
```

---

## 2. Resources (9 resources)

Browse these in MCPJam's resource explorer:

| URI | What it shows |
|-----|---------------|
| `slyds://server/info` | Server version, available themes/layouts |
| `slyds://decks` | All decks in workspace |
| `slyds://decks/demo-talk` | Deck metadata (JSON) |
| `slyds://decks/demo-talk/slides` | Slide list with positions, layouts |
| `slyds://decks/demo-talk/slides/1` | Raw HTML of slide 1 |
| `slyds://decks/demo-talk/config` | .slyds.yaml manifest |
| `slyds://decks/demo-talk/agent` | AGENT.md guide |

---

## 3. Prompts (3 prompts)

### create-presentation
In MCPJam's prompt explorer, invoke `create-presentation`:
```json
{ "topic": "Kubernetes Security", "slide_count": "8", "theme": "dark" }
```
Returns structured guidance messages with available themes, layouts, and step-by-step instructions.

### review-slides
```json
{ "name": "getting-started" }
```
Returns the full deck content with review instructions — the LLM sees all slides and provides feedback.

### suggest-speaker-notes
```json
{ "name": "getting-started", "slide": "2" }
```
Returns the specific slide content with guidance for drafting speaker notes.

---

## 4. Completions

In MCPJam, when filling in resource template parameters:

- Type in the `name` field on `slyds://decks/{name}` → auto-completes with deck names (`getting-started`, `dark-mode-talk`, etc.)
- Type `d` → filters to `dark-mode-talk`, `demo-talk`
- On `slyds://decks/{name}/slides/{n}` → `n` completes with valid position numbers

---

## 5. Sampling (improve_slide)

**Requires MCPJam sampling support enabled.**

Call `improve_slide`:
```json
{
  "deck": "getting-started",
  "slide": "2",
  "instruction": "Make the bullet points more concise and add a code example"
}
```

**What happens:**
1. Server reads the current slide HTML
2. Server sends a `sampling/createMessage` request back to MCPJam with:
   - System prompt: "You are an HTML slide editor for slyds..."
   - User message: current HTML + your instruction
3. MCPJam's LLM generates improved HTML
4. Server validates (lint + sanitize) and writes the result
5. Returns the new version

**If sampling not supported:** Returns a helpful error: "sampling not supported by this client — use edit_slide directly"

**Demo the error case too** — disconnect sampling in MCPJam, call `improve_slide` again → graceful fallback.

---

## 6. Elicitation

### Theme choice on create_deck

Call `create_deck` WITHOUT a theme:
```json
{ "name": "elicit-demo", "title": "Elicitation Demo" }
```

**What happens:**
1. Server detects no theme provided
2. Sends an `elicitation/elicit` request to MCPJam with:
   - Message: "Choose a theme for 'Elicitation Demo':"
   - Schema: `{ theme: enum["default", "dark", "minimal", "corporate", "hacker"] }`
3. MCPJam shows a form/dropdown to the user
4. User picks "hacker" → deck created with hacker theme

**If elicitation not supported:** Falls back to "default" theme silently.

### Confirmation on remove_slide

```json
{ "deck": "elicit-demo", "slide": "2" }
```

**What happens:**
1. Server resolves the slide filename
2. Sends elicitation: "Remove slide '02-slide.html' from deck 'elicit-demo'? This cannot be undone."
3. Schema: `{ confirm: boolean }`
4. User can accept or decline
5. **Accept** → slide removed. **Decline** → "Slide removal cancelled."

**Demo both paths** — accept once, decline once.

---

## 7. MCP Apps (preview iframes)

### Host theme adaptation (#100)

Call `preview_deck`:
```json
{ "deck": "getting-started" }
```

In MCPJam:
- Switch MCPJam to **dark mode** → preview chrome (nav bar, background) adapts automatically
- Switch to **light mode** → chrome reverts
- Slide content keeps its own deck theme (independent)

### Interactive navigation (#101)

While the preview is open, call these **app-side tools** (MCPJam routes them directly to the iframe):

- `next_slide` → iframe navigates forward
- `prev_slide` → iframe navigates backward  
- `goto_slide { "position": 3 }` → jumps to slide 3
- `get_current_slide` → returns `{ "position": 2, "total": 5 }`

**Key demo point:** No server round-trip — the bridge handles navigation locally in the iframe.

### Live edit refresh

1. Open preview for `getting-started`
2. In another tab/tool, call `edit_slide` on slide 1
3. Preview auto-refreshes via `toolresult` event

### Fullscreen mode

```json
{ "deck": "getting-started", "display_mode": "fullscreen" }
```
MCPJam renders the deck in fullscreen presentation mode.

---

## 8. Proto path parity

Repeat any of the above with `slyds mcp-proto` instead:
```bash
slyds mcp-proto --deck-root /tmp/slyds-demo/ --listen 127.0.0.1:8274
```

All features work identically — proto-generated tools, resources, prompts, sampling helpers, elicitation helpers. The proto path uses `SampleForImproveSlide()`, `ElicitThemeChoice()`, `ElicitRemoveSlideConfirmation()` generated from proto annotations.

---

## Quick demo script (5 minutes)

1. **Setup** (30s): `make demo && slyds mcp --deck-root /tmp/slyds-demo/` → connect MCPJam
2. **Tools** (30s): `list_decks` → `describe_deck getting-started` → show JSON response
3. **Resources** (30s): Browse `slyds://server/info`, `slyds://decks/getting-started/slides/1`
4. **Completions** (15s): Type in resource URI field, show auto-complete
5. **Prompts** (30s): `create-presentation` with topic "AI Safety" → show structured guidance
6. **Elicitation** (45s): `create_deck` without theme → choose from dropdown → verify theme applied
7. **Elicitation decline** (15s): `remove_slide` → decline → verify slide still exists
8. **Sampling** (45s): `improve_slide` → watch server↔client LLM round-trip → verify slide updated
9. **Preview** (30s): `preview_deck` → show iframe, switch dark mode, navigate with app tools

**Total: ~5 minutes covering all 6 MCP capabilities + MCP Apps bridge**
