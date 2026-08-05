---
id: T-227-draft-reviewed-spec-changes-from-configured-ideas
title: Draft reviewed spec changes from configured IDEAS
status: todo
priority: high
spec_ref: specs/v0.6.0.md#human-owned-ideas-inbox
dependencies:
    - T-226-bootstrap-a-configurable-human-owned-ideas-sidecar
    - T-163-validate-and-apply-importdraft-v2-transactionally
    - T-185-upgrade-repositories-transactionally-to-layout-3
updated_at: "2026-08-05T22:04:45Z"
---

# T-227-draft-reviewed-spec-changes-from-configured-ideas Draft reviewed spec changes from configured IDEAS

## Description

Resolve the configured free-form IDEAS source through `taskrail import ideas` and
route semantic proposals through existing reviewed import boundaries. Specs are
the default adoption target; direct task drafting remains explicit and fully
spec-anchored.

## Acceptance

- The reserved `ideas` source alias resolves exact configured path, reports a
  missing source without creating it, defaults to `--to spec`, and accepts task
  output only with explicit `--to tasks`.
- Preview/emitted prompt report canonical source path and SHA-256; ideas apply
  requires exact `--expect-source-sha256`, rejects the flag elsewhere, and refuses
  missing/malformed/mismatched source binding before semantic writes without
  changing retained ImportDraft schemas.
- Headings, lists, nested detail, and prose remain source context rather than a
  mechanically asserted item grammar; emitted prompts are provider-neutral and
  apply nothing automatically.
- Reviewed spec/task drafts retain existing exact schema, digest, anchor, body,
  dependency, review, transaction, and implicit-hold requirements; source bytes
  and digest are rechecked immediately before apply.
- Apply never edits IDEAS and reports that successful adoption still requires a
  separate reviewed human/agent source update. Failure or conflict leaves source
  and semantic destinations unchanged or recoverably transactional.
- Other import source/target behavior and explicit-path compatibility remain
  unchanged.

## Verification Notes

- Use free-form category/list/prose fixtures, default/custom/local paths, missing
  and racing sources, valid/missing/malformed/mismatched expected digests,
  spec-default output, explicit task output, invalid drafts, apply rollback, and
  unchanged-source byte assertions.
- Golden-test emitted prompt and common text/JSON results without invoking a model.

## Implementation Notes
