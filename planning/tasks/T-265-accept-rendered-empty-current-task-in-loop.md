---
id: T-265-accept-rendered-empty-current-task-in-loop
title: Accept rendered empty current task in loop preflight
status: completed
priority: high
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies:
    - T-257-add-the-temporary-source-checkout-autonomous-loop
updated_at: "2026-08-08T11:48:30Z"
---

# T-265-accept-rendered-empty-current-task-in-loop Accept rendered empty current task in loop preflight

## Description

Make temporary loop preflight interpret Taskrail's canonical quoted empty
`current_task: ""` rendering as idle while continuing to reject every real active
task value.

Follow-up derived from T-257-add-the-temporary-source-checkout-autonomous-loop's verification or discovery.

## Acceptance

- A clean synchronized repository whose state renders `current_task: ""` passes
  the current-task preflight and can reach dry-run selection.
- Non-empty quoted or unquoted current-task values still stop before queue
  selection or agent launch.
- The sandbox harness uses the canonical rendered empty form so future parsing
  regressions fail before the real repository smoke check.

## Verification Notes

- Run the shell harness and a real clean synchronized `--dry-run`; confirm both
  select the expected task without launching an agent or changing repository
  state.

## Implementation Notes

- Reproduced against the real post-verification state, changed the fixture to the
  canonical quoted-empty rendering, and normalized only that exact scalar to idle.
  All non-empty values remain fail-closed.
- 2026-08-08T11:48:29Z: Accepted the canonical quoted empty current_task scalar and added regression coverage.
- 2026-08-08T11:48:30Z: verification pass
