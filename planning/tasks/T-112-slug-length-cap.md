---
id: T-112-slug-length-cap
title: Cap title-derived slug length
status: todo
priority: medium
spec_ref: specs/v0.4.0.md#slugged-task-creation
dependencies:
    - T-109-slug-transliteration-warn
updated_at: "2026-07-17T11:21:18Z"
---

# T-112-slug-length-cap Cap title-derived slug length

## Description

`slugify` never truncates, so a long `--title` produces a long slugged id and an
equally long filename — unwieldy amid short curated siblings, and at the extreme
brushing the 255-byte filesystem name limit. The v0.4.0 Slugged Task Creation
amendment caps the *title-derived* slug while leaving an explicit `--slug`
verbatim, since the operator owns that choice.

Depends on T-109 (both edit `slugify`/its callers); land the transliteration and
warning first so the cap operates on already-transliterated text.

## Acceptance

- A title-derived slug is capped at roughly 50 characters, trimmed on a hyphen
  boundary so no word is cut mid-token and no leading/trailing hyphen survives.
  A very long `--title` yields a bounded slug and filename; assert with a
  table-driven case.
- An explicit `--slug` is normalized but NOT length-capped — the operator's
  curated slug is written verbatim (after slugify normalization).
- The cap composes with T-109 transliteration: transliteration happens first,
  then the cap, so multibyte source characters do not blow the character budget.
- After creation `validate` passes for a capped-slug task; `--json` shape
  unchanged.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
