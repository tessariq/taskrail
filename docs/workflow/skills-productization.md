# Skills Productization Decisions

Contract for how Taskrail's tracked-work skills become a supported product
surface. Records the three decisions the v0.2.0 spec
(`specs/v0.2.0.md#agent-workflow-skills`) requires be made explicitly, with
rationale and non-goals. No runtime behavior lives here; this document gates the
implementation tasks T-029, T-030, and T-034.

## Decision 1: Portability

Shipped skills must be repo-agnostic. They invoke the installed `taskrail`
binary, never `go run ./cmd/taskrail`, which only resolves inside the Taskrail
source tree.

Rationale: adopting repositories have no Go module for Taskrail and no
`./cmd/taskrail` package. A skill that shells out to `go run` cannot run in any
repository other than this one.

Since T-055 this is the only rule: the bespoke `skills/` source was retired and
this repository adopts the packaged set like any adopter, invoking the binary via
the configurable entry point `${TASKRAIL:-taskrail}` (T-051). The dogfooding-vs-
shipped portability split no longer exists.

## Decision 2: Distribution Mechanism

Exactly one initial path: **embedded skill files via `embed.FS`, written only on
explicit opt-in.** Skills are embedded in the binary and materialized only when
the user runs `taskrail init --with-skills`.

The default `taskrail init` must not write `.agents/` or `.claude/` skill
directories, or any other agent-tool directory. Provisioning agent-tool
directories is opt-in and never silent.

Rationale: embedding keeps the shipped skill text versioned with the binary that
implements the commands the skills call, so the two cannot drift. Gating on an
explicit `--with-skills` flag keeps `taskrail init` minimal and avoids writing
provider-specific directories into repositories that do not want them.

Rejected alternative:

- Documentation-only (adopters copy skill text manually): loses version
  coupling between skills and the binary; higher adoption friction.

Note: writing skills on default `taskrail init` was never an option — it is
ruled out by the constraint above, not a considered alternative.

## Decision 3: Relationship To Task-Creation Ergonomics

Shipped skills call the real `taskrail task new` command (see T-027 / T-028)
instead of hand-authoring task markdown.

Rationale: today's dogfooding skills compensate for the absence of a
task-creation command by writing task files by hand. That duplicates the task
schema inside skill text and drifts from the CLI's own validation. A real
command lets a skill create a well-formed task with one non-interactive call, so
skill text carries workflow, not schema.

The task-creation ergonomics work and the skills work are designed together: as
`taskrail task new` absorbs schema responsibility, shipped skills shrink to
workflow orchestration.

## Non-Goals

Taskrail distributes skills as static, provider-agnostic text. It does **not**:

- execute skills,
- schedule skills, or
- orchestrate skills.

Running a skill remains the agent's responsibility, consistent with the LLM and
runtime exclusions in the spec. There is no skill-execution, skill-scheduling,
or skill-orchestration runtime in Taskrail.

## The Packaged Skill Set

Input list for T-029, which owned the final selection and the portability
rewrite. Historically this table split canonical dogfooding skills (`skills/`)
from the shipped set; T-055 retired that split, so every row below is simply part
of the one packaged set under `internal/taskrail/skills/`, materialized to the
committed `.agents/skills/` and `.claude/skills/` copies:

| Skill | Disposition | Reason |
|-------|-------------|--------|
| `autonomous-backlog` | Shippable | Generic tracked-work cycle (validate, select, start, implement, verify, follow-up); no repo-local assumptions once `go run` is replaced by the installed binary. |
| `autonomous-task` | Shippable | Executes one selected task through CLI transitions; portable after the binary rewrite. |
| `autonomous-verify` | Shippable | Drives `taskrail verify` against acceptance criteria and points at product-level verification artifacts. |
| `autonomous-recovery` | Shippable | Routes every correction through `taskrail repair` (dry-run -> apply -> re-validate) and never hand-edits authoritative state. Shipped in T-054 once the widened repair surface (T-072) shrank the human-resolved residue to what the skill claims; the earlier "falls back to manual edits" premise is stale. |
| `autonomous-manual-test` | Shippable | Guides manual testing against task acceptance criteria; repo-agnostic after the binary rewrite. Shipped in T-081 **without** promoting `planning/artifacts/manual-test/` to a product invariant: its artifacts stay ephemeral and gitignored, `init` does not provision the directory, and `validate` stays unaware of it. This resolves the v0.2.0 "Artifact And Init Consistency" deferral in favor of shipping the skill rather than adding an invariant. |
| `taskrail-repair` | Shippable | Drives the conservative `taskrail repair` loop (dry-run -> apply -> re-validate) to reconcile mechanical `STATE.md` drift; repo-agnostic and never hand-edits authoritative state (T-050). |
| `taskrail-spec` | Shippable | Inspects and authors specs through the `taskrail spec` command family and anchors tracked work to real `spec_ref` headings; repo-agnostic (T-064). |
| `taskrail-decompose` | Shippable | Composes shipped primitives (`coverage --json`, `spec show --anchors`, `import --apply`) to draft spec-anchored tasks for uncovered active-spec areas; spec-driven and repo-agnostic, adds no binary surface (T-098). |

T-029 may revise this list, but must justify any change against the three
decisions above.

## Onboarding Skills

The onboarding skills target a repository that is not yet Taskrail-managed, which
Taskrail's own already-managed repository never exercises. They live in the same
packaged set (`internal/taskrail/skills/`) as the tracked-work skills and honor the
same three decisions — repo-agnostic,
installed-via-`--with-skills`, and task-creation through a real command rather
than hand-authored markdown.

| Skill | Origin | Task creation | Reason |
|-------|--------|---------------|--------|
| `taskrail-import` | T-034 | `taskrail import --apply` | Turns markdown notes/drafts into spec and task files via the agent-in-the-loop import path; the binary stays LLM-free. |
| `taskrail-retrofit` | T-043 | `taskrail import --apply` | Drives the guided retrofit bootstrap (detect -> dry-run -> confirm -> apply -> adopt -> validate) for an existing repo, adopting reviewed notes as tracked work through the import pipeline (T-042). |

## Cross-References

- Spec: `specs/v0.2.0.md#agent-workflow-skills`
- Skill catalog and packaging: `docs/workflow/skills-overview.md`
- Downstream implementation tasks: T-029 (shippable skill selection and
  portability rewrite), T-030 (`init --with-skills` distribution), T-034
  (`taskrail-import` skill), T-043 (`taskrail-retrofit` skill).
