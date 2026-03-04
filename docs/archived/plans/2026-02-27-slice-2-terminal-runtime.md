---
title: "Slice 2: Terminal Runtime Hardening"
status: completed
date: 2026-02-27
owner: platform
---

# Goal

Deliver Slice 2 acceptance:

1. deterministic command/runtime behavior under parallel usage,
2. reconnect preserves visible terminal continuity.

# Todo Breakdown

- [x] Add explicit slice todo plan document and scope.
- [x] Harden lifecycle output replay by bootstrapping monitor buffers from tmux pane snapshots.
- [x] Make `since` filtering exclusive to prevent duplicate replay rows on reconnect/poll cycles.
- [x] Add/adjust tests for monitor bootstrap and exclusive replay semantics.
- [x] Run focused backend tests for session/api runtime paths.
- [x] Run full package tests for touched modules.
- [x] Self-review changes for race/safety regressions and patch if needed.

# Notes

Scope is intentionally constrained to runtime reliability primitives and replay behavior.
UI-level terminal polish and advanced session controls remain in next slices.
