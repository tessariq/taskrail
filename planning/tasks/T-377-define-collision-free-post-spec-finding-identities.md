---
id: T-377-define-collision-free-post-spec-finding-identities
title: Define collision-free post-spec finding identities
status: todo
priority: high
spec_ref: specs/v0.5.0.md#post-spec-review-lenses
dependencies: []
updated_at: "2026-08-28T16:13:29Z"
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
