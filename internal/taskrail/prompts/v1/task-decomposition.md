Author an unpublished reviewed decomposition for {{SPEC_VERSION}} at
{{SPEC_PATH}}, bound to the approved post-spec review at {{SPEC_REVIEW_PATH}}.
Write the strict ImportDraft v2 to {{DRAFT_PATH}} and its requirement trace to
{{TRACE_PATH}}. The binary is provider-neutral and makes no model call.

Author in fresh context and bind every conclusion to the exact input snapshot.

Inspect the selected spec exact bytes, its approved post-spec review, existing
tasks, and real heading anchors before authoring. Refuse when the approved review
is absent or stale, a requirement is ambiguous, an operator decision is missing,
or required evidence cannot establish the claimed behavior. Do not invent
anchors, dependencies, scope, or approval.

Each task must establish one independently meaningful user, operator, or system
outcome and one bounded observable result. Apply this shared rubric:

- Split independently useful outcomes when they have separate acceptance or
  durable oracles, materially different real dependencies or operator gates, or
  one can be verified while another is legitimately deferred.
- Do not split one outcome by file, layer, discipline, phase, or estimate. Merge
  code, tests, documentation, migration, and cross-layer fragments that only
  establish one result. Never create coordination-only tasks.
- Name the task that owns required integrated behavior. Every supporting task
  must connect to that integrated owner through real dependencies.
- Anchor every `spec_ref` and trace entry to a real specification heading in
  {{SPEC_PATH}}. Use only real dependencies: another draft key or an existing
  exact task ID. Record requirement quotes that occur exactly once or valid line
  ranges; do not fabricate coverage.

For every acceptance criterion identify the actor, precondition, state, action,
success result, and materially different failure and boundary observations. Map
it to the cheapest sufficient evidence layer and a public or durable oracle.
Refuse a mock, call count, file existence, process exit, or bare suite pass as a
shallow oracle when it does not prove persisted or user-visible meaning. Include
negative, boundary, compatibility, recovery, concurrency, security, migration,
cleanup, and operator gates only where the outcome or specification makes them
relevant. Manual evidence must be reproducible and sandbox-first.

The draft is one JSON object with exactly schema version 2, one portable
`review_session_id`, target `tasks`, a non-empty `tasks` array, and an empty
`spec_sections` array; omit `source`. Every task has exactly `key`, `title`,
`dependencies`, `body`, and `spec_ref`, with optional `priority`. Task objects
must omit `loop_policy` and `loop_reason`; published tasks remain
implicitly held.

Each `body` must preserve its intended exact bytes and contain exactly these
non-empty H2 sections in order: `## Description`, `## Acceptance`, and
`## Verification Notes`. An optional `## Implementation Notes` may appear once,
only last, and may be empty. No other H2 or H1 is allowed; H3+ detail and fenced
examples are allowed. Description states the outcome and invariant, Acceptance
states observable behavior, and Verification Notes map each criterion to durable
evidence and its oracle.

The trace is strict schema version 1 with the same session ID, exact
`spec_path` and lower-case exact SHA-256 `spec_sha256`, and a `requirements`
array. Each requirement has exactly `requirement_id`, normalized `spec_ref`,
`source`, `task_keys`, `disposition` (`task` or `no-task`), and non-empty
`rationale`. Every draft task key must be traced.

Compute every SHA-256 from exact file bytes, never normalized or reserialized
content. Write only {{DRAFT_PATH}} and {{TRACE_PATH}} in the transient proposal.
Do not mutate the spec, approved review, tracked tasks, planning state, lifecycle,
loop policy, or durable review directories.
