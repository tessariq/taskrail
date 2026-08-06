---
id: T-221-add-source-profile-and-receipt-inventories
title: Add source profile and receipt inventories
status: todo
priority: medium
spec_ref: specs/v0.7.0.md#source-inspect-and-import-commands
dependencies:
    - T-205-add-the-built-in-openspec-planning-profile
    - T-206-add-the-built-in-spec-kit-planning-profile
    - T-208-publish-strict-planning-provenance-sidecars
    - T-209-wire-reviewed-planning-source-import
updated_at: "2026-08-05T20:24:51Z"
---

# T-221-add-source-profile-and-receipt-inventories Add source profile and receipt inventories

## Description

Add read-only discovery for built-in planning-source profiles and immutable import
receipts, including prior-import detection before reviewers author new handoffs.

## Acceptance

- `source profile list/show` reports exact built-in versions, roles, path shapes,
  and limits in deterministic text and envelope-generation-3 JSON without reading
  or writing source repositories.
- `source receipt list/show` validates the complete receipt set, supports profile filtering, and reports canonical summaries or exact receipt objects in receipt-ID order.
- Profile role/layout/limit objects, receipt filters/summaries, matching receipts, nullability, and ordering use the spec's exact nested schemas with no open-ended maps.
- `source inspect` reports a nullable existing matching receipt for the exact
  profile/version/source-trust/source-digest tuple before draft/mapping authoring.
- Malformed/unexpected receipt entries fail closed even when filters would hide them. Inventory never repairs, normalizes, refreshes, deletes, or infers current task state.
- README/help/import/decomposition/SDD-handoff/retrofit guidance uses inventory in the complete inspect-to-import workflow.

## Verification Notes

- Map empty/multiple/filtered/malformed receipts and both profile registries to exact text/JSON goldens, deterministic ordering, common-envelope errors, and zero-write snapshots.
- Run first/duplicate/changed source inspection and prove matching receipt summaries remain historical across task rename, archive, and lifecycle changes.

## Implementation Notes
