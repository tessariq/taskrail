---
id: T-135-duplicate-heading-guard-all-sections
title: Extend the duplicate-heading corpus guard beyond Implementation Notes
status: todo
priority: low
spec_ref: specs/v0.2.0.md#no-local-paths-in-task-notes
dependencies: []
updated_at: "2026-07-29T09:10:47Z"
---

# T-135-duplicate-heading-guard-all-sections Extend the duplicate-heading corpus guard beyond Implementation Notes

## Description

`TestCorpusTasksHaveOneImplementationNotesSection`
(`internal/taskrail/corpus_task_body_test.go`), wired into CI's planning fast lane
as `task check:task-bodies`, counts duplicates of exactly one heading:
`## Implementation Notes`. Every other section of a task body is unguarded.

Found while reviewing T-130: the T-130 task file itself shipped in `main` with two
`## Verification Notes` headings (removed by hand in T-130's commit). The corpus
check passed the whole time, because that heading is outside its scope.

The `## Implementation Notes` case is the one with a *writer* behind it (`verify`
and `block` append there, and T-117 fixed the writer that duplicated it), so it
deserves its dedicated guard. But the corpus test's own comment names the real
reason it exists — "a task body is also hand-editable, so the only thing that keeps
the corpus clean is checking it" — and that reason applies to every section a task
body has.

## Acceptance

- The corpus body check flags a task file carrying the same `##` section heading
  twice, not just `## Implementation Notes`. Scope to the section headings the task
  scaffold defines (`renderNewTaskBody`) rather than every possible heading, so
  legitimately repeated deeper prose headings are not flagged.
- The failure message names the file and the duplicated heading, as the current one
  does.
- The whole-line match semantics are preserved: a prose mention of a heading's
  words never counts, only the heading line itself.
- `task check:task-bodies` keeps covering it, so a planning-only change still hits
  the guard in the fast lane.
- The current `planning/tasks/` corpus passes the widened check with no further
  hand-healing — if it does not, heal the offenders in the same change and say so.
- Automated coverage: a fixture-level test for the widened rule (a fixture can
  regress where the real corpus cannot), alongside the existing corpus assertion.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
