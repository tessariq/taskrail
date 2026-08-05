---
id: T-196-integrate-stable-references-with-rename-prompts
title: Integrate stable references with rename prompts and skills
status: todo
priority: high
spec_ref: specs/v0.6.0.md#unified-workflow-and-loop-integration
dependencies:
    - T-179-resolve-stable-task-references-across-every
    - T-191-add-stable-task-inspection-and-filtered-inventory
    - T-194-add-explicit-archive-and-restore-commands
    - T-192-protect-archived-history-across-all-semantic
updated_at: "2026-08-04T23:06:23Z"
---

# T-196-integrate-stable-references-with-rename-prompts Integrate stable references with rename prompts and skills

## Description

Make import, rename, prompts, and packaged skills use stable references and
actual storage paths without mutating archived history.

## Acceptance

- Import resolves external refs across roots/families while draft keys stay
  local; all new machine relationships use stable refs.
- Rename scans durable paths, applies only live generated slugs, and refuses
  archived inbound legacy references before any write.
- Prompt resolution exposes actual TASK_PATH plus task_ref/task_id/storage and
  rejects archived authoring/implementation targets.
- Packaged skills use task show/resolved paths rather than live-directory
  assumptions and remain byte-parity clean.
- Task-local `loop_policy` and `loop_reason` remain attached to resolved task
  bytes and survive live slug rename unchanged; archived tasks remain immutable
  and no separate loop-policy identity or publication exists.

## Verification Notes

- Map criteria to import/rename/scanner/prompt/skill identity-storage matrices,
  task-local loop-field preservation, and archive sentinels.
- Run skill parity plus manual prompt, rename refusal, and loop-field
  preservation sandboxes.

## Implementation Notes
