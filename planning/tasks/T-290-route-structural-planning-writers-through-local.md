---
id: T-290-route-structural-planning-writers-through-local
title: Route structural planning writers through local storage
status: todo
priority: high
spec_ref: specs/v0.5.0.md#local-planning-mode
dependencies:
    - T-223-run-every-v0-5-command-against-local-storage
    - T-283-protect-repair-and-spec-writers-transactionally
    - T-284-protect-importdraft-v1-publication-transactionally
    - T-287-initialize-ignored-local-planning-storage-durably
updated_at: "2026-08-08T14:23:08Z"
---

# T-290-route-structural-planning-writers-through-local Route structural planning writers through local storage

## Description

Route structural inherited writers through local storage: repair apply, spec add/
activate, and ImportDraft v1 apply. Explicit local init remains owned by T-287;
implicit bootstrap policy and integration remain with T-245 and the final parity
gate.

## Acceptance

- Repair, spec, and v1 import read logical managed paths through the active context
  and publish their exact validated candidates only beneath `.taskrail/local/` in
  local mode.
- Logical spec/task/state references and command results remain byte-equivalent to
  committed-mode semantics; no durable text contains the physical overlay prefix.
- Preview/read forms remain write-free and never bootstrap; apply forms preserve
  their no-clobber/normal-transaction guarantees against an already initialized
  active context.
- Local publications remain ignored, untracked, and unstaged with ordinary Git
  status clean; decoy committed planning/spec files remain unchanged.
- Unsupported or mixed contexts refuse with common errors and never create a
  partial local/committed layout.

## Verification Notes

- Run paired committed/local repair, spec add/activate, and multi-entry v1 import
  fixtures, comparing logical output and semantic post-state.
- Assert preview purity, exact fixed-overlay writes, clean Git index/status, decoy
  committed-file preservation, collision handling, and rollback fault behavior.
- Exercise explicit T-287 initialization; T-291 owns the combined implicit-
  bootstrap and inherited-writer parity evidence.

## Implementation Notes
