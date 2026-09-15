---
id: T-402-document-lens-manifest-path-member
title: Document the path member of spec-review lens manifest entries
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#post-spec-review-lenses
dependencies: []
updated_at: "2026-09-15T16:36:33Z"
completion_id: "8f29687f9026715178ba9a5f448def4e"
last_verification_id: "f67320b78ffb82ca0b4b8786174f73b2"
last_verification_result: pass
last_verified_at: "2026-09-15T16:36:33Z"
last_verified_completion_id: "8f29687f9026715178ba9a5f448def4e"
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

- 2026-09-15T16:36:29Z: The packaged taskrail-spec-review skill now names every schema-2 manifest round lens entry member (lens, path, sha256, spec_sha256) with per-member semantics, pinned by a contract test that builds the documented list from the strict decoder's own member list; a decoder mutation case pins the filename-instead-of-path refusal the skill evaluation hit; .agents/.claude copies regenerated with check:skills parity green.
- 2026-09-15T16:36:33Z: verification pass id f67320b78ffb82ca0b4b8786174f73b2 previous none completion 8f29687f9026715178ba9a5f448def4e
