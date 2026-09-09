---
id: T-392-make-durable-transaction-observation-scale-past-a
title: Make durable transaction observation scale past a few hundred managed files
status: completed
priority: high
spec_ref: specs/v0.5.0.md#layout-compatibility-and-upgrade
dependencies: []
updated_at: "2026-09-09T10:18:41Z"
completion_id: "a64f50b5d531f98547d239a84cb92448"
last_verification_id: "4ab4c271bc4540d3e3bb5ee7d09e9ce5"
last_verification_result: pass
last_verified_at: "2026-09-09T10:18:41Z"
last_verified_completion_id: "a64f50b5d531f98547d239a84cb92448"
---

# T-392-make-durable-transaction-observation-scale-past-a Make durable transaction observation scale past a few hundred managed files

## Description

`internal/durablefs.exactName` (`internal/durablefs/durablefs.go:397`) is the
case-fold alias guard every observed path passes through: it opens the parent
directory and reads its **entire** listing to prove no other entry folds to the
same key. `observeFile` calls it per leaf and `inspectDirectory`/`observeTree`
call it per path component, so observing a directory of `n` files costs `n` full
`ReadDir` sweeps. Go's `os.(*File).ReadDir` lstats each returned entry, and one
durable transaction observes its set more than once (originals, candidate
publication, post-validation, and rollback), multiplying the same quadratic
sweep by the phase count.

This repository has 390 task files in one directory. `taskrail init --apply
--confirm-quiescent --drop-continuation-notes` on it published the migration
fence and then spun for over 24 minutes of CPU without completing; `taskrail
recover <id> --apply` on the resulting fenced repository spun the same way. A
`SIGQUIT` stack of the spinning process shows goroutine 1 inside
`durablefs.exactName -> os.(*File).ReadDir -> internal/syscall/unix.Fstatat`,
reached from `observeFile` and `inspectDirectory`. The behaviour reproduces with
a binary built from the `v0.4.0`-era `HEAD` (64cee15), so it predates T-389 and
is not caused by the layout raise.

The repository was restored through the escape the fenced-marker error itself
names — reverting `.taskrail/config.yml` through Git and discarding the retained
transaction. No task, spec, or state byte had been published.

The consequence is that the layout-2 upgrade — the remedy every T-389 refusal
tells an operator to run — does not complete on a repository of this size, so
T-389 cannot deliver a usable safety boundary here and depends on this task.

## Acceptance

- Observing one directory resolves every leaf's alias-exactness from a bounded
  number of directory reads rather than one full read per leaf, so a transaction
  over `n` files in a directory does not perform `O(n^2)` directory entry stats.
- Alias detection is unchanged in strictness: a name that NFC/case-folds onto an
  existing entry is still refused with `ErrAlias`, a required-but-absent leaf
  still fails, and a publish onto an existing name still fails.
- `taskrail init --apply --confirm-quiescent` completes on a repository carrying
  at least 400 task files in one directory, within a duration a maintainer would
  accept for an interactive command.
- `taskrail recover <id> --apply` over the same corpus completes as well, so an
  interrupted migration stays recoverable at that size.

## Verification Notes

- Add a `durablefs` package test over a temporary directory with several hundred
  entries that fails on the current per-leaf full scan (assert the observed
  directory-read or stat count, not wall-clock time, so the test is not timing
  dependent).
- Keep the existing alias, missing-leaf, and destination-exists cases green;
  add one that proves a folded collision is still detected after the change.
- Exercise the real flow end to end on a scratch copy of this repository's
  `planning/` tree: apply the layout-2 upgrade and assert it completes and
  validates, then repeat with an interrupted apply plus `recover --apply`.

## Implementation Notes

- 2026-09-09T10:18:32Z: Alias exactness now answers from one Readdirnames-backed fold index per directory instead of a full ReadDir per checked name, and a pure observation pass shares one directory snapshot across members. Mutating call sites keep uncached observation. Verified on a copy of this repository's planning tree: the layout-2 upgrade completes in ~16s instead of spinning past 24 minutes, an interrupted upgrade recovers in ~0.1s, and the retried upgrade validates at layout 2. Bounded-read regression tests added in internal/durablefs and internal/durabletx; full suite, gofmt and vet green.
- 2026-09-09T10:18:41Z: verification pass id 4ab4c271bc4540d3e3bb5ee7d09e9ce5 previous none completion a64f50b5d531f98547d239a84cb92448
