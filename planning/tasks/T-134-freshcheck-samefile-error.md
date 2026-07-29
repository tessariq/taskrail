---
id: T-134-freshcheck-samefile-error
title: Name the cause when the freshness guard cannot stat the working-tree build path
status: completed
priority: low
spec_ref: specs/v0.4.0.md#version-skew-detection
dependencies:
    - T-123-contributor-binary-resolution
updated_at: "2026-07-29T09:38:01Z"
---

# T-134-freshcheck-samefile-error Name the cause when the freshness guard cannot stat the working-tree build path

## Description

Follow-up from T-123, raised by two review passes over that change.

`internal/toolchain/cmd/freshcheck/main.go` decides between the three
"wrong binary" verdicts with:

```go
isWorkingTreeBuild, _ := binpath.SameFile(target, installed)
```

The discarded error collapses distinct causes. Today every one of them still
produces a defensible message (the caller falls through to the shadowed or
override verdict, whose remedies also cover "the build does not exist"), so this
is a diagnosability gap, not a wrong verdict. But it now gates one more branch
than when it was written — the `OverrideError` / `ShadowedError` split — so the
same swallowed error has more consumers.

The concrete confusing case: `taskrail:install` has never run, so `bin/taskrail`
does not exist. The contributor is told a bare `taskrail` runs the wrong binary
and pointed at `mise run setup`, which does happen to fix it, but the message
never says the working-tree build is simply absent.

## Acceptance

- A missing (or unstattable) working-tree build path is reported as its own cause,
  naming that the build has not been produced yet, rather than being folded into
  the shadowed/override verdict.
- The three existing verdicts (shadowed, misdirected override, toolchain
  difference, staleness) keep their current messages and remedies.
- Covered by a test that fails if the stat error is discarded again.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes

- 2026-07-29T09:37:55Z: verification pass
