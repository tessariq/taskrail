---
id: T-273-complete-machine-api-consumer-compatibility
title: Complete machine API consumer compatibility
status: completed
priority: high
spec_ref: specs/v0.5.0.md#uniform-agent-machine-results
dependencies:
    - T-270-migrate-inherited-read-only-machine-results
    - T-271-migrate-inherited-semantic-writer-machine-results
    - T-272-implement-init-status-and-warning-machine
updated_at: "2026-08-12T11:50:44Z"
---

# T-273-complete-machine-api-consumer-compatibility Complete machine API consumer compatibility

## Description

Finish the v0.5 machine API as an integrated consumer contract: migrate packaged
skills to structured results, document the direct-result compatibility break, and
prove later envelope generations reject or retain inherited documents deliberately.

## Acceptance

- A1. Packaged skills use `--json` whenever they consume IDs, paths, warnings,
  eligibility, previews, lifecycle outcomes, or failures; exact human/content text
  flows remain explicit exceptions.
- A2. Every v0.5 JSON-capable command and loop result file passes the normative
  inventory, strict decoder, producer, and consumer compatibility gate as one
  integrated surface.
- A3. User documentation describes the one-time v0.5 direct-result break without a
  hidden legacy-output switch and identifies schema version as the whole-document
  compatibility boundary.
- A4. v0.6 and v0.7 envelope generations retain the outer member names, reject
  unsupported versions, and do not silently decode incompatible inherited shapes.

## Verification Notes

- A1: exercise each packaged skill's consumed command paths and package-parity
  checks, including the documented exact-text exceptions.
- A2: run the complete command/loop compatibility matrix through strict producer
  and consumer goldens on supported platforms.
- A3: inspect README/help/CHANGELOG guidance against actual v0.5 output and absence
  of a legacy switch.
- A4: decode representative generation-1/2/3 documents with supported and
  unsupported consumers and observe explicit compatibility outcomes.

## Implementation Notes

- 2026-08-12T11:19:09Z: verification fail
- 2026-08-12T11:34:13Z: Review budget exhausted after correcting final packaged-command coverage, provenance receipt-path validation, and loop terminal-outcome consistency; one fresh correctness review of the resulting final bytes is required
- 2026-08-12T11:34:24Z: verification fail
- 2026-08-12T11:50:20Z: Fresh final-byte correctness review passed after strict generation fixture corrections
- 2026-08-12T11:50:34Z: Completed uniform machine-result consumer migration, executable packaged-skill JSON flows, direct-result compatibility documentation, and strict generation 1/2/3 compatibility fixtures; fresh final-byte review was clean and all checks passed
- 2026-08-12T11:50:44Z: verification pass
