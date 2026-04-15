# MCP Features Demo Cheatsheet

Step-by-step walkthrough showcasing all MCP capabilities in slyds. Uses VS Code Copilot as the client — it handles elicitation inline in chat, making for the best demo UX.

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

In VS Code, `.vscode/mcp.json` is already configured:
```json
{ "servers": { "slyds": { "type": "http", "url": "http://127.0.0.1:8274/mcp" } } }
```

Open Copilot Chat (Ctrl+Shift+I / Cmd+Shift+I) and switch to Agent mode.

---

## 1. Tools — "What decks do I have?"

**Say:** "What presentation decks are available?"

Copilot calls `list_decks` and shows all 3 demo decks with titles, themes, and slide counts.

**Then:** "Describe the getting-started deck"

Copilot calls `describe_deck` — shows structured metadata including per-slide layouts, word counts, and versions.

---

## 2. Elicitation — "Create a presentation" (the wow moment)

**Say:** "Create a new presentation called demo-talk titled MCP Features Demo"

**What happens:**
1. Copilot calls `create_deck` with name + title but **no theme** (the description says "omit to let the user choose interactively")
2. Mid-tool-call, the slyds server sends an elicitation request back to VS Code
3. **A theme selection form appears inline in the chat** — dropdown with default, dark, minimal, corporate, hacker
4. Pick a theme → tool completes → deck created with your choice

This is server-driven UI inside the chat — the server controls what options appear, not the LLM.

---

## 3. Elicitation — Destructive action confirmation

**Say:** "Remove the second slide from demo-talk"

**What happens:**
1. Copilot calls `remove_slide`
2. Server sends elicitation: "Remove slide '02-slide.html' from deck 'demo-talk'? This cannot be undone."
3. **A confirmation form appears** with a confirm checkbox
4. **Accept** → slide removed. **Decline** → "Slide removal cancelled."

**Demo both paths** — accept once, then create another deck and decline the removal.

---

## 4. Sampling — AI-powered slide improvement

**Say:** "Improve slide 1 of demo-talk — make it more visual with bullet points and a code example"

**What happens:**
1. Copilot calls `improve_slide`
2. Server reads the current slide HTML
3. Server sends a `sampling/createMessage` request back to VS Code — asking VS Code's LLM to rewrite the slide
4. The LLM generates improved HTML (server provides the system prompt with slyds constraints)
5. Server validates (lint + sanitize) and writes the result
6. Returns the new version

**Key point:** The server controls the prompt (knows about slyds HTML constraints, `class="slide"` requirement, no `<style>` blocks) while the client provides the LLM.

---

## 5. Prompts — Structured guidance templates

**Say:** "Use the create-presentation prompt for a talk about Kubernetes Security with 8 slides"

Or browse prompts in Copilot's prompt picker. Three available:

| Prompt | What to say | What it returns |
|--------|-------------|-----------------|
| `create-presentation` | "Use the create-presentation prompt about AI Safety" | Step-by-step guidance with available themes + layouts |
| `review-slides` | "Use the review-slides prompt for getting-started" | Full deck content with review instructions |
| `suggest-speaker-notes` | "Use the suggest-speaker-notes prompt for slide 2 of getting-started" | Slide content with speaker notes guidance |

---

## 6. Resources — Browsable deck content

In Copilot, ask about specific resources or use `@slyds` references:

| What to ask | Resource hit |
|-------------|-------------|
| "Show me the server info" | `slyds://server/info` — version, themes, layouts |
| "Show me the config for getting-started" | `slyds://decks/getting-started/config` — .slyds.yaml |
| "Show me slide 3 of dark-mode-talk" | `slyds://decks/dark-mode-talk/slides/3` — raw HTML |

---

## 7. Completions — Auto-complete in action

Completions work when VS Code fills in resource template parameters. When browsing resources:
- Typing a deck name auto-completes from available decks
- Typing a slide position auto-completes from valid positions

Best demonstrated in VS Code's MCP resource browser or when Copilot resolves template URIs.

---

## 8. Read + Edit + Verify cycle

**Say:** "Read slide 1 of getting-started, make the title bigger and bolder, then verify the change"

Copilot will:
1. Call `read_slide` → gets HTML + version
2. Call `edit_slide` with new content + `expected_version` (optimistic concurrency)
3. Call `read_slide` again to verify

**Try a conflict:** Open the slide in an editor, change it manually, then ask Copilot to edit it — it gets a `version_conflict` error with the current content, and recovers.

---

## 9. Build + Check

**Say:** "Check the getting-started deck for issues, then build it"

1. `check_deck` → validates sync, missing notes, broken assets, estimates talk time
2. `build_deck` → produces self-contained HTML with all CSS/JS/images inlined

---

## 10. Proto path parity

Stop the server and restart with the proto-generated path:
```bash
slyds mcp-proto --deck-root /tmp/slyds-demo/ --listen 127.0.0.1:8274
```

Repeat any of the above — all features work identically. The proto path uses generated helpers: `SampleForImproveSlide()`, `ElicitThemeChoice()`, `ElicitRemoveSlideConfirmation()`.

---

## Quick demo script (5 minutes)

| Step | Say this | Shows |
|------|----------|-------|
| 1 | "What decks do I have?" | **Tools** — list_decks |
| 2 | "Create a presentation called k8s-talk titled Kubernetes Overview" | **Elicitation** — theme picker form appears |
| 3 | "Remove slide 2 from k8s-talk" | **Elicitation** — confirmation dialog |
| 4 | "Improve slide 1 of k8s-talk — add bullet points about key concepts" | **Sampling** — server↔client LLM round-trip |
| 5 | "Use the review-slides prompt for getting-started" | **Prompts** — structured review guidance |
| 6 | "Show me the server info" | **Resources** — browsable content |
| 7 | "Check getting-started for issues then build it" | **Tools** — validation + build |

**Total: ~5 minutes covering all 6 MCP capabilities in a natural conversation**
