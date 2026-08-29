Independently review specification {{SPEC_VERSION}} at {{SPEC_PATH}} for missing actors,
states, failure paths, negative cases, compatibility, migration,
rollout, rollback, cleanup, observability, documentation, testing, operator
evidence, and release acceptance. Do not receive another lens's conclusions as
facts.

Write exactly one schema-v1 JSON object to {{REVIEW_PATH}}: no prose, Markdown,
or code fence. It has exactly `schema_version`, `prompt_id`,
`prompt_contract_version`, `prompt_template_sha256`, `prompt_source`,
`session_id`, `lens`, `spec_path`, `spec_sha256`, `context_mode`, `generated_at`,
and non-null `findings`. Set `schema_version` to 1, `prompt_id` to `spec-gaps`,
`prompt_contract_version` to `v1`, and `"lens":"gaps"`. Copy the
prompt-template digest and source from the resolved prompt binding supplied with
this request; use the selected spec path and digest, the shared session identity,
`fresh` or `same-context`, and canonical RFC3339 UTC time.

Each finding has exactly `finding_id`, `severity`, `evidence`, `impact`,
`recommendation`, `scope`, `disposition`, `rationale`, and only future findings
add `target_version`. IDs use the `GAPS-` prefix and are unique across the
session; severity is
`high|medium|low`, scope is `current|future|reject`, disposition is exactly
`open`, and every textual field is non-empty. High means plausible
data/security/lifecycle loss, medium material contract failure, and low bounded
quality risk. Flag sizing only when spec prose makes coherent decomposition
impossible through inseparable outcomes, contradictory boundaries, or missing
integration ownership; never assess proposed or existing task size.

Do not mutate the spec, activate a spec, create tasks, or make final
dispositions.
