---
id: T-215-add-the-generic-review-artifact-publisher
title: Add the shared review directory publisher
status: todo
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-223-run-every-v0-5-command-against-local-storage
    - T-240-implement-the-normative-review-schema-decoders
    - T-278-publish-typed-directories-without-clobbering
updated_at: "2026-08-08T08:40:49Z"
---

# T-215-add-the-generic-review-artifact-publisher Add the shared review directory publisher

## Description

Provide one reusable no-follow, no-clobber directory-publication boundary and
prove it through the task-review bundle adapter. Other bundle types and review
reading remain separate outcomes.

## Acceptance

- A1. A registered directory adapter supplies its exact proposal inventory,
  consumed snapshots, validation, and final files to one dry-run/apply pipeline;
  the pipeline rechecks them under the writer lock before one absent-directory
  commit.
- A2. The shared boundary enforces transient proposal containment, 1 MiB file
  caps, no-follow/no-alias paths, absent destinations, exact-byte copying, stable
  envelopes, and all-or-none visibility without editing reviewed subjects.
- A3. The task adapter publishes exactly one strict `review.json` to the bound
  task/session destination and rechecks task/spec bytes, IDs, digests, and path
  identities. Prompt-resolution binding is injected later by T-298.
- A4. The capability cannot mutate lifecycle, loop policy, dependencies, specs,
  imports, or verification, and admits later spec/decomposition adapters without
  embedding their manifest semantics.

## Verification Notes

- A1/A2: adapter-contract fixtures compare preview/apply candidates and inject
  snapshot races, aliases, destination substitution, oversized files, and commit
  faults; the durable destination is complete or absent.
- A3/A4: task-bundle positive/mutation fixtures assert exact published bytes and
  repository-wide sentinels prove every excluded semantic surface is unchanged.

## Implementation Notes
