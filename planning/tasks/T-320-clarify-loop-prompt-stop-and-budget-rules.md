---
id: T-320-clarify-loop-prompt-stop-and-budget-rules
title: Name the loop prompt's stop, follow-up, and budget rules
status: completed
priority: medium
spec_ref: specs/v0.5.0.md#source-checkout-bootstrap-loop-retirement
dependencies: []
updated_at: "2026-08-11T18:51:34Z"
---

# T-320-clarify-loop-prompt-stop-and-budget-rules Name the loop prompt's stop, follow-up, and budget rules

## Description

Close three ambiguities in the temporary loop's shared prompt that leave a
headless child guessing. The framing section tells it to "stop" for reviewed
decomposition without naming the only sanctioned stop mechanism; the blocked
lifecycle path never says whether a blocked child may create a follow-up, while
the runner accepts one for `blocked_fail` and treats one as fatal after a
timeout; and the review loop conditions another correctness pass on "the
configured budget", which the prompt never defines and the child cannot read.

## Acceptance

- A1. The framing section names the sanctioned stop as `block` plus a failing
  verification, so stopping requires no inference.
- A2. The lifecycle section states whether a blocked child may create a
  follow-up, matching the runner's `blocked_fail` and timeout behavior.
- A3. The review section replaces the undefined budget clause with a condition
  the child can evaluate.
- A4. The full loop harness stays green, including its prompt-content
  assertions.

## Verification Notes

- Run `scripts/autonomous-loop/test.sh` and `bash -n` over the changed scripts;
  the harness asserts rendered prompt content, so it is the evidence for A4.

## Implementation Notes

- 2026-08-11T18:51:33Z: Prompt now names the blocked stop pair, states the blocked follow-up rule, and caps correctness reviews at three.
- 2026-08-11T18:51:34Z: verification pass
