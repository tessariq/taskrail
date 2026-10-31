---
id: T-436-document-executing-and-recovering-a-loop-run
title: Document executing and recovering a loop run
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#cross-platform-autonomous-loop
dependencies: []
updated_at: "2026-10-31T11:39:13Z"
---

# T-436-document-executing-and-recovering-a-loop-run Document executing and recovering a loop run

## Description

Doc gap. docs/loop.md never shows how to execute a run: the child is the argv
after `--`, receives the prompt on stdin, and must leave exactly one direct-child
commit. It omits `--max-iterations` (default 1, which silently caps
`--parallel`), `--timeout`, `--max-review-rounds` and `--workspace-root`;
describes `--keep-workspaces` as if it were boolean (it takes
`never|failure|always`, default `failure`); does not say that an exported
`TASKRAIL` makes `loop` refuse, that result-file ancestors must not be symlinks
(macOS `/tmp`), that clones use `--single-branch --no-tags` even at full depth
(so `git describe` fails), that parallel worker clones do not run the source
repository's Git hooks, or that execution in the Taskrail source checkout is
unsupported. There is no crash-recovery guidance (stale lock, staged
executable, retained workspaces, orphaned children). docs/commands.md's effects
table omits loop execution, `task loop list/allow/hold/clear`, `spec add`,
`prompt render` and `local path/status`. The page reads as contract text; a
short user-facing quickstart should precede it. Found in the 2026-10-31 critical review of the loop family at 5672a13.

## Acceptance

- docs/loop.md starts with a quickstart that uses only real flags: allow, dry-run, run with a child argv, read the result file.
- Every loop flag in `loop --help` is described with its default.
- The limits listed in the description and a crash-recovery section are documented.
- docs/commands.md's effects table lists every writer and read-only command named in the description.

## Verification Notes

- Compare docs/loop.md flags with `taskrail loop --help`; run the quickstart in a temp repo with a stub child.

## Implementation Notes
