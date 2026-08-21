---
id: T-294-show-durable-review-artifacts-through-active
title: Show durable review artifacts through active storage
status: completed
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-223-run-every-v0-5-command-against-local-storage
updated_at: "2026-08-21T14:38:20Z"
completion_id: "ff0734d6a8dde14a516479923daefca1"
last_verification_id: "b9b1230eba7c020f50c15091c7450b6c"
last_verification_result: pass
last_verified_at: "2026-08-21T14:38:20Z"
last_verified_completion_id: "ff0734d6a8dde14a516479923daefca1"
---

# T-294-show-durable-review-artifacts-through-active Show durable review artifacts through active storage

## Description

Ship read-only `review show` so callers can retrieve one durable logical review
file through the active committed or local storage context.

## Acceptance

- A1. `review show <logical-review-path>` resolves only canonical regular files
  strictly beneath the configured durable review roots and refuses proposal,
  artifact, runtime, source, state, task, spec, prompt, alias, and traversal paths.
- A2. Text emits exact stored bytes; JSON returns exactly logical `path`, `content`,
  and lower-case raw-byte SHA-256 without exposing a local physical overlay prefix.
- A3. Missing review evidence uses `review_not_found`; incompatible layout,
  recovery fences, malformed paths, and invalid entries retain stable machine and
  text exit classification.
- A4. Reads are historical: they do not re-resolve current prompts or revalidate
  old content against changed active roots, and never write or bootstrap storage.

## Verification Notes

- A1-A3: committed/local/custom-directory fixtures cover exact bytes and JSON plus
  every forbidden root, missing file, alias, special entry, and recovery fence.
- A4: repository snapshots and changed-prompt/root fixtures prove read-only
  historical retrieval without physical-path disclosure.

## Implementation Notes

- 2026-08-21T14:38:08Z: Added read-only durable review retrieval with storage-safe exact bytes and machine errors.
- 2026-08-21T14:38:20Z: verification pass id b9b1230eba7c020f50c15091c7450b6c previous none completion ff0734d6a8dde14a516479923daefca1
