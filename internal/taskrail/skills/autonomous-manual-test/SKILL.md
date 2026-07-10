---
name: autonomous-manual-test
description: Generate and execute a manual test plan against Taskrail task acceptance criteria in a temporary sandbox
argument-hint: "[task-id]"
---

# autonomous-manual-test

Generate and execute a manual test plan against a task's acceptance criteria, then
produce a structured report.

Requires the installed `taskrail` binary on `PATH`.

## Required Flow

1. Run `${TASKRAIL:-taskrail} validate`.
2. Read the target task file and extract its acceptance criteria.
3. Create a manual test plan at
   `planning/artifacts/manual-test/<task-id>/<timestamp>/plan.md`.
4. Derive numbered test steps from the acceptance criteria.
5. Execute each test step in order.
6. If a step fails, decide whether a code fix is needed, apply the smallest fix,
   and re-run only the affected step.
7. Write `planning/artifacts/manual-test/<task-id>/<timestamp>/report.md` with
   per-step results and a final verdict.
8. Clean up any temporary manual test code after the report is written.

## Sandbox Mode

Prefer a sandbox for most manual testing.

- Run commands in a temporary directory or temporary repository.
- Exercise normal `${TASKRAIL:-taskrail} ...` flows.
- Use small temporary helper programs only when direct shell commands are not
  enough; delete their directories and files as part of the step 8 cleanup.

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

- manual-test artifacts under `planning/artifacts/manual-test/` are ephemeral,
  gitignored evidence; never commit them
- never substitute ordinary automated tests for manual test evidence when the
  change needs end-to-end judgment
- re-run only the affected test step after a fix
- do not auto-select another task after manual testing completes
- never commit temporary manual test code
