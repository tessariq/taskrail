---
id: T-124-parity-copy-skew-exemption
title: Exempt marker-free parity copies from the unknown-version skew report
status: completed
priority: medium
spec_ref: specs/v0.4.0.md#version-skew-detection
dependencies:
    - T-120-stale-skill-warning
updated_at: "2026-07-28T12:07:44Z"
---

# T-124-parity-copy-skew-exemption Exempt marker-free parity copies from the unknown-version skew report

## Description

T-120 reports skills carrying no `taskrail_version` marker once as unknown-version,
which is honest: nothing recorded a version, so no skew can be determined. But this
repository's own committed `.agents/`/`.claude/` copies are *deliberately* marker-free
forever — `task skills:regen` copies the package source so `task check:skills` stays
byte-exact (docs/workflow/skills-productization.md). The result is a standing stderr
line on every command run here, naming all eleven skills, that a contributor can never
clear. T-120 removed the misleading `--force` remedy from that message and documented
the trap, so nothing is broken; what remains is permanent verbosity in exactly the
repository that dogfoods Taskrail hardest.

The tension is structural, not cosmetic: an unmarked copy that is byte-identical to the
embedded package is not a stale install, but T-120's acceptance forbids diffing skill
contents ("it reads the recorded version, never diffs skill contents"), and the spec
repeats it. Resolving this means changing one of the two contracts, which is an
ask-first decision rather than an implementation detail.

Follow-up derived from T-120-stale-skill-warning's verification or discovery.

## Acceptance

- Decide and record which contract gives: a cheap exemption for copies identical to the
  embedded package (a bounded comparison against the in-binary copy, not against another
  install), a marker the parity check tolerates, or accepting the verbosity as intended.
- If an exemption lands, a repository whose skills are byte-identical to the embedded
  package is silent, while a genuinely unmarked *diverged* install still reports as
  unknown-version.
- `task check:skills` keeps passing unchanged, and the adopter-facing behavior T-120
  specified (known mismatch names both versions and the `--force` remedy) is untouched.
- Whatever is decided is reflected in `specs/v0.4.0.md#version-skew-detection` and
  `docs/workflow/skills-productization.md`, so the two stop disagreeing.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- 2026-07-28T12:07:39Z: verification pass
