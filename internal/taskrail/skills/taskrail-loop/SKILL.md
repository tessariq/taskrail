---
name: taskrail-loop
description: Interactively configure and supervise one bounded Taskrail loop invocation
---

# taskrail-loop

Interactively configure and supervise one bounded Taskrail `loop` invocation.
You are the operator-facing parent supervisor. Taskrail is the coordinator: it
alone selects work, owns task lifecycle writes, integrates accepted workers,
reprojects state, updates refs, and publishes its result. A caller-owned review
adapter owns its own external provider credentials and operations.

Require the installed `taskrail` binary on `PATH`. Invoke it only through
`${TASKRAIL:-taskrail}`. Never use a source-tree runner.

## Authority Boundaries

The skill does not select a task, does not cherry-pick, does not stage, does not commit,
does not merge, and does not push. It does not enter a worker or
integration workspace, repair state, change refs, clear a lock, or retry a
worker. It does not infer a provider command, credentials, a review adapter, a
CI result, approval, quota state, reset time, or destructive authorization.

Treat structured dry-run and terminal result-file envelopes as authoritative.
Streamed coordinator, worker, adapter, and provider output is diagnostic only;
it can be incomplete or sensitive and must not override the terminal envelope.

## Gather Unresolved Choices

Ask only for choices not already supplied by the caller. State these safe defaults
before asking:

- Sequential execution: `--parallel 1`.
- One task: `--max-iterations 1`.
- One broad implementation-review round: omit `--max-review-rounds` unless the
  caller explicitly needs `1` or `2`.
- No Taskrail deadline: omit `--timeout` unless the caller selects a positive
  duration.
- Parallel clone depth `1` and workspace retention `failure`.
- Local delivery: `--delivery local`; remote CI is `not_checked` unless the
  caller supplies a separate read-only CI observer.

Ask for an opaque child argv after `--`, not a shell command string. It must name
a real caller-owned executable and arguments. Stop if a missing provider command,
model option, credential decision, review adapter, or required destructive
authorization is missing. Do not guess or substitute one.

For parallel execution, ask for a positive width, an optional existing private
workspace root outside the repository, clone depth, and retention policy. For
review delivery, require `--delivery review`, one explicit caller-owned
`--review-adapter <path>`, and a new `--result-file <absolute-external-path>`;
reject adapter intent in local mode. Ask whether the operator requires only
Taskrail's local aggregate gate, or also an explicitly supplied read-only CI
observation. The CI observer is external evidence, never Taskrail attestation.

Ask for a new absolute result file destination outside the repository, Git
metadata, and managed inputs. It must be caller-owned, in an existing safe
directory, and absent. Do not reuse a result destination after any attempt.

## Preview And Confirm

Build and run the exact dry-run corresponding to the resolved choices. Dry-run
does not accept `--result-file` or a child argv:

```sh
${TASKRAIL:-taskrail} loop --dry-run --json \
  --max-iterations <n> --max-review-rounds <n> --timeout <duration> \
  --allow-prompt-override-sha256 <digest> \
  --parallel <n> --workspace-root <absolute-external-path> \
  --clone-depth <n|full> --keep-workspaces <never|failure|always> \
  --delivery <local|review> --review-adapter <path>
```

Omit flags whose safe default remains selected. If a replacement prompt is in
use, require the caller to supply its exact `--allow-prompt-override-sha256`
authorization; never compute or invent the digest.

Only `action: run` permits confirmation. Report `action:none` as a no-work,
non-launching terminal outcome, and stop on `action:invalid`; neither permits a
result destination, confirmation, or execution.

Explain the structured dry-run before asking for confirmation:

- The selected task or ordered parallel frontier, deterministic rank, and every
  excluded, held, or ineligible task.
- Frozen base/ref and Git, lock, storage, prompt, review, execution, workspace,
  clone, retention, and delivery policies.
- That workers may finish out of order but integration considers accepted results
  in deterministic rank; a failed worker receives no replacement or retry.
- The local aggregate gate, review-adapter boundary, and remote-CI boundary.
- That Taskrail, not this skill, owns lifecycle, integration, state
  reprojection, refs, and publication.

Require explicit confirmation of that exact plan. A refusal, changed plan, or
failed dry-run stops without a mutating loop invocation, recovery apply, or
external write. Do not silently run a second dry-run or change policy after
confirmation; return to the operator for a fresh review when inputs change.

## Launch Once And Supervise

After confirmation, allocate the already-reviewed new result destination and
launch the coordinator exactly once with the same invocation choices:

```sh
${TASKRAIL:-taskrail} loop \
  --max-iterations <n> --max-review-rounds <n> --timeout <duration> \
  --allow-prompt-override-sha256 <digest> \
  --result-file <absolute-external-path> --parallel <n> \
  --workspace-root <absolute-external-path> --clone-depth <n|full> \
  --keep-workspaces <never|failure|always> --delivery <local|review> \
  --review-adapter <path> -- <child-command> <args...>
```

For review delivery, do not omit `--result-file`. Preserve stdout and stderr in a safe
caller-owned destination outside committed state when possible. Warn that those
streams may be sensitive and incomplete, and report preservation failure
explicitly. Never use free-form output as authority, launch a replacement
worker, launch a second frontier, or change invocation policy while supervising.
There is no retry of a coordinator, worker, or adapter operation.

On every exit, including interruption or non-zero exit, read the result file if
it exists as a schema envelope. A `result` carries the terminal diagnostic.
A postflight `error` carries that diagnostic under `error.details`. A common error may include a recovery record in `error.details.recovery`; report that
record without inferring worker, integration, or delivery facts. A malformed
file, or an absent file, means terminal evidence is unavailable. In every case,
report only available `outcome`, `next_action`, worker, integration, delivery,
retained-workspace, and result-file facts; do not invent missing fields. Preserve
the coordinator's process result and do not claim that an interrupt succeeded.

Report each parallel worker in deterministic rank, not completion order: task,
lifecycle outcome, candidate head/commit, containment or violations,
verification evidence, integrated state, and retained workspace. Then report
serial integration: accepted and unpublished tasks, one commit per accepted
task, mechanical `STATE.md` reprojection, bounded conflict-child outcome and
affected checks, aggregate-child/full-gate result, and final integrated head.

For local delivery, success requires the coordinator's aggregate build/test gate
and its guarded fast-forward result. Report remote CI as `not_checked` unless a
caller supplied a separate read-only CI observation, and label that observation
as external evidence. For review delivery, report every adapter-provided check
state. `checks: pass` is required before saying a merge is green; `fail`,
`pending`, `unknown`, timeout, drift, or adapter refusal is non-success. Never
bypass checks, force push, infer approval, or claim remote success not returned
by the caller-owned adapter.

## Recovery And Quota Observations

For a product transaction recovery, first inspect:

```sh
${TASKRAIL:-taskrail} lock status --json
```

Only when the retained transaction is blocked by that exact reported owner,
preview recovery with the exact observed operands:

```sh
${TASKRAIL:-taskrail} recover <transaction-id> \
  --take-over-lock <lock-id> --expect-sha256 <digest> --json
```

Explain the exact-observation takeover and mechanically derived action. Require
fresh explicit confirmation before repeating those exact operands with `--apply`.
Do not clear a lock, apply recovery, reuse a failed candidate, or retry work
automatically. Route `completed_unverified` to verification-only recovery; never
repeat complete.

If a caller-owned adapter, provider CLI, or operator reports quota exhaustion,
label the source and any reset text as attributed external evidence. Quote a
timezone or offset only when supplied; never parse, normalize, or invent a reset
instant. Keep supervising the one running coordinator so launched siblings can
drain and ordinary integration, delivery, and unpublished-candidate outcomes can
settle. Do not interrupt it, refund or carry forward consumed attempts, skip an
aggregate gate, wait in the background, persist launch intent, or automatically
relaunch.

Post-reset work is a new foreground invocation only: explain retained and
unpublished work, allocate a new result destination, obtain a newly explicit
finite iteration and parallel budget, run a fresh dry-run and preflight, and
require fresh confirmation. There is no background wait, session resume, worker
resume, and the skill must never automatically relaunch.

## Final Report

Conclude with separate statuses for workers, integration, delivery, local gate,
remote CI, result file, and recovery. Claim overall success only when the
operator's confirmed delivery and CI policy are satisfied. Otherwise report the
safe next action from the terminal envelope and the remaining risk without
performing another mutation.
