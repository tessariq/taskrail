---
id: T-256-retire-bootstrap-planning-reviews
title: Retire the hand-produced bootstrap planning reviews
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#safe-review-artifact-publication
dependencies:
    - T-215-add-the-generic-review-artifact-publisher
    - T-162-productize-digest-bound-post-spec-review-lenses
updated_at: "2026-08-08T08:40:49Z"
---

# T-256-retire-bootstrap-planning-reviews Retire the hand-produced bootstrap planning reviews

## Description

`planning/bootstrap-reviews/` holds hand-produced v0.5 spec-review evidence
written before Taskrail could publish review artifacts itself: a sequence of
immutable revision reports, each paired with a `sha256sum`-format task manifest
whose digest the report records. No command, skill, or script produces them, and
no implementation, build, CI, or release command consumes them as Taskrail
artifacts.

Once the generic review publisher (T-215) and the post-spec review lenses
(T-162) ship, real schema-v1 artifacts land under the durable review roots and
this directory becomes closed history. Decide its disposition deliberately
rather than leaving an unreferenced directory behind: the reports are the only
record of how the v0.5 contract was reviewed, so retiring the *practice* is not
the same as deleting the *evidence*.

This task is bookkeeping for a directory that outlived its reason, not a
migration of its contents into the new schema. Hand-made reports were never
schema-v1 artifacts and must not be back-filled to look like they were.

## Acceptance

- The disposition of `planning/bootstrap-reviews/` is chosen and recorded:
  either retained as frozen historical evidence under an explicitly documented
  path, or removed with the reports preserved in git history and referenced
  from the v0.5 release record. Rewriting the reports into schema-v1 artifacts
  is explicitly out of scope and rejected.
- If retained, `AGENTS.md` states that the directory is closed to new revisions
  and names the real publication path that supersedes it. If removed, every
  reference to it in `AGENTS.md` is deleted in the same change.
- The `AGENTS.md` "Notes On Repository Behavior" entry describing the directory
  and its deliberate absence of a `Taskfile.yml`/CI check is updated or removed
  to match the chosen disposition, leaving no guidance for a path that no
  longer exists.
- No check is added that gates on superseded revisions verifying; partial
  verification of frozen manifests stays expected behavior, not a failure.
- Current v0.5 spec-review evidence is satisfied by real published artifacts
  before this retirement completes, and no later release gate cites a bootstrap
  report as evidence. T-173 and its downstream release chain run after this
  retirement rather than treating it as post-gate cleanup.
- `go run ./cmd/taskrail validate` passes and committed `planning/STATE.md`
  stays consistent with the task files.

## Verification Notes

- Inventory the real schema-v1 current-spec review directory and validate its
  prompt/spec/session/manifest bindings before choosing bootstrap disposition.
- Search implementation, build, Taskfile, CI, release-gate, docs, and task
  dependencies for bootstrap consumption; only explicit frozen-history guidance
  may remain when retained.
- Prove T-173 and the downstream release chain depend on this completed outcome,
  then run validation, coverage/gap reporting, task-body hygiene, and state
  projection checks after the directory/documentation change.

## Implementation Notes
