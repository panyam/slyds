---
name: slyds-add-slide
description: Insert a new slide at a position with a layout template
argument-hint: [position] [name]
allowed-tools: Bash(slyds *)
---

Insert a new slide into the deck.

If arguments are provided, use them directly:
  `slyds add . $0 --name $1 --layout content`

If no arguments, ask the user for:
- **Position** (1-based, where to insert)
- **Name** (slug for the filename, e.g. "key-metrics")
- **Layout** (one of: title, content, two-col, section, blank, closing)
- **Title** (optional display title)

Then run:
  `slyds add . <position> --name <name> --layout <layout> --title "<title>"`

After inserting, show the updated slide list:
  `slyds ls --json`
