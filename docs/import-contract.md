# Taskrail Import Contract (Draft Schema)

Status: contract and schema. The deterministic structural command surface is
implemented by T-033 (`taskrail import <src> --to tasks|spec|planning`, preview
by default with `--apply`); the agent-driven `--emit-prompt`/`--apply <draft.json>`
path is implemented later by T-034. This document is the agreement both build
against.

T-033's structural import is the crude, no-LLM baseline: it mechanically parses
markdown structure into an `ImportDraft` (headings → spec sections; subheadings
and list items → task drafts) and, on `--apply`, writes the draft as a reviewable
file under `planning/imports/`. That draft is the `--apply <draft.json>` ingest
target the agent path (T-034) refines with real spec references before ingesting
through `CreateTask`.

Spec reference: [`specs/v0.2.0.md#taskrail-import`](../specs/v0.2.0.md).

## Principle: No LLM Calls In The Binary

The Taskrail binary performs **no LLM calls**. The semantic lift — reading rough
human material and proposing structured tasks or spec sections — is the agent's
responsibility. Taskrail's job is to (1) hand the agent a prompt and the draft
schema via `--emit-prompt`, and (2) validate and ingest the agent's draft via
`--apply`. This keeps Taskrail provider-agnostic, consistent with the v0.1.0 LLM
exclusion.

## Sources And Targets

Import turns source material into Taskrail structure.

Source kinds (free-form markdown the agent reads):

- notes markdown files
- rough feature docs
- bug notes
- todo docs
- draft specs

The `--to` target selects what an applied draft produces:

| `--to`     | Produces                                              |
| ---------- | ---------------------------------------------------- |
| `tasks`    | task drafts ingested as `planning/tasks/` files      |
| `spec`     | spec section drafts for a `specs/` document          |
| `planning` | combined bootstrap: spec sections plus task drafts   |

## Flow

```
taskrail import notes.md --to tasks --emit-prompt   # binary emits prompt + schema
        -> agent reads source, returns an ImportDraft JSON document
taskrail import --apply draft.json --to tasks       # binary validates + ingests
```

`--emit-prompt` is pure output: it produces the instructions and this schema for
the agent. `--apply` is the only write path; it parses the draft, runs the
structural validation below, then ingests each task draft through the existing
`CreateTask` path (T-027) so drafts and hand-created tasks share one validation
and scaffolding path rather than diverging.

## Draft Schema (Version 1)

The draft is a single versioned JSON object. The Go shape lives in
[`internal/taskrail/import.go`](../internal/taskrail/import.go) as `ImportDraft`;
`schema_version` is `importDraftSchemaVersion` and is bumped only on an
incompatible change.

```json
{
  "schema_version": 1,
  "target": "planning",
  "source": "notes.md",
  "tasks": [
    {
      "key": "auth",
      "title": "Add auth middleware",
      "spec_ref": "specs/v0.2.0.md#taskrail-import",
      "priority": "high",
      "dependencies": ["T-027"],
      "body": "## Description\n\nWire up auth.\n"
    },
    {
      "key": "auth-tests",
      "title": "Cover auth middleware",
      "dependencies": ["auth"]
    }
  ],
  "spec_sections": [
    { "heading": "Auth", "body": "Describe the auth surface." }
  ]
}
```

### Envelope fields

| Field            | Required | Notes                                          |
| ---------------- | -------- | ---------------------------------------------- |
| `schema_version` | yes      | must equal the current draft schema version    |
| `target`         | yes      | one of `tasks`, `spec`, `planning`             |
| `source`         | no       | originating source description, for provenance |
| `tasks`          | \*       | task drafts (see below)                         |
| `spec_sections`  | \*       | spec section drafts (see below)                 |

\* a draft must contain at least one task or one spec section.

### Task draft fields (map onto T-027 task fields)

| Field          | Required | Maps to / Notes                                              |
| -------------- | -------- | ----------------------------------------------------------- |
| `key`          | no       | draft-local handle for intra-draft dependencies; not persisted |
| `title`        | yes      | task `title`                                                 |
| `spec_ref`     | no\*\*   | task `spec_ref`; `path#anchor` shape                        |
| `priority`     | no       | task `priority` (`high`/`medium`/`low`); defaults `medium`  |
| `dependencies` | no       | another draft `key` or an existing task id (`T-NNN`)         |
| `body`         | no       | task markdown body                                          |

\*\* `spec_ref` is optional in the draft because a `planning` bootstrap may emit
the spec section and the task that references it together; the reference is
resolved at apply time.

### Spec section draft fields

| Field     | Required | Notes                 |
| --------- | -------- | --------------------- |
| `heading` | yes      | section heading text  |
| `body`    | no       | section markdown body |

## Validation Rules

Validation is two-layered. **Draft-structural** rules (`ValidateImportDraft`)
never touch the filesystem so a draft can be checked before its target spec
exists. **Apply-time** rules reuse T-027's `CreateTask`, which performs the
filesystem-coupled checks once the spec and tasks are real.

Draft-structural (`ValidateImportDraft`):

- `schema_version` must equal the current version.
- `target` must be one of `tasks`, `spec`, `planning`.
- the draft must contain at least one task or spec section.
- task draft: `title` is required; `priority`, when set, must be valid;
  `spec_ref`, when set, must be a `path#anchor` shape (existence deferred);
  `dependencies` must each be a unique in-draft `key` or an existing-task-id
  pattern, and must not be the task's own `key`.
- task draft `key` values, when present, must be unique.
- spec section: `heading` is required.
- unknown JSON fields are rejected at parse time (`ParseImportDraft`).

Apply-time (reused from T-027 `CreateTask`):

- `spec_ref` file exists and the heading anchor is present.
- each dependency resolves to a real task id.
- priority validity and id assignment.

This split keeps one source of truth for the heavier checks: `--apply` does not
reimplement spec/dependency existence, it ingests through `CreateTask`.
