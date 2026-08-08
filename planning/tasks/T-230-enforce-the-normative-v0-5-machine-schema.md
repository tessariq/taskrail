---
id: T-230-enforce-the-normative-v0-5-machine-schema
title: Enforce the normative v0.5 machine schema inventory
status: todo
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies: []
updated_at: "2026-08-08T08:40:49Z"
---

# T-230-enforce-the-normative-v0-5-machine-schema Enforce the normative v0.5 machine schema inventory

## Description

Turn the co-normative v0.5 machine companion into a checked command/schema
inventory so no command, warning, error, or result shape is inferred from a Go
struct or skill parser.

## Acceptance

- A1. The checked normative inventory assigns every planned JSON-capable v0.5
  command and loop result file exactly one schema-version-1 result type, warning
  subset, error subset, and exit policy before feature implementation exists.
- A2. Strict decoders reject unknown/missing fields, wrong nullability, null arrays,
  unsupported envelope versions, invalid snapshot path-kind/path combinations,
  ambiguous prompt template/content hashes, command-local warning fields, and a
  review publisher error outside its exact prompt-aware subset.
- A3. Registration and drift checks fail when an implemented command or schema
  lacks its normative entry or disagrees with it. Feature tasks populate their
  registered results; T-173 owns final implementation/test/prompt/skill inventory
  completeness.

## Verification Notes

- A1: generate the complete normative inventory and compare currently constructed
  commands with their registered subset; future inventory entries remain checked
  declarations rather than pretending their commands already exist.
- A2: golden positive and mutation-negative fixtures cover prompt content/template
  digests and `prompt_invalid` review-publication errors alongside inherited shapes.
- A3: deliberately add an unregistered fixture command/schema and a mismatched
  registered implementation and show the drift check fails before restoration.

## Implementation Notes
