You are executing one unattended Taskrail task in the Taskrail source checkout.

Task: `{{TASK_ID}}`

This is headless. Nobody can answer questions. Work only on this task, never run
`taskrail next`, continue another task, or edit `scripts/autonomous-loop/`.

Track these concise checkpoints with the available task-list tool. Do not mark a
checkpoint complete without the evidence it names:

1. Understand and frame
2. Start and implement
3. Verify
4. Focused review
5. Fix and re-verify
6. Follow-up and lifecycle
7. Delivery metadata

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

## Verify

Run applicable formatting, targeted tests, `go vet ./...`, `go test ./...`,
Taskrail validation, skill parity, task-body checks, and sandboxed manual testing
for visible workflow behavior. Remove ephemeral manual-test code afterward.

Never mutate external systems, production data, credentials, live services, or
resources outside this repository. Read-only inspection is allowed when required.
Block instead of guessing around an operator decision or external write.

## Verification And Review

After implementation, run the applicable deterministic checks and manual tests
described above. Fix failures before requesting independent review.

Inspect the resulting diff for obvious unnecessary complexity and simplify it
when doing so clearly preserves behavior. Rerun affected checks after any such
change. A separate simplification subagent is not required.

Freeze the verified implementation snapshot, then run one broad correctness review.
Use one fresh reviewer by default. Give it the frozen implementation plus the
relevant task, specification, tests, and verification evidence. Choose an explicit
review lens based on the actual risks of the task. Review may cover behavior,
tests, security, error handling, edge cases, unnecessary complexity, and domain
fit, but do not create additional reviewers merely to cover every category.

Use a second or third concurrent reviewer only when a distinct lens is
independently relevant to the task's risk and examines something the first
reviewer is unlikely to cover. Reviewers inspect the same frozen snapshot and
must not duplicate one another's lens. Run additional reviewers concurrently when
the backend supports it. Subagents return findings; the parent applies fixes.
Parent-context self-review does not satisfy the independent review. If required
fresh delegation is unavailable or fails, block and verify fail.

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

After fixing review findings, rerun all affected deterministic checks. Do not
start another broad review merely because the implementation changed.

One broad round is the normal workflow. A clean review ends broad review
immediately; proceed to final checks and lifecycle. Use a second broad round only
when the first round exposes a distinct unresolved risk that deterministic
verification does not adequately cover and the repository's effective maximum
permits it, or when repository or task context independently warrants the
additional review. Freeze the verified repaired candidate before that optional
round. Broad review never exceeds the effective maximum or two rounds. The
review-round limit never turns current work into a follow-up.

If review fixes materially change product or test bytes, freeze the final
candidate and run one narrow final-diff review over exactly the review-induced
change. Its only lenses are fix-induced regressions, integration breakage, and
behavior drift. It never starts another broad review round.

A clean final-diff review allows closure after the final applicable build and test
checks pass, including `go build ./cmd/taskrail` and `go test ./...`. If the
final-diff review reports a current-scope finding, repair it and rerun affected
deterministic checks. If objective evidence demonstrates that the finding is
closed, such as a regression test, build, static check, or deterministic
integration or manual reproduction, no further model review is required. If the
repair cannot be demonstrated adequately by deterministic evidence, leave the
task in progress, record failing verification as rework, and stop for operator
review. If final checks fail and cannot be fixed within the task, use the blocked
path. Record the final-diff disposition, closure evidence, and residual risk in
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
the task in progress except for unresolved review rework. For review rework that
cannot be closed with adequate objective evidence, leave the task in progress and
run `${TASKRAIL:-taskrail} verify {{TASK_ID}} --result fail --summary "..."
--details "..."`; the parent accepts only completed/pass, blocked/fail, or
in-progress/fail.
Never complete a blocked or failing task. A blocked run may create one follow-up under the same single-follow-up
rules, but only for a genuinely separate outcome; work this task still owns is
never offloaded that way. If completion succeeds but passing verification fails, stop without
repeating completion or compensating with block.

Verification details concisely record acceptance/check/manual evidence, the
selected review lens, rationale for any additional lens or round, every finding
disposition, deterministic closure evidence, mutation proof, any follow-up
recommendation, and unresolved risks. Do not quote reviewer responses verbatim.

## Delivery Metadata

Leave intended code, tests, docs, task files, and CLI-regenerated `planning/STATE.md`
in the worktree. Do not stage, commit, push, fetch, pull, merge, rebase, amend,
reset, create refs, alter Git identity/configuration, bypass hooks, or modify loop
controls. The parent runner owns Git delivery.

Write exactly one valid Conventional Commit message to
`$AUTONOMOUS_COMMIT_MESSAGE_FILE` before the final response. Its subject must end
with the short task key `({{TASK_KEY}})`, not the full slugged ID. After a blank
line, include a concise body explaining the commit's intent, context, and
non-obvious decisions rather than merely restating the diff. Wrap body lines at
72 characters. Add no attribution trailer.

After writing the file, run the repository's exact prospective-message check:

```bash
scripts/check-commit-msg.sh "$AUTONOMOUS_COMMIT_MESSAGE_FILE"
```

If it fails, repair the same file and rerun the checker until it passes. Then
mechanically check the selected short task key:

```bash
commit_subject="$(grep -vE '^[[:space:]]*#' "$AUTONOMOUS_COMMIT_MESSAGE_FILE" | sed '/^[[:space:]]*$/d' | head -n 1)"
[[ "$commit_subject" == *"({{TASK_KEY}})" ]]
```

If that check fails, repair the file and repeat both checks. Complete this
correction inside this one child process; it is not another child launch or
autonomous retry. The parent independently revalidates both conditions before
staging or delivery, so its checks remain a trust-boundary backstop.

The parent may terminate this process at its configured deadline. Timeout never
retries. Do not spawn detached processes.

Exit zero only after publishing the commit message and leaving a valid
completed/pass, blocked/fail, or in-progress/fail outcome. Otherwise exit non-zero. Keep the final
response terse: outcome, checks, review dispositions, follow-up decision, and
remaining risks.
