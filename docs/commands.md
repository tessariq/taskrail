# Taskrail Command Reference

Deep-dive reference for command behaviors that need more than a flag list. The
[README](../README.md) covers the core loop (`validate → next → start → complete
→ verify`) and installation; this page holds the contracts and edge cases behind
the individual commands.

## Machine output envelope

Every `--json` command — including the `start`, `complete`, and `block`
lifecycle writers — emits one versioned envelope: `schema_version`, the
canonical `command`, `warnings`, and exactly one of `result` or `error`. A
failure carries a registered `error.code` plus `details` recording whether the
operation committed and which paths it touched.

This is the one-time v0.5 break from pre-v0.5 bare result objects: consumers
read command payloads under `result`, and there is no legacy-output switch.
`schema_version` versions the whole document, including result, warning, error,
enum, nullability, and semantic contracts, not only the outer member names.
Consumers must reject unsupported versions rather than decode an inherited shape
optimistically. Text output stays human-oriented and unchanged.

## Next-task selection and the active spec

Idle `next` selection is anchored to the active spec: it considers only `todo`
tasks whose `spec_ref` points at the active spec, so higher-priority older-spec
work is skipped rather than selected. When only older-spec work is runnable,
`next` reports no eligible task and lists the skipped tasks under `warnings`
(`skipped_non_active_spec`). An already-active task that points outside the
active spec is still returned so you can continue or resolve it, with a
`selected_non_active_spec` warning. Recover older work explicitly with
`start <id>`, or run `next --include-off-spec` for a one-shot pick that ranks
`todo` tasks across all specs and flags an off-spec selection (`off_spec` /
`selected_off_spec`). To move an off-spec task *onto* the active spec instead of
running it where it is, use [`task repoint`](#re-pointing-a-task-onto-another-spec-area).

## Recovery admission fence

All semantic command classes share one recovery admission fence. Retained or
malformed transaction state beneath the canonical repository runtime root makes
readers and writers fail with `recovery_pending`; no semantic result is printed
and writers do not begin. Git linked worktrees and committed/local storage share
the Git-common fence, while non-Git committed repositories use their root-local
runtime directory.

## Coverage vs gap analysis

`coverage` and `coverage --gaps` sit one word apart and answer different
questions — keep them distinct:

- `coverage` answers **"is this spec area linked to any task?"** — decomposition
  coverage, orphan tasks, and two-directional drift.
- `coverage --gaps` answers **"does a *covered* area lack a verification/companion
  task, have a dependency-graph anomaly, or look under-decomposed?"** — it emits
  structural candidates (`missing-verification`, `dependency-anomaly`,
  `under-decomposed-area`) over areas that already have tasks.

Both are **read-only** — they never write `STATE.md` or task files and never make
`validate` fail — and **advisory** by default. `--gaps` opts into gating only
through `--fail-on <category>`, which exits non-zero when a signal of that
category is present (mirroring `coverage --min`); the report itself is
unchanged.

The hard limit: `--gaps` is **mechanical only**. It reports count, graph, and
state signals — never a semantic "this needs a test" judgement. Its signals are
**candidates, not violations**: false positives are expected, and each one is
something a human or agent inspects and promotes into a real task, not a rule the
repo broke. For the semantic half — "is this area *actually* missing work?" — use
the `taskrail-gap` skill, which layers agent judgement on top of these structural
candidates.

## Review stages

Taskrail keeps mechanically testable state separate from agent or human semantic
review. Current `coverage`/`coverage --gaps` report structure; `validate` checks
repository invariants; `verify` records evidence against one task. The active
v0.5 roadmap adds distinct advisory stages rather than one overloaded "review":

- post-spec consistency, gap, addition, and adversarial lenses before decomposition;
- one existing-task review for alignment, dependencies, acceptance, and evidence;
- adversarial review of an unpublished decomposed task set;
- separate implementation review before completion and passing verification; and
- post-implementation workflow-adversarial probes with bounded review memory.

Semantic findings never become `validate` violations automatically. Humans adopt
accepted changes through the bounded task/spec/import commands. Publisher-backed
review prompts may be replaced whole-file at repository scope; their durable leaf
artifacts record the exact prompt template resolution without claiming that
Taskrail observed or certified the external review process. Implementation review
remains instructed by the separate `task-implementation` flow and is outside
review-artifact publication.

## The slug-in-id invariant

A task's `id` and its filename are two encodings of one identifier: `validate`
enforces `filename == "<id>.md"`, so a slugged filename requires a slugged id.
`task new` produces that pairing directly — `--title "X"` derives a slug and
writes `T-<n>-x-slug` with a matching `T-<n>-x-slug.md`, `--slug` overrides the
slug source, and passing neither keeps the bare `T-<n>` / `T-<n>.md` form. Every
case passes `validate` with no follow-up edit. Accented letters transliterate to
ASCII first, so `--title "Über Fußball"` yields `T-<n>-ueber-fussball` and
`--title "Łódź Điện"` yields `T-<n>-lodz-dien` rather than a mangled slug — however
your keyboard encoded the accent. Title-derived slugs keep only complete tokens up
to the roughly 50-character cap; if the first token alone exceeds the cap, the
bounded bare-id fallback is used instead of cutting that token. If the value you
pass normalizes to no slug at all (`--slug ""`, `--slug "!!!"`,
a fully non-Latin title), the bare `T-<n>` id is written and a warning naming the
source goes to stderr — `--json` on stdout stays clean.

Because the id and filename move together, you cannot rename a file for
readability on its own. A bare `git mv T-<n>.md T-<n>-add-slug.md` changes only
the filename, leaving the frontmatter `id:` as `T-<n>`, so the next `validate`
fails with `task <id> filename must be <id>.md`. The fix is `task rename`, which
re-slugs atomically: it rewrites the `id:` field, renames the file, repoints the
body's `# <id> <title>` heading, rewrites every inbound `dependencies:` reference
to the task, re-projects `STATE.md`, and re-runs `validate`.

```sh
taskrail task rename T-<n> --slug add-slug     # or --title "Add slug"; --dry-run previews
```

Rename is symmetric with creation: an explicitly empty selector, or one that
normalizes to no slug, strips
the slug instead of failing, renaming `T-<n>-<slug>.md` back to `T-<n>.md` (with
the same stderr warning), so a bad slug can be undone. The length cap is
symmetric too — a `--title`-derived slug is capped the same way `task new`
caps it, while an explicit `--slug` is normalized but not length-capped.

`task rename` re-encodes the identifier only: it changes the id/slug and filename
but never rewrites the `title:` frontmatter field. Re-slugging a task and
retitling its human-readable title are distinct operations, and there is no
`task retitle` command in this version — so `task rename --title "New Title"`
derives a new slug and leaves the title unchanged, by design. To change the
visible title, edit the task's `title:` field directly.

`--dry-run` writes nothing, and its reported validation previews the state the
rename *would* leave behind, not the one it would replace — so re-slugging to heal
a `filename must be <id>.md` drift answers "would this fix it?". The preview covers
the whole change set (id, filename, inbound dependency refs, and the `current_task`
pointer when it names the task); violations the rename does not touch still show up.

## Re-pointing a task onto another spec area

After `spec activate`, open tasks still pointing at the previous spec are off-spec:
`next` skips them, `status` lists them under the active-spec drift breakdown, and
`next --include-off-spec` recovers one to run where it is. To move an open task
*onto* the active spec instead, `task repoint` rewrites its `spec_ref` — the one
edit that would otherwise mean hand-editing frontmatter.

```sh
taskrail task repoint T-<n> --area status-active-spec-drift-breakdown  # active-spec anchor
taskrail task repoint T-<n> --spec-ref specs/v0.2.0.md#some-area       # explicit, cross-spec
taskrail task repoint T-<n> --area some-area --dry-run                 # preview, writes nothing
```

`--area` resolves the anchor against `STATE.md`'s active spec exactly as `task new
--area` does, so an unknown anchor fails before any write and points at `spec show
<active-version> --anchors`. `--area` and `--spec-ref` are mutually exclusive.

`--dry-run` writes nothing, and its reported validation previews the state the
repoint *would* leave behind, not the one it would replace — so previewing a fix
for a broken `spec_ref` answers "would this make the repo valid?". Violations the
repoint does not touch still show up.

Repoint re-encodes one reference field: it never touches the id, slug, filename,
title, status, or dependencies, and never rewrites another task file. It is not a
status mutator and not a bulk migrator. Completed and cancelled tasks are delivered
history and are rejected. Because it re-projects `planning/STATE.md`, run `git
status` afterwards and stage the regenerated file with the change.

## Editing one dependency edge

Use exact full persisted task IDs to apply one accepted dependency-review change:

```sh
taskrail task dependency add T-010-api T-009-model --dry-run
taskrail task dependency add T-010-api T-009-model
taskrail task dependency remove T-010-api T-009-model
```

The target must be `todo`, `in_progress`, or `blocked`. Add appends without
reordering and rejects missing, self, duplicate, cancelled, or cyclic edges;
remove rejects an absent edge. Both operations preserve all other task bytes,
transactionally publish the task with a reprojected `STATE.md`, and support the
common `--json` envelope.

## Import drafts and partial writes

Bootstrap drafts from rough notes without any LLM — preview first, then apply:

```sh
taskrail import notes.md --to tasks                # preview the structural task drafts
taskrail import notes.md --to tasks --emit-prompt  # print an agent prompt for a richer draft
taskrail import --apply draft.json                 # validate an agent draft and write real files
```

An apply that fails during writing exits non-zero and still reports what it wrote
or may have touched — the spec and task paths in text mode, and with `--json` a
`partial_write` error whose `details.paths` name them. Review those paths before
retrying: a failed spec write may leave an empty or truncated file, and
re-applying the same draft creates any already-written tasks a second time under
new ids.
