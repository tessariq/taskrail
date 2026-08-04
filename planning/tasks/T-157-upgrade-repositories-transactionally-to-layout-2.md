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
layout marker on one validated publication boundary.

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
- Older binaries refuse layout 2, and downgrade guidance is Git reversion rather
  than marker editing.

## Verification Notes

- Map the five criteria to preview output, migration command output, exact file
  snapshots, old/new binary observations, and sandbox reports.
- Exercise candidate failure, lock contention, old-writer mutation,
  interruption, rollback, old-binary refusal, skill parity, and
  policy-equivalence boundaries.

## Implementation Notes
