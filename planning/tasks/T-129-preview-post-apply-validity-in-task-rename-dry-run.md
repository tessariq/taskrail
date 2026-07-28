---
id: T-129-preview-post-apply-validity-in-task-rename-dry-run
title: Preview post-apply validity in task rename --dry-run
status: todo
priority: low
spec_ref: specs/v0.4.0.md#task-rename-and-re-slug
dependencies: []
updated_at: "2026-07-28T08:53:02Z"
---

# T-129-preview-post-apply-validity-in-task-rename-dry-run Preview post-apply validity in task rename --dry-run

## Description

`RenameTask` reports `s.Validate()` on a dry run (`internal/taskrail/rename.go:114`),
so `task rename --dry-run` answers with the validity of the state it is about to
replace, not the state it would produce. Its own comment states this: "Validation
reflects current state on a dry run and post-apply state otherwise."

That inverts the answer in exactly the case the preview is for. An operator
re-slugging a task to heal a `filename must be <id>.md` violation runs the dry run
to ask "would this fix it?" and is told `validation: invalid` — the violation the
rename would remove.

T-114 fixed the same defect in `task repoint` and left the shared machinery in
place: `validateInMemory(state, tasks)` and `layoutViolations()`
(`internal/taskrail/validation.go`) apply the full rule set to a caller-supplied
in-memory task set without writing. Rename needs a wider preview than repoint's
single-field `withSpecRef` — the id, the filename, and the inbound `dependencies:`
rewrites (`renameChanges`/`inboundDependents`) all move together — but the pattern
is the same. Discovered during T-114 review; behavior is pre-existing, not a
regression.

## Acceptance

- `task rename --dry-run` reports the validity the rename *would* produce: a dry run
  over a repo whose only violation the rename heals reports valid, and a dry run
  that would introduce a violation reports it.
- The preview covers the whole rename change set (frontmatter id, filename, inbound
  dependency refs, and the `current_task` pointer when it names the task), not just
  the target task's id.
- The dry run stays side-effect-free: no task file, no `STATE.md`, and no in-memory
  loaded task is mutated by previewing.
- Violations the rename does not touch still appear in the preview.
- The `rename.go` comment and `RenameTaskResult`'s doc comment are corrected — both
  currently document the old pre-apply semantics.
- Reuses `validateInMemory`/`layoutViolations` rather than re-implementing the rule
  set, so repoint and rename cannot drift apart.
- Automated coverage: a service-level test that fails against the current pre-apply
  behavior (a currently-invalid repo whose dry-run preview must report valid), plus
  the unrelated-violation-survives case.
- README's rename section documents the dry-run preview semantics, matching the
  wording already added for `task repoint`.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
