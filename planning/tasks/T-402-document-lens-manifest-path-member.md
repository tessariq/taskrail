---
id: T-402-document-lens-manifest-path-member
title: Document the path member of spec-review lens manifest entries
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#post-spec-review-lenses
dependencies: []
updated_at: "2026-09-11T16:28:45Z"
---

# T-402-document-lens-manifest-path-member Document the path member of spec-review lens manifest entries

## Description

Both `taskrail-spec-review` arms in the v0.5.0 skill evaluation first wrote lens
manifest entries with `filename` and were refused with `missing member "path"`.
The skill's manifest description never names that member, so agents guess.

Outcome: the skill documents every required lens manifest entry member.

## Acceptance

- `taskrail-spec-review` names each required lens entry member, including
  `path`, matching the strict decoder.
- `task check:skills` passes after regeneration.

## Verification Notes

- Contract test comparing documented members with the decoder's fields, if
  practical; otherwise a focused skill-text assertion.

## Implementation Notes
