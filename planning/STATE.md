---
schema_version: 1
updated_at: "2026-08-14T13:35:45Z"
active_spec_version: v0.5.0
active_spec_path: specs/v0.5.0.md
current_task: ""
current_task_title: ""
status_summary: blocked
blockers:
    - 'T-277-add-durable-transaction-journals-and-recovery: Correctness review found the proposed portable path-based journal cannot satisfy A1-A4: preparation can strand an unrecoverable fence, recovery lacks lock-bound final CAS, path identity is not no-follow/handle-bound, retained fences do not block normal readers/writers, and post-rename fsync failure can lose recovery evidence. Operator must approve decomposition around a handle-bound filesystem primitive plus global fence integration, or revise the portability contract.'
    - 'T-322-provide-handle-bound-durable-filesystem-primitives: A1-A2 require atomic wrong-leaf refusal, but Linux/macOS renameat/unlinkat mutate names without an expected retained-handle identity CAS; leaf and hard-link substitution remains possible between every check and mutation. Generic restart IDs also cannot prove non-reuse on all required filesystems. Revising concurrency scope or approving narrower platform/filesystem mechanisms is required.'
next_action: Resolve verification findings for T-322-provide-handle-bound-durable-filesystem-primitives
last_verification_result: fail for T-322-provide-handle-bound-durable-filesystem-primitives at 2026-08-14T13:35:45Z
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

- T-277-add-durable-transaction-journals-and-recovery: Correctness review found the proposed portable path-based journal cannot satisfy A1-A4: preparation can strand an unrecoverable fence, recovery lacks lock-bound final CAS, path identity is not no-follow/handle-bound, retained fences do not block normal readers/writers, and post-rename fsync failure can lose recovery evidence. Operator must approve decomposition around a handle-bound filesystem primitive plus global fence integration, or revise the portability contract.
- T-322-provide-handle-bound-durable-filesystem-primitives: A1-A2 require atomic wrong-leaf refusal, but Linux/macOS renameat/unlinkat mutate names without an expected retained-handle identity CAS; leaf and hard-link substitution remains possible between every check and mutation. Generic restart IDs also cannot prove non-reuse on all required filesystems. Revising concurrency scope or approving narrower platform/filesystem mechanisms is required.

## Last Verification

- fail for T-322-provide-handle-bound-durable-filesystem-primitives at 2026-08-14T13:35:45Z

## Next Action

- Resolve verification findings for T-322-provide-handle-bound-durable-filesystem-primitives

## Relevant Artifacts

- None

## Notes

- This repository is temporarily dogfooding bootstrap workflow tooling until Taskrail v0.1.0 exists.

## Task Counts

- todo: 136
- in_progress: 0
- completed: 184
- blocked: 2
- cancelled: 0
