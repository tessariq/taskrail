---
id: T-179-resolve-stable-task-references-across-every
title: Resolve stable task references across every identity surface
status: todo
priority: high
spec_ref: specs/v0.6.0.md#shared-task-reference-resolver-and-inspection
dependencies:
    - T-176-classify-persisted-and-legacy-task-identities
    - T-178-load-live-and-archived-tasks-as-one-immutable
updated_at: "2026-08-04T23:06:23Z"
---

# T-179-resolve-stable-task-references-across-every Resolve stable task references across every identity surface

## Description

Build the complete-ledger resolver and core representation helpers so slug
changes, legacy spellings, opaque IDs, and storage moves do not change durable
identity. Exhaustive command adoption belongs to later integration tasks.

## Acceptance

- Bare positive refs inspect the full numeric claimant set before exact lookup;
  other selectors use exact full ID then reserved digest resolution without
  fuzzy/title/stale-slug matching.
- Resolver output always carries task_ref, full task_id, status, storage, and
  path and lists every claimant on ambiguity.
- Core relationship validation rejects semantic self/duplicate aliases after
  resolution and exposes stable-reference normalization for writers.
- Stable-reference normalization applies to task and relationship references;
  task-local `loop_policy` and `loop_reason` remain part of task bytes and are
  never resolved as independently keyed policy data.
- YAML/JSON/TSV/text/artifact and graph-key helpers implement exact quoting,
  control escaping, safe artifact component, and injective node identity
  behavior.
- Resolver APIs retain source-layout exact-full-ID mode for collision-map
  migration before enabling layout-3 ambiguity semantics.

## Verification Notes

- Map criteria to selector/alias matrices spanning all identity/storage
  classes, duplicate claimants, source-layout mapping semantics, task-local
  loop-field preservation, scalar strings, controls, and graph punctuation.
- Prove resolver output is independent of loader root order and never mutates
  task bytes.

## Implementation Notes
