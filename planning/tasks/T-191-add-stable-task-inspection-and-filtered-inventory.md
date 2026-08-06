---
id: T-191-add-stable-task-inspection-and-filtered-inventory
title: Add stable task inspection and filtered inventory
status: todo
priority: medium
spec_ref: specs/v0.6.0.md#shared-task-reference-resolver-and-inspection
dependencies:
    - T-179-resolve-stable-task-references-across-every
    - T-178-load-live-and-archived-tasks-as-one-immutable
    - T-185-upgrade-repositories-transactionally-to-layout-3
    - T-183-validate-cancellation-generation-and-archive
    - T-190-validate-unified-ledger-semantics-and-stable-read
    - T-182-define-exact-v0-6-machine-result-schemas
updated_at: "2026-08-04T23:06:23Z"
---

# T-191-add-stable-task-inspection-and-filtered-inventory Add stable task inspection and filtered inventory

## Description

Extend inherited read-only task show and add filtered inventory over source and
layout-3 ledgers, exposing storage and stable identity without guessed physical
paths or unsafe terminal controls.

## Acceptance

- Show emits exact Markdown/path to non-terminal output, requires raw terminal
  acceptance for unsafe controls, and emits safely escaped resolver/content JSON.
- V0.6 explicitly replaces the inherited show JSON payload with exact stable
  reference, canonical ID/logical path, storage, content, and digest fields.
- Show and unfiltered inspection operate read-only on layouts 1/2; layout-3-only
  storage/eligibility filters fail clearly before upgrade.
- List composes active-spec, storage, status, and archive-eligible filters using
  completed/cancelled eligibility owners over full ledger/global order.
- Inventory JSON uses exact non-null filter/task/blocker schemas, empty arrays,
  and deterministic eligibility blockers.
- Both commands use stable read snapshots, write no state, and prompts/skills
  consume resolver output rather than guessed paths.

## Verification Notes

- Map criteria to source/layout3, TTY/raw/path/JSON controls,
  eligibility/status/storage/filter tables, zero results, malformed refs, order,
  and archived active history.
- Snapshot inputs before/after and race readers against publication.

## Implementation Notes
