---
id: T-278-publish-typed-directories-without-clobbering
title: Publish typed directories without clobbering
status: completed
priority: high
spec_ref: specs/v0.5.0.md#repository-discovery-locking-and-recovery
dependencies:
    - T-156-protect-existing-semantic-writers-with-snapshot
updated_at: "2026-08-15T11:40:18Z"
---

# T-278-publish-typed-directories-without-clobbering Publish typed directories without clobbering

## Description

Publish validated task, spec, and decomposition review bundles through one typed,
absent-directory commit point so exact reviewed bytes become visible together and
an existing or substituted destination is never clobbered.

## Acceptance

- A1. A complete validated typed bundle publishes its fixed file set and exact
  bytes through one absent destination-directory commit point.
- A2. Existing destinations, aliases, symlink/reparse traversal, non-regular
  entries, cross-type paths, and destination substitution refuse without exposing
  a partial final directory or changing existing bytes.
- A3. Handled staging or publication failure leaves no final directory, while a
  successful commit reports deterministic typed file destinations and digests.
- A4. Concurrent publishers for the same destination produce at most one complete
  winner and one no-clobber refusal.

## Verification Notes

- A1: publish each supported directory bundle type and compare exact proposal and
  final bytes, membership, and digests.
- A2: exercise every blocked-path and destination-conflict class and inspect the
  untouched destination/final root.
- A3: inject validation, staging, and commit failures and observe no partial final
  directory; compare a successful receipt with published bytes.
- A4: race two valid candidates for one destination and observe one complete
  publication without overwrite or merge.

## Implementation Notes

- 2026-08-15T11:40:03Z: Added context-aware typed review-directory publication with fixed inventories, lock-derived storage routing, command/write capability bounds, exact-byte receipts, no-follow staged validation, native no-replace commits, durable rollback, and race/fault coverage.
- 2026-08-15T11:40:18Z: verification pass
