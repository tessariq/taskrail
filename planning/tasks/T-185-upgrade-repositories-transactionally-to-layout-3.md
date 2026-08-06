---
id: T-185-upgrade-repositories-transactionally-to-layout-3
title: Upgrade repositories transactionally to layout 3
status: todo
priority: high
spec_ref: specs/v0.6.0.md#layout-3-migration-and-compatibility
dependencies:
    - T-175-implement-arbitrary-width-generated-task-keys
    - T-176-classify-persisted-and-legacy-task-identities
    - T-177-validate-opaque-ids-and-importdraft-v3-structure
    - T-183-validate-cancellation-generation-and-archive
    - T-178-load-live-and-archived-tasks-as-one-immutable
    - T-179-resolve-stable-task-references-across-every
    - T-181-detect-durable-physical-task-path-references
    - T-184-recover-retained-semantic-transactions-explicitly
    - T-182-define-exact-v0-6-machine-result-schemas
    - T-157-upgrade-repositories-transactionally-to-layout-2
    - T-219-prepare-inherited-writers-for-layout-3
    - T-226-bootstrap-a-configurable-human-owned-ideas-sidecar
updated_at: "2026-08-04T23:06:23Z"
---

# T-185-upgrade-repositories-transactionally-to-layout-3 Upgrade repositories transactionally to layout 3

## Description

Implement map-free marker-plus-no-clobber-IDEAS 2-to-3 and atomic 1-to-2-to-3
migration, debt validation, archive adoption, and deterministic root discovery.

## Acceptance

- Apply owns the transaction before original-marker/repository resolution and
  holds it through candidate reads, publication, validation, rollback, or
  recovery.
- Direct 2-to-3 writes explicit `ideas_path`, creates its template only when
  absent, and preserves existing ideas/task bytes, including `loop_policy` and
  `loop_reason`. Every migration path rejects stale
  `AUTONOMY.tsv` rather than migrating it. Multi-hop skills require consent,
  stage current bytes, and roll back marker/skills transactionally.
- Multi-hop 1-to-2-to-3 removes schema-1 `continuation_notes` and the rendered
  `## Notes` section at the schema-2 hop. Preview exposes existing notes; apply
  requires explicit no-clobber extraction or `--drop-continuation-notes` when any
  are non-empty and reports either option as unnecessary otherwise. The flag remains supported while schema-1
  multi-hop migration is supported; state schema/body, skills, and marker
  publish or roll back together. Direct 2-to-3 rejects the inapplicable flag.
- Multi-hop preview also offers explicit no-clobber extraction into an absent
  human NOTES sidecar; selected extraction publishes or rolls back with state,
  skills, and the final marker, while an existing sidecar stops for manual merge.
- Direct and multi-hop preview report the configured IDEAS candidate. Apply
  creates only an absent safe destination; existing regular content is
  byte-identical, unsafe destinations refuse, and marker/ideas/state/skills
  publish or roll back on one transaction boundary.
- Layout-3 local promotion carries live and archived tasks, IDEAS, specs, state,
  notes, prompts, and durable reviews while preserving storage class and refusing
  unknown durable local entries.
- Migration cannot publish layout 3 until inherited semantic writers already use
  combined-ledger resolution/allocation, live-only write sets, and archived-target
  refusal behind the layout guard.
- Preview reports identity/portability/lifecycle/archive debt with exact
  warnings, zero-exit valid-with-warnings, non-null JSON, and zero writes.
- Existing archive follows absent/empty/adoptable/debt/blocker matrix and
  map-free apply changes no task bytes.
- Marker rollback rechecks all semantics; root discovery/lock choice, direct
  skill refresh, pre-write-only downgrade, and no hand-lowering are exact.

## Verification Notes

- Map criteria to lock-before-read, layouts/skills/old binary,
  archive/debt/warnings, rollback races, discovery/mismatch/cwd locks,
  downgrade/recovery, and previews.
- Persist clean, debt-heavy, adopted-archive, non-Git, and multi-hop reports.
- Include multi-hop state with absent, empty, and non-empty continuation notes,
  unnecessary/refused/accepted acknowledgement, extraction/drop and existing
  NOTES refusal, schema/body removal, and
  rollback after state publication; prove no loop-policy sidecar migration.
- Exercise default/custom ideas paths in committed/local mode, absent/existing/
  unsafe destinations, direct and multi-hop rollback, and unchanged IDEAS bytes.

## Implementation Notes
