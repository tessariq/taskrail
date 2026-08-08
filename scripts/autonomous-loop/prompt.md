You are executing one unattended Taskrail task in the Taskrail source checkout.

Task: `{{TASK_ID}}`

Follow this exact workflow:

1. Read root `AGENTS.md`, `CLAUDE.md`, the selected task, its dependencies, its
   referenced spec section, and the relevant implementation and tests.
2. Work only on `{{TASK_ID}}`. Never invoke `taskrail next`, continue another
   task, or edit anything under `scripts/autonomous-loop/`.
3. Invoke Taskrail only through `${TASKRAIL:-taskrail}`. The outer runner points
   `TASKRAIL` at a freshness-checking wrapper. Immediately before every Taskrail
   command that writes tracked state, also run
   `TASKRAIL="$AUTONOMOUS_TASKRAIL_BINARY" task taskrail:check`. Stop if it fails
   and apply only the remedy it names. Never invoke `bin/taskrail` directly.
4. After the initial freshness check, run
   `${TASKRAIL:-taskrail} start {{TASK_ID}}`.
5. Implement the smallest correct change. Start behavior changes with a failing
   test when practical, preserve repository architecture, and avoid unrelated
   refactors.
6. Run applicable formatting, targeted tests, `go vet ./...`, `go test ./...`,
   Taskrail validation, skill/task-body checks, and sandboxed manual testing for
   visible workflow behavior. Remove all ephemeral manual-test code afterward.
7. First use one fresh subagent for a behavior-preserving simplification pass.
   Apply accepted simplifications, rerun affected checks, and freeze the resulting
   snapshot. Then use a separate fresh subagent for correctness review. Before
   each pass, inspect the available installed skills and subagents. Select the
   most specialized applicable capability. Prefer dedicated code-simplifier and
   code-reviewer capabilities; otherwise use a general-purpose fresh subagent
   with an explicit simplification or correctness lens. Give each subagent the
   frozen current implementation snapshot plus relevant task, spec, and test
   context. Subagents return findings or proposed changes; the parent applies
   fixes. Parent-context self-review does not satisfy either pass. If fresh
   subagent delegation is unavailable or fails, block and verify fail.
8. Classify review findings as high, medium, or low.
   Fix high- and medium-severity current-scope findings within the configured
   review budget.
   Leave low-severity observations report-only unless acceptance criteria, the
   specification, an invariant, or required test evidence makes correction
   mandatory; a mandatory low is current scope and must be fixed. After any
   material correctness fix, rerun affected checks, freeze the changed final
   bytes, and obtain another fresh correctness review while the budget remains.
   A clean correctness review stops. If any required finding remains unresolved
   or final bytes requiring re-review cannot be reviewed within budget, block and
   verify fail. Record each pass's capability, findings, and dispositions in
   verification details for operator review.
9. Do not create follow-up tasks during an unattended iteration because the
   operator-owned queue cannot be updated by the child. Block instead when a
   prerequisite follow-up is required.
10. If source changed after the initial start, run `task taskrail:install`, then
   run `task taskrail:check` before the final lifecycle writers.
11. On success, run `${TASKRAIL:-taskrail} complete {{TASK_ID}} --note "..."`,
   then run `${TASKRAIL:-taskrail} verify {{TASK_ID}} --result pass --summary
   "..." --details "..."`. Never verify pass before completion.
12. If implementation cannot safely proceed, run
    `${TASKRAIL:-taskrail} block {{TASK_ID}} --reason "..."`, then
    `${TASKRAIL:-taskrail} verify {{TASK_ID}} --result fail --summary "..."
    --details "..."`. Never complete a blocked task. If completion succeeds but
    verification fails, stop without rewriting the lifecycle.
13. Leave all intended code, tests, docs, task files, and CLI-regenerated
    `planning/STATE.md` in the worktree. Do not stage, commit, push, fetch, pull,
    merge, rebase, amend, reset, create refs, alter Git identity/configuration, or
    bypass hooks. The outer runner owns Git delivery.
14. Write exactly one valid Conventional Commit message to
    `$AUTONOMOUS_COMMIT_MESSAGE_FILE`. Its subject must describe this task and end
    with `({{TASK_ID}})`. Add no attribution trailer.

Exit zero only after publishing the commit message and leaving either a valid
completed/pass outcome or a valid blocked/fail outcome. Otherwise exit non-zero.
