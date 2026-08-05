---
id: T-211-verify-v0-7-planning-source-interoperability
title: Verify v0.7 planning-source interoperability
status: todo
priority: high
spec_ref: specs/v0.7.0.md#behavioral-and-release-acceptance
dependencies:
    - T-205-add-the-built-in-openspec-planning-profile
    - T-206-add-the-built-in-spec-kit-planning-profile
    - T-208-publish-strict-planning-provenance-sidecars
    - T-209-wire-reviewed-planning-source-import
    - T-210-integrate-planning-source-workflow-guidance
updated_at: "2026-08-05T19:18:27Z"
---

# T-211-verify-v0-7-planning-source-interoperability Verify v0.7 planning-source interoperability

## Description

Complete integrated automated, adversarial, native-platform, packaged-binary,
and manual verification proving that v0.7 planning-source interoperability is
deterministic, reviewed, append-only, atomic, and backward-compatible. Verify
the complete workflow across both profiles and the repository trust boundary,
not only command registration or isolated happy paths.

## Acceptance

- Automated profile matrices cover every required and optional OpenSpec v1 and
  Spec Kit v1 role, exact unknown-layout refusals, deterministic sorting and
  aggregate golden vectors, raw/CRLF bytes, root/path/role changes, aliases,
  empty optional directories, and all file-count/byte limits.
- Git/path trust-boundary matrices cover staged, unstaged, untracked,
  ignored-present, sparse, conflicted, skip-worktree, assume-unchanged, missing,
  racily changed, filtered, symlink, junction/reparse, hard-link ambiguity,
  gitlink, nested repository, special file, case/Unicode alias, worktree/root,
  and input-race refusals on Linux, macOS, and Windows.
- Strict mapping and input matrices cover UTF-8/BOM/NUL/size failures, exact
  JSON schemas, digests and review fields, complete source coverage and ranges,
  `task`/`no-task`, all draft keys, live local anchors, mechanically valid but
  semantically questionable reviewer-owned mappings, and every forbidden
  external or wrong-spec reference.
- Source import matrices prove existing `ImportDraft` v3 generated/opaque
  identities, complete-ledger allocation and dependencies, complete bodies,
  target/spec-section constraints, and explicit v1/v2/v4 rejection without a
  new schema, update behavior, or semantic inference.
- Inspect, preview, and apply verification covers exact text/JSON schemas and
  errors, read-only purity, prediction/reallocation, first import,
  same-snapshot duplicate refusal, changed-snapshot fresh import, collisions,
  canonical receipts, malformed receipt refusal, and proof that no path updates
  an old task or receipt.
- Failure injection covers every task/state/directory/receipt publication,
  backup, fsync, post-validation, compare-and-swap rollback, retained-recovery,
  and retry boundary, proving all-or-none recovery and no writes to source,
  spec, draft, mapping, existing tasks, or unrelated repository files.
- Receipt sentinels prove every existing v0.7 writer and `repair --apply`
  preserves bytes and paths across task rename/re-slug/status/archive behavior.
  An actual packaged v0.6 binary reads and transitions imported tasks without
  requiring or modifying provenance sidecars; its inability to validate
  receipts or perform source import is documented and verified.
- Manual sandbox evidence completes one OpenSpec and one Spec Kit
  inspect/preview/apply handoff, duplicate refusal, changed-snapshot append-only
  import, rollback diagnostics, receipt immutability after rename/archive, and
  real v0.6 compatibility on reproducible candidate bytes.

## Verification Notes

- Map profile and trust-boundary criteria to focused unit/integration matrices,
  digest golden files, Git fixture manifests, race-test output, and native
  Windows/macOS/Linux run URLs recording commit, binary digest, Git/toolchain,
  filesystem capability, command, and result.
- Map mapping/v3 criteria to strict decoder, source-coverage, local-anchor,
  review-binding, identity/allocation, dependency, body, and version-rejection
  test reports under `internal/taskrail/`; include mutation cases proving
  external refs and semantic inference cannot slip through.
- Map command, receipt, duplicate, and no-update criteria to exact output
  goldens plus integrated repository tests with before/after byte manifests and
  task/receipt IDs for first, duplicate, changed, malformed, and collision runs.
- Map atomicity criteria to failure-injection and race reports enumerating every
  publication/fsync/rollback boundary and retained recovery artifact, with
  assertions over the complete allowed and forbidden write sets.
- Map compatibility criteria to the writer receipt-sentinel registry and a
  packaged-v0.6 sandbox report that records the old binary checksum, imported
  task transitions, unchanged receipt checksum, and expected lack of source
  commands.
- Store the integrated manual plan and report at
  `planning/artifacts/manual-test/T-211/<timestamp>/plan.md` and
  `planning/artifacts/manual-test/T-211/<timestamp>/report.md`; link platform,
  CI, golden, race, and compatibility evidence from the report without
  committing artifact contents.

## Implementation Notes
