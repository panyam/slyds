---
name: slyds-check
description: Validate the deck for missing files, broken includes, and other issues
allowed-tools: Bash(slyds *)
---

Validate the presentation deck:

1. Run: `slyds check --json`
2. Parse the JSON output
3. Summarize findings:
   - Total slides and sync status
   - Errors (must fix) — list each with detail
   - Warnings (should fix) — list each with detail
   - Estimated talk time if available
4. If there are errors, suggest how to fix each one
