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

## Inspecting and clearing stale locks

Every Taskrail semantic writer holds one repository mutation lock: Git
worktrees place it beneath the Git common directory (so linked worktrees
coordinate) and non-Git repositories beneath `.taskrail/runtime/`. A writer
that dies abruptly leaves its lock behind, and Taskrail never clears a lock
automatically — PID, host, and age are evidence about an owner, never a lease
over it.

`taskrail lock status` is read-only: it reports either absence or the exact
owner metadata (lock ID, command, PID, host, start time, repository identity,
transaction, and — for delegated owners — executable and delegation-token
*digests*, never the token itself) plus the raw lock-file digest. It writes
nothing, so it is always safe to run against a lock a live writer holds. It is
also admitted through a valid recovery fence, so an operator can observe the
lock that blocks recovery. Malformed or substituted fence state still fails
closed.

`taskrail lock clear <lock-id> --expect-sha256 <digest>` is the guarded
compare-and-delete for an abandoned lock: it removes only the unchanged lock
record named by both the ID and the digest observed via `lock status`, refuses
when the bytes moved on (`source_changed`), when no lock matches the expected
digest (`invalid_digest`), and when the recorded owner is provably alive on
this host (`lock_held`) — signal-level liveness, not an age heuristic.
Clearing ownership never removes retained transaction data sharing the lock
root. A pending recovery fence still refuses `lock clear` (`recovery_pending`).

## Recovering retained transactions

`taskrail recover <transaction-id>` is the one command the recovery admission
fence admits. It previews exactly one mechanically safe action derived from the
retained journal evidence and the complete current snapshot set —
`restore_original` (put back the recorded originals, changing only components
still equal to the recorded candidate bytes), `accept_candidate` (keep a
complete, command-validated candidate), or `clear_fence` (nothing semantic was
published) — and never Git reset, checkout, or semantic inference. Preview is
read-only and reports the typed whole-set evidence; `--apply` performs exactly
the previewed action and is itself interruption-safe, so an interrupted apply
can simply be retried. When the interrupted writer left its mutation lock, first
use `lock status`, then supply its exact observation to `recover
<transaction-id> --take-over-lock <lock-id> --expect-sha256 <digest>`. Both
operands are required in preview and apply; the recorded lock transaction must
match the requested transaction. Preview leaves the lock and fenced bytes
unchanged. Apply compare-and-deletes that unchanged lock, acquires recovery
ownership, rechecks the complete transaction snapshot, and performs only the
previewed action. A local provably live owner refuses with `lock_held`; a
different-host owner is never inferred dead, so its exact operands are the
operator's explicit authorization.

Recovery requires the repository mutation lock, acquired naming exactly the
retained transaction: any holder — live or abandoned — refuses with `lock_held`
before evidence is read, and the held lock is left untouched. Unexpected bytes,
a substituted directory, or a mixed set that matches neither recorded state
refuse with `write_conflict` and preserve every byte plus the complete
evidence. An `accept_candidate` derivation additionally requires the owning
command's registered recovery validator; a command that has not shipped one
refuses with `validation_failed` rather than letting the binary choose
semantic content on a writer's behalf. Snapshots carry each path class exactly:
managed logical paths stay logical in local storage (a physical overlay
location is never published as semantic data), skill and runtime destinations
are worktree-physical, and Git metadata such as the exclusion store is a
canonical absolute path.

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
to the task, re-projects `STATE.md`, and re-runs `validate`. The coupled change
publishes through one locked transaction as plain filesystem operations — nothing
is staged through Git — so a collision, a concurrent edit, or a handled failure
leaves the tree either fully renamed or untouched.

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

## Releasing interrupted active work

When a direct operator deliberately gives up an interrupted or rework-pending
active task, release it rather than inventing a blocker or cancelling it:

```sh
taskrail task release T-217-release-interrupted-active-work-safely --reason "rework the candidate"
taskrail task release T-217-release-interrupted-active-work-safely --reason "rework the candidate" --dry-run
```

Release accepts one exact full-ID `in_progress` task and a trimmed portable reason
of 1 through 512 bytes. It records that reason in Implementation Notes, returns the
task to `todo`, preserves its completion/verification and loop-policy metadata, and
clears both active-task fields only when they consistently name the target. The
candidate reprojects `STATE.md` from the remaining blocker ledger. `--dry-run` is
read-only and reports its exact selected-task candidate digests; apply reports the
exact bytes it publishes. The timestamped note means separate invocations can have
different after digests.

Release is deliberately distinct from `block`, which records a real impediment;
`unblock`, which resumes an already blocked task; cancelled terminal history; and
automatic continuation, which never invokes release. Stale, missing, unrelated, or
contradictory active pointers refuse instead of being cleared, and delegated loop
children cannot release work.

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

## Import drafts

Bootstrap drafts from rough notes without any LLM — preview first, then apply:

```sh
taskrail import notes.md --to tasks                # preview the structural task drafts
taskrail import notes.md --to tasks --emit-prompt  # print an agent prompt for a richer draft
taskrail import --apply draft.json                 # validate an agent draft and write real files
```

Apply holds the repository mutation lock, snapshots the draft and every consumed
or destination path, validates the complete spec/task/state candidate, and then
publishes it as one transaction. A source or destination race refuses with
`write_conflict`; handled publication failures roll back unchanged candidates
and preserve any external edits with common transaction evidence. A successful
result names only the spec and tasks that were committed.
