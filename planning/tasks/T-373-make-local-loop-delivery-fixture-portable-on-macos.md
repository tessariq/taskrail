---
id: T-373-make-local-loop-delivery-fixture-portable-on-macos
title: Make local loop delivery fixture portable on macOS
status: completed
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-225-prove-local-autonomous-delivery-across-git
updated_at: "2026-08-27T14:55:57Z"
completion_id: "902b1cee3e4253db6ce98fd68b577e70"
last_verification_id: "04b0aa73e1e62e95af827edecaa086fd"
last_verification_result: fail
last_verified_at: "2026-08-27T14:55:57Z"
last_verification_previous_id: "8fb900e6e4ce2ccd9d234248a5c6a3b8"
---

# T-373-make-local-loop-delivery-fixture-portable-on-macos Make local loop delivery fixture portable on macOS

## Description

Replace recursive copied-test-binary delegation in the T-225 local delivery
fixture with a real built Taskrail executable and direct delegated lifecycle
commands, preserving strict executable identity and product-only delivery
evidence.

This task owns integrated delivery of the deferred outcome and its invariant after T-225-prove-local-autonomous-delivery-across-git's verification.

## Acceptance

- The local loop delivery fixture launches a real built `cmd/taskrail` executable
  as the pinned delegated writer instead of recursively executing a copied Go test
  binary.
- Delegated start, complete, and passing verification still produce exactly one
  product-only commit while local Taskrail metadata remains ignored and untracked.
- Production executable identity checks are unchanged, the focused test passes on
  Linux and macOS, and the exact-head CI matrix is green.

## Verification Notes

- Run the focused local delivery test, the loop test subset, and the full Go suite.
- Record the exact macOS CI run that passes the previously failing fixture.

## Implementation Notes

- 2026-08-27T14:42:47Z: Pinned a real built Taskrail executable in the local delivery fixture and invoked delegated lifecycle commands directly; focused, loop-subset, full-suite, vet, formatting, validation, and independent portability review passed.
- 2026-08-27T14:42:54Z: verification pass id 8fb900e6e4ce2ccd9d234248a5c6a3b8 previous none completion 902b1cee3e4253db6ce98fd68b577e70
- 2026-08-27T14:55:57Z: verification fail id 04b0aa73e1e62e95af827edecaa086fd previous 8fb900e6e4ce2ccd9d234248a5c6a3b8 completion none
