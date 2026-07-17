---
id: T-111-rename-not-retitle
title: Document that rename re-encodes id only, not the frontmatter title
status: todo
priority: medium
spec_ref: specs/v0.4.0.md#slug-in-id-invariant-documentation
dependencies: []
updated_at: "2026-07-17T11:20:55Z"
---

# T-111-rename-not-retitle Document that rename re-encodes id only, not the frontmatter title

## Description

`task rename --title "New Title"` derives a new slug but never rewrites the
`title:` frontmatter field — rename re-encodes the identifier only. This is
intentional (see the v0.4.0 Slug-In-Id Invariant Documentation amendment) but
easy to misread: an operator may expect the visible title to change too, and there
is no `task retitle` command. Document the distinction where operators meet the
slug-in-id model so the behavior is discoverable rather than surprising.

Documentation-only task; no CLI behavior change.

## Acceptance

- The slug-in-id / rename documentation (the T-097 doc surface: `README.md` and/or
  the packaged slug-in-id docs) states that `task rename` changes the id/slug and
  filename only and never rewrites the `title:` field, and that there is no
  `task retitle` in this version.
- The note explicitly calls out that `rename --title "New Title"` produces a new
  slug with an unchanged human-readable title, by design.
- If the packaged skills under `internal/taskrail/skills/` carry this guidance,
  regenerate the committed `.agents/`/`.claude/` copies (`task skills:regen`) so
  the parity check passes.
- No change to `slugify` or the `rename` command behavior; `validate` and the
  skill parity check pass.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
