---
schema_version: 1
updated_at: "2026-09-08T11:56:17Z"
active_spec_version: v0.5.0
active_spec_path: specs/v0.5.0.md
current_task: ""
current_task_title: ""
status_summary: blocked
blockers:
    - 'T-174-run-the-v0-5-0-gap-and-drift-release-gate: Release gate blocked: the final skill-evaluation scenarios did not establish the committed and positive-path states their prompts claimed, allowing deterministic passes over semantically mismatched fixtures; T-385 owns remediation before one fresh final run.'
    - 'T-385-make-skill-evaluation-scenarios-match-their-claims: Deterministic scope is complete and green: the registry refuses a case whose setup does not commit a real HEAD and create a tracked-work subject, a failed setup action now grades the arm fail, git-worktree-clean requires an actually empty worktree, and all 32 cases execute end to end against the working binary. The remaining acceptance criterion is a focused model-backed replay through the caller-owned provider adapter; provider execution and credentials are operator-owned, so this unattended run did not invoke one.'
next_action: Resolve verification findings for T-385-make-skill-evaluation-scenarios-match-their-claims
last_verification_result: fail for T-385-make-skill-evaluation-scenarios-match-their-claims at 2026-09-08T11:56:17Z id e2300a54329669025922479ac302c35b
last_verification_id: e2300a54329669025922479ac302c35b
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

- T-174-run-the-v0-5-0-gap-and-drift-release-gate: Release gate blocked: the final skill-evaluation scenarios did not establish the committed and positive-path states their prompts claimed, allowing deterministic passes over semantically mismatched fixtures; T-385 owns remediation before one fresh final run.
- T-385-make-skill-evaluation-scenarios-match-their-claims: Deterministic scope is complete and green: the registry refuses a case whose setup does not commit a real HEAD and create a tracked-work subject, a failed setup action now grades the arm fail, git-worktree-clean requires an actually empty worktree, and all 32 cases execute end to end against the working binary. The remaining acceptance criterion is a focused model-backed replay through the caller-owned provider adapter; provider execution and credentials are operator-owned, so this unattended run did not invoke one.

## Last Verification

- fail for T-385-make-skill-evaluation-scenarios-match-their-claims at 2026-09-08T11:56:17Z id e2300a54329669025922479ac302c35b

## Next Action

- Resolve verification findings for T-385-make-skill-evaluation-scenarios-match-their-claims

## Relevant Artifacts

- None

## Notes

- This repository is temporarily dogfooding bootstrap workflow tooling until Taskrail v0.1.0 exists.

## Task Counts

- todo: 51
- in_progress: 0
- completed: 334
- blocked: 2
- cancelled: 0
