---
id: T-268-decode-the-strict-common-machine-envelope
title: Decode the strict common machine envelope
status: completed
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies:
    - T-230-enforce-the-normative-v0-5-machine-schema
updated_at: "2026-08-08T17:45:32Z"
---

# T-268-decode-the-strict-common-machine-envelope Decode the strict common machine envelope

## Description

Give machine consumers one strict schema-version-1 decoder for the common success
and error envelope, warnings, violations, snapshots, and recovery references
defined by the uniform agent machine-result contract.

## Acceptance

- A1. Valid success and error documents decode only when they contain the exact
  common envelope members, canonical command path, non-null warnings, and exactly
  one of `result` or `error`.
- A2. Unsupported versions, unknown or missing members, duplicate keys, trailing
  values, wrong nullability, and null required arrays are rejected without
  accepting a partial document.
- A3. Common errors accept only the closed code registry and exact ordered details,
  including valid violation, typed-snapshot, and nullable recovery shapes.
- A4. Warning decoding accepts only the closed discriminated union and rejects
  command-local, ambiguous, or malformed variants.

## Verification Notes

- A1: decode representative success and error goldens and observe the exact common
  values available to a consumer.
- A2: mutate version, keys, exclusivity, nullability, array shape, and document
  framing and observe deterministic rejection at the decoder boundary.
- A3: exercise every common error-detail variant, invalid path-kind/path pairings,
  digest casing, ordering, and recovery phase.
- A4: exercise each warning variant plus unknown codes and cross-variant fields.

## Implementation Notes

- 2026-08-08T17:45:11Z: Added a strict schema-version-1 decoder for the common machine document: envelope framing, closed warning union, and common error details (violations, paths, typed snapshots, nullable recovery). Command result payloads stay raw bytes; loop postflight codes are rejected as loop-diagnostic carriers.
- 2026-08-08T17:45:32Z: verification pass
