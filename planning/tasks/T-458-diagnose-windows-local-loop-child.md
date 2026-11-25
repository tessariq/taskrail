---
id: T-458-diagnose-windows-local-loop-child
title: Diagnose Windows local loop child_failed after delivery
status: in_progress
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-456-map-windows-move-collision
updated_at: "2026-11-25T23:17:31Z"
---

# T-458-diagnose-windows-local-loop-child Diagnose Windows local loop child_failed after delivery

## Description

Follow-up from T-455-exercise-windows-durable-paths. Native Windows CI run
45711904788 at `474eab0` ran `TestLoopExecuteDeliversLocalLifecycleWithOneProductCommit`
for the first time. Before T-455 it was gated on the old directory-sync probe.
The loop reported `child_failed`, although postflight validation was valid, Git
was clean with one product commit, and there were no mutation or process
violations.

Diagnosis: the `delegate-local-lifecycle` test helper never reads its
delegated prompt from stdin. The loop writes the prompt and closes the pipe;
when the child exits first, the write fails and the loop correctly classifies
the stdin transport error as `child_failed`. The default 7007-byte prompt
fits Linux's 64 KiB pipe buffer but not Windows' smaller default buffer. T-424
fixed the same defect in the `delegate-unverified` helper. Reproduced on Linux
with an oversized authorized prompt; draining stdin turns it green.

Outcome: the local-lifecycle helper consumes its prompt like the other
lifecycle helpers, and the test pins this on every platform. Production failure
classification is unchanged.

## Acceptance

- The `delegate-local-lifecycle` helper drains stdin before running lifecycle
  commands.
- The test runs with an oversized authorized prompt, so an undrained prompt
  fails on Linux as it did on Windows. All existing delivery, lifecycle,
  validation and Git assertions are unchanged.
- No production loop transport or classification code changes.

## Verification Notes

- Linux red: an oversized prompt without the drain reports `child_failed` with
  valid postflight, a clean Git state and empty violations, the run 45711904788
  signature. Green with the drain.
- `gofmt`, `go vet` for linux, windows and darwin, Windows test build, targeted
  and full Linux taskrail tests.
- Native confirmation needs the next approved Windows run.

## Implementation Notes
