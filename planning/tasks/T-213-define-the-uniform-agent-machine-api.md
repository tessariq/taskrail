---
id: T-213-define-the-uniform-agent-machine-api
title: Define the uniform agent machine API
status: todo
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies:
    - T-230-enforce-the-normative-v0-5-machine-schema
updated_at: "2026-08-08T08:40:49Z"
---

# T-213-define-the-uniform-agent-machine-api Define the uniform agent machine API

## Description

Introduce one versioned JSON success/error envelope across every agent-consumed
command. Close inherited lifecycle gaps, make skill parsing policy explicit, and
give later versions one machine contract to extend rather than replace.

## Acceptance

- Every existing `--json` command emits one strict common envelope with clean stdout, non-null warnings, stable errors, canonical command paths, registered `(command,error.code)` pairs, and exact result types; text and JSON classify exits identically.
- Completed `task loop list` and loop dry-run reports retain result envelopes when their reported violations make exit non-zero; every writer refusal and inability to produce a promised report uses an error envelope.
- `start`, `complete`, and `block` gain JSON without changing human text behavior. Parser/argument, validation, conflict, partial-write, rollback, and recovery errors after command selection are structured.
- Lifecycle and init success payloads, the closed v0.5 error-code vocabulary, and
  common violation/typed-snapshot/recovery details match the exact normative fields;
  the outer schema version describes those inner payloads and later incompatible
  releases increment it globally.
- The closed warning union retains inherited slug, selection, and skill-skew
  signals and adds exact local-bootstrap, local-head-drift, and verify-order variants; inherited
  coverage gates remain completed non-zero result reports.
- The common init result type defines layout, human-notes, skill-file, and exact
  skill-exclusion candidates/results in one preview/apply shape; absent
  `--with-skills` produces empty skill and exclusion lists, while opted-in
  installs report normal assistant discovery paths. Ordinary committed installs
  report no exclusions; local installs report one exact subtree per packaged
  skill in each assistant root, while pending-skill committed refresh preserves
  that inventory. Refusals use error envelopes, never a result action. T-214
  supplies the notes behavior that populates the note fields.
- Status reports exact storage mode/root and the resolved repository-root-relative
  physical `artifacts_dir` so skills select delivery behavior and ignored
  transient staging without reading the layout marker or deriving configured
  paths; roots are `.` for committed and `.taskrail/local` for local, and the
  artifacts path never becomes a durable citation or Git-visible provenance.
  This task proves the exact schema, committed behavior, and mapper behavior over
  explicitly supplied committed/fixed-local contexts; T-223 owns end-to-end local
  discovery and status integration.
- Packaged skills use JSON whenever they consume IDs, paths, warnings, previews, or failures; content-producing text flows remain explicit exceptions.
- v0.6/v0.7 emit envelope generations 2/3 while retaining outer member names.
  README/help/CHANGELOG document the one-time v0.5 direct-result break.

## Verification Notes

- Map every JSON-capable command to exact success/refusal goldens, stdout/stderr checks, human/JSON exit parity, committed custom-planning-directory fixtures plus explicitly supplied fixed-local mapper contexts, mixed-class snapshot fixtures, and unknown-field decoder tests. T-223 supplies real local repository discovery fixtures.
- Mutation-test the skill command registry and run package parity plus Linux/macOS/Windows CLI smoke coverage.

## Implementation Notes
