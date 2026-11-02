---
id: T-449-remove-obsolete-llm-adapter-roadmap-wording-from
title: Remove obsolete LLM adapter roadmap wording from skills
status: completed
priority: low
spec_ref: specs/v0.5.0.md#recommendation-about-llm-support
dependencies: []
updated_at: "2026-11-02T23:43:17Z"
completion_id: "58d032428444f9715b8fdaee1208e663"
last_verification_id: "d43c81ad1ef5ed877212343b94a24612"
last_verification_result: pass
last_verified_at: "2026-11-02T23:43:17Z"
last_verified_completion_id: "58d032428444f9715b8fdaee1208e663"
---

# T-449-remove-obsolete-llm-adapter-roadmap-wording-from Remove obsolete LLM adapter roadmap wording from skills

## Description

Adopter skill audit P8 flags references to a nonexistent thin `--llm` adapter.
The current taskrail-import and taskrail-retrofit skills still call it
"deferred to a later version by design", while the active spec excludes
direct model/provider calls. taskrail-decompose no longer has this wording.

Remove the obsolete roadmap promise from installed guidance. Preserve the
actual contract: agents perform semantic work outside the provider-neutral,
deterministic CLI. No new flag, adapter, or spec change belongs in this task.

## Acceptance

- Import and retrofit no longer describe `--llm` as an available or planned
  adapter; inspect other packaged skills for the same obsolete promise.
- Retain explicit agent/binary responsibility boundaries without implying
  direct model calls will ship later.
- Regenerate committed skill copies and retain package parity.

## Verification Notes

- Search all packaged skill files for `--llm` and deferred-adapter wording;
  verify no obsolete instruction or roadmap promise remains.
- Run the relevant skill-contract tests and `task check:skills` after copy
  regeneration. No CLI behavior or provider integration is introduced.

## Implementation Notes

- 2026-11-02T23:43:01Z: Removed obsolete future adapter promises from import and retrofit while retaining agent semantic work and deterministic provider-neutral binary boundaries. Regenerated both skill mirrors; structural checks, full Go tests, vet, dedicated simplification and independent review passed.
- 2026-11-02T23:43:17Z: verification pass id d43c81ad1ef5ed877212343b94a24612 previous none completion 58d032428444f9715b8fdaee1208e663
