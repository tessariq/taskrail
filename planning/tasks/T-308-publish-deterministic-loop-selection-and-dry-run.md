---
id: T-308-publish-deterministic-loop-selection-and-dry-run
title: Publish deterministic loop selection and dry-run
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-170-add-deterministic-autonomous-loop-preflight-and
updated_at: "2026-08-22T12:24:11Z"
completion_id: "171eafb1dd36c3a834f8a05d786b8461"
last_verification_id: "aff6c098dff00e9beef7d2cad04c17d1"
last_verification_result: pass
last_verified_at: "2026-08-22T12:24:11Z"
last_verified_completion_id: "171eafb1dd36c3a834f8a05d786b8461"
---

# T-308-publish-deterministic-loop-selection-and-dry-run Publish deterministic loop selection and dry-run

## Description

Use the frozen T-170 preflight to select explicit task-local allowances,
authorize the exact implementation prompt template, render the selected task,
and publish the complete read-only loop dry-run contract. Execution remains
outside this task.

## Acceptance

- Selection applies ordinary deterministic active-spec ranking to explicit
  task-local `allow` entries. Independent held tasks are transparent, a held
  dependency disqualifies only its dependents, and no eligible allowance returns
  `action:none` without selecting work or launching a process.
- Built-in prompts are inherently authorized. A replacement is runnable only when
  its exact template-byte SHA-256 equals
  `--allow-prompt-override-sha256`; absent or mismatched consent returns
  `action:invalid`, while consent supplied without a replacement is
  `invalid_arguments`. Authorization never hashes rendered content.
- For `action:run`, prompt source/path/template bytes and hash are taken from the
  preflight snapshot, rendering is frozen for the selected task, and the reported
  rendered SHA-256 matches the exact UTF-8 content later eligible for stdin.
  `action:none` fabricates neither a selected task nor a prompt.
- Dry-run emits the uniform envelope with the exact `LoopDryRunResult` fields,
  nullability, enums, task-loop row shape, configured/effective broad-round
  limits with default `1`, three-reviewer ceiling as capability rather than the
  normal reviewer count, conditional final-diff rule,
  execution/delivery facts, and ordered violations defined by the v0.5 machine
  API. Valid `run` and `none` exit zero; report-result `invalid` exits non-zero.
- Committed and local dry-runs perform all applicable repository, policy, prompt,
  and lock checks while leaving managed semantic bytes, Git index/status/refs,
  exclusions, and runtime state byte-identical and launching no child.

## Verification Notes

- Golden CLI fixtures cover exact text/JSON for `run`, `none`, and each `invalid`
  boundary, including nullability, ordering, review override, timeout, storage,
  delivery, held-task bypass, and held-dependency isolation. Review fixtures prove
  configured/effective default `1` and explicit override `2`.
- Built-in/replacement fixtures perturb template and rendered bytes independently
  to prove template-only authorization, stale digest refusal, and exact rendering
  hashes.
- No-launch helpers plus before/after committed/local repository snapshots prove
  dry-run purity and report-result exit classification.

## Implementation Notes

- 2026-08-22T12:23:58Z: Published deterministic loop dry-run selection and frozen prompt authorization.
- 2026-08-22T12:24:11Z: verification pass id aff6c098dff00e9beef7d2cad04c17d1 previous none completion 171eafb1dd36c3a834f8a05d786b8461
