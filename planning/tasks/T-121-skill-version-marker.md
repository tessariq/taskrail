---
id: T-121-skill-version-marker
title: Stamp the writing Taskrail version into materialized skill files
status: todo
priority: high
spec_ref: specs/v0.4.0.md#version-skew-detection
dependencies: []
updated_at: "2026-07-27T13:49:41Z"
---

# T-121-skill-version-marker Stamp the writing Taskrail version into materialized skill files

## Description

Materialized skill files carry no version information — their frontmatter is just
`name` and `description` — so nothing can tell a skill written by v0.3.0 from the
current one without diffing content against the embedded package. That missing
marker is why the skew described in the Version Skew Detection amendment cannot be
reported today, so it is the enabling half of T-120 and lands first.

`WriteShippableSkills` already knows it is writing the embedded copy; it just does
not record which binary wrote it.

## Acceptance

- Files written by `init --with-skills` record the Taskrail version that wrote them,
  in a form an agent tool reading the skill ignores harmlessly (skills are consumed
  by agents, so the marker must not change how they are interpreted).
- The marker is written on both fresh installs and `--force` reinstalls, and the
  installed-vs-embedded comparison stays content-based so an unmodified re-run still
  writes nothing and accumulates no backups (the current no-op guarantee).
- A helper reports, for a repository, which installed skills were written by which
  version — the read side T-120 consumes. It is read-only and tolerates skills with
  no marker (installed before this change) by reporting them as unknown rather than
  failing.
- The committed `.agents/`/`.claude/` copies stay byte-identical to the package, so
  `task check:skills` passes with the marker present in both.
- The marker survives a round trip: install, read back, and compare against the
  running version.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
