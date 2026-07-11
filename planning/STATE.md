---
schema_version: 1
updated_at: "2026-07-11T08:28:11Z"
active_spec_version: v0.3.0
active_spec_path: specs/v0.3.0.md
current_task: ""
current_task_title: ""
status_summary: blocked
blockers:
    - 'T-078: Human ops required; not doable by autonomous agent. Evidence 2026-07-11: Tessariq/winget-pkgs fork absent (gh api -> HTTP 404); WINGET_TOKEN secret absent (gh secret list tessariq/taskrail -> empty). Classic PAT creation has no GitHub API (web UI only) and the token value is a human-owned credential, so WINGET_TOKEN cannot be provisioned programmatically. Handoff: (1) fork microsoft/winget-pkgs into Tessariq org (matches .goreleaser.yaml winget.repository owner=Tessariq name=winget-pkgs), (2) create classic PAT with public_repo scope, (3) gh secret set WINGET_TOKEN --repo tessariq/taskrail. Then unblock. No code changes; goreleaser config (T-058) already correct.'
next_action: 'Start task T-066: Add taskrail coverage --min <pct>: opt-in CI exit code (validate stays advisory)'
last_verification_result: pass for T-064 at 2026-07-11T08:19:45Z
relevant_artifacts: []
continuation_notes:
    - This repository is temporarily dogfooding bootstrap workflow tooling until Taskrail v0.1.0 exists.
---

# STATE

## Active Spec

- `specs/v0.3.0.md`

## Current Focus

- Task: none

## Status

- blocked

## Blockers

- T-078: Human ops required; not doable by autonomous agent. Evidence 2026-07-11: Tessariq/winget-pkgs fork absent (gh api -> HTTP 404); WINGET_TOKEN secret absent (gh secret list tessariq/taskrail -> empty). Classic PAT creation has no GitHub API (web UI only) and the token value is a human-owned credential, so WINGET_TOKEN cannot be provisioned programmatically. Handoff: (1) fork microsoft/winget-pkgs into Tessariq org (matches .goreleaser.yaml winget.repository owner=Tessariq name=winget-pkgs), (2) create classic PAT with public_repo scope, (3) gh secret set WINGET_TOKEN --repo tessariq/taskrail. Then unblock. No code changes; goreleaser config (T-058) already correct.

## Last Verification

- pass for T-064 at 2026-07-11T08:19:45Z

## Next Action

- Start task T-066: Add taskrail coverage --min <pct>: opt-in CI exit code (validate stays advisory)

## Relevant Artifacts

- None

## Notes

- This repository is temporarily dogfooding bootstrap workflow tooling until Taskrail v0.1.0 exists.

## Task Counts

- todo: 14
- in_progress: 0
- completed: 72
- blocked: 1
- cancelled: 0
