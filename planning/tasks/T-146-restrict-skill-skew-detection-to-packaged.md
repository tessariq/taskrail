---
id: T-146-restrict-skill-skew-detection-to-packaged
title: Restrict skill skew detection to packaged Taskrail skills
status: todo
priority: high
spec_ref: specs/v0.4.0.md#version-skew-detection
dependencies:
    - T-120-stale-skill-warning
updated_at: "2026-07-29T13:04:41Z"
---

# T-146-restrict-skill-skew-detection-to-packaged Restrict skill skew detection to packaged Taskrail skills

## Description

Installed-skill version detection walks every `SKILL.md` below `.agents/skills`
and `.claude/skills`. Adopter-owned skills that Taskrail never materialized are
therefore reported as unknown-version skew, making an ordinary custom skill a
permanent warning source contrary to the no-noise requirement.

## Acceptance

- Version detection inspects only paths represented by the embedded Taskrail skill
  package for each supported install target.
- Adopter-owned sibling and nested skills remain silent and do not appear in text
  or JSON warning data.
- Packaged skills still report older, newer, unknown-divergent, and marker-free
  parity-copy states exactly as specified.
- Detection remains deterministic, read-only, cheap, and covered for both install
  targets.

## Verification Notes

- T-140 sandbox evidence added an adopter-owned `custom` skill and `validate`
  warned that its Taskrail version could not be determined.

## Implementation Notes

- Derive the expected relative paths from the embedded package; do not hardcode a
  second skill-name list.
