---
id: T-159-add-a-versioned-workflow-prompt-catalog
title: Add a versioned workflow prompt catalog
status: todo
priority: high
spec_ref: specs/v0.5.0.md#workflow-prompt-catalog-and-overrides
dependencies:
    - T-157-upgrade-repositories-transactionally-to-layout-2
updated_at: "2026-08-04T21:32:13Z"
---

# T-159-add-a-versioned-workflow-prompt-catalog Add a versioned workflow prompt catalog

## Description

Expose embedded workflow prompts as a versioned, inspectable read-only catalog
with deterministic list/show and committed replacement resolution. Contextual
rendering, task inspection, and local replacement mapping are separate outcomes.

## Acceptance

- Prompt list and show implement exact text/JSON, deterministic registry order,
  explicit contract-version selection, built-in retrieval, and complete whole-file
  committed replacement resolution.
- The registry includes `task-review`; all JSON representations use the common
  envelope while preserving exact prompt result payloads and clean text output.
- The task-implementation declaration contains only `TASK_ID`, `TASK_PATH`,
  `ACTIVE_SPEC_VERSION`, `ACTIVE_SPEC_PATH`,
  `IMPLEMENTATION_REVIEW_MAX_ROUNDS`, and `STORAGE_MODE`; no policy path or
  policy-file render input exists.
- Resolution order is repository override then built-in, with canonical
  in-repository regular-file and ancestor checks, UTF-8 and size limits, and
  explicit ID/version/source/logical-path/template-hash reporting over the exact
  resolved bytes later consumed by review publication.
- Neither default nor skill-installing init materializes built-ins, placeholders,
  or `.taskrail/prompts/`; local overrides are created only by users.
- T-250 owns context flags, placeholder substitution, review-round rendering, and
  storage-neutral subject reads; T-236 owns local replacement resolution.
- Unknown prompts, versions, tokens, files, contexts, providers, and write
  conflicts fail without output mutation.

## Verification Notes

- Map criteria to golden list/show fixtures, init snapshots, and negative
  path/encoding/size/version/alias cases.
- Mutate one template byte and add/remove an equal-byte replacement to prove the
  digest and source class independently identify the exact resolution.
- Check committed package parity and packaged behavior while proving catalog
  commands and failed renders are read-only.

## Implementation Notes
