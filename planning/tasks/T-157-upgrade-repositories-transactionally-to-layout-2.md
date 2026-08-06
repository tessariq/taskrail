---
id: T-157-upgrade-repositories-transactionally-to-layout-2
title: Upgrade repositories transactionally to layout 2
status: todo
priority: high
spec_ref: specs/v0.5.0.md#layout-compatibility-and-upgrade
dependencies:
    - T-168-parse-and-validate-an-optional-autonomous-run
    - T-201-make-packaged-skills-agent-skills-compliant
    - T-214-bootstrap-and-migrate-human-owned-repository-notes
    - T-232-recover-v0-5-transactions-through-one-command
updated_at: "2026-08-04T21:32:13Z"
---

# T-157-upgrade-repositories-transactionally-to-layout-2 Upgrade repositories transactionally to layout 2

## Description

Introduce layout 2 on the shared writer-lock foundation. Make upgrade preview
read-only, make apply transactional, and keep installed packaged skills and the
layout marker on one validated publication boundary. Move `STATE.md` to schema 2
on that same boundary, removing the legacy continuation-note surface without
silently discarding authored text. Add strict committed/local storage and bounded
implementation-review configuration to the layout contract.

## Acceptance

- Layout-1 repositories permit inspection and migration only; layout 2 becomes
  the enforceable prerequisite API used by each downstream v0.5 writer.
- Layout 2 strictly records `storage_mode: committed|local` and explicit
  `implementation_review_max_iterations` in `1..5`; migration defaults existing
  repositories to committed mode and review maximum 2. Unknown/invalid fields
  never disappear through a later marker rewrite; T-222 owns fresh local
  initialization and discovery against this completed marker contract.
- Preview validates a complete candidate without writes, while apply requires
  explicit `--confirm-quiescent`, snapshots semantic files, durably records
  recovery, publishes the exact fenced config before semantic bytes, and removes
  the fence only by publishing the final strict marker after post-validation.
- Failure and interruption restore only unchanged originals, never overwrite
  concurrent bytes, and provide an exact recovery path; an already-running old
  writer makes migration unsafe and is covered explicitly.
- Marker-free destinations byte-identical to the embedded package are preserved as
  parity mirrors. Installed copies require combined forced refresh, normalize
  compatible legacy markers to nested metadata, and divergent/conflicting copies
  refuse before marker publication.
- Fresh layout-2 state uses schema 2 without `continuation_notes` or a rendered
  `## Notes` section. Upgrade preview explicitly decodes schema 1, reports every
  legacy note and machine-readable preservation choices, and apply requires
  either explicit no-clobber extraction or `--drop-continuation-notes` exactly
  when the legacy list is non-empty.
- State schema 2 accepts only the inherited current-snapshot fields plus optional
  verification ID, predecessor ID, and completion binding, with exact omission and
  canonical summary rules; fresh and migrated legacy state never invent identity.
- State-schema migration, body re-render, marker publication, and installed-skill
  refresh share the transaction and rollback boundary. Schema-2 decoding rejects
  a reintroduced `continuation_notes` key instead of silently dropping it.
- Preview offers explicit no-clobber extraction of non-empty legacy notes into an
  absent human-owned NOTES sidecar; extraction, state/schema publication, skills,
  and marker share one rollback boundary. Existing notes stop for manual merge.
- The migration-only acknowledgement remains available for every supported
  direct or multi-hop schema-1 upgrade and is retired only with schema-1
  migration support; it is reported as unnecessary when no note needs removal.
- Non-empty legacy notes require exactly one of
  `--extract-continuation-notes` or `--drop-continuation-notes`; migration-only
  note flags and quiescence confirmation reject inapplicable
  fresh/current/direct-schema-2 flows.
- Init JSON represents NOTES file action separately from continuation-note
  extract/drop choices so preview can expose both alternatives and apply records
  one without changing the candidate path set.
- Layout 2 recognizes the optional paired `loop_policy` and `loop_reason` task
  fields under strict decoding, protects them through every task re-render, and
  upgrades tasks with neither field as implicit holds without granting unattended
  authorization.
- Preview and apply inspect the exact configured `<planning-dir>/AUTONOMY.tsv`
  path without following symlink or reparse traversal and refuse any legacy entry
  there with manual translation/removal guidance; similarly named files elsewhere
  are unrelated and no TSV contents are parsed or migrated.
- Older binaries refuse layout 2, and downgrade guidance is Git reversion rather
  than marker editing.
- Migration preview/apply report physical and logical roots, committed storage
  mode, and configured review maximum without weakening read-only preview.

## Verification Notes

- Map every acceptance criterion to preview output, migration command output,
  exact file snapshots, old/new binary observations, and sandbox reports.
- Exercise candidate failure, lock contention, old-writer mutation,
  interruption, rollback, old-binary refusal, skill parity, and
  task-local-policy preservation boundaries.
- Exercise absent, empty, single, multiple, multiline, and YAML-quoted legacy
  notes; acknowledgement refusal; explicit extraction/drop, existing NOTES
  refusal; strict schema-2 rejection;
  fresh-init output; rollback; and direct/multi-hop compatibility.
- Exercise absent and paired task-local loop fields, preservation across body and
  lifecycle re-renders, no-authority migration, nested skill metadata, and exact
  legacy-path refusal including symlink/reparse and same-basename non-matches.

## Implementation Notes
