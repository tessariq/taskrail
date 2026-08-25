---
id: T-242-align-full-task-skills-with-the-canonical
title: Align full-task skills with the canonical lifecycle
status: completed
priority: high
spec_ref: specs/v0.5.0.md#lifecycle-complete-skill-flows
dependencies:
    - T-160-ship-the-lifecycle-complete-task-implementation
    - T-201-make-packaged-skills-agent-skills-compliant
updated_at: "2026-08-25T21:04:56Z"
completion_id: "bb8786dcf9aa1a0ec8fc1dcf3b39b259"
last_verification_id: "ba28d66b9a94939fc42094aaca357d94"
last_verification_result: pass
last_verified_at: "2026-08-25T21:04:56Z"
last_verified_completion_id: "bb8786dcf9aa1a0ec8fc1dcf3b39b259"
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
- A3. Every full-task skill mirrors T-160's review contract: deterministic checks
  precede one fresh reviewer and one broad round by default, while additional
  distinct-lens reviewers and a second round require independently relevant risk.
  Simplification does not require a fresh subagent; findings receive deterministic
  re-verification and material review-induced changes receive one conditional
  non-recursive final-diff review. Objective evidence may close a final-diff
  repair; inadequately demonstrated repairs remain in-progress/fail.
- A4. Embedded and committed copies remain Agent Skills-compliant, marker-free,
  byte-identical, and provider-neutral.
- A5. Full-task skills consume managed subjects through Taskrail commands and use
  reported storage mode for delivery: committed mode includes Taskrail metadata,
  while local mode never force-adds ignored metadata and commits only required
  visible product changes. Local delivery follows repository-visible Git policy,
  preserves caller identity/configuration, and excludes incidental private
  planning provenance. Frozen repository-visible policy governs generic Git
  conventions, but only caller-owned instruction outside managed planning
  authorizes exposing a local Taskrail identity/path in commit metadata;
  outcome-required product bytes do not independently authorize them.
- A6. Before invoking `start`, every full-task skill applies the T-251 sizing
  rubric and stops for reviewed replanning on bundled outcomes, non-valuable
  fragments, or unclear integration ownership. Follow-ups remain limited to newly
  discovered independently meaningful out-of-scope outcomes, never unfinished
  pieces of the selected outcome.

## Verification Notes

- A1: executable skill fixtures observe exact command order and final repository state.
- A2: injected writer/audit failures prove stop/recovery guidance and no later command.
- A3: executable fixtures cover one default reviewer, risk-justified additional
  lenses and second-round use, clean early stop, post-fix deterministic checks,
  final-diff clean success, objective repair closure, and judgment-heavy rework.
- A4: frontmatter, package parity, skew, and command-registry checks provide evidence.
- A5: committed/local fixtures with decoy logical files and private identifiers
  prove subject-command reads, exact Git tree/commit provenance, unchanged Git
  identity/configuration, trusted public-reference authorization, product-byte
  exceptions, and current-run self-authorization refusal.
- A6: executable oversized, fragmented, unclear-integration, in-scope-discovery,
  and valid-follow-up fixtures prove no lifecycle writer runs before replanning
  and no required selected-task scope escapes into a follow-up.

## Implementation Notes

- 2026-08-25T21:04:41Z: Aligned full-task skills with the canonical lifecycle, review, recovery, and delivery contract.
- 2026-08-25T21:04:56Z: verification pass id ba28d66b9a94939fc42094aaca357d94 previous none completion bb8786dcf9aa1a0ec8fc1dcf3b39b259
