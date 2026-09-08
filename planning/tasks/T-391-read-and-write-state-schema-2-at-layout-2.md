---
id: T-391-read-and-write-state-schema-2-at-layout-2
title: Read and write state schema 2 at layout 2
status: todo
priority: high
spec_ref: specs/v0.5.0.md#layout-compatibility-and-upgrade
dependencies: []
updated_at: "2026-09-08T19:34:05Z"
---

# T-391-read-and-write-state-schema-2-at-layout-2 Read and write state schema 2 at layout 2

## Description

`specs/v0.5.0.md#layout-compatibility-and-upgrade` states that "Layout 2 raises
`STATE.md` to state schema 2", that schema 2 "changes `schema_version` to
integer `2`", and that "Fresh layout-2 initialization writes schema 2 directly".
T-157 shipped only the migration-side half: `decodeStateStrict`/`stateV2Frontmatter`
in `internal/taskrail/layout2_candidate.go` construct and validate a schema-2
candidate, and `init --apply --confirm-quiescent` publishes it.

The ordinary runtime never learned schema 2. `stateSchemaVersion` is still `1`
(`internal/taskrail/types.go:5`), `validateState` hard-requires it
(`internal/taskrail/validation.go:79`), and both state renderers stamp it
(`internal/taskrail/templates.go:11`, `internal/taskrail/transitions.go:372`).

The result is that the layout-2 upgrade produces a repository its own binary
reports as invalid. Observed end to end in a temporary sandbox with the
working-tree binary: `init` (fresh, layout 1), then
`init --apply --confirm-quiescent` publishes `layout_version: 2` and
`schema_version: 2`, and `validate` then exits non-zero with the single
violation `state schema_version must be 1`. Lifecycle writers still run and
preserve the on-disk `schema_version: 2`, but every one of them reports
`validation.valid: false` with that same violation, so every write on an
upgraded repository is published as failing validation and any
validation-gated caller (CI, hooks, the release gate) stays red permanently.

This is a prerequisite for T-389, not a parallel concern. T-389 raises the
binary's current layout to 2 and refuses layout-1 writers, which requires this
repository's own layout-1 marker to be upgraded; performing that upgrade today
makes `taskrail validate` fail here, so T-389 cannot reach a verified result
until the ordinary reader, validator, and state writers accept schema 2.

## Acceptance

- State decoding accepts schema 2 and rejects the removed `continuation_notes`
  frontmatter field and rendered `## Notes` section, while schema 1 continues to
  decode unchanged at layout 1.
- `validateState` requires the schema version the repository's layout implies —
  schema 2 at layout 2, schema 1 at layout 1 — instead of a single constant, and
  accepts the optional verification tuple keys schema 2 adds.
- Every state writer (`start`, `complete`, `block`, `unblock`, `verify`, `next`,
  `task release`, task mutation, `spec activate`, `repair`) re-renders a layout-2
  repository at schema 2 and a layout-1 repository at schema 1, never rewriting
  one as the other.
- Fresh layout-2 initialization writes schema 2 directly and seeds no
  continuation prose; fresh layout-1 initialization is unchanged.
- A repository upgraded by `init --apply --confirm-quiescent` passes `validate`
  and every lifecycle writer reports `validation.valid: true`.

## Verification Notes

- Regression oracle for the defect above: in a temporary repository, run fresh
  `init`, then `init --apply --confirm-quiescent`, then `validate`; assert a
  zero exit and no violations, where today it reports
  `state schema_version must be 1`.
- Assert a layout-2 lifecycle sequence (`task new`, `start`, `complete`,
  `verify`) reports `validation.valid: true` and leaves `schema_version: 2` on
  disk.
- Assert a layout-1 repository still renders and validates at schema 1, so the
  raise is layout-conditioned rather than global.
- Re-run `go test ./...`, `taskrail validate`, and `task check:skills`.

## Implementation Notes
