---
id: T-384-prevent-decomposition-from-mutating-unmet-preconditions
title: Prevent decomposition from mutating unmet repository preconditions
status: completed
priority: high
spec_ref: specs/v0.5.0.md#maintainer-skill-release-evaluations
dependencies: []
updated_at: "2026-08-29T20:28:27Z"
completion_id: "e33ac7e1488aebcaa6bcde018bb51bd8"
last_verification_id: "ca64a96d614ab9a827fa94280bedb865"
last_verification_result: pass
last_verified_at: "2026-08-29T20:28:27Z"
last_verified_completion_id: "e33ac7e1488aebcaa6bcde018bb51bd8"
---

# T-384-prevent-decomposition-from-mutating-unmet-preconditions Prevent decomposition from mutating unmet repository preconditions

## Description

Make `taskrail-decompose` treat repository initialization and layout migration as
caller-owned preconditions. The skill must inspect the existing managed repository
without using fixture metadata or unmet prerequisites as authority to mutate it.

## Acceptance

- The packaged skill requires a read-only repository preflight before its
  decomposition flow and never runs `taskrail init` or performs a layout migration.
- Missing layout 2, active-spec, or published post-spec-review prerequisites cause
  an explicit stop without changing repository state.
- Embedded and committed skill copies remain byte-identical and focused automated
  coverage enforces the preflight boundary.
- A fresh replay of the untouched `taskrail-decompose-committed` case preserves
  repository validity and worktree state when its prerequisites are unmet; T-174
  owns the subsequent full clean-head paired release evaluation.

## Verification Notes

- Assert the embedded skill names the read-only preflight, caller-owned
  initialization/migration boundary, and stop behavior.
- Run skill parity, focused package tests, the full Go test suite, vet, validation,
  and the fresh paired release evaluation required by T-174.

## Implementation Notes

- 2026-08-29: The sealed release run exposed a candidate deterministic failure
  after the skill interpreted fixture metadata as authority to migrate an already
  initialized repository. Added a storage-neutral read-only preflight and made
  initialization and migration explicitly caller-owned.
- 2026-08-29: Fresh isolated replay passed with candidate deterministic grade
  `pass`; evidence is under
  `planning/artifacts/skill-evals/v0.5.0/v050-t384-replay-20260829t202533z/`.
- 2026-08-29T20:28:14Z: verification pass id a71a140221fd216489d96560382d4234 previous none completion none
- 2026-08-29T20:28:27Z: Added and verified a read-only caller-owned repository preflight for taskrail-decompose.
- 2026-08-29T20:28:27Z: verification pass id ca64a96d614ab9a827fa94280bedb865 previous none completion e33ac7e1488aebcaa6bcde018bb51bd8
