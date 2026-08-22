---
id: T-303-align-native-task-producers-with-the-body-contract
title: Align native task producers with the body contract
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#outcome-focused-task-authoring
dependencies:
    - T-251-ship-the-outcome-focused-task-authoring-prompt
updated_at: "2026-08-22T10:47:34Z"
completion_id: "ab74d55cddabb11f9a86fdde9003c932"
last_verification_id: "0423f52a8b319cdc7fd7fff74de717ac"
last_verification_result: pass
last_verified_at: "2026-08-22T10:47:34Z"
last_verified_completion_id: "ab74d55cddabb11f9a86fdde9003c932"
---

# T-303-align-native-task-producers-with-the-body-contract Align native task producers with the body contract

## Description

Align Taskrail's native task scaffold and verification/implementation follow-up
creation with the shared body shape and implicit-hold contract.

## Acceptance

- A1. `task new` scaffolds exactly one ordered Description, Acceptance,
  Verification Notes, and Implementation Notes structure that asks for
  criterion-to-evidence mapping and later evidence paths without claiming semantic
  sizing certification.
- A2. Native verification and implementation follow-ups use the same required
  body sections and outcome-focused guidance; generated descriptions identify the
  independently meaningful deferred outcome and integrated owner where applicable.
- A3. Every native scaffold/follow-up omits `loop_policy` and `loop_reason`, remains
  implicitly held, and cannot derive unattended authorization from body text.
- A4. Existing identity, dependency, provenance, lifecycle, and state-projection
  behavior is preserved; no duplicate scaffold heading is emitted.

## Verification Notes

- A1/A2: golden normal and follow-up task outputs map each required section to the
  shared contract and include fragmented/oversized guidance fixtures.
- A3/A4: loop-policy escalation, duplicate-heading, dependency/provenance, and
  projection fixtures prove implicit hold and unchanged native semantics.

## Implementation Notes

- 2026-08-22T10:47:33Z: Aligned native task producers with the shared outcome-focused body contract and safeguards.
- 2026-08-22T10:47:34Z: verification pass id 0423f52a8b319cdc7fd7fff74de717ac previous none completion ab74d55cddabb11f9a86fdde9003c932
