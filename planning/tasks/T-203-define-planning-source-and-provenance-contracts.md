---
id: T-203-define-planning-source-and-provenance-contracts
title: Define planning-source and provenance contracts
status: todo
priority: high
spec_ref: specs/v0.7.0.md#planning-source-trust-boundary
dependencies:
    - T-199-run-the-v0-6-0-identity-and-archival-release-gate
updated_at: "2026-08-05T19:17:46Z"
---

# T-203-define-planning-source-and-provenance-contracts Define planning-source and provenance contracts

## Description

Implement the strict, profile-neutral contract foundation for digest-bound planning-source interoperability. The outcome is executable validation and canonical data contracts, not documentation alone: later snapshot, profile, mapping, receipt, and import tasks must share one definition of repository-local trust, exact JSON shapes, value boundaries, diagnostic envelopes, and compatibility limits. Inputs are treated as untrusted tracked files, provenance remains an immutable sidecar concept rather than task or state data, and v0.7 remains within layout 3, state schema 2, and `ImportDraft` v3.

## Acceptance

- A1. Shared path validation for source roots, drafts, mappings, selected specs, descriptor entries, and receipt paths accepts only canonical slash-separated UTF-8 repository-relative paths of at most 1024 bytes. It rejects empty, `.` or `..` components, absolute, drive-relative, UNC, URL, file-URI, non-portable, and case/Unicode-alias spellings.
- A2. The generic trust contract requires one non-bare Git worktree whose root equals the configured Taskrail root. Every consumable source, spec, draft, and mapping is a tracked stage-0 regular file inside that repository, reached without link, junction, mount-substitution, or reparse traversal; unsupported identity guarantees fail closed.
- A3. Shared cleanliness and text contracts distinguish and reject staged, unstaged, conflicted, missing, sparse, skip-worktree, assume-unchanged, intent-to-add, untracked, ignored-present, gitlink, nested-repository, special-file, hard-link-ambiguous, filter-indeterminate, BOM, NUL, invalid UTF-8, oversized, and racily changed inputs rather than choosing a convenient byte representation.
- A4. The profile-neutral descriptor contract has exactly `profile{name,version}`, `root`, `aggregate_sha256`, and non-null `sources`; each source has exactly `role`, `path`, `size`, and `sha256`. Names, versions, roles, paths, sizes, digest spelling, duplicate rejection, nullability, and deterministic ordering follow v0.7's stated boundaries.
- A5. Mapping schema v1 is represented and strictly decoded with exactly the specified review session, profile, source, target spec, draft digest, review assertion, and non-null item/source/spec-ref/task-key fields. Unknown, duplicate, missing, trailing, null, out-of-range, non-canonical timestamp, invalid enum, and invalid digest data is rejected without coercion. Cross-file semantic linkage is left to the reviewed-import task, but this contract cannot admit an unrepresentable or widened schema.
- A6. Receipt schema v1 is represented with exactly the specified receipt ID, profile, complete source descriptor entries, target spec, `ImportDraft` v3 identity, mapping identity, and resulting-task records. Provenance is modeled only as an immutable sidecar; no receipt field is added to task frontmatter, task bodies, `STATE.md`, archive metadata, or verification data.
- A7. Source command machine envelopes use `schema_version:1`, exact `command`, non-null `warnings`, and exactly one of `result` or `error`. Warning, error, and violation objects have the exact fields and nullability from v0.7; stable error codes, unsigned-byte path sorting, phase-stable violation order, one-document JSON output, and matching human/JSON exit classification are contractually enforced.
- A8. Inspect result contracts expose exactly the profile, root, aggregate digest, and ordered sources. Import result contracts expose exactly profile, root, aggregate digest, selected spec, draft, mapping, `applied`, receipt ID/path, and ordered predicted/resulting tasks. Additional internal or provenance claims cannot leak into either public shape.
- A9. Validation limits are explicit and reusable: source files are at most 2 MiB, a source set at most 256 files and 16 MiB, spec/draft/mapping inputs at most 4 MiB, mapping items at most 4096, portable IDs/keys at most 128 bytes, reviewer text at most 256 bytes, and rationale at most 2048 bytes, with limits measured at the specified byte boundary.
- A10. The implementation introduces no layout 4, state schema 3, `ImportDraft` v4, profile plugin/configuration surface, model/provider behavior, source execution, or mutable synchronization/provenance field. Existing generic import support remains unchanged; later `source import` work is constrained to `ImportDraft` v3.

## Verification Notes

- A1-A3: Unit and repository-boundary integration matrices should cover every accepted path form and each Git/filesystem refusal class, including unsupported no-follow checks. Assert deterministic error classification and no repository writes on failure.
- A4-A6: Strict decoder/encoder golden tests should exercise exact valid descriptor, mapping, and receipt objects plus unknown/duplicate/missing fields, nulls, trailing JSON, malformed integers, invalid enums/timestamps/digests, duplicate identities, and boundary lengths. Round trips must preserve the canonical public shape without silently retaining unknown data.
- A7-A8: Public contract and handler tests, independent of later root-command wiring, should compare text and one-document JSON classification for representative success and every stable top-level error code, asserting exact field sets, empty arrays as `[]`, nullable paths where specified, deterministic ordering, and mutual exclusivity of `result` and `error`.
- A9: Boundary-value tests should cover each limit at, below, and above the threshold before decoding, including aggregate count/byte overflow and multi-byte UTF-8 byte lengths.
- A10: Integration checks should prove existing layout/state markers and generic import behavior are unchanged and that v1, v2, and v4 are not accepted by the planning-source import contract. Repository validation before and after the contract implementation must show no migration or new persisted field.
- Manual evidence should use a temporary Git worktree to demonstrate a clean repo-local input is admitted to the contract layer while an external path, dirty tracked input, and symlinked input fail closed. Record commands, expected observations, actual observations, and cleanup.

## Implementation Notes
