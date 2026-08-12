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
- Agent attempts default to a two-hour timeout; override it with `--timeout 30m`
  or another positive `s`, `m`, or `h` duration. Timeout never retries. A valid
  terminal outcome interrupted before delivery names a private XDG-state bundle;
  inspect it, then use `--resume-delivery <absolute-bundle-path>` to revalidate and
  perform delivery without launching another agent.
- The shared prompt requires fresh subagents for simplification and correctness
  review. Each backend must inspect installed capabilities, prefer specialist
  skills or subagents, and fail closed when delegation is unavailable. A review
  round uses one to three fresh reviewers with different explicit lenses and the
  same frozen snapshot. At most two rounds run, with early exit after a clean
  round. Fix high and medium current-scope findings; low findings are report-only
  unless acceptance, specification, invariants, or required evidence makes them
  mandatory. After fixing any findings from a round, run a fresh code simplifier
  and affected checks. Changed bytes after round two receive one narrow,
  non-recursive final-diff review. A clean final diff plus green checks permits
  completion; a finding is repaired and simplified but stops as in-progress/fail
  because those resulting bytes are unreviewed. The round limit never creates a
  follow-up for unfinished current scope.
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
