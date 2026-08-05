---
id: T-159-add-a-versioned-workflow-prompt-catalog
title: Add a versioned workflow prompt catalog
status: todo
priority: high
spec_ref: specs/v0.5.0.md#workflow-prompt-catalog-and-overrides
dependencies:
    - T-213-define-the-uniform-agent-machine-api
updated_at: "2026-08-04T21:32:13Z"
---

# T-159-add-a-versioned-workflow-prompt-catalog Add a versioned workflow prompt catalog

## Description

Expose embedded workflow prompts as a versioned, inspectable read-only catalog
with deterministic list, show, and render behavior. Support repository-local
replacements without adding provider integration or hidden execution.

## Acceptance

- Prompt list, show, and render implement the exact text and JSON contracts,
  explicit contract-version selection, required path-valued subject flags,
  declared token grammar, one-pass substitution, and no embedded task/spec file
  contents.
- The registry includes `task-review`; all JSON representations use the common
  envelope while preserving exact prompt result payloads and clean text output.
- The task-implementation declaration contains only `TASK_ID`, `TASK_PATH`,
  `ACTIVE_SPEC_VERSION`, and `ACTIVE_SPEC_PATH`; no policy path or policy-file
  render input exists.
- Resolution order is repository override then built-in, with canonical
  in-repository regular-file and ancestor checks, UTF-8 and size limits, and
  explicit source/path/hash reporting.
- Neither default nor skill-installing init materializes built-ins, placeholders,
  or `.taskrail/prompts/`; local overrides are created only by users.
- Render remains strictly read-only and validates only context/path/token inputs;
  the generic review publisher owns durable external-agent output transactions.
- Unknown prompts, versions, tokens, files, contexts, providers, and write
  conflicts fail without output mutation.

## Verification Notes

- Map criteria to golden text/JSON/render fixtures, init snapshots, and negative
  token/path/encoding/size/version/alias cases.
- Check committed package parity and packaged behavior while proving catalog
  commands and failed renders are read-only.

## Implementation Notes
