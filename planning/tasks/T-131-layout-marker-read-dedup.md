---
id: T-131-layout-marker-read-dedup
title: Deduplicate the layout marker read path
status: todo
priority: low
spec_ref: specs/v0.4.0.md#layout-compatibility-beyond-init
dependencies:
    - T-122-layout-version-guard
updated_at: "2026-07-28T11:37:50Z"
---

# T-131-layout-marker-read-dedup Deduplicate the layout marker read path

## Description

Follow-up discovered while implementing T-122-layout-version-guard.

`loadLayoutConfig` and `readMarker` in `internal/taskrail/paths.go` now run the same
three steps in the same order: read `.taskrail/config.yml`, unmarshal onto
`defaultLayoutConfig()`, then `ensureSupportedLayoutVersion`. T-122 added the third
step to both, so the duplication is now three steps deep rather than two, and a
future marker-level rule has two places to land in.

The two are not trivially foldable. They differ deliberately: `loadLayoutConfig`
synthesizes the default layout when the file is absent (discovery stays purely
additive), while `readMarker` reports absence so `init` can distinguish an unmarked
repository from a marked one. They also emit deliberately different wording —
`read/parse layout config` versus `read/parse layout marker` — and that wording is
what an adopter sees, so it is contract, not incidental.

This is code hygiene with no behavior change attached. It is deliberately low
priority: two call sites is a tolerable amount of duplication, and a fold that
flattens the absent-file distinction or the error wording would be worse than the
duplication it removes.

## Acceptance

- The read/unmarshal/version-guard sequence exists once, with `loadLayoutConfig`
  and `readMarker` expressed in terms of it rather than repeating it.
- Behavior is unchanged in every respect: the absent-marker distinction (synthesized
  default versus `found=false`), the `layout config` versus `layout marker` error
  wording, and the version refusal all stay exactly as they are.
- If no fold preserves all three without contorting the code, the task closes as
  cancelled with the reason recorded — keeping the duplication is an acceptable
  outcome here.
- Existing tests in `paths_test.go`, `layout_version_test.go`, and `init_test.go`
  pass untouched; a change that needs them edited is a behavior change, not this task.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
