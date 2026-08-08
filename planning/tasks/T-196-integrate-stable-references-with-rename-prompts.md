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
updated_at: "2026-08-08T08:40:49Z"
---

# T-196-integrate-stable-references-with-rename-prompts Integrate stable references with rename prompts and skills

## Description

Make import, rename, prompts, and packaged skills use stable references and
canonical logical storage paths without mutating archived history.

## Acceptance

- Import resolves external refs across roots/families while draft keys stay
  local; all new machine relationships use stable refs.
- Rename scans durable paths, applies only live generated slugs, and refuses
  archived inbound legacy references before any write.
- Prompt resolution exposes canonical logical TASK_PATH plus task_ref/task_id/storage and
  rejects archived authoring/implementation targets through exact task-targeting
  prompt contract v2 placeholders `TASK_REF` and `TASK_STORAGE`; explicit v1 stays
  compatible, default resolution selects v2, replacement lookup uses the v2
  directory, non-task prompts remain v1, new schema-v2 task reviews bind v2, and
  loop prompt diagnostics identify contract v2 explicitly.
- Packaged skills use task show/spec show rather than live-directory or physical-overlay
  assumptions and remain byte-parity clean. Stable-reference preference applies
  only when a reference is otherwise authorized; local commit metadata retains
  v0.5's boundary where visible policy governs generic conventions but only
  caller-owned instruction outside managed planning authorizes exposing local
  Taskrail identity/path provenance.
- New task-review artifacts use schema v2 with stable `task_ref` plus historical
  snapshot ID/path, and workflow memory v2 migrates tracked findings to paired
  stable reference and historical ID without rewriting immutable v1 reports.
- Rename/archive/restore/validate scanners exempt only top-level `task_path` in a
  strictly decoded schema-v1/v2 task review; malformed review JSON, prose, and
  every other path occurrence retain ordinary blocking behavior. This task
  registers and verifies v2 through T-181's exception mechanism without making the
  upstream scanner depend on v2.
- Task-local `loop_policy` and `loop_reason` remain attached to resolved task
  bytes and survive live slug rename unchanged; archived tasks remain immutable
  and no separate loop-policy identity or publication exists.

## Verification Notes

- Map criteria to import/rename/scanner/prompt/skill identity-storage matrices,
  task-local loop-field preservation, and archive sentinels.
- Run skill parity plus manual prompt, rename refusal, and loop-field
  preservation sandboxes, including local delivery with generic visible policy,
  caller-owned reference authorization, and delayed/current planning-derived
  self-authorization attempts.

## Implementation Notes
