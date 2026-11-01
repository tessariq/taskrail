# Bounded Loop Execution

The `loop` family runs tracked work unattended within explicit, operator-set
bounds. It is part of the active `v0.5.0` specification. The
[README](../README.md) covers the interactive core loop; this page holds the
loop contract.

Taskrail never launches unattended work implicitly: a task is eligible only when
an operator has explicitly allowed it, and every launch boundary — task
selection, prompt bytes, clone width, delivery target — is frozen before a child
process starts.

## Loop policy on tasks

Tasks may carry paired `loop_policy: allow|hold` and `loop_reason: <reason>`
frontmatter. Omitting both means an implicit hold with reason
`implicit hold: loop policy is not set`; `taskrail validate` rejects incomplete
or malformed pairs. Lifecycle and task writers preserve explicit policy metadata,
and `STATE.md` does not duplicate it.

```sh
taskrail task loop list [--json]                 # effective policy for every task
taskrail task loop allow <task-id> --reason "…"  # todo/blocked tasks only
taskrail task loop hold  <task-id> --reason "…"
taskrail task loop clear <task-id>               # back to the implicit hold
```

`task loop list` reports each task's effective loop policy, held dependency
closure, and unattended eligibility without changing task files, `STATE.md`, or
ordinary lifecycle selection. The `allow`/`hold`/`clear` writers preserve all
other task bytes, reproject `STATE.md` transactionally, and refuse delegated
loop children. All accept `--dry-run` and `--json`.

## Previewing a run

```sh
taskrail loop --dry-run --json
```

Dry run publishes the highest-ranked explicitly allowed task, its frozen
implementation prompt, and the delivery/review limits — without launching a
child process.

## Caller-owned sequential check

Optionally supply `--check <path>` with `--parallel 1` to gate each delivered
completed pass before Taskrail selects another task:

```sh
taskrail loop --dry-run --check ./read-only-check --json
taskrail loop --check ./read-only-check --timeout 10m \
  --result-file /outside/repository/new-result.json -- child-command arg
```

Choose the executable yourself; Taskrail does not choose a tool or interpret its
output. Bare names resolve through caller `PATH`; paths containing a separator
resolve from the caller's original working directory. Missing or non-executable
checks refuse before a child launches. Dry-run resolves and hashes without
running the check. Effective parallel widths above one require `--check`, even
when only one task is eligible. Each replayed integration commit is checked;
failure rejects and rolls back that candidate while later independent candidates
can proceed. A mutating check stops the batch without publication rather than
trusting reset to restore arbitrary files, refs, or configuration. After the
aggregate child passes, the final check must pass on the same unchanged head
before local fast-forward publication. Candidate and final diagnostics retain
the checked head and process evidence, including after rollback.

Effective width is `min(--parallel, --max-iterations)`, not frontier size.
Parallel review delivery is temporarily unavailable: with a supplied check,
`--delivery review` refuses `unsupported` before workspaces, workers, or adapter
calls. Without a check, the missing-check refusal takes precedence. The existing
adapter protocol merges before aggregate validation and cannot yet provide the
required pre-publication guarantee; Taskrail does not claim remote rollback.
The review-adapter description below documents that protocol, not an enabled
safe-delivery mode. Width-one parallel-only flag refusals are unchanged.

After a final per-child `completed_pass`, the check runs once at the repository
root, with no arguments, no shell, empty stdin, the caller environment without
added delegation credentials, and the same independent `--timeout` budget. Both
check streams go to Taskrail's stderr. Exit zero without timeout, signal,
transport/containment failure, or repository mutation passes. Taskrail compares
managed inputs, worktree content including ignored files, Git index/refs/root-ref
candidates, and repository-owned Git configuration before and after the check.

A failed check stops with `check_failed` and non-zero exit; it is never retried.
Taskrail preserves the delivered commit, completion, and verification. It does
not undo mutations made by the check. The result file retains the pre-check Git
delivery observations and records absolute path, executable SHA-256, nullable
exit code/signal, timeout, elapsed integer milliseconds through process drain,
stream SHA-256 digests, and ordered mutation/process violations under
`last_iteration.check` (inside `error.details` on failure). Absent or unattempted
checks use null; empty streams use the empty-byte digest. A setup/spawn failure
has null exit and signal with a process violation. Inspect this evidence and the
current repository before deciding on any recovery. Without `--check`, the
sequential workflow is unchanged apart from the nullable evidence member.

## Parallel clone frontier

`--parallel <n>` previews one ranked, dependency-ready clone frontier without
creating workspaces or launching workers; clone and delivery options are refused
when their effective width is one.

With a width above one, execution runs that frozen frontier in private
`--no-local` shallow clones, replays valid worker commits in selection order in a
separate integration clone, and fast-forwards the source branch only after
aggregate validation. `--clone-depth full` opts out of the shallow boundary;
`--keep-workspaces` retains failed workspaces by default.

Aggregate validation is read-only and binds the exact integration commit eligible
for publication. The source is rechecked after fetch before its guarded
fast-forward.

**A final fast-forward is not a durable all-or-none transaction.** If interruption
leaves uncertain branch, index, or worktree state, Taskrail reports what it
observed and never retries or overwrites drift. Inspect and repair that Git state
deliberately before a new loop invocation; preflight refuses a dirty worktree.

## Delivery

Local delivery is the default. `--delivery review --review-adapter <path>` sends
each provider-neutral JSON request directly to one caller-owned executable and
requires `--result-file <external-path>` so its terminal adapter outcome remains
durable after the foreground run. It does not embed credentials, provider APIs,
or shell evaluation. The adapter may wrap tools such as `gh` or `glab`, but owns
their authentication and provider semantics.

Passing worker branches and changes can open concurrently; Taskrail inspects,
merges, and refreshes remaining changes in frozen frontier order. A failed
worker, adapter operation, check, or refresh is not retried, but other
independently valid changes remain eligible. A replacement implementation prompt
requires its exact template SHA-256 through `--allow-prompt-override-sha256`.

Execution keeps child output streaming, so local delivery may use
`--result-file <external-path>` and review delivery requires it to receive the
one terminal schema-1 envelope. The target must be absent in an existing,
non-symlinked directory outside the worktree and Git metadata; Taskrail rechecks
that directory and creates the file without replacement after postflight.

## Recovering an abruptly killed coordinator

Each invocation stages its executable under the canonical Git common directory's
`taskrail/executables/` directory. The filename is the binary's SHA-256 plus a
random staging ID (and the executable suffix on Windows). A matching
`.owner-<random-id>` hard link guards its file identity. Normal cleanup validates
the digest and both identities before removing only that invocation's pair.
Abrupt termination can leave either or both entries; later invocations use fresh
paths and do not reuse, overwrite, or automatically delete these leftovers.

1. Stop new writers and loop launches across **all linked worktrees** for the
   duration of recovery. Establish that the old coordinator and its descendants
   have exited; clearing a lock does not terminate or revoke an orphan child.
   Inspect and reconcile any task or Git changes before rerunning the loop.
2. Use `taskrail lock status --json` to inspect the recorded PID, host, lock ID,
   raw-file digest, and delegated executable digest when present. Clear only
   the abandoned lock with `taskrail lock clear <lock-id> --expect-sha256 <digest>`.
   A live same-host owner or changed lock refuses; do not bypass that refusal.
   [Transaction recovery](commands.md#recovering-retained-transactions) takes
   precedence if a recovery fence exists.
3. A new run with the same binary can now stage and launch even if the abandoned
   files remain. File removal is optional, not a prerequisite for rerunning.
4. To reclaim disk space, keep the repository quiescent as in step 1. Resolve the
   absolute common directory with `git rev-parse --path-format=absolute
   --git-common-dir`, then inspect its `taskrail/executables/` entries without
   following symlinks. Select only exact abandoned paths, never a wildcard or
   the whole directory. Check that the files are regular, their SHA-256 matches
   the filename's digest, and any owner link has the same file identity. Legacy
   versions used the digest alone as filename. Recheck identities and bytes
   immediately before removing only those inspected entries and their matching
   owner links with OS file tools. Refuse unexpected paths, links, changed bytes,
   uncertain ownership, or an inability to exclude concurrent launches; leave
   them for investigation. On Windows, also wait for executable handles to close.

`lock clear` removes only the lock record, never executable files or transaction
data. It is not a staged-file garbage collector. Taskrail does not infer that a
staged file is abandoned from its age or from lock absence.

## The `taskrail-loop` skill

Use the packaged `taskrail-loop` skill when an operator needs one interactive,
provider-neutral loop run. It elicits unresolved caller-owned choices, explains
the structured dry-run and requires confirmation, then supervises the one
coordinator invocation through its external result file. The skill reports
worker, integration, delivery, and optional caller-owned CI evidence, but never
selects work, mutates Git or planning state, or substitutes for Taskrail's
coordinator or review adapter.
