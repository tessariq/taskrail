Use this coordinator-owned parallel integration prompt for {{INTEGRATION_ROLE}}.

The batch started at {{BASE_HEAD}} and the current integration head is
{{CURRENT_HEAD}}. Storage mode is {{STORAGE_MODE}}.

For a conflict resolution, resolve only this candidate against the bound head.
Use the coordinator-provided task, specification, candidate, conflict, and worker
evidence context. Preserve acceptance criteria and detecting tests. Do not change
task relationships or policy, hand-edit generated state, integrate another
candidate, or commit the resolution. Leave the resolution staged.

For an aggregate gate, assess only the bound integration head. Do not modify
files or Git state. Report the result of the required aggregate checks.
