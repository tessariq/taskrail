---
id: T-327-validate-temporary-loop-commit-metadata
title: Validate temporary-loop commit metadata before child exit
status: completed
priority: high
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies:
    - T-316-harden-temporary-autonomous-loop-timeout-and
updated_at: "2026-08-17T14:36:32Z"
---

# T-327-validate-temporary-loop-commit-metadata Validate temporary-loop commit metadata before child exit

## Description

Close the temporary parent-owned delivery handoff gap by requiring the selected
child to validate and repair its prospective commit message before exiting. Keep
the parent checker and hook enforcement as independent trust-boundary backstops,
without adding another child launch or autonomous retry.

## Acceptance

- A1. The bootstrap-loop contract and rendered child prompt require the child to
  run the repository's exact commit-message checker after writing the message,
  fix any failure within the same process, and exit zero only after it passes.
- A2. The child also mechanically checks that the subject ends with the selected
  short task key; the parent independently retains both validations.
- A3. Harness coverage pins the self-validation instructions for both backends
  and proves malformed metadata cannot commit, push, or launch another child.
- A4. Timeout, recovery-bundle, queue, hook, and no-retry behavior remain
  unchanged; same-process metadata correction is explicitly not a retry.

## Verification Notes

- A1/A2: prompt-contract assertions inspect the exact rendered T-900 fixture.
- A3: malformed-message fixtures cover body wrapping and task-key suffix while
  observing unchanged HEAD/remote and one child invocation.
- A4: run the complete temporary-loop harness and repository workflow checks.

## Implementation Notes

- 2026-08-17T14:36:23Z: Required the temporary-loop child to run the exact repository commit-message checker and selected short-key check before zero exit, repair failures within the same process, and retain independent parent validation; aligned the narrow v0.5 bootstrap contract, operator guidance, and sandbox harness.
- 2026-08-17T14:36:32Z: verification pass
