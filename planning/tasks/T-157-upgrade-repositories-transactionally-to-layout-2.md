---
id: T-157-upgrade-repositories-transactionally-to-layout-2
title: Upgrade repositories transactionally to layout 2
status: todo
priority: high
spec_ref: specs/v0.5.0.md#layout-compatibility-and-upgrade
dependencies:
    - T-156-protect-existing-semantic-writers-with-snapshot
updated_at: "2026-08-04T21:32:13Z"
---

# T-157-upgrade-repositories-transactionally-to-layout-2 Upgrade repositories transactionally to layout 2

## Description

Introduce layout 2 on the shared writer-lock foundation. Make upgrade preview
read-only, make apply transactional, and keep installed packaged skills and the
layout marker on one validated publication boundary. Move `STATE.md` to schema 2
on that same boundary, removing the legacy continuation-note surface without
silently discarding authored text.

## Acceptance

- Layout-1 repositories permit inspection and migration only; layout 2 becomes
  the enforceable prerequisite API used by each downstream v0.5 writer.
- Preview validates a complete candidate without writes, while apply requires
  operator-confirmed old-process quiescence, snapshots semantic files, and
  publishes the early migration fence, skills when present, and final marker
  atomically.
- Failure and interruption restore only unchanged originals, never overwrite
  concurrent bytes, and provide an exact recovery path; an already-running old
  writer makes migration unsafe and is covered explicitly.
- Installed packaged skills require the combined forced refresh; repositories
  without installed skills do not create them.
- Fresh layout-2 state uses schema 2 without `continuation_notes` or a rendered
  `## Notes` section. Upgrade preview explicitly decodes schema 1, reports every
  legacy note and a machine-readable drop-acknowledgement requirement, and apply
  requires `--drop-continuation-notes` exactly when the legacy list is non-empty.
- State-schema migration, body re-render, marker publication, and installed-skill
  refresh share the transaction and rollback boundary. Schema-2 decoding rejects
  a reintroduced `continuation_notes` key instead of silently dropping it.
- The migration-only acknowledgement remains available for every supported
  direct or multi-hop schema-1 upgrade and is retired only with schema-1
  migration support; it is reported as unnecessary when no note needs removal.
- Older binaries refuse layout 2, and downgrade guidance is Git reversion rather
  than marker editing.

## Verification Notes

- Map the five criteria to preview output, migration command output, exact file
  snapshots, old/new binary observations, and sandbox reports.
- Exercise candidate failure, lock contention, old-writer mutation,
  interruption, rollback, old-binary refusal, skill parity, and
  policy-equivalence boundaries.
- Exercise absent, empty, single, multiple, multiline, and YAML-quoted legacy
  notes; acknowledgement refusal; explicit drop; strict schema-2 rejection;
  fresh-init output; rollback; and direct/multi-hop compatibility.

## Implementation Notes
