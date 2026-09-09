---
schema_version: 2
updated_at: "2026-09-09T12:23:50Z"
active_spec_version: v0.5.0
active_spec_path: specs/v0.5.0.md
current_task: ""
current_task_title: ""
status_summary: blocked
blockers:
    - 'T-174-run-the-v0-5-0-gap-and-drift-release-gate: Release gate blocked on two confirmed current-version defects plus the operator-owned final evaluation. T-389: v0.5.0 never raises the layout to 2 (currentLayoutVersion is still 1, fresh init writes a layout-1 marker) and only 3 of the semantic writers gate on layout 2, so a layout-1 repository accepts v0.5 lifecycle, loop-policy, and verification writes and a v0.4.0 binary then silently erases loop_policy and loop_reason. T-390: taskrail loop --help describes the executing command as a preview. Both are direct gate dependencies. The final paired skill-evaluation report is still absent and remains operator-owned: no rendered report exists, the four local sessions stopped at the human worksheet, and packaged skills changed since v0.4.0. Restart the gate on fresh bytes after remediation.'
next_action: Select the next eligible task
last_verification_result: pass for T-389-require-layout-2-for-every-semantic-writer at 2026-09-09T12:23:50Z id 7fa89a74b48e0b9e9463dc56141cf79f
last_verification_id: 7fa89a74b48e0b9e9463dc56141cf79f
last_verified_completion_id: 321984797bdfb96c9114405c937f3375
relevant_artifacts: []
---

# STATE

## Active Spec

- `specs/v0.5.0.md`

## Current Focus

- Task: none

## Status

- blocked

## Blockers

- T-174-run-the-v0-5-0-gap-and-drift-release-gate: Release gate blocked on two confirmed current-version defects plus the operator-owned final evaluation. T-389: v0.5.0 never raises the layout to 2 (currentLayoutVersion is still 1, fresh init writes a layout-1 marker) and only 3 of the semantic writers gate on layout 2, so a layout-1 repository accepts v0.5 lifecycle, loop-policy, and verification writes and a v0.4.0 binary then silently erases loop_policy and loop_reason. T-390: taskrail loop --help describes the executing command as a preview. Both are direct gate dependencies. The final paired skill-evaluation report is still absent and remains operator-owned: no rendered report exists, the four local sessions stopped at the human worksheet, and packaged skills changed since v0.4.0. Restart the gate on fresh bytes after remediation.

## Last Verification

- pass for T-389-require-layout-2-for-every-semantic-writer at 2026-09-09T12:23:50Z id 7fa89a74b48e0b9e9463dc56141cf79f

## Next Action

- Select the next eligible task

## Relevant Artifacts

- None

## Task Counts

- todo: 51
- in_progress: 0
- completed: 340
- blocked: 1
- cancelled: 0
