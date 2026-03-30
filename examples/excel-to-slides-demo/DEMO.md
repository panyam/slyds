# Excel-to-Slides Demo Script

Live demo: one prompt turns a financial Excel file into a polished presentation.

## Pre-recording Setup

### 1. Download sample data

Go to: https://learn.microsoft.com/en-us/power-bi/create-reports/sample-financial-download

Save as `~/Desktop/financial-sample.xlsx`

The file has: revenue, profit, COGS, units sold by segment, country, product, month/year.

### 2. Verify slyds

```bash
slyds version
```

### 3. Create a clean working directory

```bash
mkdir ~/Desktop/finance-demo && cd ~/Desktop/finance-demo
```

### 4. Set up Python environment

We need openpyxl to read the Excel file. Use uv for a clean, isolated setup:

```bash
# Install uv if you don't have it
curl -LsSf https://astral.sh/uv/install.sh | sh

# Create a venv and install openpyxl
cd ~/Desktop/finance-demo
uv venv
source .venv/bin/activate
uv pip install openpyxl
```

Verify:
```bash
python -c "import openpyxl; print('ok')"
```

### 5. Have a browser ready to open the final file

---

## The Recording

### Scene 1 — Show the tool onboarding (~1 min, you in terminal)

> "I've got a clean directory with a Python venv — just openpyxl for reading Excel.
> Let me init a deck."

```bash
cd ~/Desktop/finance-demo
ls .venv/   # show the venv exists — one dependency, isolated
slyds init "Q4 Financial Review" --theme corporate
```

Then show the generated files:

```bash
cat AGENT.md
```

> "slyds generates an AGENT.md — and symlinks it as CLAUDE.md — so any coding
> agent that lands in this directory automatically knows the commands, layouts,
> slots, and conventions. The agent doesn't have to guess."

Show the symlink:

```bash
ls -la CLAUDE.md   # -> AGENT.md
```

### Scene 2 — Show the data (~15s)

> "Here's my Excel file — financial data with revenue, profit, segments,
> countries, products, by month."

Optionally quick-preview the Excel file.

### Scene 3 — One prompt, let the agent run (~3-5 min)

Open Claude Code in `~/Desktop/finance-demo` (the deck is already init'd,
CLAUDE.md is loaded) and paste:

```
Read ~/Desktop/financial-sample.xlsx using python/openpyxl to understand the data.

Build me a quarterly business review in this deck.

Requirements:
- Decide what story the data tells
- Use Chart.js (via CDN <script> tag) for any charts, rendered inline in slides
- 6-8 slides: title, executive summary, revenue trend over time,
  segment performance comparison, geographic breakdown, top/bottom products,
  key takeaways
- Use actual numbers from the data, not placeholders
- Run slyds build when done so I get a single distributable HTML file
```

The agent already has CLAUDE.md loaded so it knows all slyds commands. It should:
- Run python to read and analyze the xlsx
- `slyds add` for each slide with appropriate layouts
- `slyds query` to inject Chart.js canvases and data tables with real numbers
- `slyds build` at the end

### Scene 4 — Show the result (~30s)

```bash
open dist/index.html
```

Click through slides. Show:
- Charts rendering with real data from the spreadsheet
- Corporate theme applied consistently
- Press `N` to open presenter notes window
- Click the download/export button — show it packages as a zip

### Scene 5 — The punchline (~15s, you talking)

> "The agent read the CLAUDE.md, knew exactly what commands to use.
> Python read the Excel. Chart.js rendered the charts. Slyds handled files,
> ordering, theming, and packaging. One prompt, real data, distributable output.
> The agent composed existing tools — nothing custom, nothing new to install."

---

## Troubleshooting

**Agent doesn't know slyds commands:**
After `slyds init`, the deck gets an AGENT.md with all commands. If the agent
still writes raw files, nudge with: "use slyds add and slyds query, don't write
slide files directly"

**openpyxl not installed / wrong python:**
Make sure the venv is activated (`source .venv/bin/activate`) so the agent's
`python` calls use the right environment. If the agent uses `python3` instead
of `python`, both should work inside an activated venv.

**Chart.js doesn't render offline:**
The built file uses `<script src="https://cdn.jsdelivr.net/npm/chart.js">`.
Needs internet to render. For fully offline, vendor the script into the deck.

**Agent generates too few/many slides:**
Adjust the prompt — be more specific about which data dimensions to highlight.
