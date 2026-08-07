---
id: T-230-enforce-the-normative-v0-5-machine-schema
title: Enforce the normative v0.5 machine schema inventory
status: todo
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies: []
updated_at: "2026-08-06T13:46:30Z"
---

# T-230-enforce-the-normative-v0-5-machine-schema Enforce the normative v0.5 machine schema inventory

## Description

Turn the co-normative v0.5 machine companion into a checked command/schema
inventory so no command, warning, error, or result shape is inferred from a Go
struct or skill parser.

## Acceptance

- A1. Every JSON-capable v0.5 command and loop result file has exactly one
  schema-version-1 result type, warning subset, error subset, and exit policy.
- A2. Strict decoders reject unknown/missing fields, wrong nullability, null arrays,
  unsupported envelope versions, invalid snapshot path-kind/path combinations,
  ambiguous prompt template/content hashes, command-local warning fields, and a
  review publisher error outside its exact prompt-aware subset.
- A3. Registry drift fails when a command or schema exists in implementation,
  tests, prompts, or skills without a normative inventory entry.

## Verification Notes

- A1: generate a registry report from command construction and compare it with the
  companion inventory; expected observation is a complete one-to-one mapping.
- A2: golden positive and mutation-negative fixtures cover prompt content/template
  digests and `prompt_invalid` review-publication errors alongside inherited shapes.
- A3: deliberately add an unregistered fixture command/schema and show the drift
  check fails before restoring the registry.

## Implementation Notes
