---
id: T-176-classify-persisted-and-legacy-task-identities
title: Classify persisted and legacy task identities portably
status: todo
priority: high
spec_ref: specs/v0.6.0.md#generated-task-keys-and-stable-references
dependencies:
    - T-175-implement-arbitrary-width-generated-task-keys
updated_at: "2026-08-04T23:06:23Z"
---

# T-176-classify-persisted-and-legacy-task-identities Classify persisted and legacy task identities portably

## Description

Implement the ordered persisted-ID classifier, portable legacy matrix, stable
reference derivation, and cross-platform collision rules independently of
command writers.

## Acceptance

- Invalid UTF-8 persisted IDs are violations; valid UTF-8 then classifies
  exactly as generated, numeric-looking legacy, opaque, or other legacy,
  including leading-zero, folded/malformed suffix, zero, control-bearing, and
  nonportable cases.
- Positive numeric legacy claims canonical generated key/ref/order; portable
  zero/other legacy uses exact ref; nonportable zero/other legacy uses
  collision-checked `L~<sha256>`.
- Portable-safe and nonportable/archive-immutability outcomes follow the exact
  numeric/portability cross-product without conflicting references.
- Component/device, ASCII fold, NFC/Unicode case-fold, root/sibling, and
  reserved digest-reference collisions are deterministic and preserve existing
  bytes.
- Global ordering is numeric claimants, opaque ASCII IDs, then zero/nonnumeric
  legacy UTF-8 IDs, with no slug tie-break.

## Verification Notes

- Map criteria to invalid encoding versus valid-nonportable fixtures, every
  family/cross-product, Unicode aliases, devices, controls, digest collisions,
  and ordering.
- Verify no classifier path normalizes or rekeys persisted full IDs.

## Implementation Notes
