---
id: T-226-bootstrap-a-configurable-human-owned-ideas-sidecar
title: Bootstrap a configurable human-owned IDEAS sidecar
status: todo
priority: high
spec_ref: specs/v0.6.0.md#human-owned-ideas-inbox
dependencies:
    - T-180-make-semantic-publication-durably-transactional
    - T-219-prepare-inherited-writers-for-layout-3
updated_at: "2026-08-05T22:04:40Z"
---

# T-226-bootstrap-a-configurable-human-owned-ideas-sidecar Bootstrap a configurable human-owned IDEAS sidecar

## Description

Add layout-3 `ideas_path` plus reusable no-clobber candidate/publication helpers,
and safely create its human-owned free-form Markdown sidecar during fresh init and
retrofit without interpreting existing content or turning ideas into task state.

## Acceptance

- Layout 3 writes an explicit canonical `ideas_path`, defaulting to
  `<planning-dir>/IDEAS.md`; committed mode permits another contained path and
  local mode requires its ignored storage root.
- Preview reports the no-clobber candidate with zero writes. Apply creates a short
  commented template only when absent and preserves every existing regular byte.
- Alias, symlink/reparse, special/non-regular, escaping, portability-collision,
  and concurrent destination changes refuse or roll back with exact diagnostics.
- IDEAS is optional after bootstrap, free-form, and absent from state, task counts,
  dependencies, coverage, gaps, validation, selection, and loop policy.
- Provide the validated no-clobber candidate/publication primitive consumed by
  T-185; this task owns fresh layout-3 init/retrofit publication of marker plus
  IDEAS, but not direct or multi-hop migration from an older layout.

## Verification Notes

- Cover committed/local default/custom paths, absent/existing/unsafe destinations,
  preview purity, fresh layout-3 init/retrofit all-or-none publication, races,
  rollback, and exact helper output consumed by migration.
- Compare state/task/coverage output before and after arbitrary IDEAS prose to
  prove zero lifecycle interpretation.

## Implementation Notes
