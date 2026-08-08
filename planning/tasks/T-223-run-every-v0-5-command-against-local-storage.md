---
id: T-223-run-every-v0-5-command-against-local-storage
title: Make shared readers, renderers, and validation storage-neutral
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-222-initialize-and-discover-ignored-local-taskrail
updated_at: "2026-08-08T08:40:49Z"
---

# T-223-run-every-v0-5-command-against-local-storage Make shared readers, renderers, and validation storage-neutral

## Description

Refactor shared task/spec/state readers, renderers, validation, artifact guards,
and path helpers to consume the active storage context while retaining logical
semantic paths. Command-family writers and the complete inherited parity gate are
owned by T-289 through T-291.

## Acceptance

- Shared loaders enumerate and open config/spec/task/state/note/review bytes through
  logical-to-physical mapping, returning logical identities in parsed models and
  diagnostics in both storage modes.
- State/task renderers and strict validation preserve logical `specs/...` and
  `planning/...` references; local overlay prefixes never enter durable semantic
  text or canonical summaries.
- Shared path, collision, artifact, and rename guards operate on context-resolved
  physical paths while reporting the contract's logical or explicitly transient
  path kind; no helper assumes repository-root `specs/` or `planning/`.
- Existing read-only report computations produce storage-equivalent semantic
  models from byte-equivalent committed/local fixtures and remain write-free.
- Unsupported context capabilities and mixed-state evidence refuse explicitly;
  helpers never probe or fall back to a second storage mode.

## Verification Notes

- Feed byte-equivalent committed/local and custom-directory fixtures through each
  shared loader, renderer, validator, report computation, and guard; compare models
  and logical output exactly.
- Use decoy logical files at repository-root paths to detect bypasses of the active
  context and assert no fallback reads occur.
- Snapshot all fixture bytes around read-only coverage and expose reusable context
  fixtures for T-289 through T-291 and downstream prompt/review/loop tasks.

## Implementation Notes
