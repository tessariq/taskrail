---
id: T-242-align-full-task-skills-with-the-canonical
title: Align full-task skills with the canonical lifecycle
status: todo
priority: high
spec_ref: specs/v0.5.0.md#lifecycle-complete-skill-flows
dependencies:
    - T-160-ship-the-lifecycle-complete-task-implementation
    - T-201-make-packaged-skills-agent-skills-compliant
updated_at: "2026-08-06T13:46:30Z"
---

# T-242-align-full-task-skills-with-the-canonical Align full-task skills with the canonical lifecycle

## Description

Make every full-task packaged skill execute the canonical success, blocked, and
rework branches with checked JSON writers instead of ending in prose before the
lifecycle transition.

## Acceptance

- A1. Success invokes complete then passing verify; cannot-proceed invokes block
  then failing verify; deliberate rework records fail and remains in progress.
- A2. Every writer exit is checked, source-checkout freshness guards remain, and
  completed-unverified/audit-fail recovery never repeats complete or fabricates block.
- A3. Embedded and committed copies remain Agent Skills-compliant, marker-free,
  byte-identical, and provider-neutral.

## Verification Notes

- A1: executable skill fixtures observe exact command order and final repository state.
- A2: injected writer/audit failures prove stop/recovery guidance and no later command.
- A3: frontmatter, package parity, skew, and command-registry checks provide evidence.

## Implementation Notes
