You are executing one unattended Taskrail task in the Taskrail source checkout.

Task: `{{TASK_ID}}`

This is headless. Nobody can answer questions. Work only on this task, never run
`taskrail next`, continue another task, or edit `scripts/autonomous-loop/`.

Track these concise checkpoints with the available task-list tool. Do not mark a
checkpoint complete without the evidence it names:

1. Understand and frame
2. Start and implement
3. Checks and manual testing
4. Simplification
5. Correctness review
6. Fix and recheck
7. Follow-up and lifecycle
8. Delivery metadata

## Understand And Frame

Read root `AGENTS.md`, `CLAUDE.md`, the selected task, its dependencies, its
referenced spec section, and relevant implementation and tests. Before editing,
record the independently meaningful observable outcome, user or operator impact,
affected invariants, acceptance/spec boundaries, and intended evidence. If the
task cannot reach one verified result without unresolved scope, stop for reviewed
decomposition or clarification instead of implementing an arbitrary slice.

Stopping always means the blocked path below: `${TASKRAIL:-taskrail} block` with
a reason, then a failing verification that names what an operator must decide.
Exiting without that pair is never a stop.

## Start And Implement

Invoke Taskrail only through `${TASKRAIL:-taskrail}`. The runner supplies a
freshness-checking wrapper. Stop if it fails and apply only the remedy it names.
Never invoke `bin/taskrail` directly.

Run `${TASKRAIL:-taskrail} start {{TASK_ID}}`, then implement the smallest correct
change. Start behavior changes with a failing test when practical. Preserve the
repository architecture and avoid unrelated refactors.

## Checks And Manual Testing

Run applicable formatting, targeted tests, `go vet ./...`, `go test ./...`,
Taskrail validation, skill parity, task-body checks, and sandboxed manual testing
for visible workflow behavior. Remove ephemeral manual-test code afterward.

Never mutate external systems, production data, credentials, live services, or
resources outside this repository. Read-only inspection is allowed when required.
Block instead of guessing around an operator decision or external write.

## Simplification And Review

First use one fresh subagent for a behavior-preserving simplification pass. Apply
accepted simplifications, rerun affected checks, and freeze the resulting
snapshot. Then run a correctness-review round over the frozen current
implementation snapshot.

Before each pass, inspect the available installed skills and subagents. Prefer
dedicated code-simplifier and code-reviewer capabilities; otherwise use a
general-purpose fresh subagent with an explicit simplification or correctness
lens. Each review round uses one to three fresh review subagents. Give every
reviewer the same frozen snapshot plus task, spec, and test context, but assign
each a different explicit review lens so their scopes do not duplicate one
another. Run them concurrently when the backend supports it. Subagents return
findings; the parent applies fixes. Parent-context self-review does not satisfy
either pass. If fresh delegation is unavailable or fails, block and verify fail.

Correctness review covers behavior, tests, security, error handling, edge cases,
unnecessary complexity, and domain fit. Findings cite concrete evidence such as
a file and line, reproduced behavior, test result, or contract clause.

Classify every finding with severity and exactly one disposition: `fix-now`,
`separate-followup`, `blocked`, or `rejected`. Record a concise rationale and
evidence. Fix high- and medium-severity current-scope findings. Leave low-severity
observations report-only unless acceptance, specification, an invariant, or
required evidence makes them mandatory; a mandatory low is current scope and
must be fixed. A review-round limit never turns current work into a follow-up.

When a finding identifies a missing or weak test, add the strengthened test,
temporarily introduce the specific regression, demonstrate that the test fails,
restore the correct implementation, demonstrate that it passes, and remove all
deliberate regression code. Record both outcomes concisely.

Run at most two correctness-review rounds. A clean round stops early. After fixing
any findings from a round, use a fresh code-simplifier subagent on the changed
implementation, apply accepted behavior-preserving simplifications, and rerun
affected checks. Freeze those bytes before starting round two.

After the second round, do not request another correctness review. Fix its
required findings, run the required post-fix simplifier, and require that the
final applicable build and test checks pass, including `go build ./cmd/taskrail`
and `go test ./...`. The resulting simplified bytes do not require a third review.
Continue to successful completion even though the post-review fixes and
simplifications were not reviewed again. If final checks fail and cannot be fixed
within the task, use the blocked path. Record any residual review risk in
verification details.

## Follow-Up And Lifecycle

A `separate-followup` must be a genuinely separate, spec-anchored outcome. It may
not offload unfinished acceptance, evidence, or integration, and must not be
speculative, duplicate, layer-only, file-only, or non-actionable.

Under delegation, create at most one follow-up and only through the selected
task's fresh `${TASKRAIL:-taskrail} verify {{TASK_ID}} --create-followup` command.
The verification report must name it. Include an advisory `run` or `hold`
recommendation and rationale in verification details. The recommendation does
not authorize execution: the parent runner always queues it as held, and the
child never edits `queue.tsv`. If more than one follow-up is necessary, do not
create an arbitrary subset; block, verify fail, describe the proposals, and stop.
Write the advisory exactly once, in this exact form:
`follow-up recommendation: run|hold - <rationale>`. It may start its own line or
follow other details prose; the rationale runs to the end of that line and must
not be empty. A second marker, another mode word, or a missing rationale is
rejected and stops delivery.

If source changed after start, run `task taskrail:install`, then
`task taskrail:check` before final lifecycle writers.

On success, run `${TASKRAIL:-taskrail} complete {{TASK_ID}} --note "..."`, then
`${TASKRAIL:-taskrail} verify {{TASK_ID}} --result pass --summary "..." --details
"..."`. Add follow-up flags to that verification only when one valid separate
follow-up is required. Never verify pass before completion.

If implementation cannot safely proceed, run `${TASKRAIL:-taskrail} block
{{TASK_ID}} --reason "..."`, then `${TASKRAIL:-taskrail} verify {{TASK_ID}}
--result fail --summary "..." --details "..."`. Never verify fail while leaving
the task in progress; the parent accepts only completed/pass or blocked/fail.
Never complete a blocked or failing task. A blocked run may create one follow-up under the same single-follow-up
rules, but only for a genuinely separate outcome; work this task still owns is
never offloaded that way. If completion succeeds but passing verification fails, stop without
repeating completion or compensating with block.

Verification details concisely record acceptance/check/manual evidence, each
review capability and finding disposition, mutation proof, any follow-up
recommendation, and unresolved risks. Do not quote reviewer responses verbatim.

## Delivery Metadata

Leave intended code, tests, docs, task files, and CLI-regenerated `planning/STATE.md`
in the worktree. Do not stage, commit, push, fetch, pull, merge, rebase, amend,
reset, create refs, alter Git identity/configuration, bypass hooks, or modify loop
controls. The parent runner owns Git delivery.

Write exactly one valid Conventional Commit message to
`$AUTONOMOUS_COMMIT_MESSAGE_FILE` before the final response. Its subject must end
with the short task key `({{TASK_KEY}})`, not the full slugged ID. Add no
attribution trailer.

The parent may terminate this process at its configured deadline. Timeout never
retries. Do not spawn detached processes.

Exit zero only after publishing the commit message and leaving a valid
completed/pass or blocked/fail outcome. Otherwise exit non-zero. Keep the final
response terse: outcome, checks, review dispositions, follow-up decision, and
remaining risks.
