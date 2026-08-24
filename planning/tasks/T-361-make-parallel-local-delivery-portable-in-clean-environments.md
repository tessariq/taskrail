---
id: T-361-make-parallel-local-delivery-portable-in-clean-environments
title: Make parallel local delivery portable in clean environments
status: completed
priority: high
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies:
    - T-334-deliver-parallel-clone-batches-locally
updated_at: "2026-08-24T10:48:04Z"
completion_id: "0bfe4d713912c718b11996aa7ac708af"
last_verification_id: "0759ebd31a0a5880b2b3a4b2cd2a6554"
last_verification_result: pass
last_verified_at: "2026-08-24T10:48:04Z"
last_verified_completion_id: "0bfe4d713912c718b11996aa7ac708af"
last_verification_previous_id: "729193e6529e836b04031deafd7384f5"
---

# T-361-make-parallel-local-delivery-portable-in-clean-environments Make parallel local delivery portable in clean environments

## Description

Make T-334's local parallel delivery work from clean cross-platform Git
environments. Integration must not depend on clone-local user configuration,
and lock identity plus the deterministic test worker must use portable canonical
paths and executables.

## Acceptance

- Integration commits preserve candidate commit metadata without requiring a
  globally or clone-locally configured Git identity.
- Delegated lifecycle writers accept equivalent canonical repository paths on
  macOS while continuing to reject a genuinely different repository.
- The parallel delivery fixture uses a native cross-platform child executable
  and passes on Linux, macOS, Windows, and Linux ARM.
- Local aggregate validation, cleanup-before-publication, exact one-commit
  replay, source-drift rejection, and local fast-forward delivery remain intact.

## Verification Notes

- Reproduce the clean-environment matrix failures from exact head `fb4d222`,
  then run the focused parallel and lock suites without inherited Git identity.
- Run formatting, vet, full tests, validation, queue/task-body checks, and the
  exact-head CI matrix before resuming T-335.

## Implementation Notes

- 2026-08-24T10:34:48Z: Preserved candidate commit identity in clean integration clones, accepted filesystem-equivalent delegated roots, and replaced the shell-only parallel fixture with a native child helper; focused repetitions, full tests, vet, and manual acceptance passed.
- 2026-08-24T10:34:57Z: verification pass id 729193e6529e836b04031deafd7384f5 previous none completion 0bfe4d713912c718b11996aa7ac708af
- 2026-08-24T10:48:04Z: verification pass id 0759ebd31a0a5880b2b3a4b2cd2a6554 previous 729193e6529e836b04031deafd7384f5 completion 0bfe4d713912c718b11996aa7ac708af
