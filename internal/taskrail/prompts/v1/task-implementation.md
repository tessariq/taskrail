Implement the selected task {{TASK_ID}}. Its logical path is {{TASK_PATH}}.
The active specification is {{ACTIVE_SPEC_VERSION}} at {{ACTIVE_SPEC_PATH}}.
The implementation-review maximum is {{IMPLEMENTATION_REVIEW_MAX_ROUNDS}} and
storage mode is {{STORAGE_MODE}}.

Use `taskrail task show {{TASK_ID}} --json` and `taskrail spec show
{{ACTIVE_SPEC_VERSION}} --json` to inspect managed bytes; do not read logical
paths directly. Read repository instructions, the selected task, its dependencies,
the referenced spec section, and relevant implementation and tests. Inventory
existing repository primitives before proposing an abstraction and record why
extension or reuse is insufficient. Understand the independently meaningful
observable outcome, user or operator impact, affected invariants, acceptance
and specification boundaries, and intended evidence. Trace acceptance
requirements to observable executable or inspectable evidence before
implementation.

Before `start`, apply the outcome-focused sizing rubric: require one
independently meaningful outcome, a bounded observable result, explicit
dependencies and operator gates, and clear integrated-behavior ownership. Stop
for reviewed decomposition or clarification rather than implement an arbitrary
slice when the task bundles independently useful outcomes, fragments one
outcome, or leaves integration ownership unclear. Do not rewrite scope after
lifecycle work begins.

Use the repository's documented Taskrail binary invocation. Before every
Taskrail state writer, run the source-checkout freshness guard when the
repository provides one; stop and apply its named remedy if it fails. Pass
`--json` to every consumed Taskrail command, parse its common result envelope,
and check command and writer exits. Start the selected task. Implement the
smallest safe change; begin
behavior changes with a failing test whenever practical. Run applicable
formatting, focused tests, deterministic checks, and sandbox-first manual
testing for visible workflow behavior. Do not mutate external systems,
production data, credentials, billed resources, live services, or resources
outside the repository.

Inspect the verified implementation for unnecessary complexity and simplify
only when behavior is preserved; independent simplification delegation is
optional. Rerun affected checks after every simplification or repair.

Freeze the verified implementation, then run one broad
implementation-review round with one fresh reviewer by default. Choose an
explicit lens based on the task's behavior, tests, security, error handling,
edge cases, complexity, and domain risks. Parent-context self-review does not
satisfy this review. A broad round has one to three reviewers, each with a
named, non-duplicative lens. Use additional concurrent reviewers only for
distinct independently relevant risks the first lens is unlikely to cover, and
give every reviewer the same frozen snapshot. A reviewer crash, timeout,
unavailable fresh context, or malformed output fails closed rather than retrying
invisibly.

Classify every finding as `fix-now`, `separate-followup`, `blocked`, or
`rejected` with rationale and evidence. Use `fix-now` when the change introduced
or exposed the issue, current acceptance or specification requires it, an
affected invariant depends on it, or changed evidence is too weak. Use
`separate-followup` only for a distinct outcome outside that scope. Repair every
current-scope finding; budget exhaustion and severity do not turn required work
into a follow-up. For a test-strength finding, strengthen the test, demonstrate
that a deliberate relevant regression fails, restore the correct implementation,
and demonstrate the test passes. Rerun every affected deterministic check after
repairs.

One broad round is the default; use a second broad round only within the maximum
and for a distinct unresolved risk that deterministic verification does not
adequately cover. Do not start another broad round merely because findings were
repaired. If review fixes materially change product or test bytes, freeze the
repaired candidate and run one narrow final-diff review limited to fix-induced
regressions, integration breakage, and behavior drift. That review never starts
another broad round. A final-diff finding needs repair and affected checks;
objective closure evidence permits completion without another model review,
otherwise leave the task in progress, record failing verification, and stop for
operator review.

For headless ambiguity, credentials, destructive scope, production data, billed
resources, live consoles, or operator decisions, stop for a human rather than
guessing. If work cannot proceed, block the selected task with a portable
reason, check the writer exit, then publish failing verification that names the
operator decision. Deliberate rework may remain in progress with failing
verification. For interrupted or deliberate manual rework, direct an operator
to `task release`; a delegated child must not relinquish its selected task.

Create only newly discovered, independently meaningful out-of-scope follow-ups
through selected-task `verify --create-followup`; never defer current-task
acceptance, integration, or evidence. There is no arbitrary numeric cap, but
each generated follow-up must be named in a fresh selected-task verification
report, depend on the selected task, omit `loop_policy` and `loop_reason`, and
remain implicitly held. Follow-ups must be concrete, spec-anchored outcomes,
not layer, file, test-only, documentation-only, speculative, duplicate, or
numeric-cap fragments. A delegated child may use only its granted lifecycle and
follow-up write sets; it cannot mutate task-local loop policy or derive
unattended authorization from a follow-up body.

On success, complete before passing verification; check writer exits. In
committed mode, run lifecycle first and commit the complete implementation plus
generated task and state bytes. In local mode, commit visible product changes
only; metadata-only blocked or rework outcomes do not fabricate an empty commit.
For completed-unverified or audit recovery, rerun only the safe missing step
before any required commit. Follow repository-visible commit, identity,
attribution, signing, hook, and ref policy without changing Git identity
configuration or copying managed paths, review or verification identities,
storage details, or Taskrail or agent attribution into commit metadata or
unrelated product text. Only a caller-owned instruction outside managed planning
may authorize exposing a local Taskrail identity or path. Leave provider
commands, credentials, remote pushes, sandboxing, and reviewer identity
attestation to callers.
