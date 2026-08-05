---
id: T-213-define-the-uniform-agent-machine-api
title: Define the uniform agent machine API
status: todo
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies: []
updated_at: "2026-08-05T20:24:07Z"
---

# T-213-define-the-uniform-agent-machine-api Define the uniform agent machine API

## Description

Introduce one versioned JSON success/error envelope across every agent-consumed
command. Close inherited lifecycle gaps, make skill parsing policy explicit, and
give later versions one machine contract to extend rather than replace.

## Acceptance

- Every existing `--json` command emits one strict common envelope with clean stdout, non-null warnings, stable errors, and exact result types; text and JSON classify exits identically.
- `start`, `complete`, and `block` gain JSON without changing human text behavior. Parser/argument, validation, conflict, partial-write, rollback, and recovery errors after command selection are structured.
- The common init result type defines layout, human-notes, and skill destination candidates/results in one preview/apply shape; T-214 supplies the notes behavior that populates those fields.
- Packaged skills use JSON whenever they consume IDs, paths, warnings, previews, or failures; content-producing text flows remain explicit exceptions.
- v0.6/v0.7 schema tasks reuse the envelope and shared registries. README/help/CHANGELOG document the one-time v0.5 compatibility break.

## Verification Notes

- Map every JSON-capable command to exact success/refusal goldens, stdout/stderr checks, human/JSON exit parity, and unknown-field decoder tests.
- Mutation-test the skill command registry and run package parity plus Linux/macOS/Windows CLI smoke coverage.

## Implementation Notes
