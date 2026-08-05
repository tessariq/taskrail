---
id: T-204-snapshot-planning-sources-with-digest-boundaries
title: Snapshot planning sources with digest boundaries
status: todo
priority: high
spec_ref: specs/v0.7.0.md#deterministic-source-descriptor
dependencies:
    - T-203-define-planning-source-and-provenance-contracts
updated_at: "2026-08-05T19:17:55Z"
---

# T-204-snapshot-planning-sources-with-digest-boundaries Snapshot planning sources with digest boundaries

## Description

Implement profile-neutral planning-source snapshotting that accounts for the complete selected root without following filesystem links and binds accepted files to exact Git-clean bytes. Given a profile's role assignments, the snapshot produces one deterministic descriptor over role, canonical repository path, exact size, and SHA-256. No successful result can call a source clean if entries were hidden, changed during inspection, exceeded bounds, or could not be checked consistently on the host platform.

## Acceptance

- A1. Snapshot enumeration accounts for every entry beneath `--root` in both the stage-0 index and worktree, ignores only empty directories, follows no symlink, junction, mount substitution, or reparse point, and fails when no-follow identity checks are unsupported.
- A2. Every accepted source is a tracked stage-0 regular file whose `HEAD`, index, and worktree bytes plus executable mode agree. Staged, unstaged, conflicted, missing, sparse, skip-worktree, assume-unchanged, intent-to-add, untracked, ignored-present, symlink, gitlink, nested-repository, special-file, hard-link-ambiguous, and filter-indeterminate entries fail the snapshot.
- A3. Profile matching receives the complete bounded entry set. An unassigned file or non-empty directory is reported as unknown layout rather than omitted, and discovery never invokes a network, shell, source-system executable, hook, template, script, or generated command.
- A4. Each source descriptor contains its profile-assigned ASCII role, canonical repository-relative path, exact byte size, and lower-case SHA-256 of exact worktree bytes after Git equality is established. No line-ending or Unicode normalization occurs.
- A5. Sources sort by unsigned UTF-8 role bytes and then path bytes; duplicate paths or role/path pairs fail. Text and JSON consumers receive the same order and values.
- A6. The aggregate SHA-256 uses exactly the `taskrail-planning-source-v1` NUL-delimited sequence specified by v0.7, including profile name/version, root, and each ordered role/path/decimal-size/file-digest record. Renames, root changes, roles, membership, bytes, or profile/version change the aggregate; timestamps, host separators, invocation directory, and unrelated permissions do not.
- A7. UTF-8, BOM, NUL, and size checks happen before text interpretation. One source is limited to 2 MiB, one profile to 256 files and 16 MiB total, and overflow or limit excess fails without a partial descriptor.
- A8. One command freezes the complete consumed-file set and Git/index identities. Inspection performs a second read-set check before returning; any membership, identity, mode, or byte race yields a conflict instead of a mixed snapshot. The same snapshot contract supports a lock-held recheck by later import work.
- A9. Canonical path, case/Unicode alias, no-follow, Git cleanliness, and race behavior is consistent on supported Linux, macOS, and Windows hosts, with platform inability reported as a refusal rather than reduced assurance.

## Verification Notes

- A1-A3: Repository integration matrices should build temporary Git roots containing each index/worktree state and filesystem entry type, including ignored-present files and non-empty unknown directories. Assert complete accounting, no-follow refusal, stable error codes, and no writes.
- A4-A6: Unit golden vectors should cover empty and multi-entry descriptors, unsigned-byte ordering, exact raw/CRLF bytes, root/path/role/profile changes, decimal sizes, duplicate rejection, and the normative aggregate byte sequence. Independently recompute file and aggregate digests as the oracle.
- A7: Boundary tests should cover valid UTF-8 and exact caps, then BOM, NUL, malformed UTF-8, one-byte overflow, file-count overflow, total-byte overflow, and integer-overflow guards; no failing case may expose a partial success descriptor.
- A8: Deterministic race integration tests should mutate bytes, mode, membership, index identity, and path identity between reads and at the later lock-held recheck boundary. Each case must return a conflict and never combine old and new observations.
- A9: Run native platform checks for link/reparse handling, canonical separators, case aliases, Git cleanliness, and descriptor equivalence. Cross-builds establish portability only; they do not replace native no-follow evidence.
- A1-A9: A sandbox service-harness probe should assign deterministic test roles,
  inspect one clean source twice, and compare byte-identical descriptors, then
  dirty one tracked source and confirm refusal without depending on a downstream
  profile or root CLI. Record setup, expected/actual output, and cleanup.

## Implementation Notes
