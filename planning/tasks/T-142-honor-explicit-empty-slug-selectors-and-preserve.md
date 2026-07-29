---
id: T-142-honor-explicit-empty-slug-selectors-and-preserve
title: Honor explicit empty slug selectors and preserve token boundaries
status: completed
priority: high
spec_ref: specs/v0.4.0.md#slugged-task-creation
dependencies:
    - T-112-slug-length-cap
    - T-126-rename-slug-length-cap
updated_at: "2026-07-29T14:51:47Z"
---

# T-142-honor-explicit-empty-slug-selectors-and-preserve Honor explicit empty slug selectors and preserve token boundaries

## Description

Flag presence is currently inferred from trimmed values, so an explicitly supplied
empty `--slug` is treated as absent by `task new` and as no selector by `task
rename`. Also, a title that slugifies to one token longer than the cap is hard-cut
mid-token. Preserve selector presence and honor the spec's warned bare-id fallback
and token-boundary cap.

## Acceptance

- `task new --title X --slug ""` and whitespace/punctuation-only explicit slugs
  use the explicit selector, write a bare id, and emit the empty-slug warning.
- `task rename <id> --slug ""` and equivalent empty-normalizing selectors de-slug
  the task instead of reporting that no selector was supplied.
- Title-derived slugs are never cut mid-token; define and document the bounded
  fallback when the first token alone exceeds the nominal cap.
- Explicit non-empty `--slug` values remain normalized and uncapped, JSON stdout
  stays clean, and creation/rename tests cover flag presence separately from value.

## Verification Notes

- T-140 sandbox evidence created `T-001-visible-title` from an explicit empty slug,
  rejected an empty rename selector, and cut a 68-character token at byte 50.

## Implementation Notes

- Cobra's `Flag.Changed` (or an equivalent explicit-presence field at the command
  boundary) is needed; a string value alone cannot distinguish absent from empty.
- 2026-07-29T14:51:34Z: verification pass
- 2026-07-29T14:51:47Z: Implemented explicit selector presence and token-boundary slug capping; full, race, vet, validation, review, and sandbox checks pass
