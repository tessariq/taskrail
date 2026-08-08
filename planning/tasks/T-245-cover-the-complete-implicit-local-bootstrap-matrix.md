---
id: T-245-cover-the-complete-implicit-local-bootstrap-matrix
title: Cover the complete implicit local bootstrap matrix
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-161-apply-reviewed-task-bodies-with-compare-and-swap
    - T-163-validate-and-apply-importdraft-v2-transactionally
    - T-287-initialize-ignored-local-planning-storage-durably
    - T-314-integrate-loop-continuation-and-terminal
    - T-238-mutate-task-local-loop-policy-safely
    - T-239-edit-exact-id-dependencies-safely
updated_at: "2026-08-06T13:46:30Z"
---

# T-245-cover-the-complete-implicit-local-bootstrap-matrix Cover the complete implicit local bootstrap matrix

## Description

Apply durable local initialization before only the exact eligible writer surfaces,
including new v0.5 commands, while every read, preview, and excluded publisher
remains write-free.

## Acceptance

- A1. Every specified apply/execution writer bootstraps after syntax/Git/path checks
  and emits `local_initialized` even when later semantic work refuses.
- A2. Init, retrofit, promote, review publish, all previews/dry-runs, loop-policy
  list, and every other reader never bootstrap.
- A3. A crash during bootstrap is recoverable through the shared transaction; a
  completed bootstrap remains valid, ignored, and discoverable after later refusal.
- A4. Every implicit bootstrap leaves `.agents/`, `.claude/`, and skill exclusion
  bytes untouched; packaged skill installation or refresh never occurs implicitly.

## Verification Notes

- A1/A2: a generated command matrix runs each positive and negative surface in a
  fresh Git worktree and compares config/exclude/storage bytes plus envelopes.
- A3: failure injection at exclusion/config/spec/state/note boundaries exercises
  shared recovery and clean visible Git status.
- A4: command-matrix snapshots include both assistant roots and every effective
  exclusion source.

## Implementation Notes
