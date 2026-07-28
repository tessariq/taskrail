---
id: T-126-rename-slug-length-cap
title: Cap title-derived slug on task rename
status: completed
priority: low
spec_ref: specs/v0.4.0.md#task-rename-and-re-slug
dependencies:
    - T-112-slug-length-cap
updated_at: "2026-07-28T13:58:53Z"
---

# T-126-rename-slug-length-cap Cap title-derived slug on task rename

## Description

T-112 capped the *title-derived* slug on `task new` but left `task rename --title`
uncapped: `renameSlug` (`internal/taskrail/rename.go`) calls `slugify(source)` with
no `capSlug`, so a long `--title` on rename still yields an unbounded slug, id, and
filename — the same length problem T-112 closed for creation. The two paths share
`slugify`, so a long title produces inconsistent lengths depending on which command
wrote it.

The v0.4.0 cap requirement is written only under `#slugged-task-creation`, not
`#task-rename-and-re-slug`, so leaving rename uncapped is spec-compliant as
literally scoped. This is a discretionary consistency follow-up, not a spec
violation — discovered during T-112 review (go-reviewer, medium finding).

Reuse T-112's `capSlug`/`slugMaxLen`; apply it only on the `--title` path, never on
an explicit `--slug` (symmetric with creation, where the operator owns the curated
slug verbatim).

## Acceptance

- `task rename <id> --title <long>` produces a slug capped at roughly 50 characters,
  trimmed on a hyphen boundary, matching `task new --title <long>`. Assert the id,
  filename, and inbound dependency rewrites all use the capped id.
- `task rename <id> --slug <long>` writes the curated slug verbatim after
  normalization (not capped), symmetric with creation.
- The de-slug and empty-slug-warning behavior is unchanged.
- After a capped rename `validate` passes and `--json` shape is unchanged.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- 2026-07-28T13:58:39Z: verification pass
- 2026-07-28T13:58:53Z: Capped the title-derived slug on task rename by reusing T-112's capSlug; explicit --slug stays verbatim. Full suite, vet, gofmt clean; go-reviewer approved with no high/medium findings; sandbox manual test passed all seven acceptance checks. Docs updated in README, CHANGELOG, and the rename --help text.
