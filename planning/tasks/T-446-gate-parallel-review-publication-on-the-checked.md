---
id: T-446-gate-parallel-review-publication-on-the-checked
title: Gate parallel review publication on the checked combined head
status: completed
priority: high
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies:
    - T-444-require-loop-check-for-parallel-candidates
updated_at: "2026-11-01T15:35:51Z"
completion_id: "be65dbbf5fd4ce65106584c2bb8017d6"
last_verification_id: "a565ec0b50bcde119971b3de657b5827"
last_verification_result: pass
last_verified_at: "2026-11-01T15:35:51Z"
last_verified_completion_id: "be65dbbf5fd4ce65106584c2bb8017d6"
---

# T-446-gate-parallel-review-publication-on-the-checked Gate parallel review publication on the checked combined head

## Description

Restore parallel review delivery only when the caller-owned adapter can cooperate
with pre-publication verification of the exact combined integration head.
The current protocol merges each change before aggregate validation, so T-444
refuses checked parallel review delivery rather than claiming safe publication
or remote rollback. Deliver one provider-neutral protocol and coordinator flow
that enforces the same bound-head safety contract before remote merge.

## Acceptance

- Define and obtain approval for the bounded adapter cooperation and evidence
  contract needed to verify the exact combined head before publication; do not
  infer hosted provider APIs, credentials, or remote rollback.
- Run candidate checks and the aggregate child followed by the caller-owned
  check on the same unchanged bound combined head before any unsafe remote merge.
  A failing final check leaves remote publication unapplied, with explicit
  refusal and retained diagnostic evidence.
- Re-observe adapter target/change identities immediately before publication;
  target, candidate, check, or approval drift refuses without retry, force push,
  silent refresh, or mutation of unrelated changes.
- Record explicit adapter-reported remote merge identity and applied/refused
  state separately from locally observed commits and checks. Do not represent
  unknown remote outcomes as unapplied or imply transactional remote rollback.
- Keep the staged review refusal until the supported path proves these gates;
  unsupported/non-cooperating adapters fail closed before unsafe publication.

## Verification Notes

- Use disposable repositories and deterministic local adapter executables to
  prove positive publication, final-check failure with zero unsafe merge calls,
  target/candidate/approval drift, unknown adapter outcomes, and retained binding
  evidence. No provider/model evaluation or production remote writes are needed.
- Assert exact source, combined integration, checked-head and adapter-reported
  merge identities, persisted task lifecycle, refusal ordering and workspace
  retention through the public CLI/API. Run applicable machine-contract,
  provider-neutral adapter, CLI and native-platform checks.

## Implementation Notes

- 2026-11-01T15:35:39Z: Added opt-in checked-combined-v1 schema-2 review delivery: locally check separate task integration commits and aggregate/final H before conditional B-to-H publication. Default and unsupported adapters remain refused; receipts preserve applied/refused/unknown effects and late output/recovery failures. Independent review findings resolved; deterministic public-CLI QA and full tests passed.
- 2026-11-01T15:35:51Z: verification pass id a565ec0b50bcde119971b3de657b5827 previous none completion be65dbbf5fd4ce65106584c2bb8017d6
