---
id: T-389-require-layout-2-for-every-semantic-writer
title: Require layout 2 for every v0.5 semantic writer
status: completed
priority: high
spec_ref: specs/v0.5.0.md#layout-compatibility-and-upgrade
dependencies:
    - T-391-read-and-write-state-schema-2-at-layout-2
    - T-392-make-durable-transaction-observation-scale-past-a
updated_at: "2026-09-09T12:23:50Z"
completion_id: "321984797bdfb96c9114405c937f3375"
last_verification_id: "7fa89a74b48e0b9e9463dc56141cf79f"
last_verification_result: pass
last_verified_at: "2026-09-09T12:23:50Z"
last_verified_completion_id: "321984797bdfb96c9114405c937f3375"
---

# T-389-require-layout-2-for-every-semantic-writer Require layout 2 for every v0.5 semantic writer

## Description

`specs/v0.5.0.md#layout-compatibility-and-upgrade` states that `v0.5.0` raises
layout to `2`, that the final strict marker contains exactly `layout_version`,
`specs_dir`, `planning_dir`, `storage_mode`, and
`implementation_review_max_rounds`, and that "Every v0.5 semantic writer
(lifecycle/task/spec/import/review publisher and loop execution) requires layout
2 first; layout 1 permits read-only inspection and init migration only,
preventing older writers from later erasing v0.5 IDs, v2 references, or
loop-policy semantics."

Neither half ships. `currentLayoutVersion` is still `1`
(`internal/taskrail/paths.go:17`), so a fresh `taskrail init` on the v0.5 binary
writes a three-field layout-1 marker rather than the strict layout-2 marker, and
layout 2 exists only as a modeled upgrade target. Only three of the semantic
writers gate on it: `internal/taskrail/task_author.go:173`,
`internal/taskrail/import_apply.go:61`, and
`internal/taskrail/loop_preflight.go:543`. The lifecycle writers (`start`,
`complete`, `block`, `verify`, `unblock`, `release`), `task loop allow|hold`,
`task repoint`, `task rename`, `spec activate`, and `review publish` carry no
gate, and no writer ever emits the `incompatible_layout` code that
`specs/contracts/v0.5.0-machine-api.md` lists in `WriterErrors`.

The result is the exact silent erasure the requirement exists to prevent. In a
layout-1 sandbox the v0.5 binary accepted `task loop allow`, `start`,
`complete`, and `verify`, persisting `loop_policy`, `loop_reason`,
`completion_id`, `last_verification_id`, and `last_verified_completion_id`; a
binary built from the `v0.4.0` tag then ran `start` against the same repository
without refusal and rewrote the task frontmatter from its own typed struct,
dropping `loop_policy: allow` and `loop_reason` entirely.

Ship the layout-2 raise and the universal writer gate as one outcome. They are a
single atomic safety boundary: gating writers on layout 2 while fresh `init`
still writes layout 1 would make every writer refuse on a newly initialized
repository, and raising the layout without the gate leaves the erasure open.

## Acceptance

- The binary's current layout is `2`. Fresh `init` and retrofit write the strict
  final marker containing exactly `layout_version`, `specs_dir`, `planning_dir`,
  `storage_mode`, and `implementation_review_max_rounds`, with unknown fields and
  invalid values refused rather than dropped.
- Every v0.5 semantic writer — lifecycle (`start`, `complete`, `block`,
  `unblock`, `verify`, `task release`), task mutation (`task new`, `task author`,
  `task rename`, `task repoint`, `task dependency add|remove`,
  `task loop allow|hold`), `spec activate`, `import --apply`, `review publish`,
  and `loop` execution — refuses a layout-1 repository with the
  `incompatible_layout` machine code before writing any byte.
- Read-only commands (`validate`, `status`, `stats`, `coverage`, `spec
  list|show|diff`, `task show`, `prompt`, `local status|path`, `lock status`) and
  `init` migration continue to succeed at layout 1.
- A layout-1 repository carrying no v0.5 fields is unchanged by the refusals; the
  refusal names the `init --apply --confirm-quiescent` upgrade as its remedy.
- The existing layout-2 upgrade preview/apply, its migration fence, rollback, and
  note-handling behavior are unchanged by the raise.

## Verification Notes

- Assert the refusal per writer in package tests over a temporary layout-1
  repository: each writer returns `incompatible_layout` and the task/state bytes
  are byte-identical before and after.
- Assert fresh `init --json` emits `layout_version: 2` and exactly the five
  strict marker keys, and that the read-only command set still succeeds at
  layout 1.
- Regression oracle for the erasure: with the gate in place, the layout-1
  sequence that previously persisted `loop_policy`/`loop_reason` and the
  verification tuple must now refuse, so no v0.5 field reaches a repository an
  older binary will accept.
- Re-run `taskrail validate`, `task check:skills`, and the full suite; this
  repository's own `.taskrail/config.yml` is layout 1 and will need the
  documented upgrade.

## Implementation Notes

- Blocked before `start` on a prerequisite discovered while sizing: the layout-2
  raise requires this repository's own layout-1 marker to be upgraded, and the
  upgrade currently produces a repository the same binary reports as invalid
  (`validate` fails with `state schema_version must be 1`, and every lifecycle
  writer reports `validation.valid: false`). The ordinary state reader,
  validator, and writers never learned state schema 2; only the T-157 migration
  candidate did. Recorded as dependency T-391, which must land first.
- 2026-09-09T08:47:02Z: Implementation is complete and green (layout raised to 2, strict five-key marker on fresh init and retrofit, universal incompatible_layout writer gate, full suite passing), but it cannot be delivered: the upgrade every refusal names as its remedy does not complete on this repository. taskrail init --apply --confirm-quiescent --drop-continuation-notes published the migration fence and then spun past 24 minutes of CPU, and recover --apply spun the same way; a SIGQUIT stack shows durablefs.exactName reading the whole 390-entry tasks directory once per observed leaf. It reproduces with a binary built from HEAD, so it predates this change. The repository was restored through the Git escape the fenced-marker error names; no task, spec, or state byte was published. Recorded as dependency T-392, which must land first: committing this gate while the upgrade cannot finish would leave this repository writable by nothing.
- 2026-09-09T08:47:10Z: verification fail id b4611dd5cbc7cd2d4c48d9b82a8363eb previous none completion none
- 2026-09-09T11:52:14Z: Both recorded blockers are cleared: T-391 taught the ordinary readers, validator, and writers state schema 2, and T-392 made durable observation scale, so the upgrade this gate names as its remedy now completes. Returning to todo for re-verification of the shipped implementation against current bytes.
- 2026-09-09T12:23:41Z: Re-verified the shipped layout-2 raise and writer gate against current bytes, now that the upgrade its refusals name completes. Three acceptance gaps found and closed: import --apply given a legacy ImportDraft v1 was ungated and created tasks and rewrote state on a layout-1 repository; retrofit published a zero-byte marker plus schema-1 state, so its own init then failed with parse layout marker EOF; and retrofit did not refresh the layout it had just published, so the same process refused its own next write. Layout-1 refusal and read-only coverage extended to the full acceptance list.
- 2026-09-09T12:23:50Z: verification pass id 7fa89a74b48e0b9e9463dc56141cf79f previous none completion 321984797bdfb96c9114405c937f3375
