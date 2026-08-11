---
id: T-318-accept-inline-loop-follow-up-recommendations
title: Accept inline loop follow-up recommendations safely
status: completed
priority: high
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies: []
updated_at: "2026-08-11T18:41:05Z"
---

# T-318-accept-inline-loop-follow-up-recommendations Accept inline loop follow-up recommendations safely

## Description

Fix the temporary autonomous loop's verification-report parser so the exact
`follow-up recommendation: run|hold - <rationale>` marker is recognized when it
appears inside the single paragraph emitted by `taskrail verify --details`, not
only when it begins a physical line. The T-156 run reached a completed/pass
outcome and wrote a valid inline hold recommendation, but the parent rejected the
report before queueing the follow-up or creating its delivery bundle, leaving the
verified candidate uncommitted.

## Acceptance

- A1. A report containing exactly one valid recommendation marker is accepted
  whether the marker begins its own line or follows other verification prose.
- A2. Missing, duplicate, unsupported-mode, and empty-rationale markers are
  rejected deterministically before queue mutation or Git delivery.
- A3. The disposable loop harness reproduces the T-156 one-paragraph report
  shape and proves the parent queues its exact follow-up as `hold-operator`, then
  commits and pushes the complete owning task outcome.
- A4. The shared prompt and parser describe one unambiguous marker contract, and
  existing normal, timeout, recovery, and malformed-report cases remain green.

## Verification Notes

- Run shell syntax checks, the complete autonomous-loop harness, and Taskrail
  queue/state validation. Manually exercise an inline recommendation in a
  disposable repository and confirm one held queue row plus one pushed commit.

## Implementation Notes

- 2026-08-11T18:30:05Z: Blocked: every acceptance surface (A1/A2 parser check-report.go, A3 harness test.sh plus run.sh queue/delivery, A4 shared prompt.md) lives under scripts/autonomous-loop/, which the delegated runner forbids editing and which scripts/autonomous-loop/AGENTS.md reserves from ordinary queued tasks. Needs operator-owned execution, not a queued child.
- 2026-08-11T18:30:59Z: verification fail
- 2026-08-11T18:40:48Z: Inline recommendation markers accepted; duplicate, unsupported-mode, and empty-rationale markers rejected before queue mutation or delivery.
- 2026-08-11T18:41:05Z: verification pass
