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
- Keep the runner finite and fail-closed. Never add retries, automatic recovery,
  existing-task continuation, hook bypass, force push, fetch/pull/rebase, amend,
  reset, or hidden queue mutation. Delivery recovery is explicit, bundle-bound,
  and never resumes agent execution.
- Only the designated held cleanup task may delete this complete directory.
- This complete directory and every live invocation reference must be absent
  before v0.5.0 is released.
