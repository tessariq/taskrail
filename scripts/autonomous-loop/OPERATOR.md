# Temporary Parent-Agent Operator Bridge

`operator.sh` is a temporary, repository-local bridge for a parent agent operating
the source-checkout loop. It is not Taskrail product behavior, adopter guidance,
or part of any embedded or committed packaged skill. T-258 removes this file,
`operator.sh`, its tests, and all recovery guidance with the complete
`scripts/autonomous-loop/` directory.

Run it interactively from a clean `main` checkout:

```sh
scripts/autonomous-loop/operator.sh
```

The bridge asks only unresolved runner and CI-observation choices, verifies local
tools and Taskrail binary freshness, executes and displays the exact runner
dry-run, explains its authority boundaries, and requires `RUN` before invoking
the matching live runner at most once. The runner remains the sole authority for
worker lifecycle, serial integration, mechanical `STATE.md` reprojection, Git
changes, aggregate gates, fast-forward delivery, and push. The bridge only
supervises and reports those outcomes.

After a runner-owned push, the bridge uses read-only `gh run list` observations to
wait for every workflow attached to the exact pushed commit and for every
operator-requested workflow name. Missing, pending, failed, and cancelled runs are
non-success; local checks and push success never imply remote green.

On failure, keep the reported ignored coordinator, worker, wrapper, and operator
logs and any retained workspace. They are transient diagnostics, may contain
sensitive provider output, may be incomplete, and are not Taskrail evidence.
Only an absolute private XDG bundle explicitly reported by the runner is eligible
for delivery recovery. The bridge validates its complete/delivered marker,
repository, task, outcome, base, report, message, queue, candidate identity, and
current source preconditions before offering the exact delivery-only command and
requiring `RESUME-DELIVERY`. A retained workspace or free-form child output is
never a bundle.

Quota and reset statements come only from the selected backend or operator. The
bridge labels the exact statement and source as attributed, potentially heuristic
external evidence. It does not interrupt the current runner: launched siblings,
ordinary integration, delivery, and unpublished outcomes settle first. It never
retries or replaces a worker, changes the queue at runtime, refunds an attempt,
skips a gate, resumes agent execution, schedules a wake-up, or automatically
launches after a reset. Later execution starts this bridge anew with a fresh
preflight, exact dry-run, finite budget, and confirmation.
