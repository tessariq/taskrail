---
id: T-113-spec-diff
title: 'spec diff: mechanical anchor-set delta between two spec versions'
status: todo
priority: medium
spec_ref: specs/v0.4.0.md#spec-version-diff
dependencies: []
updated_at: "2026-07-17T12:04:24Z"
---

# T-113-spec-diff spec diff: mechanical anchor-set delta between two spec versions

## Description

The `spec` command family (`activate`, `add`, `list`, `show`) cannot answer "what
changed between two spec versions?" `coverage`/`status` report per-task drift against
one active spec, but a `spec activate` bump gives no visible worklist of new areas to
cover or old areas that now orphan tasks. Add `taskrail spec diff <v1> <v2>` per the
v0.4.0 Spec Version Diff amendment: a read-only, mechanical anchor-set delta reusing
the existing heading-anchor slug logic (`collectHeadingAnchors` / the same anchors
`spec show --anchors`, `coverage`, and `validate` compute).

Read-only and side-effect-free like `coverage`/`validate` — it never writes
`planning/STATE.md` or task files and never gates or makes `validate` fail. This is a
reporting aid, not a migrator: it never creates tasks, re-points `spec_ref`, or
advances status.

## Acceptance

- `taskrail spec diff <v1> <v2>` prints the coverable-area anchors **added** in
  `<v2>` and **removed** relative to `<v1>`, computed from the same anchor logic as
  `spec show --anchors`. Added areas are labeled as needing decomposition; removed
  areas are labeled as orphaning existing tasks.
- Renamed areas are reported best-effort only (e.g. an added and a removed anchor
  sharing a normalized stem) and clearly labeled as a candidate rename, never
  asserted as fact; no semantic guessing.
- The command is read-only: running it leaves the working tree clean (no
  `planning/STATE.md` or task-file writes), mirroring `coverage`/`validate`.
- An unknown version argument fails before doing any work, resolved the same way as
  the rest of the `spec` family.
- `--json` mirrors the human output with structured `added` / `removed` / `renamed`
  anchor lists.
- Automated coverage: service-level diff tests (added-only, removed-only, mixed,
  identical specs, candidate rename) and a CLI smoke test for human and `--json`
  output.
- README, workflow docs, and the Unreleased changelog document `spec diff` as a
  read-only anchor-set delta.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- Backlog origin: the "Deferred to v0.4.0" `spec diff` item, adopted into the spec
  during the v0.4.0 task review.
- Pairs with a future `spec migrate` (guided act-on-the-delta flow) which stays out
  of scope here.
