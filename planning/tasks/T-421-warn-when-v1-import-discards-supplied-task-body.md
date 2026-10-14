---
id: T-421-warn-when-v1-import-discards-supplied-task-body
title: Warn when v1 import discards supplied task body detail
status: todo
priority: medium
spec_ref: specs/v0.5.0.md#outcome-focused-task-authoring
dependencies: []
updated_at: "2026-10-14T05:41:39Z"
---

# T-421-warn-when-v1-import-discards-supplied-task-body Warn when v1 import discards supplied task body detail

## Description

Bug: a successful ImportDraft v1 apply silently discards supplied task body
detail. Operators and agents receive created IDs/paths but no diagnostic that
their authored Description and Acceptance were replaced by TODO scaffolding.

Reported with released v0.4.0 in fmueller/latinitas-cards; that release's emitted
prompt calls body "markdown detail for the task file" while its apply path never
passes Body to CreateTask. Reproduced against current source on 2026-10-14:
apply returns success with warnings: [] and the authored marker is absent.

Current docs/import-contract.md explicitly defines v1 body as accepted but
ignored, and the current emitted prompt now states that limitation. Completed
T-304 aligned scaffolding with this active-spec authoring contract; it did not
add a discard warning. specs/v0.5.0.md#adversarial-spec-to-task-decomposition
preserves v1 scaffold/body behavior and defines digest-bound reviewed v2 body
publication. The defect to fix is missing disclosure, not v1 body preservation.

Repro in a disposable repository initialized with the current CLI:

```sh
taskrail init
cat > draft.json <<'JSON'
{"schema_version":1,"target":"tasks","tasks":[{"title":"Body warning probe","spec_ref":"specs/v0.1.0.md#summary","body":"## Description\n\nUNIQUE_BODY_DETAIL\n\n## Acceptance\n\n- Authored acceptance."}]}
JSON
taskrail import --apply draft.json --json
cat planning/tasks/*body-warning-probe.md
```

Observed: warnings is empty and the generated file contains TODO sections, not
UNIQUE_BODY_DETAIL. Expected: successful scaffold-only import explicitly tells
the caller which supplied task detail was omitted and how to author it safely.

## Acceptance

- A1. Successful v1 apply with non-empty task body detail reports a deterministic
  warning identifying each affected created task, explaining that the body was
  ignored and the standard scaffold emitted, and naming the supported authoring
  or reviewed-v2 route. Text mode and JSON mode expose equivalent diagnostics.
- A2. V1 bodies remain accepted without publication; IDs, frontmatter,
  dependencies, scaffold content, implicit hold, and transactional guarantees
  retain their existing behavior. Omitted/empty bodies produce no discard warning;
  reviewed v2 preserves exact body bytes without a v1 discard warning. Failed or
  rolled-back apply never claims that unpublished tasks were created.
- A3. Import documentation and agent guidance agree with apply diagnostics and
  do not imply that v1 publishes supplied body content. Any new JSON warning
  variant is explicitly reviewed and approved against the closed machine-result
  contract before implementation; do not introduce a free-form warning shape or
  change v1 body/schema semantics as a workaround.

## Verification Notes

- A1/A2: use public CLI probes in temporary repositories with distinct body
  markers, multiple tasks, absent/empty bodies, text/JSON output, and a refused
  dependency. Assert diagnostics together with persisted task content, not just
  exit status; demonstrate the populated-body regression fails before the fix.
- A2: retain reviewed-v2 exact-byte and import rollback coverage; verify that a
  handled publication failure does not emit success-only discard diagnostics.
- A3: inspect the approved warning contract and emitted prompt/documentation for
  matching semantics; regenerate packaged skill copies only if their source is
  changed. Persist sandbox manual-test plan/report and later evidence paths.

## Implementation Notes

- Investigation only: no import implementation or spec contract changed here.
- Source evidence: internal/taskrail/import_apply.go selects renderNewTaskBody
  for v1 and only uses draftTask.Body for reviewed import; import_prompt.go now
  describes body as ignored. T-304 is completed and no existing task was found
  for the missing apply-time diagnostic.
