---
id: T-208-publish-strict-planning-provenance-sidecars
title: Publish strict planning provenance sidecars
status: todo
priority: high
spec_ref: specs/v0.7.0.md#immutable-planning-source-receipts
dependencies:
    - T-207-normalize-planning-sources-into-importdraft-v3
updated_at: "2026-08-05T19:18:11Z"
---

# T-208-publish-strict-planning-provenance-sidecars Publish strict planning provenance sidecars

## Description

Implement canonical immutable planning-source receipts as append-only historical
sidecars. A receipt must identify one completed import from its exact canonical
payload, make duplicate profile/version/source snapshots mechanically
detectable, and remain valid without live bindings to source inputs or current
task state. No lifecycle or repair path may update a receipt or reinterpret it
as synchronization state.

## Acceptance

- Receipt schema version 1 has exactly the specified profile, ordered source
  entries, target spec, v3 draft, v1 mapping, and resulting-task fields. Unknown,
  missing, duplicate, malformed, or `null` fields and non-canonical values are
  rejected rather than tolerated or upgraded.
- The receipt ID is exactly `psr-<lower-case 64-hex>` computed from the specified
  magic/NUL prefix and compact canonical payload without `receipt_id`; payload
  field order, JSON escaping, no HTML escaping, source/task ordering, filename,
  two-space on-disk JSON, and one final LF are byte-canonical and covered by
  golden vectors.
- Receipts live only as direct regular
  `<planning-dir>/provenance/planning-sources/<receipt-id>.json` children. The
  directory is created only by a successful first apply and contains no
  `.gitkeep`; aliases, unexpected names, nested directories, links/reparse
  points, special files, filename/ID mismatches, and reformatted or truncated
  receipts fail closed.
- Before preview or apply, all receipts are validated and the tuple
  `(profile.name, profile.version, source.aggregate_sha256)` is repository-unique.
  A match fails as `duplicate_source_import` and identifies the existing receipt
  and task refs regardless of target spec, mapping, draft, or requested IDs. The
  aggregate already binds the canonical root, so moving the same file bytes to a
  different root creates a distinct snapshot rather than a duplicate tuple.
- A changed aggregate digest is a distinct append-only event that may create
  only fresh tasks and one fresh receipt. Existing opaque-ID or destination
  collisions fail; no path updates, replaces, merges, cancels, supersedes,
  deduplicates, or links tasks or receipts from an earlier import.
- Receipt validation is self-contained historical validation. Later source,
  draft, mapping, spec, task path/slug/status/storage, or archive changes do not
  create receipt drift and do not rewrite the recorded apply outcome.
- Every existing lifecycle, archive, verification, spec, loop, generic import,
  recovery, and repair writer preserves receipt bytes and paths exactly. There
  is no receipt edit, refresh, adoption, redaction, migration, deletion, or
  reconstruction command; malformed existing receipts block new source imports
  until Git restores valid historical bytes.
- Receipt publication composes with candidate tasks and projected state under the
  common durable transaction as one all-or-none write set; failure or rollback
  ambiguity can never leave a receipt that certifies only a partial task outcome.

## Verification Notes

- Map schema, serialization, and ID criteria to receipt codec unit tests and
  checked-in golden vectors under `internal/taskrail/`, including canonical
  payload bytes, on-disk bytes, recomputed IDs, field/order mutations, and every
  malformed directory entry.
- Map duplicate semantics to repository fixture tests spanning equal tuples
  across target specs/drafts/mappings at one bound root, root-move aggregate
  changes, changed-digest fresh imports, explicit identity collisions, and
  diagnostics naming the historical receipt and task refs.
- Map historical-not-live behavior to sentinel tests that rename, re-slug,
  transition, archive, restore, and remove original input paths while receipt
  validation continues to use only canonical receipt bytes.
- Map immutability to a writer registry test that snapshots receipt paths and
  digests around every existing v0.7 mutation surface and `repair --apply`, plus
  negative tests proving malformed receipts fail validation/import and are
  never silently rewritten.
- Map transactional composition to failure-injection tests over receipt, task,
  state, directory, fsync, rollback, and retained-recovery boundaries using the
  common publication protocol; command wiring remains owned by T-209.
- Record byte-level receipt inspection, duplicate refusal, changed-snapshot
  append-only behavior, and malformed-receipt recovery evidence in
  `planning/artifacts/manual-test/T-208/<timestamp>/report.md`; include receipt
  IDs, SHA-256 values, commands, and before/after tree manifests.

## Implementation Notes
