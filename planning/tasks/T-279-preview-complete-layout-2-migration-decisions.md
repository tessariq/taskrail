---
id: T-279-preview-complete-layout-2-migration-decisions
title: Preview complete layout 2 migration decisions
status: todo
priority: high
spec_ref: specs/v0.5.0.md#layout-compatibility-and-upgrade
dependencies:
    - T-157-upgrade-repositories-transactionally-to-layout-2
updated_at: "2026-08-08T14:23:08Z"
---

# T-279-preview-complete-layout-2-migration-decisions Preview complete layout 2 migration decisions

## Description

Expose a complete, read-only layout-2 migration preview that resolves every
operator decision before apply, including continuation-note preservation, skill
refresh eligibility, compatibility blockers, and quiescence requirements.

## Acceptance

- A1. Upgrade preview reports the complete validated candidate paths, logical and
  physical roots, committed storage mode, default broad review-round maximum `1`,
  source/target versions, skill classifications, and validation outcome without
  changing repository bytes.
- A2. Every decoded legacy continuation note is reported in order with exact
  applicable extract/drop choices; empty notes, existing NOTES, and direct
  schema-2 sources expose only valid decisions.
- A3. Preview classifies parity mirrors for preservation, refreshable installed
  skills for combined forced refresh, and divergent/conflicting copies as blockers
  before any migration can apply.
- A4. Exact legacy-policy-path entries, unsafe aliases, invalid candidates, and
  inapplicable note/quiescence flags produce actionable refusals while unrelated
  same-basename files remain irrelevant.
- A5. Preview and apply inputs identify the same candidate path set and all
  required operator gates, including explicit quiescence for an actual upgrade.

## Verification Notes

- A1: run preview over representative layout-1 repositories and compare complete
  machine/text observations, including review maximum `1`, plus unchanged
  filesystem snapshots.
- A2: exercise absent, empty, single, multiple, multiline, quoted notes, existing
  NOTES, extraction, drop, and direct-schema-2 decision matrices.
- A3: exercise parity-mirror, stamped, legacy-only, matching-dual, divergent, and
  conflicting skill copies with and without required combined flags.
- A4: test invalid candidates, exact legacy path entry types/aliases, decoys, and
  each inapplicable flag for focused guidance and no writes.
- A5: compare preview candidate paths/choices with an apply-plan observation and
  verify quiescence is required only for a real upgrade.

## Implementation Notes
