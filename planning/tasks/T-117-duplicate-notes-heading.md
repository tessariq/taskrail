---
id: T-117-duplicate-notes-heading
title: Stop appending a duplicate Implementation Notes heading
status: completed
priority: medium
spec_ref: specs/v0.2.0.md#no-local-paths-in-task-notes
dependencies: []
updated_at: "2026-07-27T12:57:40Z"
---

# T-117-duplicate-notes-heading Stop appending a duplicate Implementation Notes heading

## Description

`appendTaskNote` (`internal/taskrail/tasks.go`) probes for the literal
`"## Implementation Notes\n\n"` — heading *plus* a blank line — before appending a
note, and scaffolds the section when the probe misses. But `renderNewTaskBody`
(`internal/taskrail/templates.go`) ends the body at `## Implementation Notes\n`
with no blank line after it, so the probe never matches a fresh scaffold and the
first `verify`/`block` note appends a **second** copy of the heading. Every note
after that appends to the end, so the file keeps one stray duplicate heading — 33
committed task files already carry two (`grep -c '^## Implementation Notes$'
planning/tasks/*.md`).

The v0.2.0 No Local Paths In Task Notes amendment makes this writer responsible for
the committed shape of an appended note. A heading it duplicates on the very
scaffold it ships is the same class of defect: the note's committed form must be
correct without hand-repair.

## Acceptance

- Appending a note to a body whose `## Implementation Notes` heading is the last
  line adds the note under that heading and does **not** emit a second heading.
- The section keeps its blank line between heading and first note, matching the
  existing corpus style.
- A body with the heading followed by existing notes still appends at the end,
  unchanged from today.
- A body with no `## Implementation Notes` section at all still gets the section
  scaffolded once (the follow-up scaffold `renderFollowupTaskBody` omits it by
  design, so this path stays live).
- The heading match is exact-line, so a heading like
  `## Implementation Notes For Reviewers` does not count as the section.
- Covered by unit tests over `appendTaskNote` for each body shape, plus an
  end-to-end assertion that a scaffolded task verified once contains exactly one
  `## Implementation Notes` heading.
- Existing task files that already carry a duplicate heading are left as-is; this
  task fixes the writer, not the corpus.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- 2026-07-27T12:57:40Z: verification pass
