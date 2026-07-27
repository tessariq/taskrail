---
id: T-116-rename-body-heading
title: Rewrite the task body H1 id on task rename
status: completed
priority: medium
spec_ref: specs/v0.4.0.md#task-rename-and-re-slug
dependencies: []
updated_at: "2026-07-27T12:57:40Z"
---

# T-116-rename-body-heading Rewrite the task body H1 id on task rename

## Description

`task rename` re-encodes a task's identifier everywhere it is machine-readable —
the `id:` frontmatter, the filename, and every inbound `dependencies:` reference —
but never touches the body's `# <id> <title>` H1 (`renameWrites` in
`internal/taskrail/rename.go` mutates `Frontmatter.ID` and `Filename` only). After
any rename the heading still names the pre-rename id, so a reader opening the file
sees one id in the frontmatter and a different one on the first line. The drift has
been present since `task rename` shipped (T-096) and every rename widens it.

`validate` does not check the heading, so nothing catches this. Per the v0.4.0 Task
Rename And Re-Slug amendment the rename performs its coupled edits as one outcome
and `--dry-run` "reports exactly what would change", so the heading rewrite belongs
in the change set rather than happening invisibly.

## Acceptance

- Renaming a task rewrites its body H1 from `# <old-id> <title>` to
  `# <new-id> <title>`, leaving the title text untouched (rename re-encodes the
  identifier only — it never retitles).
- The rewrite is conservative: only a leading H1 whose first token is exactly the
  old id is rewritten. A body with no such heading, or one naming a different id,
  is left alone and the rename still succeeds.
- The heading edit appears in the reported change set as its own `RenameChange`
  (so `--dry-run` and `--json` disclose it), emitted only when a matching heading
  is actually present.
- `--dry-run` still writes nothing, including the heading.
- A failed rename rolls the heading back with the rest of the task file: the
  existing `renameUndo` snapshot must still restore the original bytes.
- Covered by a rename test asserting the new id in the H1 and the old id absent,
  plus a case for a body without a matching heading.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- 2026-07-27T12:57:40Z: verification pass
