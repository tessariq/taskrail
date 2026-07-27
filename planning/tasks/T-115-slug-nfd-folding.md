---
id: T-115-slug-nfd-folding
title: Fold NFD-decomposed accents in slug transliteration
status: completed
priority: medium
spec_ref: specs/v0.4.0.md#slugged-task-creation
dependencies:
    - T-109-slug-transliteration-warn
updated_at: "2026-07-27T12:57:40Z"
---

# T-115-slug-nfd-folding Fold NFD-decomposed accents in slug transliteration

## Description

T-109's transliteration table (`internal/taskrail/slug.go`) maps **precomposed**
Latin-1 runes (`ü` = U+00FC). Input in NFD form — a base letter followed by a
combining mark, which macOS terminals and some IMEs produce — matches no entry, so
the base letter passes through and the combining mark is silently dropped by the
`[^a-z0-9]+` collapse. An NFD `ü` therefore degrades to `u` instead of expanding to
`ue`, and the same title yields two different slugs depending on how the operator's
keyboard encoded it.

Per the v0.4.0 Slugged Task Creation amendment, transliteration is meant to give
readable slugs for accented titles regardless of input encoding. Close the NFD hole
without adding `golang.org/x/text` — AGENTS.md prefers the standard library, and
the fold needed here is narrow enough for it.

## Acceptance

- Slugifying an NFD-decomposed title yields the same slug as its precomposed form:
  `slugify("Über Fußball")` equals `slugify("Über Fußball")` equals
  `ueber-fussball`, and `slugify("Café Niño")` yields `cafe-nino`.
- The German two-letter expansion survives decomposition: base `a`/`o`/`u` followed
  by U+0308 folds to `ae`/`oe`/`ue`, not to the bare base letter.
- Every other combining mark folds away to its base letter (`e` + U+0301 → `e`),
  matching the precomposed accented-to-base rule.
- Implemented with the standard library only — no new runtime dependency.
- Cover with table-driven cases in `slug_test.go` that assert NFD and precomposed
  inputs agree, so the two encodings cannot drift apart.
- `task new` and `task rename` inherit the fix through the shared `slugify`.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- 2026-07-27T12:57:40Z: verification pass
