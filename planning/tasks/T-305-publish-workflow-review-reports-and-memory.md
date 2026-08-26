---
id: T-305-publish-workflow-review-reports-and-memory
title: Publish workflow review reports and memory atomically
status: completed
priority: high
spec_ref: specs/v0.5.0.md#workflow-adversarial-review-memory
dependencies:
    - T-166-publish-workflow-review-index-and-reports-with-cas
    - T-215-add-the-generic-review-artifact-publisher
    - T-232-recover-v0-5-transactions-through-one-command
    - T-255-bind-review-artifacts-to-resolved-prompt-templates
updated_at: "2026-08-26T10:48:21Z"
completion_id: "7262576551125f8bd64a5d0d4e122d0e"
last_verification_id: "619b5a4f91bcad6499c883698bc623a0"
last_verification_result: pass
last_verified_at: "2026-08-26T10:48:21Z"
last_verified_completion_id: "7262576551125f8bd64a5d0d4e122d0e"
---

# T-305-publish-workflow-review-reports-and-memory Publish workflow review reports and memory atomically

## Description

Publish one validated immutable workflow report and Taskrail-derived canonical
memory as one durable, compare-and-swap protected logical outcome.

## Acceptance

- A1. `review publish --type workflow` accepts only its declared report, memory,
  spec, HEAD, product, and expected-memory flags; first-run absence and existing
  memory are distinguished exactly.
- A2. Preview/apply validate and recheck report schema, role-mandated prompt
  resolution, selected spec, clean full HEAD/product tree, prior index snapshot,
  global review ID, destination absence, paths, caps, and T-166's candidate digest.
- A3. Under the shared lock, publication durably fences exact original memory,
  report bytes, and derived index bytes, creates the report no-clobber, CAS-replaces
  memory, and clears the fence; it introduces no second review lock.
- A4. Readers never logically observe one output. Interruption uses shared recovery
  to accept or restore the pair, while conflicts, aliases, and concurrent IDs lose
  no prior rows and overwrite no unexpected bytes.
- A5. The write set is only the final report and `INDEX.json`; task, state, spec,
  prompt, lifecycle, loop policy, and findings promotion are excluded. Human
  commit/discard remains the handoff before another clean run.

## Verification Notes

- A1/A2: first/subsequent-run and mutation fixtures cover flags, snapshots,
  prompt/config drift, derived digest, caps, paths, IDs, and stale memory.
- A3/A4: concurrent publishers, CAS races, aliases, and fault injection at every
  fence/report/index phase prove exactly one visible pair or recoverable fence.
- A5: repository sentinels and allowed-diff assertions prove the narrow write set.

## Implementation Notes

- 2026-08-26T10:48:09Z: Published workflow review report/index pairs through durable fenced recovery, with snapshot and recovery tests.
- 2026-08-26T10:48:21Z: verification pass id 619b5a4f91bcad6499c883698bc623a0 previous none completion 7262576551125f8bd64a5d0d4e122d0e
