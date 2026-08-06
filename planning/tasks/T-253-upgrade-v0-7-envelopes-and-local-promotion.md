---
id: T-253-upgrade-v0-7-envelopes-and-local-promotion
title: Upgrade v0.7 envelopes and local promotion
status: todo
priority: high
spec_ref: specs/v0.7.0.md#atomic-publication-and-compatibility
dependencies:
    - T-203-define-planning-source-and-provenance-contracts
    - T-208-publish-strict-planning-provenance-sidecars
updated_at: "2026-08-06T14:16:49Z"
---

# T-253-upgrade-v0-7-envelopes-and-local-promotion Upgrade v0.7 envelopes and local promotion

## Description

Upgrade every inherited JSON producer and loop result file to envelope generation
3, then make local promotion preserve immutable provenance receipts without
widening any unrelated v0.6 behavior.

## Acceptance

- A1. Every JSON-capable inherited command emits schema version 3 with exact v0.6
  shapes and meanings except the documented provenance-aware promotion delta;
  schema versions 1 and 2 are never emitted by a v0.7 binary.
- A2. Strict registry and golden tests cover every inherited producer, warning,
  error, report-result exception, and loop result file so a partial version bump
  cannot ship.
- A3. `local promote` includes canonical receipt bytes and required provenance
  directories, reports write kind `provenance`, and refuses malformed or unknown
  durable local entries before publication.
- A4. Preview/apply use the shared durable transaction and preserve receipt bytes
  exactly; rollback, retained recovery, downgrade refusal, and committed/local
  path mapping cannot omit or rewrite provenance.

## Verification Notes

- A1-A2: generate the command/schema registry and mutate one inherited producer at
  a time to prove mixed envelope generations fail contract tests.
- A3-A4: use committed/local receipt sentinels and failure injection at directory,
  receipt, marker, and cleanup boundaries; compare exact bytes and promotion
  manifests before/after preview, apply, rollback, and recovery.

## Implementation Notes
