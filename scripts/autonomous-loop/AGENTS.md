# Temporary Autonomous Loop

- These files are temporary operator-owned source-checkout controls, not Taskrail
  product behavior or adopter guidance.
- Ordinary queued tasks must not modify this directory.
- `queue.tsv` is reviewed source policy, remains immutable during a child run,
  and is never runtime state. It owns task order and run/hold policy, not agent
  selection.
- The runner uses Claude by default. Select one backend for the complete invocation
  with `--backend claude` or `--backend opencode`; dry-run reports that selection.
- Do not add logs, results, session data, credentials, or generated files here.
  Runtime output belongs under ignored `planning/artifacts/runs/` or external
  temporary storage.
- Keep the runner finite and fail-closed. Never add retries, automatic recovery,
  existing-task continuation, hook bypass, force push, fetch/pull/rebase, amend,
  reset, or hidden queue mutation.
- Only the designated held cleanup task may delete this complete directory.
- This complete directory and every live invocation reference must be absent
  before v0.5.0 is released.
