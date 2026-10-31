---
id: T-443-gate-sequential-loop-runs-on-caller-owned-check
title: Gate sequential loop runs on a caller-owned check
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-10-31T18:56:45Z"
completion_id: "200e729c0d594267b906b7c72b928de1"
last_verification_id: "798d05d02b6a1f6680f98c2f0e491306"
last_verification_result: pass
last_verified_at: "2026-10-31T18:56:45Z"
last_verified_completion_id: "200e729c0d594267b906b7c72b928de1"
---

# T-443-gate-sequential-loop-runs-on-caller-owned-check Gate sequential loop runs on a caller-owned check

## Description

Implement the optional `loop --check <path>` gate added to the
Cross-Platform Autonomous Loop execution contract. Today a loop child grades
its own work: it runs `complete` and `verify --result pass` itself, and
postflight accepts that as `completed_pass`. The check gives operators one
deterministic, caller-owned gate that Taskrail runs itself, without
interpreting what it runs and without provider, shell, or LLM integration.

Scope is sequential execution (`--parallel 1`): flag parsing and path
resolution like the child executable, running the check once after a
`completed_pass` per-child candidate on the delivered HEAD, read-only
enforcement through the existing before/after snapshot comparison, recorded
evidence, the new `check_failed` per-child outcome (already registered in the
spec, the machine-API companion and the code registry), and the exact
companion shapes for dry-run and result-file check evidence. Parallel use is
T-444.

## Acceptance

- `loop --check <path>` and `loop --dry-run --check <path>` are accepted; the path resolves like the child executable, and a missing or non-executable path is `invalid_arguments` before any child launches.
- After a per-child `completed_pass`, the check runs once from the repository root with no arguments, no shell, the caller environment, and `--timeout` applied; exit zero without signal or timeout passes.
- A failing, signalled, timed-out, or mutating check yields per-child outcome `check_failed`, stops the loop with a non-zero exit, and reports the delivered commit without resetting it.
- Check evidence (absolute path, executable SHA-256, exit code or signal, duration, stdout and stderr SHA-256) appears in the result file, and dry-run reports the resolved check; specs/contracts/v0.5.0-machine-api.md defines both exact shapes.
- Without `--check`, sequential behavior and output are unchanged apart from the new nullable member.
- docs/loop.md, `loop --help`, and the packaged taskrail-loop skill describe the check as a caller-owned choice.

## Verification Notes

- Integration tests through `LoopExecute` with helper checks that pass, fail, time out, and write to the worktree.
- Contract test decoding a result file strictly against the companion shapes.
- Manual CLI run in a temp repo with a stub child and a shell-script check.

## Implementation Notes

- 2026-10-31T18:56:33Z: Implemented optional sequential caller-owned checks with frozen executable evidence, read-only enforcement, timeout containment, preserved delivery diagnostics, and matching help, docs and packaged skill guidance. Bounded independent review and disposable integrated QA passed.
- 2026-10-31T18:56:45Z: verification pass id 798d05d02b6a1f6680f98c2f0e491306 previous none completion 200e729c0d594267b906b7c72b928de1
