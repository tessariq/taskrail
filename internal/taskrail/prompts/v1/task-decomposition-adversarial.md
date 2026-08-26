Adversarially review the unpublished decomposition for {{SPEC_VERSION}} at
{{SPEC_PATH}} using the approved post-spec review {{SPEC_REVIEW_PATH}}, exact
draft {{DRAFT_PATH}}, and exact trace {{TRACE_PATH}}. Write only the strict review
JSON to {{REVIEW_PATH}}. Use fresh context in a fresh review process or fresh
agent context; do not rely on the author's reasoning or a prior pass, and do not
mutate any reviewed input.

Re-read the selected spec, approved review, existing tasks, real anchors, exact
draft, and exact trace. Verify every lower-case exact SHA-256 against raw bytes.
Refuse stale, missing, cross-session, normalized, reserialized, or otherwise
unbound inputs rather than reviewing a substitute snapshot.

Challenge every task against the shared outcome and evidence rubric:

- It must establish one independently meaningful user, operator, or system
  outcome with one bounded observable result.
- Split independently useful outcomes with separate acceptance or durable
  oracles, materially different real dependencies or operator gates, or
  independently deferrable value. Do not split one outcome by file, layer,
  discipline, phase, or estimate; reject coordination-only fragments.
- Require one named owner for required integrated behavior and real dependencies
  connecting supporting work. Reject invented task IDs and references not tied
  to a real specification heading in {{SPEC_PATH}}.
- Require actor, precondition, state, action, success, materially different
  failure and boundary observations, and the cheapest sufficient evidence for
  each criterion. Require a public or durable oracle. Flag a mock, call count,
  file existence, process exit, or bare suite pass as a shallow oracle when it
  does not prove persisted or user-visible meaning.
- Check relevant negative, boundary, compatibility, recovery, concurrency,
  security, migration, cleanup, and operator gates without demanding speculative
  checklist scope.

Enforce strict ImportDraft v2: exact schema version 2 and session identity,
target `tasks`, non-empty tasks, empty `spec_sections`, no `source`, and task
members exactly `key`, `title`, `dependencies`, `body`, `spec_ref`, plus optional
`priority`. Every body must have exactly one non-empty `## Description`,
`## Acceptance`, and `## Verification Notes` in that order. Optional
`## Implementation Notes` may occur once only last and may be empty. Reject all
other H2s, reordering, duplicates, H1/frontmatter, and body normalization; permit
H3+ detail and fenced examples. Require real anchors, real dependencies, complete
trace coverage, and exact source quotes or line ranges. Task objects must omit
`loop_policy` and `loop_reason`; imported tasks must remain implicitly held.

Return one strict decomposition-review schema version 1 object with the existing
prompt binding, portable session ID, the current consecutive `pass_number`, exact
`spec_path`, `draft_path` `draft.json`, `trace_path` `trace.json`, lower-case exact
SHA-256 bindings, `context_mode` exactly `fresh`, timestamp, and `findings`.
Each finding has exactly unique `finding_id`, severity `high`, `medium`, or `low`,
and evidence, impact, and recommendation strings. Empty findings means this exact
snapshot passed; it does not certify later bytes.

Do not repair draft or trace bytes during review. A human may disposition findings
and author one revised candidate; any changed spec, draft, or trace invalidates
the prior binding and requires at most one second fresh-context pass. Never run
more than two passes in one session. Any change after pass 2 invalidates the
session; stop rather than publish unreviewed bytes. If prompt resolution changes
before publication, abandon the session rather than edit review metadata. A
manifest may bind only one or two consecutive fresh-context reviews and the final
exact spec, draft, and trace digests; every finding needs a disposition, and high
or medium findings cannot be deferred. Do not mutate the spec, approved review,
draft, trace, tracked tasks, planning state, lifecycle, loop policy, manifest, or
durable review directories.
