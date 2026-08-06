---
id: T-207-normalize-planning-sources-into-importdraft-v3
title: Normalize planning sources into ImportDraft v3
status: todo
priority: high
spec_ref: specs/v0.7.0.md#reviewed-semantic-mapping
dependencies:
    - T-203-define-planning-source-and-provenance-contracts
    - T-204-snapshot-planning-sources-with-digest-boundaries
updated_at: "2026-08-05T19:18:08Z"
---

# T-207-normalize-planning-sources-into-importdraft-v3 Normalize planning sources into ImportDraft v3

## Description

Implement the strict reviewed-mapping boundary that connects a deterministic
planning-source descriptor to a final `ImportDraft` v3 and real anchors in one
selected local Taskrail spec. The mapping is a human/agent-authored semantic
bridge: Taskrail proves exact source coverage, digest and review bindings,
draft-key linkage, and local-anchor validity, but never infers source meaning or
accepts source-system evidence as a task `spec_ref`.

## Acceptance

- Mapping schema version 1 is decoded as one exact UTF-8 JSON object: BOM, NUL,
  trailing values, duplicate or unknown keys, missing fields, `null`, invalid
  integer forms, invalid portable IDs/keys, duplicate array members, and
  non-canonical review timestamps are rejected deterministically.
- The mapping binds exactly to the freshly inspected profile name/version,
  canonical source root/trust/aggregate digest, selected local spec
  version/path/raw digest/trust, final draft raw digest/trust, and the mapping's
  derived trust plus v3 `review_session_id`; stale or mismatched bindings fail
  without repair or normalization of the reviewed file.
- Every descriptor role/path pair appears in at least one mapping item with a
  valid inclusive one-based range in the exact decoded source, including CRLF
  and final-unterminated-line handling. Each item has sources, live local
  anchors, rationale, and a valid `task` or `no-task` disposition with the
  required task-key cardinality.
- Every final draft key is named by at least one `task` item, every named key
  exists, and the task's one normalized `spec_ref` appears in that same item's
  `spec_refs`. References resolve to live headings in the selected local spec;
  external paths, URLs, source-system paths or anchors, another spec version,
  and traversal spellings are refused.
- Planning-source normalization accepts exactly the existing `ImportDraft`
  schema version 3 with `target: tasks`, required `spec_sections: []`, unique non-empty
  keys, complete reviewed v2 bodies, and exactly one selected-spec `spec_ref`
  per task. Source import rejects v1, v2, and v4 rather than introducing or
  implying an `ImportDraft` v4.
- Generated and supplied opaque identities, complete-ledger collision checks,
  draft-local and external dependencies, cancellation/archive constraints,
  quoting, filename portability, and all-or-none preflight retain existing v3
  behavior. Normalization creates only fresh live candidates and never treats
  an identity collision as permission to overwrite, merge, reopen, or update.
- Validation remains mechanical: review mode and rationale are recorded
  assertions, not authenticated semantic proof. The binary does not infer
  titles, priorities, task boundaries, dependencies, identities, acceptance
  criteria, source equivalence, or anchor choices from planning-source prose.

## Verification Notes

- Map the strict-decoder criterion to focused mapping schema table tests and
  golden fixtures under `internal/taskrail/`, covering every malformed JSON,
  grammar, review, timestamp, duplicate, and cardinality case.
- Map source coverage and binding criteria to descriptor/mapping/spec/draft
  fixture tests that exercise exact role/path coverage, LF/CRLF line ranges,
  stale raw digests, wrong sessions/profiles/roots/specs, and both dispositions.
- Map draft-key and anchor criteria to tests for unknown and unmapped keys,
  missing headings, cross-version and external references, and successful
  multiple-item coverage of one key without semantic inference.
- Map v3-only and preserved-import behavior to source-import preflight tests for
  explicit v1/v2/v4 rejection, generated and opaque IDs, archive-inclusive
  allocation, dependency resolution, complete bodies, target restrictions, and
  collision refusal with no writes.
- Record the reviewed semantic-handoff sandbox and command transcript in
  `planning/artifacts/manual-test/T-207/<timestamp>/report.md`, including the
  final descriptor, spec, draft, mapping, expected failures, and clean-tree
  proof; verification artifacts remain uncommitted.

## Implementation Notes
