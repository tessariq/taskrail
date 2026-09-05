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

## The `taskrail-loop` skill

Use the packaged `taskrail-loop` skill when an operator needs one interactive,
provider-neutral loop run. It elicits unresolved caller-owned choices, explains
the structured dry-run and requires confirmation, then supervises the one
coordinator invocation through its external result file. The skill reports
worker, integration, delivery, and optional caller-owned CI evidence, but never
selects work, mutates Git or planning state, or substitutes for Taskrail's
coordinator or review adapter.
