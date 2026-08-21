---
id: T-285-add-stable-verification-identities-and-predecessor
title: Add stable verification identities and predecessor chains
status: completed
priority: high
spec_ref: specs/v0.5.0.md#canonical-transition-order
dependencies:
    - T-158-bind-completion-and-verification-with-stable
    - T-281-protect-verification-and-follow-up-writers
updated_at: "2026-08-21T11:52:58Z"
completion_id: "937cb70bdee202db455d9c66911a8c21"
last_verification_id: "f822a6d6adb63e3c0172957850932dfa"
last_verification_result: pass
last_verified_at: "2026-08-21T11:52:58Z"
---

# T-285-add-stable-verification-identities-and-predecessor Add stable verification identities and predecessor chains

## Description

Give every verification a stable identity and exact predecessor link, then publish
that identity consistently across task/state metadata, command output, report,
portable note, summary, and artifact directory. Completion binding and legacy
adoption remain with T-286.

## Acceptance

- Every verify creates a preflight-absent random lower-case 32-hex
  `verification_id`; `previous_verification_id` is null without task-level history
  and otherwise equals the exact preflight latest verification ID.
- Verify replaces the task and repository latest verification tuples and writes
  exact matching IDs/result/predecessor across JSON, `report.json`, canonical state
  summary, task note, and `<timestamp>-<verification-id>` artifact directory.
- A non-null predecessor is valid only when it identifies the exact prior
  task-level verification and report; timestamp ordering cannot substitute for a
  missing or broken chain.
- Freshness is ID/set based: the new ID was absent from the preflight artifact and
  summary snapshot and all published surfaces agree after success.
- Verification-created follow-ups are named by the fresh report and omit
  `loop_policy`/`loop_reason`, leaving them implicitly held.

## Verification Notes

- Inject deterministic IDs and frozen/equal/reversed clocks across first, repeated,
  pass/fail, and completed-audit verification chains; compare exact goldens.
- Corrupt or remove each predecessor report/surface in turn and assert validation
  rejects the chain without inferring chronology.
- Snapshot artifact trees, task/state bytes, notes, JSON, and follow-up metadata;
  inject transaction faults to prove one consistent publication.

## Implementation Notes

- 2026-08-21T11:52:41Z: Published stable verification identities and direct predecessor validation.
- 2026-08-21T11:52:58Z: verification pass id f822a6d6adb63e3c0172957850932dfa previous none completion none
