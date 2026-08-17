---
id: T-309-launch-loop-children-with-exact-prompt-transport
title: Launch loop children with exact prompt transport
status: todo
priority: high
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies:
    - T-171-contain-and-pin-autonomous-loop-child-processes
updated_at: "2026-08-08T14:23:09Z"
---

# T-309-launch-loop-children-with-exact-prompt-transport Launch loop children with exact prompt transport

## Description

Launch one fresh generic foreground child for a selected task using the T-171
staged identity and the T-308 frozen rendering. Preserve argv, environment,
working-directory, finite-stdin, EOF, and stream semantics exactly without
adding process-tree containment or postflight classification.

## Acceptance

- A bare executable resolves through inherited `PATH`; a command path containing
  a separator resolves against the caller's original working directory before
  cwd changes. The resulting executable and argv are passed directly to the OS
  with no shell evaluation or provider-specific behavior.
- Each child starts in the repository root and inherits caller-owned environment
  and authentication except that the four exact staged/delegation variables are
  set to T-171's frozen values. The child observes the frozen storage mode/root
  and effective broad implementation-review-round maximum through the rendered
  contract, including one reviewer by default, additional reviewers only for
  distinct relevant risk within the three-reviewer ceiling, and conditional
  final-diff rules, and cannot replace the staged writer identity.
- Stdin contains exactly the selected task's frozen UTF-8 rendered prompt bytes,
  with no framing or added newline, and closes immediately at EOF. Child stdout
  and stderr stream faithfully to the corresponding Taskrail streams.
- Spawn, stdin, or stream-copy failure is returned as explicit child execution
  evidence for later containment and outcome handling; it cannot be reported as
  a successful child or trigger an additional task.
- Execution stays foreground and sequential and launches exactly one fresh process
  per selected task; unsupported finite-stdin consumers require an external
  caller adapter rather than hidden retries or background execution.

## Verification Notes

- Portable helper executables record absolute executable, argv boundaries,
  repository cwd, selected environment, exact stdin bytes/EOF, stdout, stderr,
  and process identity for bare-PATH and separator-path invocations.
- Adversarial argv and shell-metacharacter fixtures prove no join, quoting rewrite,
  interpolation, or shell execution occurs; prompt fixtures cover empty/final-LF/
  no-final-LF and non-ASCII UTF-8 bytes.
- Spawn, broken stdin, stdout/stderr copy, and non-zero child fixtures expose
  distinct evidence and prove no retry or second launch.

## Implementation Notes
