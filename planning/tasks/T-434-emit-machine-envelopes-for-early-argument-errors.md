---
id: T-434-emit-machine-envelopes-for-early-argument-errors
title: Emit machine envelopes for early argument errors
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies: []
updated_at: "2026-10-31T11:39:12Z"
---

# T-434-emit-machine-envelopes-for-early-argument-errors Emit machine envelopes for early argument errors

## Description

Bug. An unknown flag placed before `--json` prints plain text to stderr and
nothing on stdout (`task loop allow T-001-x --reason r --bogus --json`), because
cobra stops parsing at the unknown flag and `machineJSONRequested`
(cmd/taskrail/machine.go) never sees `--json`. The spec says that once a
command is selected, argument failures use the envelope. In addition `next`,
`status`, `validate` and `stats` accept extra positional arguments silently;
`next --json extra` exits 0 and writes STATE.md. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- An unknown flag before `--json` yields exactly one `invalid_arguments` envelope on stdout.
- `next`, `status`, `validate` and `stats` reject extra positional arguments with `invalid_arguments`, and `next` writes nothing in that case.

## Verification Notes

- Smoke tests in cmd/taskrail for both cases.

## Implementation Notes
