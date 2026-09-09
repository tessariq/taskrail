---
schema_version: 1
updated_at: "2026-09-09T08:47:10Z"
active_spec_version: v0.5.0
active_spec_path: specs/v0.5.0.md
current_task: ""
current_task_title: ""
status_summary: blocked
blockers:
    - 'T-174-run-the-v0-5-0-gap-and-drift-release-gate: Release gate blocked on two confirmed current-version defects plus the operator-owned final evaluation. T-389: v0.5.0 never raises the layout to 2 (currentLayoutVersion is still 1, fresh init writes a layout-1 marker) and only 3 of the semantic writers gate on layout 2, so a layout-1 repository accepts v0.5 lifecycle, loop-policy, and verification writes and a v0.4.0 binary then silently erases loop_policy and loop_reason. T-390: taskrail loop --help describes the executing command as a preview. Both are direct gate dependencies. The final paired skill-evaluation report is still absent and remains operator-owned: no rendered report exists, the four local sessions stopped at the human worksheet, and packaged skills changed since v0.4.0. Restart the gate on fresh bytes after remediation.'
    - 'T-389-require-layout-2-for-every-semantic-writer: Implementation is complete and green (layout raised to 2, strict five-key marker on fresh init and retrofit, universal incompatible_layout writer gate, full suite passing), but it cannot be delivered: the upgrade every refusal names as its remedy does not complete on this repository. taskrail init --apply --confirm-quiescent --drop-continuation-notes published the migration fence and then spun past 24 minutes of CPU, and recover --apply spun the same way; a SIGQUIT stack shows durablefs.exactName reading the whole 390-entry tasks directory once per observed leaf. It reproduces with a binary built from HEAD, so it predates this change. The repository was restored through the Git escape the fenced-marker error names; no task, spec, or state byte was published. Recorded as dependency T-392, which must land first: committing this gate while the upgrade cannot finish would leave this repository writable by nothing.'
next_action: Resolve verification findings for T-389-require-layout-2-for-every-semantic-writer
last_verification_result: fail for T-389-require-layout-2-for-every-semantic-writer at 2026-09-09T08:47:10Z id b4611dd5cbc7cd2d4c48d9b82a8363eb
last_verification_id: b4611dd5cbc7cd2d4c48d9b82a8363eb
relevant_artifacts: []
continuation_notes:
    - This repository is temporarily dogfooding bootstrap workflow tooling until Taskrail v0.1.0 exists.
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

- fail for T-389-require-layout-2-for-every-semantic-writer at 2026-09-09T08:47:10Z id b4611dd5cbc7cd2d4c48d9b82a8363eb

## Next Action

- Resolve verification findings for T-389-require-layout-2-for-every-semantic-writer

## Relevant Artifacts

- None

## Notes

- This repository is temporarily dogfooding bootstrap workflow tooling until Taskrail v0.1.0 exists.

## Task Counts

- todo: 52
- in_progress: 0
- completed: 337
- blocked: 2
- cancelled: 0
