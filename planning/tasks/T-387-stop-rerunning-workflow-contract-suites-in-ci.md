---
id: T-387-stop-rerunning-workflow-contract-suites-in-ci
title: Stop re-running workflow-contract suites in CI
status: todo
priority: medium
spec_ref: specs/v0.6.0.md#behavioral-and-release-acceptance
dependencies: []
updated_at: "2026-09-08T07:27:34Z"
---

# T-387-stop-rerunning-workflow-contract-suites-in-ci Stop re-running workflow-contract suites in CI

## Description

`.github/workflows/ci.yml` runs three overlapping lanes on every matrix leg:
`task test:filesystem`, `task test:workflow-contract`, and `task test`.
`test:workflow-contract` shells out to `go test` with the manifest's `TestRun`
regexes (`internal/taskrail/workflow_contract_runner.go`), and `test` then runs
`go test ./...`, which includes every one of those tests and every package
`test:filesystem` covers. The suites run twice on four runners.

The value of the workflow-contract runner is the manifest: a committed,
machine-readable statement of which tests constitute the behavioural contract
(T-173, T-248). That value survives without re-executing the selected tests.

The independently meaningful outcome is that each test executes once per CI leg
while the contract manifest remains checked and published.

## Acceptance

- CI keeps a step that generates the workflow-contract manifest
  (`workflow-contract --manifest`) and fails when any listed `TestRun` pattern
  matches zero tests, so a renamed test cannot silently drop out of the
  contract; the tests themselves execute only through `task test`.
- `task test:filesystem` remains available for local verbose capability-skip
  inspection and runs verbosely on at most one CI leg, not the full matrix.
- Total per-leg CI wall time drops and the set of tests executed per leg is
  unchanged; nothing is skipped that ran before.
- `docs/workflow/development-workflow.md` and the manifest documentation describe
  the manifest as the contract index and `task test` as the executor.
- Changing CI checks is an ask-first change: the task records maintainer
  approval in its implementation notes before the workflow edit lands.

## Verification Notes

- Compare `go test -list` output for the manifest packages before and after
  against the manifest regexes to prove the executed set is identical.
- Inspect the CI run for one PR: each matrix leg shows exactly one execution of
  the contract tests; record the per-leg durations before and after.
- Evidence: the CI run URL, the `--manifest` output, and the durations table.

## Implementation Notes
