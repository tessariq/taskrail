---
id: T-196-integrate-stable-references-with-rename-prompts
title: Integrate stable references with rename prompts skills and policy
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

# T-196-integrate-stable-references-with-rename-prompts Integrate stable references with rename prompts skills and policy

## Description

Make import, rename, prompts, packaged skills, and autonomy policy use stable
references and actual storage paths without mutating archived history.

## Acceptance

- Import resolves external refs across roots/families while draft keys stay
  local; all new machine relationships use stable refs.
- Rename scans durable paths, applies only live generated slugs, and refuses
  archived inbound legacy references before any write.
- Prompt resolution exposes actual TASK_PATH plus task_ref/task_id/storage and
  rejects archived authoring/implementation targets.
- Packaged skills use task show/resolved paths rather than live-directory
  assumptions and remain byte-parity clean.
- Policy resolves stable/full aliases across roots, retires archived terminal
  rows, and rejects open archive targets or semantic duplicate rows.

## Verification Notes

- Map criteria to import/rename/scanner/prompt/skill/policy identity-storage
  matrices and archive sentinels.
- Run skill parity plus manual prompt, rename refusal, and policy alias
  sandboxes.

## Implementation Notes
