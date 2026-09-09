---
schema_version: 2
updated_at: "2026-09-09T11:02:30Z"
active_spec_version: v0.5.0
active_spec_path: specs/v0.5.0.md
current_task: ""
current_task_title: ""
status_summary: blocked
blockers:
    - 'T-174-run-the-v0-5-0-gap-and-drift-release-gate: Release gate blocked on two confirmed current-version defects plus the operator-owned final evaluation. T-389: v0.5.0 never raises the layout to 2 (currentLayoutVersion is still 1, fresh init writes a layout-1 marker) and only 3 of the semantic writers gate on layout 2, so a layout-1 repository accepts v0.5 lifecycle, loop-policy, and verification writes and a v0.4.0 binary then silently erases loop_policy and loop_reason. T-390: taskrail loop --help describes the executing command as a preview. Both are direct gate dependencies. The final paired skill-evaluation report is still absent and remains operator-owned: no rendered report exists, the four local sessions stopped at the human worksheet, and packaged skills changed since v0.4.0. Restart the gate on fresh bytes after remediation.'
    - 'T-389-require-layout-2-for-every-semantic-writer: Implementation is complete and green (layout raised to 2, strict five-key marker on fresh init and retrofit, universal incompatible_layout writer gate, full suite passing), but it cannot be delivered: the upgrade every refusal names as its remedy does not complete on this repository. taskrail init --apply --confirm-quiescent --drop-continuation-notes published the migration fence and then spun past 24 minutes of CPU, and recover --apply spun the same way; a SIGQUIT stack shows durablefs.exactName reading the whole 390-entry tasks directory once per observed leaf. It reproduces with a binary built from HEAD, so it predates this change. The repository was restored through the Git escape the fenced-marker error names; no task, spec, or state byte was published. Recorded as dependency T-392, which must land first: committing this gate while the upgrade cannot finish would leave this repository writable by nothing.'
next_action: Select the next eligible task
last_verification_result: pass for T-393-report-lock-takeover-operands-when-recovery-is at 2026-09-09T11:02:30Z id aae5f1bd06a3aa44c643d8656a0a3c63
last_verification_id: aae5f1bd06a3aa44c643d8656a0a3c63
last_verified_completion_id: 0d5ed744a08c02c570a7a0c87f01cf68
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
- T-389-require-layout-2-for-every-semantic-writer: Implementation is complete and green (layout raised to 2, strict five-key marker on fresh init and retrofit, universal incompatible_layout writer gate, full suite passing), but it cannot be delivered: the upgrade every refusal names as its remedy does not complete on this repository. taskrail init --apply --confirm-quiescent --drop-continuation-notes published the migration fence and then spun past 24 minutes of CPU, and recover --apply spun the same way; a SIGQUIT stack shows durablefs.exactName reading the whole 390-entry tasks directory once per observed leaf. It reproduces with a binary built from HEAD, so it predates this change. The repository was restored through the Git escape the fenced-marker error names; no task, spec, or state byte was published. Recorded as dependency T-392, which must land first: committing this gate while the upgrade cannot finish would leave this repository writable by nothing.

## Last Verification

- pass for T-393-report-lock-takeover-operands-when-recovery-is at 2026-09-09T11:02:30Z id aae5f1bd06a3aa44c643d8656a0a3c63

## Next Action

- Select the next eligible task

## Relevant Artifacts

- None

## Task Counts

- todo: 51
- in_progress: 0
- completed: 339
- blocked: 2
- cancelled: 0
