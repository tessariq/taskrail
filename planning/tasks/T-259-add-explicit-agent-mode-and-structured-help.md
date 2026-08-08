---
id: T-259-add-explicit-agent-mode-and-structured-help
title: Add explicit agent mode and structured help
status: todo
priority: high
spec_ref: specs/v0.6.0.md#embedded-skill-inspection-and-agent-mode
dependencies:
    - T-182-define-exact-v0-6-machine-result-schemas
updated_at: "2026-08-08T11:19:59Z"
---

# T-259-add-explicit-agent-mode-and-structured-help Add explicit agent mode and structured help

## Description

Add an explicit invocation-local agent mode that selects existing JSON envelopes
by default and exposes deterministic structured help without changing command
semantics or write consent.

## Acceptance

- Root `--agent-mode` and strict `TASKRAIL_AGENT_MODE` parsing implement the
  specified precedence; invalid values fail clearly and no unrelated environment
  marker enables the mode.
- Explicit `--json=true|false` and declared non-JSON formats override the default;
  every other JSON-capable command emits its ordinary schema-version-2 envelope.
- Root, parent, targeted, flag, and explicit `help --json` paths emit the exact
  deterministic help schema directly from command metadata without repository
  discovery or skew warnings; human help remains unchanged.
- Streaming loop execution, loop dry-run, exact-content output, version text,
  parse failures, warnings, and exit classification follow the normative
  exceptions with uncontaminated stdout.

## Verification Notes

- Golden-test environment/flag/output precedence and every help shape, including
  malformed environment values and commands used outside a repository.
- Mutation-test command metadata and format exceptions so a new command or flag
  cannot silently disappear from structured help.

## Implementation Notes
