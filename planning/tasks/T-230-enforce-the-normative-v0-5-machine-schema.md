---
id: T-230-enforce-the-normative-v0-5-machine-schema
title: Inventory the normative v0.5 machine schemas
status: todo
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies: []
updated_at: "2026-08-08T08:40:49Z"
---

# T-230-enforce-the-normative-v0-5-machine-schema Inventory the normative v0.5 machine schemas

## Description

Provide one inspectable inventory of every v0.5 JSON-capable command and loop
result file so later producers, decoders, and consumers share the co-normative
machine companion as their only wire-contract authority.

## Acceptance

- A1. Every planned v0.5 JSON-capable command and the loop result file has exactly
  one schema-version-1 inventory entry naming its result, warning subset, error
  subset, and report-result exit exceptions.
- A2. Inventory entries preserve the companion's canonical command paths and
  distinguish planned commands from commands currently constructed by the CLI.
- A3. The inventory exposes one deterministic, complete view that later schema
  decoding and drift enforcement can consume without deriving contracts from
  implementation structs, examples, or skills.

## Verification Notes

- A1: compare the inventory against every command/result/error entry in the
  co-normative companion and record exact agreement, including loop exceptions.
- A2: inspect deterministic inventory output for canonical paths and correct
  planned-versus-constructed classification.
- A3: exercise representative consumers against the inventory and show no second
  schema authority is required; T-269 owns deliberate drift failures.

## Implementation Notes
