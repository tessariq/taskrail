---
id: T-147-prevent-stale-working-tree-binaries-from-writing
title: Prevent stale working-tree binaries from writing tracked state
status: completed
priority: high
spec_ref: specs/v0.4.0.md#version-skew-detection
dependencies:
    - T-123-contributor-binary-resolution
updated_at: "2026-07-29T17:14:44Z"
---

# T-147-prevent-stale-working-tree-binaries-from-writing Prevent stale working-tree binaries from writing tracked state

## Description

This source repository can invoke an older on-PATH Taskrail binary that accepts a
state-writing command and stamps tracked files with stale behavior. The current
freshness check runs in an optional pre-commit hook, after the write occurred. Close
the spec's before-write guarantee for repositories that build Taskrail themselves.

## Acceptance

- The repository's normal autonomous and human state-writing paths check the exact
  `${TASKRAIL:-taskrail}` binary against the working-tree build before invoking
  `start`, `next`, `verify`, `complete`, `block`, or other tracked-state writers.
- A stale or wrongly resolved binary fails before any task or `STATE.md` bytes
  change and names the correct build-vs-resolution remedy.
- Installed adopter repositories that do not build Taskrail locally are unaffected.
- Cross-platform tests or inspectable workflow guards prove both refusal and the
  fresh-binary success path; pre-commit remains defense in depth, not first notice.

## Verification Notes

- T-140 semantic review confirmed `lefthook.yml` detects skew only at commit time;
  prior T-123 incident notes document stale writes succeeding silently.

## Implementation Notes

- Keep the core binary provider/tooling independent; this is source-repository
  workflow protection, not a general runtime build manager.
- 2026-07-29T17:14:35Z: verification pass
