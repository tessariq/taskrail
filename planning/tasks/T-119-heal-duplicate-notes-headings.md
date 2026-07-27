---
id: T-119-heal-duplicate-notes-headings
title: Heal duplicate Implementation Notes headings in the task corpus
status: completed
priority: low
spec_ref: specs/v0.2.0.md#no-local-paths-in-task-notes
dependencies:
    - T-117-duplicate-notes-heading
updated_at: "2026-07-27T13:37:20Z"
---

# T-119-heal-duplicate-notes-headings Heal duplicate Implementation Notes headings in the task corpus

## Description

T-117 fixed the writer that stamped a second `## Implementation Notes` heading into
task files, but deliberately left the damage it had already done. 32 committed task
files still carry the duplicate. Every one has the identical shape — the two
headings separated by a blank line and nothing else — so the heal is mechanical
rather than a judgement call about content.

This is the corpus half of T-117: with the writer fixed, the stray headings can be
removed once and then held at zero by a test, so the invariant is enforced rather
than periodically re-discovered.

## Acceptance

- No file under `planning/tasks/` contains more than one `## Implementation Notes`
  heading.
- A test asserts that invariant over the real corpus, so a regression in the writer
  or a hand-authored duplicate fails the build instead of accumulating silently. It
  must fail against the corpus as it stands before the heal.
- The heal removes only the redundant heading and the blank line that follows it:
  no note content, ordering, frontmatter, or status field changes. `git diff` shows
  exactly two deleted lines per affected file.
- Every affected file keeps its remaining `## Implementation Notes` section and the
  notes under it.
- `taskrail validate` passes and `planning/STATE.md` is unchanged by the heal (task
  bodies are not part of the projection).

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- 2026-07-27T13:37:20Z: verification pass
