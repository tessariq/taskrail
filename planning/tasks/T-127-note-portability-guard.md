---
id: T-127-note-portability-guard
title: Reject transition notes that embed gitignored artifact paths
status: completed
priority: medium
spec_ref: specs/v0.2.0.md#no-local-paths-in-task-notes
dependencies: []
updated_at: "2026-07-27T23:39:13Z"
---

# T-127-note-portability-guard Reject transition notes that embed gitignored artifact paths

## Description

Per specs/v0.2.0.md#no-local-paths-in-task-notes, committed task notes must not
carry a path into gitignored `planning/artifacts/`. `verify` already writes a
path-free note, but `complete --note`, `block --reason`, and `unblock --reason`
append the raw operator text verbatim into the committed task body (and `block`
also stores the reason in the validated `blockers` ledger). A note containing a
gitignored artifact path therefore writes state that `validate` immediately
rejects — the transition "succeeds" but leaves the repo invalid (observed while
completing T-113).

Fix: reject such a note at the transition boundary (before any write), reusing the
same `danglingArtifactPaths` detector `validate` uses, and point the operator at
recording a path-free summary.

## Acceptance

- `complete`, `block`, and `unblock` fail before writing when their note/reason
  embeds a concrete gitignored artifact path (e.g.
  `planning/artifacts/manual-test/T-x/<ts>/report.md`), with a message naming the
  offending path and advising a path-free summary.
- On rejection the working tree stays clean: no task-file or `STATE.md` write, no
  status transition.
- Prose that only references the gitignored *directory* prefix or a placeholder
  (the forms `validate` deliberately allows) is still accepted.
- The guard reuses the shared `danglingArtifactPaths` detector so it accepts
  exactly what `validate` accepts — no second slug/path rule.
- Automated coverage: service tests for reject (complete/block/unblock) and
  accept (portable summary, directory-prefix prose); CLI smoke that a rejected
  `complete --note` leaves state valid and unchanged.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- 2026-07-27T23:39:07Z: verification pass
- 2026-07-27T23:39:13Z: note-portability guard shipped for complete/block/unblock + verify --create-followup; manual-test and verify artifacts recorded locally (gitignored)
