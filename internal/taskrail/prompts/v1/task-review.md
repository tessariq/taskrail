Review the existing tracked task {{TASK_ID}} at {{TASK_PATH}} against its
referenced specification {{SPEC_VERSION}} at {{SPEC_PATH}}. Stage exactly one
strict JSON proposal at {{REVIEW_PATH}}. This is advisory review, not task
authoring, implementation review, verification, mechanical gap analysis, or a
second copy of the four post-spec lenses.

Read the selected task with `taskrail task show {{TASK_ID}} --json` and the selected
spec with `taskrail spec show {{SPEC_VERSION}} --json`; do not open logical managed
paths directly. Resolve and inventory the task's exact dependency IDs, relevant
same-area tasks, and repository contracts before judging it. Record the raw
task and spec SHA-256 values from the resolved command results. Inspect each
relevant task through `taskrail task show <full-task-id> --json`, not through a
physical task-file path.

Judge one task boundary, not a task set. Cover outcome focus and spec alignment;
semantic sizing under the shared rubric; overlap; dependency direction;
integration ownership; observable acceptance; negative and boundary behavior;
criterion-to-evidence mapping; public or durable evidence oracles; operator
gates; and unnecessary implementation prescription. State whether both the
split and do-not-split tests support the current boundary. When they do not,
recommend a concrete split or consolidation and name the task that owns
integrated delivery. Do not use file count, implementation layer, criterion
count, or estimate as a size proxy. Do not duplicate the post-spec consistency,
gaps, additions, or adversarial lenses.

Write one UTF-8 `review.json` object without a BOM, duplicate keys, unknown
members, null values, or trailing data. Its exact top-level fields, in this
order, are `schema_version`, `prompt_id`, `prompt_contract_version`,
`prompt_template_sha256`, `prompt_source`, `session_id`, `task_id`,
`task_path`, `task_sha256`, `spec_path`, `spec_sha256`, `context_mode`,
`generated_at`, and `findings`. Set `schema_version` to `1`; `prompt_id` to
`task-review`; `prompt_contract_version` to `v1`; and copy the prompt source
and pre-substitution template digest from the resolved prompt result. Use the
same portable session key for `session_id` and the final destination basename,
canonical logical paths and lower-case raw-byte SHA-256 values for the task and
spec, `fresh` or `same-context` for `context_mode`, and canonical RFC3339 UTC
for `generated_at`. `findings` is a non-null array. Each finding has exactly
`finding_id`, `severity`, `evidence`, `impact`, `recommendation`,
`disposition`, and `rationale`; all textual values are non-empty, IDs are unique
within the session, severity is `high`, `medium`, or `low`, and disposition is
`open`, `accepted`, `rejected`, or `deferred`.

Findings remain advisory. Recommend accepted body-only clarification through
the digest-bound `task author` flow, one accepted dependency correction through
exact-ID `task dependency add` or `task dependency remove`, and a genuinely new
outcome through reviewed implicit-hold follow-up creation. Route genuine
split/merge work through a reviewed task-producing flow. Never change task
status, task-local loop policy, specifications, dependencies, or task bodies
directly. A `todo` task may continue into authoring after review. Other statuses
remain reviewable but are not rewritten through authoring. After authored bytes
change, start another task review only when an explicitly invoked consuming
workflow or the human requires final-byte review; unchanged bytes and
confidence-seeking alone do not justify another session.
