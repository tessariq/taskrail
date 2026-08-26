---
id: T-365-protect-loop-owned-git-configuration-through
title: Protect loop-owned Git configuration through postflight
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-312-enforce-frozen-input-and-ledger-mutation-integrity
updated_at: "2026-08-26T09:03:45Z"
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
