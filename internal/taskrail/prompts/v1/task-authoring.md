Review task {{TASK_ID}} at {{TASK_PATH}} against specification {{SPEC_VERSION}}
at {{SPEC_PATH}}. Produce read-only authoring guidance or a proposed task body;
do not apply the proposal.

First inspect the task's current body, status, dependencies, referenced spec
section, and relevant repository context. Use `taskrail task show {{TASK_ID}}`
and `taskrail spec show {{SPEC_VERSION}}` to inspect managed bytes; do not read
logical paths directly. Refuse to author a task whose status is not exactly
`todo`. Refuse when material product behavior, destructive scope, credentials,
operator policy, or another required decision is ambiguous; request reviewed
decomposition or clarification rather than inventing scope. Before proposing a
new abstraction, inventory relevant existing repository primitives and record
why extension or reuse is insufficient.

The task must establish one independently meaningful user, operator, or system
outcome and the invariant it establishes. Judge its boundary semantically:

- An aligned proposal has one bounded observable result, explicit dependencies
  and operator gates, and can be implemented, reviewed, and verified as one unit.
- Split an oversized proposal when independently useful parts have separate
  acceptance or durable oracles, materially different dependencies or operator
  decisions, or can be verified while another part is legitimately deferred.
  Name which resulting task owns required integrated behavior.
- Merge a fragmented proposal when code, tests, documentation, migration, and
  cross-layer changes together establish one observable result. Do not create
  coordination-only fragments.
- Do not use file count, criterion count, implementation layers, or estimates as
  size proxies. Do not split by component, phase, discipline, or arbitrary
  estimate.

Make every acceptance criterion observable and implementation-neutral. For every
acceptance criterion, map relevant actor, precondition, state, action, and
expected success; include failure and boundary observations where they materially
differ. Map this to the cheapest sufficient evidence layer: unit, boundary
integration, CLI or workflow, reproducible sandboxed manual probe, or operator
evidence. Use a public or durable oracle for the claimed meaning. A mock, call
count, file existence, process exit, or bare suite pass is a shallow oracle when
it does not establish persisted or user-visible behavior. Identify
regression-sensitive evidence for bug fixes and high-risk behavior; when
practical, show a new or strengthened test fails for the prior defect or a
relevant deliberate perturbation before restoring the correct behavior. Manual
probes must be reproducible and sandbox-first, name prerequisites and cleanup,
and distinguish expected behavior from observed evidence. Non-code tasks must
name an equivalent inspectable check or explain why a test category is not
applicable.

Address recovery, idempotency, concurrency, security, migration, compatibility,
and cleanup only when this outcome or specification makes them relevant. Refuse
vague suite-pass evidence, unnecessary internal prescription, speculative
checklist scope, and direct task mutation. Do not prescribe exact functions,
test files, mocks, or internal structure unless a public contract or known
regression requires it.

If the task is ready, propose non-empty `## Description`, `## Acceptance`, and
`## Verification Notes` sections in that order. Describe the outcome and
invariant, give bounded observable criteria, and map each criterion to its
evidence and oracle. Do not mutate the task, its status, dependencies, spec, or
repository files.
