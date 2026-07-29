---
id: T-150-refresh-v0-4-workflow-and-packaged-skill-guidance
title: Refresh v0.4 workflow and packaged skill guidance
status: completed
priority: medium
spec_ref: specs/v0.4.0.md#explicitly-excluded
dependencies:
    - T-104
    - T-113-spec-diff
    - T-114-task-repoint
updated_at: "2026-07-29T18:01:23Z"
---

# T-150-refresh-v0-4-workflow-and-packaged-skill-guidance Refresh v0.4 workflow and packaged skill guidance

## Description

Repository workflow docs and the packaged spec skill lag the v0.4.0 contract. Two
workflow pages tell contributors to commit gitignored manual-test artifacts, the
autonomous contract names a retired skill source and `go run` transition path, the
spec skill omits the v0.4 migration/authoring commands, and BACKLOG still calls
implemented `spec diff` deferred.

## Acceptance

- Human/development workflow docs consistently say manual-test and verify artifacts
  are ephemeral and never committed, matching README, `.gitignore`, AGENTS, and the
  packaged manual-test skill.
- `docs/workflow/autonomous-contract.md` names `internal/taskrail/skills` as the
  package source, committed parity copies correctly, and `${TASKRAIL:-taskrail}` as
  the transition path.
- The packaged `taskrail-spec` skill covers `spec diff`, `task new --area`, and
  `task repoint` in the active-spec migration loop without inventing runtime
  orchestration.
- BACKLOG wording treats `spec diff` and `task repoint` as shipped prerequisites;
  AGENTS lists `specs/v0.4.0.md` among source-of-truth specs.
- Regenerated `.agents`/`.claude` copies pass `task check:skills`, task-body hygiene
  passes, and docs no longer contradict the explicit artifact exclusion.

## Verification Notes

- T-140 semantic review found the conflicting guidance while walking documentation
  and packaged skills against the explicit exclusions.

## Implementation Notes

- Run `task skills:regen` after changing packaged skill source; do not edit parity
  copies independently.
- 2026-07-29T18:01:16Z: verification pass
- 2026-07-29T18:01:23Z: Passed go test ./..., go vet ./..., skill parity, task-body hygiene, validate, dedicated simplifier/review, and manual acceptance evidence under planning/artifacts/manual-test/T-150-refresh-v0-4-workflow-and-packaged-skill-guidance/20260729T175958Z/
