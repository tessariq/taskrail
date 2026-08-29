---
id: T-377-define-collision-free-post-spec-finding-identities
title: Define collision-free post-spec finding identities
status: completed
priority: high
spec_ref: specs/v0.5.0.md#post-spec-review-lenses
dependencies: []
updated_at: "2026-08-29T07:27:37Z"
completion_id: "0a590ff664dab91ca7f1df54ba2ba2a8"
last_verification_id: "a870bff9c627063846781a97e1997657"
last_verification_result: pass
last_verified_at: "2026-08-29T07:27:37Z"
last_verified_completion_id: "0a590ff664dab91ca7f1df54ba2ba2a8"
---

# T-377-define-collision-free-post-spec-finding-identities Define collision-free post-spec finding identities

## Description

Give each isolated post-spec review lens a deterministic disjoint finding-ID
namespace so independently produced observations cannot collide at publication.
This resolves final v0.5 finding GAPS-001 without letting one lens inspect another.

## Acceptance

- The v0.5 spec and all four built-in lens prompts assign and require distinct
  finding-ID prefixes while preserving session-wide uniqueness.
- Strict review validation rejects a finding using another lens's namespace and
  accepts independently produced bundles with the assigned namespaces.
- Packaged skill guidance and prompt/package parity remain current.

## Verification Notes

- Add focused schema and publication tests for correct and cross-lens IDs, then run
  prompt, review-publication, parity, full-test, and workflow-contract checks.

## Implementation Notes

- 2026-08-29T07:27:23Z: Defined disjoint CONS-, GAPS-, ADDS-, and ADV- post-spec finding namespaces; strict proposal publication now rejects cross-lens IDs while historical published bundles remain readable. Verified focused schema/publication/prompt coverage, skill parity, full tests, vet, workflow contracts, task-body hygiene, and manual acceptance evidence.
- 2026-08-29T07:27:37Z: verification pass id a870bff9c627063846781a97e1997657 previous none completion 0a590ff664dab91ca7f1df54ba2ba2a8
