# Agent Steering

Prompt guidance for deterministic tracked-work execution in Taskrail.

## Baseline Rules

- Use `go run ./cmd/taskrail ...` for tracked-work transitions.
- Never hand-edit `planning/STATE.md` frontmatter. Re-project mechanical drift
  with `taskrail repair --apply`.
- Treat `planning/STATE.md` as current state, never as a task/session log. Put
  durable context in task implementation notes, blocker reasons, portable
  verification summaries/reports, or follow-up tasks.
- Treat optional `planning/NOTES.md` as human-owned repository context. Read it
  when relevant; edit it only on explicit human instruction.
- Never hand-edit task status fields; route every transition through the CLI.
- Follow TDD for code changes.
- Keep tests focused and deterministic.
- Keep product scope anchored to the Taskrail specs.
- Run manual testing and persist artifacts when the change affects visible Taskrail workflow behavior.

## Review Cadence

Review is a bounded workflow with an explicit trigger, not a default loop after
every planning edit.

| Situation | Required review cadence |
|---|---|
| Ordinary spec or task-file edit | Run the applicable deterministic checks once on settled bytes. Do not start semantic review unless explicitly requested. |
| Requested advisory review | Run one review wave over one frozen snapshot, batch dispositions and accepted fixes, then stop. |
| Formal post-spec publication | Run the four required lenses and only the exact-byte reruns required by the active spec contract. |
| Reviewed decomposition | Run at most two passes in one explicitly initiated session. A later change invalidates that session and stops; another session requires explicit human initiation. |
| Release gate | Run at the named release boundary. Restart only after a concrete blocker is remediated and the gate is explicitly resumed. |

"Fresh context" means isolation for one required lens or pass, not repeated review
of unchanged bytes. Confidence-seeking alone is not a trigger for another wave.

## Autonomous Backlog

1. Validate state.
2. Select the next eligible task deterministically.
3. Start the selected task.
4. Implement it in a TDD loop.
5. Run the appropriate test tiers.
6. Run manual testing when the task changes user-visible Taskrail behavior.
7. Create a follow-up task for unresolved backlog-worthy findings.
8. On success, run `complete` and then `verify --result pass`.
9. If the task cannot safely proceed, run `block --reason` and then `verify --result fail`; deliberate rework may verify fail while remaining `in_progress`.

## Directed Task

1. Validate state.
2. Read the requested task file.
3. Start that task only.
4. Implement only the requested scope.
5. Run manual testing when the task changes visible Taskrail behavior.
6. Finish the lifecycle branch first, then verify it through the Taskrail CLI.

## Verification Runs

Use verification-focused runs when:

- you need to record evidence for a completed task
- you need to capture follow-up work
- you need to audit whether the repo is aligned with its active spec
