---
id: T-130-normalize-spec-ref-path-on-write
title: Normalize spec_ref path on write
status: completed
priority: low
spec_ref: specs/v0.4.0.md#task-spec-ref-re-pointing
dependencies: []
updated_at: "2026-07-29T09:11:14Z"
---

# T-130-normalize-spec-ref-path-on-write Normalize spec_ref path on write

## Description

`spec_ref` is accepted and stored as a raw string. Its path half is never
normalized on write, so several spellings of one reference all land verbatim in
task frontmatter. Reproduced 2026-07-28 against the working-tree binary in a
sandbox repo:

```
taskrail task new --title "Doubled" --spec-ref "specs//v0.1.0.md#goals"   # accepted
taskrail task new --title "Dotted"  --spec-ref "./specs/v0.1.0.md#goals"  # accepted

spec_ref: specs//v0.1.0.md#goals
spec_ref: ./specs/v0.1.0.md#goals

taskrail validate  ->  state valid
```

Downstream consumers are unaffected: matching is path-cleaned, so `status`
reported `2 open on active spec, 0 open away`, `next` selected normally, and
coverage reported no orphans. This is a consistency defect in what gets written,
not broken selection or reporting.

It does have one concrete behavioral consequence. `task repoint`'s no-op guard
(`internal/taskrail/repoint.go`) compares `spec_ref` strings literally, so a
re-point between two spellings of the same reference is not recognized as a no-op:

```
taskrail task repoint T-001-doubled --spec-ref "specs/v0.1.0.md#goals"
repointed T-001-doubled: "specs//v0.1.0.md#goals" -> "specs/v0.1.0.md#goals"
```

That rewrites the task file and `STATE.md` and bumps `updated_at` for a change that
means nothing — exactly what the guard exists to prevent. Normalizing on write
fixes the guard as a side effect, since the stored form becomes canonical.

The write paths are `CreateTask` (`internal/taskrail/transitions.go`, both the
explicit `--spec-ref` and the `--area`-resolved forms), `RepointTask`
(`internal/taskrail/repoint.go`), and the import apply path
(`internal/taskrail/import_apply.go`). Discovered while reviewing T-114.

## Acceptance

- The path half of a `spec_ref` is normalized before it is written to task
  frontmatter, so `specs//v0.1.0.md#goals` and `./specs/v0.1.0.md#goals` both land
  as `specs/v0.1.0.md#goals`. The anchor half is untouched.
- Normalization is applied by every writer — `task new` (explicit `--spec-ref` and
  `--area`-resolved), `task repoint`, and `import --apply` — through one shared
  helper, so a future writer cannot skip it.
- `task repoint`'s no-op guard consequently rejects a re-point between two
  spellings of the same reference instead of rewriting the task file and
  `STATE.md`.
- Normalization never widens what is accepted: a reference that `validate` rejects
  today is still rejected, and the existing traversal guard in `parseSpecRef`
  (`internal/taskrail/validation.go`) still runs against the normalized value.
- Existing task files with an un-normalized `spec_ref` keep validating; this is a
  write-path fix, not a migration. Do not rewrite stored refs in bulk — `taskrail
  repair` stays limited to re-projecting `STATE.md`.
- Automated coverage: service-level tests for each writer asserting the persisted
  form is canonical, plus the repoint no-op-guard case, plus a test that a
  traversal-shaped ref is still rejected after normalization.

## Verification Notes

- Sandbox repro evidence for the current behavior is quoted in the Description;
  re-run it against the fixed binary and record the artifact path here.
- TODO: record verification evidence paths.

## Implementation Notes

- 2026-07-29T09:11:09Z: verification pass
- 2026-07-29T09:11:14Z: Shared normalizeSpecRef canonicalizes the spec_ref path half on write; validateSpecRef/validateSpecRefWithPending/validateTaskCreatable now return the canonical form so no writer can skip it. Follow-up T-135 filed for the duplicate-heading corpus guard.
