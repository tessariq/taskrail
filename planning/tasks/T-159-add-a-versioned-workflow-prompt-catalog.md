---
id: T-159-add-a-versioned-workflow-prompt-catalog
title: Add a versioned workflow prompt catalog
status: todo
priority: high
spec_ref: specs/v0.5.0.md#workflow-prompt-catalog-and-overrides
dependencies:
    - T-213-define-the-uniform-agent-machine-api
    - T-157-upgrade-repositories-transactionally-to-layout-2
    - T-223-run-every-v0-5-command-against-local-storage
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
- Read-only `task show` resolves exact full v0.5 IDs through committed or local storage, emits exact Markdown in text mode and the exact logical-path/content/digest JSON result, and gives prompts/skills a storage-neutral way to read task bytes.
- The registry includes `task-review`; all JSON representations use the common
  envelope while preserving exact prompt result payloads and clean text output.
- The task-implementation declaration contains only `TASK_ID`, `TASK_PATH`,
  `ACTIVE_SPEC_VERSION`, `ACTIVE_SPEC_PATH`,
  `IMPLEMENTATION_REVIEW_MAX_ITERATIONS`, and `STORAGE_MODE`; no policy path or
  policy-file render input exists.
- Task-implementation rendering resolves repository review maximum 2 by default,
  accepts only a per-render `--max-review-iterations` override in `1..5`, applies
  deterministic override-before-repository precedence, and rejects that flag for
  every other prompt. Loop diagnostics, not generic prompt-render output, report
  the effective value's source.
- Resolution order is repository override then built-in, with canonical
  in-repository regular-file and ancestor checks, UTF-8 and size limits, and
  explicit source/path/hash reporting.
- Neither default nor skill-installing init materializes built-ins, placeholders,
  or `.taskrail/prompts/`; local overrides are created only by users.
- Render remains strictly read-only and validates only context/path/token inputs;
  the generic review publisher owns durable external-agent output transactions.
- Path placeholders remain logical identifiers in both storage modes; built-ins and packaged skills use `task show` and `spec show` rather than opening or reconstructing local-overlay paths.
- Durable review inputs use storage-neutral `review show`; only transient proposal output placeholders may use the physical ignored `artifacts_dir` returned by `local path`.
- Unknown prompts, versions, tokens, files, contexts, providers, and write
  conflicts fail without output mutation.

## Verification Notes

- Map criteria to golden text/JSON/render fixtures, init snapshots, and negative
  token/path/encoding/size/version/alias cases.
- Check committed package parity and packaged behavior while proving catalog
  commands and failed renders are read-only.

## Implementation Notes
