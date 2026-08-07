---
id: T-240-implement-the-normative-review-schema-decoders
title: Implement the normative review schema decoders
status: todo
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-230-enforce-the-normative-v0-5-machine-schema
updated_at: "2026-08-06T13:46:30Z"
---

# T-240-implement-the-normative-review-schema-decoders Implement the normative review schema decoders

## Description

Implement the single strict decoder set for every v0.5 task, spec, decomposition,
and workflow review object before publication commands consume untrusted proposals.

## Acceptance

- A1. Decoders enforce exact fields, enums, nullability, canonical times/paths,
  unique IDs/references, role-mandated v1 prompt IDs, lower-case template digests,
  `builtin|replacement` source, and the 1 MiB per-file cap without normalizing bytes.
- A2. Spec lens findings remain open observations and manifests are the only final
  disposition authority; terminal workflow evidence cannot form reference cycles.
- A3. The decoder inventory is shared by preview, apply, validation, review show,
  workflow index derivation, and golden schema tests.

## Verification Notes

- A1: positive goldens plus unknown/missing/duplicate/null/oversize, wrong-role,
  wrong-contract, malformed-digest, and invalid-source mutation cases provide
  strict boundary evidence.
- A2: conflicting disposition and circular evidence fixtures fail deterministically.
- A3: registry tests prove every consumer points to the same decoder/version.

## Implementation Notes
