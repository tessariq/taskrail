---
id: T-365-protect-loop-owned-git-configuration-through
title: Protect loop-owned Git configuration through postflight
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-312-enforce-frozen-input-and-ledger-mutation-integrity
updated_at: "2026-08-26T10:01:51Z"
completion_id: "182d2ea408487be6983098ea76725d20"
last_verification_id: "62720cccec44c0e3de94811d3c7ee6df"
last_verification_result: pass
last_verified_at: "2026-08-26T10:01:51Z"
last_verified_completion_id: "182d2ea408487be6983098ea76725d20"
---

# T-365-protect-loop-owned-git-configuration-through Protect loop-owned Git configuration through postflight

## Description

Make loop delivery fail closed when a delegated child changes repository-owned
Git configuration. A successful loop must leave the effective worktree and
common-directory configuration byte- and identity-identical to preflight without
claiming control over user-global or system configuration.

## Acceptance

- Sequential and parallel preflight capture absent/present no-follow identities
  and exact bytes for repository-owned Git configuration.
- Creation, deletion, replacement, aliasing, or byte mutation produces
  `invalid_postflight` and prevents delivery; unchanged configuration passes.
- Native Unix and Windows fixtures cover common and per-worktree configuration
  without extending the guarantee to global or system configuration.

## Verification Notes

- Exercise child mutations of each protected configuration location, path
  substitution, absent-to-present creation, and an unchanged successful run.
- Run focused loop postflight tests, full tests, race tests, vet, and cross-platform
  CI.

## Implementation Notes

- 2026-08-26T10:01:36Z: Protect no-follow Git configuration snapshots through sequential and parallel loop postflight.
- 2026-08-26T10:01:51Z: verification pass id 62720cccec44c0e3de94811d3c7ee6df previous none completion 182d2ea408487be6983098ea76725d20
