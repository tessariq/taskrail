---
id: T-175-implement-arbitrary-width-generated-task-keys
title: Implement arbitrary-width generated task keys
status: todo
priority: high
spec_ref: specs/v0.6.0.md#generated-task-keys-and-stable-references
dependencies:
    - T-174-run-the-v0-5-0-gap-and-drift-release-gate
updated_at: "2026-08-04T23:06:23Z"
---

# T-175-implement-arbitrary-width-generated-task-keys Implement arbitrary-width generated task keys

## Description

Replace machine-integer and fixed-width assumptions with reusable decimal-string
key operations and allocation planning. Complete-ledger entry-point adoption
follows after the physical ledger exists.

## Acceptance

- Parsing, normalization, comparison, sorting, collision detection, increment,
  and max-plus-one use arbitrary-width decimal strings with leading-zero
  identity and minimum-three-digit output.
- Generated full IDs enforce positive keys, exact grammar, 252-byte component
  boundary, and lowercase portable slug rules.
- Allocation planning caps derived slugs to remaining bytes, rejects oversized
  explicit slugs, and fails without wrap/truncation/reuse at the 250-digit
  boundary.
- Numeric values and generations serialize only as strings, never machine
  integers, floats, or JSON numbers.
- The API accepts a supplied normalized claimant set and has no live-directory
  or filesystem-width assumptions.

## Verification Notes

- Map criteria to table tests beyond 64 bits, long carries, leading zeros, empty
  through 1000, maximum lengths, malformed values, and exhaustion.
- Property-test decimal comparison/increment and prove output formatting
  independent of host word size.

## Implementation Notes
