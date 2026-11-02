---
id: T-449-remove-obsolete-llm-adapter-roadmap-wording-from
title: Remove obsolete LLM adapter roadmap wording from skills
status: todo
priority: low
spec_ref: specs/v0.5.0.md#recommendation-about-llm-support
dependencies: []
updated_at: "2026-11-02T21:57:31Z"
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
