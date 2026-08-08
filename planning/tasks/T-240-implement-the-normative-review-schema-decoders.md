---
id: T-240-implement-the-normative-review-schema-decoders
title: Decode common task and spec review schemas strictly
status: todo
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-230-enforce-the-normative-v0-5-machine-schema
updated_at: "2026-08-08T08:40:49Z"
---

# T-240-implement-the-normative-review-schema-decoders Decode common task and spec review schemas strictly

## Description

Provide the shared strict review decoding rules plus complete task-review and
post-spec lens/manifest decoding, giving publication a trustworthy boundary for
the first independently publishable review bundle types.

## Acceptance

- A1. Common review decoding rejects invalid framing, exact-field violations,
  wrong nullability, malformed canonical values, unsupported schema versions, and
  files over 1 MiB without normalizing accepted bytes.
- A2. Task-review documents enforce exact task/spec bindings, role-mandated prompt
  identity, portable session identity, unique findings, and the closed
  severity/disposition vocabulary.
- A3. Spec lens files enforce their role-specific prompt identity, common subject
  snapshot, open observation shape, and session-wide unique finding identities.
- A4. Spec manifests bind the fixed ordered lenses by exact digest and are the sole
  complete disposition authority, rejecting missing, unknown, conflicting, or
  unresolved required decisions.

## Verification Notes

- A1: accepted-byte goldens plus unknown/missing/duplicate/null/trailing/oversize
  and malformed canonical-value mutations demonstrate the common boundary.
- A2: task-review positive and wrong-role, wrong-contract, duplicate-ID, enum, and
  subject-binding cases demonstrate exact decoding.
- A3: decode four aligned lens goldens and reject role, snapshot, session, and
  cross-lens identity conflicts.
- A4: exercise complete disposition coverage and digest/order bindings plus missing,
  duplicate, unknown, and unresolved high/medium finding cases.

## Implementation Notes
