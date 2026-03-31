---
name: autonomous-manual-test
description: Generate and execute a manual test plan against Taskrail task acceptance criteria in a temporary sandbox
argument-hint: "[task-id]"
---

# autonomous-manual-test

Generate and execute a manual test plan against task acceptance criteria, then produce a structured report.

## Required Flow

1. Run `go run ./cmd/taskrail validate`.
2. Read the target task file and extract its acceptance criteria.
3. Create a manual test plan at `planning/artifacts/manual-test/<task-id>/<timestamp>/plan.md`.
4. Derive numbered test steps from the acceptance criteria.
5. Execute each test step in order.
6. If a step fails, decide whether a code fix is needed, apply the smallest fix, and re-run only the affected step.
7. Write `planning/artifacts/manual-test/<task-id>/<timestamp>/report.md` with per-step results and a final verdict.
8. Clean up any temporary manual test code after the report is written.

## Test Modes

### Sandbox Mode (default)

Use sandbox mode for most Taskrail work.

- Run commands in a temporary directory or temporary repository.
- Exercise normal `go run ./cmd/taskrail ...` flows.
- Use small temporary helper programs only when direct shell commands are not enough.
- Delete any `cmd/manual-test-*/` helper directories after writing the report.

### Optional manual_test Go-tag mode

Use this only when shell-driven validation is awkward for a real CLI flow.

- Write `_manual_test.go` files with build tag `//go:build manual_test`.
- Name tests `TestManual_<Name>`.
- Run them with `go test -tags=manual_test ./<package> -run TestManual_<Name> -v -count=1`.
- Delete the `_manual_test.go` files after writing the report.

## Artifact Format

### plan.md

- task id
- generated timestamp
- test steps derived from acceptance criteria
- commands to run
- expected outcomes

### report.md

- task id
- executed timestamp
- verdict: pass, pass-with-fixes, or fail
- per-step observations
- any fixes that were applied

## Rules

- Manual testing here is for Taskrail's internal development workflow, not a required Taskrail product invariant.
- Never substitute ordinary automated tests for manual test evidence when the change needs end-to-end judgment.
- Re-run only the affected test step after a fix.
- Do not auto-select another task after manual testing completes.
- Never commit temporary manual test code.
