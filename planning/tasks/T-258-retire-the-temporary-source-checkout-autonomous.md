---
id: T-258-retire-the-temporary-source-checkout-autonomous
title: Retire the temporary source-checkout autonomous loop
status: completed
priority: high
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies:
    - T-225-prove-local-autonomous-delivery-across-git
    - T-257-add-the-temporary-source-checkout-autonomous-loop
    - T-338-guide-temporary-loop-orchestration-and-delivery
updated_at: "2026-08-28T12:16:54Z"
completion_id: "965b19bc38c99106354aeefe9fc0074f"
last_verification_id: "107c849f34caf3ab6d5cab6880adab7d"
last_verification_result: pass
last_verified_at: "2026-08-28T12:16:54Z"
last_verified_completion_id: "965b19bc38c99106354aeefe9fc0074f"
---

# T-258-retire-the-temporary-source-checkout-autonomous Retire the temporary source-checkout autonomous loop

## Description

Remove the complete temporary source-checkout autonomous mechanism after the
proper loop command and local Git delivery contract are implemented. Restore the
repository to human-invoked task execution before cross-surface and release gates.

## Acceptance

- Remove `scripts/autonomous-loop/` in full, including runner, queue, prompt,
  parent-agent operator bridge, delivery-recovery guidance, tests, and local
  guidance; remove every live documentation, task-runner, and check reference
  that treats it as executable tooling.
- Preserve only historical task/spec statements needed to explain that bootstrap
  tooling was retired; no release or adopter guidance can instruct its use.
- T-225 evidence proves the product loop before cleanup, but the v0.5 source-
  checkout exclusion remains unchanged and no unsupported self-hosting claim is
  introduced.
- T-173 and all downstream release checks run only after this task completes, and
  deterministic searches prove no temporary mechanism can ship in v0.5.0.

## Verification Notes

- Search tracked files for the removed path, operator bridge, recovery-bundle
  guidance, and command names, then run full formatting, vet, tests, task-body
  hygiene, skill parity, and Taskrail validation.
- Inspect release artifacts to prove no deleted bootstrap file is packaged.

## Implementation Notes

- 2026-08-28T12:16:41Z: Retired the source-checkout bootstrap by deleting scripts/autonomous-loop in full, removing unreleased live changelog claims, and reducing the active spec section to historical retirement evidence while preserving the v0.5 source-checkout exclusion.
- 2026-08-28T12:16:54Z: verification pass id 107c849f34caf3ab6d5cab6880adab7d previous none completion 965b19bc38c99106354aeefe9fc0074f
