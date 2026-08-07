---
id: T-235-show-a-task-by-exact-id-through-active-storage
title: Show a task by exact ID through active storage
status: todo
priority: high
spec_ref: specs/v0.5.0.md#workflow-prompt-catalog-and-overrides
dependencies:
    - T-157-upgrade-repositories-transactionally-to-layout-2
    - T-223-run-every-v0-5-command-against-local-storage
updated_at: "2026-08-06T13:46:30Z"
---

# T-235-show-a-task-by-exact-id-through-active-storage Show a task by exact ID through active storage

## Description

Add storage-neutral read-only task inspection so prompts and skills never open a
logical task path directly or expose the local physical overlay.

## Acceptance

- A1. `task show` resolves one exact full v0.5 ID and emits exact Markdown in text
  or the normative schema-1 path/content/digest result.
- A2. Committed and local storage return the same logical path and semantic bytes;
  unknown, fuzzy, malformed, or recovery-fenced subjects fail without writes.
- A3. Physical local prefixes, runtime paths, and unrelated task content never
  appear in output.

## Verification Notes

- A1: exact and slug-prefix fixtures compare command bytes and raw SHA-256.
- A2: mirrored committed/local repositories and recovery fences exercise success
  and refusal with identical Git status.
- A3: output sentinels assert no `.taskrail/local/` or unrelated body leakage.

## Implementation Notes
