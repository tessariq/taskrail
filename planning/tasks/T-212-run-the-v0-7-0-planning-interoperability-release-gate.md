---
id: T-212-run-the-v0-7-0-planning-interoperability-release-gate
title: Run the v0.7.0 planning interoperability release gate
status: todo
priority: high
spec_ref: specs/v0.7.0.md#goals
dependencies:
    - T-211-verify-v0-7-planning-source-interoperability
updated_at: "2026-08-05T19:18:31Z"
---

# T-212-run-the-v0-7-0-planning-interoperability-release-gate Run the v0.7.0 planning interoperability release gate

## Description

Run the final v0.7.0 semantic-gap, drift, exclusion, compatibility, and
release-readiness gate against fresh candidate bytes after all implementation
and remediation. Produce an auditable release decision for digest-bound
planning-source interoperability without creating a tag or publishing a
release.

## Acceptance

- Every v0.7.0 goal, feature requirement, caution, LLM recommendation, explicit
  exclusion, and behavioral/release acceptance item is classified against
  implementation, tests, docs/help/skills/prompts, release notes, and concrete
  evidence; no unreviewed semantic or structural coverage gap remains.
- Final drift review proves exact narrow profile v1 layouts, clean tracked-byte
  trust boundaries, reviewed local-anchor mapping, existing `ImportDraft` v3,
  preview-first atomic publication, canonical immutable historical receipts,
  duplicate tuple refusal, no task/receipt updates, and layout 3/state schema 2
  compatibility remain aligned across code and documentation.
- Adversarial review finds no inference, external `spec_ref`, source/input
  write, hidden executable integration, provider coupling, synchronization,
  universal OpenSpec/Spec Kit support claim, receipt repair/live binding, silent
  unknown-layout omission, partial publication, or old-task mutation path.
- `taskrail coverage --min 100`, `taskrail coverage --gaps`, targeted semantic
  review, task/dependency review, and specification drift checks are fully
  disposed. Any blocker creates a
  standalone remediation task that is a direct dependency of this gate before
  the gate's final transition; remediation is implemented and all affected
  evidence is rerun rather than recorded as post-release follow-up.
- Fresh formatting, vet, full tests, race tests, native Windows/macOS/Linux
  runs, cross-builds, skill parity, task-body hygiene, Taskrail validation,
  packaged-binary smoke checks, v0.6 compatibility, remote CI, and the standard
  release checklist all pass for the same candidate commit and recorded binary
  digests/toolchains.
- Final README, help, workflow/provenance/upgrade guidance, packaged skills and
  prompts, and `CHANGELOG.md` accurately describe profile limits, reviewed
  handoff, preview/apply behavior, duplicate and changed-snapshot semantics,
  immutable receipts, rollback/recovery, compatibility limits, and no-sync
  boundary with no stale or contradictory claims.
- The final verification result has no open remediation or recovery state and
  binds the reviewed candidate commit, source tree status, artifacts, CI/native
  runs, commands, and results. This task stops at a release-ready decision:
  tagging, pushing, GoReleaser, and publication remain maintainer-owned and are
  not performed.

## Verification Notes

- Map the complete spec to a digest-bound release matrix under the T-212 manual
  evidence directory, with one row per goal/requirement/caution/recommendation/
  exclusion and links to exact code, tests, docs, reports, or an explicit
  non-applicability rationale.
- Map drift and adversarial criteria to final source/docs/schema/help scans,
  candidate receipt/profile/mapping golden digests, forbidden-behavior searches,
  and a clean-tree diff review tied to the candidate commit SHA.
- Map gap disposition and remediation criteria to `coverage --gaps` output,
  semantic review notes, dependency-graph output, and a sandboxed blocker drill
  proving direct non-cyclic gate dependencies and mandatory evidence restart.
- Map release checks to captured commands/results for formatting, vet, full and
  race tests, all native platforms, cross-builds, parity, body hygiene,
  validation, packaged/current and v0.6 binary smoke tests, CI run URLs, and each
  item in `docs/workflow/releasing.md`.
- Store the final plan, semantic matrix, evidence index, and no-tag release
  decision in `planning/artifacts/manual-test/T-212/<timestamp>/`; record the
  candidate commit, binary SHA-256 values, platform/toolchain versions, clean
  status, and confirmation that no tag, push, or publication occurred.

## Implementation Notes
