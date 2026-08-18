# Temporary Autonomous Loop

- These files are temporary operator-owned source-checkout controls, not Taskrail
  product behavior or adopter guidance.
- Ordinary queued tasks must not modify this directory. A `run` row whose open
  task file names `scripts/autonomous-loop` is rejected by queue validation before
  any agent launches, because such a task can only block; make it `hold-operator`
  and execute it as operator. The check is a literal path match and exempts
  completed and cancelled tasks.
- `queue.tsv` is reviewed source policy and remains immutable during a child run.
  The parent may append only the exact fresh verification-created follow-up as
  `hold-operator`; no child recommendation authorizes execution and the new row
  is frozen out of the current invocation.
- The runner uses Claude by default. Select one backend for the complete invocation
  with `--backend claude` or `--backend opencode`; dry-run reports that selection.
- Select an invocation-wide model with `--model <model>` and reasoning level with
  `--effort <level>`. Omitted values preserve backend defaults. The runner passes
  the model through unchanged, maps effort to Claude's `--effort` or OpenCode's
  provider-specific `--variant`, and leaves value validation to the selected CLI.
  For example, use `--backend claude --model opus --effort high` or
  `--backend opencode --model anthropic/claude-opus-4-1 --effort high`.
- `--parallel <n>` opts into one bounded batch per invocation. The effective
  width is the smaller of `<n>` and the `--max-iterations` budget; width `1` is
  byte-identical to the sequential invocation and rejects the parallel-only
  `--clone-depth` and `--keep-workspaces` flags. A batch selects one
  dependency-ready frontier from `run` rows in queue order (todo status,
  completed dependencies, never held rows), and refuses before creating any
  workspace, clone, or child when the checkout is dirty, detached, bare, off
  `main`, has a branch tip differing from `HEAD`, or fails the existing binary
  freshness and queue preconditions. `--dry-run --parallel <n>` prints the
  exact frontier, effective width, frozen base, workspace/clone/retention
  policy, and the per-row reason every other open row was excluded, creating
  nothing.
- Each selected task runs in one private shallow clone (`--no-local
  --single-branch --no-tags --depth 1`, overridable with `--clone-depth
  <positive|full>`) beneath an invocation-private workspace root outside the
  worktree. Workers never select work, never reach the source checkout or
  another clone, and are never retried; the first failure launches no
  replacement and no new frontier. Delivery is serial and local: one
  integration clone at the frozen base replays accepted candidates in frontier
  order into one commit each, re-projects `planning/STATE.md` through
  `taskrail repair --apply`, and permits exactly one bounded integration child
  per semantic conflict (which may not drop acceptance, delete a detecting
  test, or integrate another candidate). The repository's full gate runs once
  over the final integration head; publication re-verifies source cleanliness,
  attached ref, and base `HEAD`, performs one non-force fast-forward plus the
  ordinary push, and refuses on drift without reset, checkout overwrite,
  rebase, or stash. Zero accepted candidates is a failed batch; failed
  workspaces are retained per `--keep-workspaces never|failure|always`
  (default `failure`) and retained paths never enter committed state. Queue
  mutation stays parent-owned and post-batch: only exact fresh
  verification-created follow-ups are appended as `hold-operator` inside the
  owning candidate's integration commit. This bootstrap batch satisfies none
  of T-333, T-334, or T-335; the product parallel tasks and tests remain
  independently required, and the batch is removed with this directory at
  retirement.
- Agent attempts default to a two-hour timeout; override it with `--timeout 30m`
  or another positive `s`, `m`, or `h` duration. Timeout never retries. A valid
  terminal outcome interrupted before delivery names a private XDG-state bundle;
  inspect it, then use `--resume-delivery <absolute-bundle-path>` to revalidate and
  perform delivery without launching another agent.
- The child validates its prospective commit message with the repository's exact
  checker and selected short task key before zero exit, repairing metadata and
  repeating both checks inside the same process when necessary. That correction
  is not a retry. The parent repeats both checks independently before staging or
  delivery; never weaken those trust-boundary backstops or rely on hooks alone.
- The shared prompt runs deterministic verification before one focused fresh
  correctness reviewer by default. Simplification remains required consideration,
  but separate simplification delegation is optional. Additional concurrent
  reviewers require distinct independently relevant risks and inspect the same
  frozen snapshot; the ceiling remains three. One broad round is normal, and a
  second is exceptional, risk-driven, and bounded by repository policy. Fix high
  and medium current-scope findings; low findings are report-only unless
  acceptance, specification, invariants, or required evidence makes them
  mandatory. Repairs receive affected deterministic checks, not an automatic
  broad rerun. Material review-induced product or test changes receive one narrow,
  non-recursive final-diff review. A clean final diff plus green checks permits
  completion. A final-diff finding is repaired and may complete when objective
  evidence demonstrates closure; otherwise it stops as in-progress/fail for
  operator review. Required fresh review delegation remains fail-closed, and the
  round limit never creates a follow-up for unfinished current scope.
- Do not add logs, results, session data, credentials, or generated files here.
  Runtime output belongs under ignored `planning/artifacts/runs/` or external
  temporary storage.
- `recovery.sh` is sourced by the runner and contains only explicit XDG bundle
  publication and delivery-resume checks; it never launches or resumes an agent.
- `queue.sh` (task-file parsing and queue validation) and `parallel.sh` (the
  opt-in bounded batch) are likewise sourced by the runner; `test-parallel.sh`
  is sourced by `test.sh` and holds the batch fixtures.
- Keep the runner finite and fail-closed. Never add retries, automatic recovery,
  existing-task continuation, hook bypass, force push, fetch/pull/rebase, amend,
  reset, or hidden queue mutation. Delivery recovery is explicit, bundle-bound,
  and never resumes agent execution.
- Only the designated held cleanup task may delete this complete directory.
- This complete directory and every live invocation reference must be absent
  before v0.5.0 is released.
