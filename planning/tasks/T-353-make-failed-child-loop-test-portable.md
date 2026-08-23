---
id: T-353-make-failed-child-loop-test-portable
title: Make failed-child loop test portable
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-314-integrate-loop-continuation-and-terminal
updated_at: "2026-08-23T19:06:08Z"
completion_id: "ebbd15481074a11d4ae5c3a2bec0570e"
last_verification_id: "f850b42dd4290ed75d122302e2f19bb0"
last_verification_result: pass
last_verified_at: "2026-08-23T19:06:08Z"
last_verified_completion_id: "ebbd15481074a11d4ae5c3a2bec0570e"
---

# T-353-make-failed-child-loop-test-portable Make failed-child loop test portable

## Description

Keep the failed-child loop execution regression portable so the native Windows
CI leg exercises the same terminated-child postflight behavior as Unix instead
of accidentally testing an unavailable `/bin/sh` launch.

## Acceptance

- The failed-child execution test launches a repository-owned cross-platform
  helper that exits non-zero on Unix and Windows.
- The test continues to prove one terminated child produces the expected invalid
  delivery outcome and completed-iteration count.
- The full native Windows CI test job passes.

## Verification Notes

- Run the focused loop execution test and the full Go test suite locally.
- Confirm the exact repair commit passes the GitHub Actions Windows matrix leg.

## Implementation Notes

- 2026-08-23T19:06:04Z: Replaced the Unix-only shell child with the cross-platform Go helper; focused and full local checks pass.
- 2026-08-23T19:06:08Z: verification pass id f850b42dd4290ed75d122302e2f19bb0 previous none completion ebbd15481074a11d4ae5c3a2bec0570e
