---
id: T-120-stale-skill-warning
title: Warn when installed skill files are outdated
status: todo
priority: high
spec_ref: specs/v0.4.0.md#version-skew-detection
dependencies:
    - T-121-skill-version-marker
updated_at: "2026-07-27T13:49:41Z"
---

# T-120-stale-skill-warning Warn when installed skill files are outdated

## Description

Installing skills is deliberately non-destructive, and `skills.go` states the
consequence outright: "upgrading the binary never refreshes materialized copies."
An adopter who installs skills on one version and later upgrades the binary keeps
following the older skill's instructions indefinitely — no error, no signal, and no
way to notice short of diffing files by hand.

Both skew directions are quiet in their own way. Older skills than binary means
silently missing new capability; newer skills than binary means the skill invokes a
flag the binary lacks and fails at the point of use, long after the upgrade that
caused it. Per the Version Skew Detection amendment, Taskrail should say so itself
and name the fix.

Depends on T-121 for the version marker that makes the skew detectable.

## Acceptance

- A command run in a repository whose installed skills were written by a different
  Taskrail version prints a warning to stderr naming the affected skills, both
  versions, and `taskrail init --with-skills --force` as the resolution.
- Advisory only: it goes to stderr, leaves `--json` stdout parseable, never makes
  `validate` fail, and never blocks a transition — the same contract the
  empty-derived-slug warning follows.
- Silent when there is nothing to say: a repository with no materialized skills, or
  one whose skills match the running version, prints nothing. Skills with no marker
  at all (installed before T-121) are reported once as unknown-version rather than
  as a false upgrade prompt.
- Cheap enough for ordinary commands: it reads the recorded version, never diffs
  skill contents.
- Read-only. The warning never rewrites an adopter's skill files; `--force` stays
  the explicit opt-in.
- Covered by tests for each case: matching version, older skills, newer skills,
  unmarked skills, and no skills installed.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
