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
7. Use one fresh simplification review and one fresh correctness review. Resolve
   every concrete current-scope finding within the task's configured review
   budget, then rerun affected checks. Do not create follow-up tasks during an
   unattended iteration because the operator-owned queue cannot be updated by the
   child. Record separate outcomes in verification details for operator review;
   block instead when a prerequisite follow-up is required.
8. If source changed after the initial start, run `task taskrail:install`, then
   run `task taskrail:check` before the final lifecycle writers.
9. On success, run `${TASKRAIL:-taskrail} complete {{TASK_ID}} --note "..."`,
   then run `${TASKRAIL:-taskrail} verify {{TASK_ID}} --result pass --summary
   "..." --details "..."`. Never verify pass before completion.
10. If implementation cannot safely proceed, run
    `${TASKRAIL:-taskrail} block {{TASK_ID}} --reason "..."`, then
    `${TASKRAIL:-taskrail} verify {{TASK_ID}} --result fail --summary "..."
    --details "..."`. Never complete a blocked task. If completion succeeds but
    verification fails, stop without rewriting the lifecycle.
11. Leave all intended code, tests, docs, task files, and CLI-regenerated
    `planning/STATE.md` in the worktree. Do not stage, commit, push, fetch, pull,
    merge, rebase, amend, reset, create refs, alter Git identity/configuration, or
    bypass hooks. The outer runner owns Git delivery.
12. Write exactly one valid Conventional Commit message to
    `$AUTONOMOUS_COMMIT_MESSAGE_FILE`. Its subject must describe this task and end
    with `({{TASK_ID}})`. Add no attribution trailer.

Exit zero only after publishing the commit message and leaving either a valid
completed/pass outcome or a valid blocked/fail outcome. Otherwise exit non-zero.
