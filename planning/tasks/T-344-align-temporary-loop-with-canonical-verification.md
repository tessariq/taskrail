---
id: T-344-align-temporary-loop-with-canonical-verification
title: Align temporary loop with canonical verification evidence
status: completed
priority: high
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies:
    - T-342-bind-verification-identities-across-temporary-loop
updated_at: "2026-08-21T11:08:22Z"
completion_id: "9ab99cd3dca0a17d45972ff9258d2ac9"
---

# T-344-align-temporary-loop-with-canonical-verification Align temporary loop with canonical verification evidence

## Description

Align the temporary loop's report/state binding with the v0.5 canonical summary
syntax and make its child contract explicit that the selected tracked task must
publish exactly one terminal verification report. Repeated-verification behavior
belongs in focused tests or an isolated sandbox, never the tracked lifecycle.

Follow-up derived from T-342-bind-verification-identities-across-temporary-loop's verification or discovery.

## Acceptance

- Identity-aware sequential, parallel, and recovery paths require the canonical
  `<result> for <task> at <time> id <verification-id>` state summary.
- Legacy identity-free reports retain their existing summary compatibility.
- The shared child prompt requires exactly one selected-task verification and
  directs repeated-verification acceptance testing to tests or a sandbox.
- The loop harness proves canonical identity delivery, mismatch refusal, and the
  rendered exact-one evidence instruction without weakening multiple-report
  rejection.

## Verification Notes

- Run the complete autonomous-loop harness, queue validation, repository
  validation, vet, and the full Go suite.

## Implementation Notes

- 2026-08-21T11:08:22Z: verification pass
