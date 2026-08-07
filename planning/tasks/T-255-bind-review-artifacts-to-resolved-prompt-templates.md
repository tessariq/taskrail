---
id: T-255-bind-review-artifacts-to-resolved-prompt-templates
title: Bind review artifacts to resolved prompt templates
status: todo
priority: high
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-215-add-the-generic-review-artifact-publisher
    - T-240-implement-the-normative-review-schema-decoders
    - T-250-render-prompts-from-storage-neutral-context
updated_at: "2026-08-07T20:40:30Z"
---

# T-255-bind-review-artifacts-to-resolved-prompt-templates Bind review artifacts to resolved prompt templates

## Description

Bind every prompt-produced v0.5 review observation to the exact role-mandated
prompt template resolution current at publication, without claiming that Taskrail
delivered the prompt to or certified the external reviewer.

## Acceptance

- A1. Task, spec-lens, decomposition-pass, and workflow-report schemas carry exact
  prompt ID, v1 contract, lower-case template SHA-256, and `builtin|replacement`
  source fields; manifests, draft/trace files, and workflow memory do not duplicate them.
- A2. Each publisher infers the required prompt ID from artifact role, resolves
  that explicit contract through committed or local active storage, and rejects a
  malformed, wrong-role, stale-source, stale-template, or invalid replacement
  before exposing a logical destination.
- A3. Preview and apply snapshot and recheck configuration plus exact prompt bytes
  alongside proposal and subject inputs. Built-in and equal-byte replacements
  remain distinguishable by source; no physical local path becomes durable.
- A4. Prompt drift requires a new external review observation rather than a
  metadata-only repair. Later prompt changes do not rewrite or invalidate an
  already published historical artifact or its downstream import/read behavior.
- A5. The binding is described only as publication-time template resolution. It
  does not attest prompt delivery/use, reviewer identity or independence,
  provider/model identity, replacement safety, or semantic quality.

## Verification Notes

- A1-A2: run a complete artifact-role matrix with built-ins, committed/local
  replacements, wrong IDs/contracts/sources/digests, invalid files, and exact
  no-publication repository snapshots.
- A3-A4: mutate prompt bytes and source class between preview/apply and during
  apply, then read/import previously published evidence after later prompt changes.
- A5: assert text/JSON/help/docs contain no execution, identity, independence, or
  certification claim and run the shared schema/error/transaction registries.

## Implementation Notes
