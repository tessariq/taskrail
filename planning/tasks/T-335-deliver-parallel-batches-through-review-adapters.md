---
id: T-335-deliver-parallel-batches-through-review-adapters
title: Deliver parallel batches through review adapters
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#parallel-isolated-clone-batches
dependencies:
    - T-334-deliver-parallel-clone-batches-locally
    - T-361-make-parallel-local-delivery-portable-in-clean-environments
updated_at: "2026-08-24T11:46:06Z"
completion_id: "0cd8f1c359fdaa019e060ee6ca7ba6ee"
last_verification_id: "a440f37a9fbcf8fe9e301285e24fe6bb"
last_verification_result: pass
last_verified_at: "2026-08-24T11:46:06Z"
last_verified_completion_id: "0cd8f1c359fdaa019e060ee6ca7ba6ee"
last_verification_previous_id: "6c005d39a2d895aa20fc32d7bb9a66b1"
---

# T-335-deliver-parallel-batches-through-review-adapters Deliver parallel batches through review adapters

## Description

Extend the proven parallel clone batch with provider-neutral hosted review
delivery. Publish passing workers concurrently as caller-owned change requests,
then refresh and merge them serially through a strict external adapter without
embedding GitHub, GitLab, credentials, or provider-specific state in Taskrail.

## Acceptance

- `--delivery review` requires one explicit resolvable `--review-adapter` and
  otherwise reuses T-334's frozen frontier, worker, candidate, partial-success,
  containment, and no-refill behavior. Local delivery rejects adapter intent and
  remains the default.
- Taskrail invokes the adapter directly as an argv-safe executable with one exact
  finite-stdin `ReviewAdapterRequest` and one strict finite-stdout
  `ReviewAdapterResult` for `publish_branch`, `open_change`, `inspect_change`,
  `update_change`, and `merge_change`. Stderr streams diagnostically; malformed,
  mismatched, extra, stale, or non-zero responses are `adapter_failed` and never
  become inferred remote success.
- Requests/results use only the provider-neutral fields in the v0.5 machine
  companion. Credentials and provider behavior remain inherited caller-owned
  adapter concerns; Taskrail contains no named provider, API client, token,
  session, URL convention, or provider-specific status/approval field.
- Passing worker branches and change requests may be published concurrently, but
  merge consideration follows deterministic frontier order. After each accepted
  merge, each remaining request receives one bounded integration-child refresh
  against the exact new target, mechanical Taskrail state reprojection, affected
  checks, and an update of that same change before it may merge.
- Failed checks/review, adapter refusal, target drift, or unresolved one-attempt
  semantic integration leaves that change open and unpublished, launches no
  replacement task, and does not prevent later dependency-independent valid
  changes from consideration. Successful siblings merge and the invocation
  reports exact non-zero partial outcome.
- Review delivery is foreground, timeout-bounded, result-file reported, and
  explicitly authorized. It never force-pushes, bypasses hosted checks, invents
  approval, retries adapter or agent operations, or claims a merge/ref/check not
  returned by the adapter. Documentation includes `gh`/`glab` only as external
  adapter examples, never built-in integrations.

## Verification Notes

- Deterministic fake adapters cover every operation and exact request/result
  field, finite stdin/EOF, stdout/stderr separation, malformed/duplicate/stale
  responses, non-zero exits, timeout, target movement, failed/pending/unknown
  checks, merge identity, credentials non-disclosure, and no shell evaluation.
- Hosted-flow fixtures publish multiple passing branches, vary worker and adapter
  completion order, merge serially, refresh remaining changes, repair state,
  resolve one conflict, leave another open, and prove successful independent
  requests still merge under a partial non-zero result.
- Manual adapter evaluation uses caller-owned sandbox repositories for one GitHub
  PR and one GitLab MR when credentials are explicitly available; unavailable
  providers are recorded as missing manual arms and never replace credential-free
  fake-adapter CI coverage.

## Implementation Notes

- 2026-08-24T11:32:32Z: Delivered strict provider-neutral review-adapter parallel delivery with ordered merges and refreshes.
- 2026-08-24T11:32:45Z: verification pass id 6c005d39a2d895aa20fc32d7bb9a66b1 previous none completion 0cd8f1c359fdaa019e060ee6ca7ba6ee
- 2026-08-24T11:46:06Z: verification pass id a440f37a9fbcf8fe9e301285e24fe6bb previous 6c005d39a2d895aa20fc32d7bb9a66b1 completion 0cd8f1c359fdaa019e060ee6ca7ba6ee
