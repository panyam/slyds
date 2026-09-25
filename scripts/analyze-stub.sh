#!/bin/sh
# Stand-in analyzer for `make demo-tasks`: reads the prompt on stdin, waits a
# moment so the per-slide task status is visible, and prints a canned review.
# Swap in a real one with ANALYZE_CMD='agent -p --output-format text {prompt}'.
words=$(wc -w | tr -d ' ')
sleep "${ANALYZE_STUB_DELAY:-1}"
echo "Stub review: the prompt for this slide was $words words. Point ANALYZE_CMD at a real model for an actual critique."
