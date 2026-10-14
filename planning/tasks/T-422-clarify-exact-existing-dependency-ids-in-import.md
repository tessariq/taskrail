---
id: T-422-clarify-exact-existing-dependency-ids-in-import
title: Clarify exact existing dependency IDs in import guidance
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#outcome-focused-task-authoring
dependencies: []
updated_at: "2026-10-14T05:42:55Z"
---

# T-422-clarify-exact-existing-dependency-ids-in-import Clarify exact existing dependency IDs in import guidance

## Description

Bug: the import prompt describes dependencies as an in-draft key or an existing
task ID "like T-012" without explaining that existing-task references require
the complete exact frontmatter ID. docs/import-contract.md similarly presents
existing IDs as T-NNN. Agents following this guidance can supply a short task key
for a slugged task and receive "dependency T-038 does not exist" even though that
numbered task exists. This violates the active authoring guidance's requirement
to wire real dependencies by making the accepted reference ambiguous.

Reported with released v0.4.0 in fmueller/latinitas-cards: T-038 was rejected while
T-038-publish-releases-on-pypi worked. Confirmed in current source on 2026-10-14:
import_prompt.go retains the T-012 example, import accepts the short ID's shape,
and apply validates dependency existence against exact IDs, not prefix lookup.
Completed T-239 establishes exact-ID dependency editing, but does not fix import
guidance. T-196 targets future v0.6 stable-reference integration, not this
currently misleading import contract. No existing task was found for this fix.

Repro in a disposable repository initialized with the current CLI:

```sh
taskrail init
taskrail task new --title 'Existing dependency' --area summary --json
taskrail import specs/v0.1.0.md --to tasks --emit-prompt
cat > draft.json <<'JSON'
{"schema_version":1,"target":"tasks","tasks":[{"title":"Dependency probe","spec_ref":"specs/v0.1.0.md#summary","dependencies":["T-001"]}]}
JSON
taskrail import --apply draft.json --json
# Replace T-001 in the draft with the emitted full ID T-001-existing-dependency.
taskrail import --apply draft.json --json
```

Observed: the first apply refuses with "dependency T-001 does not exist"; the
second creates a task with dependency T-001-existing-dependency. Expected: the
prompt and contract make exact-ID usage clear before the draft is authored.
Bare IDs remain valid when they are the actual stored ID; do not imply that
all tasks require a slug or add prefix resolution as part of this fix.

## Acceptance

- A1. Emitted import guidance and import contract examples distinguish in-draft
  keys from complete existing-task frontmatter IDs. Include a slugged-ID example
  and explicitly state that a short key such as T-012 cannot abbreviate
  T-012-some-slug; a genuinely bare stored ID T-012 remains valid.
- A2. Follow the corrected prompt in a disposable repository: both a draft-local
  key and a full slugged existing ID apply successfully and persist the intended
  dependencies. A short prefix of that existing ID is still refused without
  writes, while a genuine bare stored ID is accepted. No ID resolution, draft
  schema, dependency validation, or reviewed-v2 semantics change.
- A3. Applicable import-facing documentation and packaged skill guidance agree
  with the corrected prompt; regenerate committed skill copies if their packaged
  source changes. Do not broaden this into future stable-reference work.

## Verification Notes

- A1: inspect emitted prompt and documented schema/examples together; demonstrate
  that the previous guidance omits the slugged-ID versus short-key distinction.
- A2: use public CLI sandbox probes with asymmetric IDs and in-draft keys; assert
  persisted dependencies and refusal/no-write behavior, not just successful exits.
  Reuse existing exact-ID/import tests where they already cover this behavior.
- A3: check import-facing guidance and package parity if changed, then run
  taskrail validate. Persist sandbox manual-test plan/report and evidence paths.

## Implementation Notes

- Investigation only: no dependency implementation or spec contract changed here.
- This task is independent of T-421's missing v1 body-discard diagnostic; either
  fix is useful without the other and needs no dependency edge.
