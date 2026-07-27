---
id: T-109-slug-transliteration-warn
title: Transliterate Latin-1 in slugs and warn on empty derived slug
status: completed
priority: medium
spec_ref: specs/v0.4.0.md#slugged-task-creation
dependencies: []
updated_at: "2026-07-27T12:31:59Z"
---

# T-109-slug-transliteration-warn Transliterate Latin-1 in slugs and warn on empty derived slug

## Description

`slugify` (`internal/taskrail/slug.go`) collapses every non-`[a-z0-9]` run to a
hyphen with no transliteration, so accented titles lose characters silently:
`--title "Über Fußball"` yields `ber-fu-ball` (ü and ß dropped), and a fully
non-Latin title yields an empty slug that falls back to a bare `T-<n>` id with no
signal. The same silent empty-slug fallback hits an explicit `--slug "!!!"`.

Per the v0.4.0 Slugged Task Creation amendment, harden slug derivation so
common Latin-1 letters transliterate to ASCII before slugifying, and so a
supplied `--title`/`--slug` that normalizes to empty produces a visible warning
rather than a silent bare id. `slugify` is shared by `task new` (T-095) and
`task rename` (T-096), so both paths benefit from one change.

## Acceptance

- A transliteration step folds common Latin-1 letters to ASCII before slugifying:
  at minimum `ä→ae`, `ö→oe`, `ü→ue`, `ß→ss`, and accented Latin letters to their
  unaccented base (`é→e`, `ñ→n`, `ç→c`). `slugify("Über Fußball")` yields
  `ueber-fussball` (or `uber-fussball` if ligatures are folded without the `e`),
  not `ber-fu-ball`. Cover with table-driven cases in `slug_test.go`.
- When a supplied `--title` or `--slug` normalizes to an empty slug, `task new`
  still writes the legitimate bare `T-<n>` id and filename, `validate` passes, and
  the command prints a warning to stderr naming the reason (title/slug produced no
  slug segment). No warning is printed when neither `--title` nor `--slug` is
  supplied (the intentional bare-id case).
- `--json` output is unchanged in shape; the warning goes to stderr and does not
  corrupt machine-readable stdout.
- Transliteration and the empty-slug warning apply identically on the
  `task rename` path (shared `slugify`), verified by a rename test.
- On `task rename`, a `--slug`/`--title` that normalizes to an empty slug
  **de-slugs** the task to the bare `T-<n>` id: `T-<n>-<slug>.md` is renamed to
  `T-<n>.md`, inbound references are rewritten, the same empty-slug warning is
  printed to stderr, and `validate` passes. Assert with a rename test that starts
  from a slugged id and ends at the bare id (symmetric with creation's bare-id
  fallback, per the v0.4.0 Task Rename And Re-Slug amendment).

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- 2026-07-27T12:31:48Z: verification pass
