# Bug: `taskrail task new` assigns colliding task IDs (ignores `T-NNN-slug` filenames)

_Reported against taskrail **v0.2.0**. Two issues: a primary ID-generation bug and a secondary
`validate` blind spot that lets the collision through undetected._

## Summary

`taskrail task new` is supposed to scaffold a task file with **the next free ID**. In a repository
whose task files are named `T-NNN-descriptive-slug.md` (with frontmatter `id: T-NNN-descriptive-slug`),
`task new` instead restarts numbering at **T-001** and writes a bare `T-001.md`, colliding with the
already-existing `T-001-*` task on its numeric prefix. Creating five tasks in a row produced
`T-001.md` … `T-005.md`, colliding with the existing milestone tasks `T-001-milestone-v0.1.0` …
`T-005-milestone-v0.5.0` — including a second `T-003` alongside the active v0.3.0 milestone.

`taskrail validate` then reports **`state valid`** and does not flag the collision, because it
compares the full slug IDs (`T-001` vs `T-001-milestone-v0.1.0`) as distinct strings.

## Environment

- taskrail: `0.2.0` (`taskrail --version`)
- Repo task convention: every file in `planning/tasks/` is `T-NNN-slug.md`; frontmatter `id`
  equals the filename stem (e.g. `id: T-076-ingestion-commands`).
- Highest existing numeric ID at repro time: `T-102`. Next free ID **should** have been `T-103`.

## Reproduction

1. Start in a repo whose `planning/tasks/` contains only slug-suffixed files, e.g.
   `T-001-milestone-v0.1.0.md`, `T-002-milestone-v0.2.0.md`, … up to `T-102-quality-check-cli-command.md`.
   (No bare `T-NNN.md` files exist.)

2. Run:

   ```
   taskrail task new --title "Some new task" --spec-ref "specs/v0.3.0.md#scope" --priority high --json
   ```

3. Observe the output:

   ```json
   {
     "task_id": "T-001",
     "title": "Some new task",
     "priority": "high",
     "spec_ref": "specs/v0.3.0.md#scope",
     "path": "planning/tasks/T-001.md"
   }
   ```

4. Run it four more times → `T-002.md` … `T-005.md`, each colliding with the existing
   `T-00N-milestone-*` task on the numeric prefix.

5. Run `taskrail validate` → prints `state valid`. The collision is **not** detected.

## Expected vs actual

| | Expected | Actual |
|---|---|---|
| Next ID | `T-103` (max existing numeric + 1) | `T-001` (restarts at 1) |
| Filename | `T-103-some-new-task.md` (or at least a non-colliding number) | `T-001.md` |
| `validate` on collision | error: duplicate numeric ID / two tasks share prefix `T-001` | `state valid` |

## Root-cause hypothesis

The next-free-ID scan appears to match only the exact pattern `T-NNN.md` (bare number, no slug).
Since this repo names every task `T-NNN-slug.md`, the scan finds **zero** matches, concludes the
namespace is empty, and starts at `T-001`. The generator is keyed on filenames of one shape while
the repo uses another.

Likely fix: derive the next number from the **numeric prefix of every task's `id`** (parse
`^T-(\d+)` from each task's frontmatter `id`, or from any filename matching `^T-(\d+)(-.*)?\.md$`),
take the max, add 1 — rather than requiring an exact bare-`T-NNN.md` filename match. Then scaffold
the new file as `T-<next>-<slug-from-title>.md` so it matches the repo convention, instead of a bare
`T-<next>.md`.

## Secondary issue — `validate` does not detect numeric-prefix collisions

`taskrail validate` treats `T-001` and `T-001-milestone-v0.1.0` as unrelated IDs, so two tasks
sharing the numeric prefix `T-001` pass validation. Since dependency references and human/agent
reasoning use the numeric prefix as the identity (the maintainers previously renumbered colliding
`T-001`/`T-002` → `T-099`/`T-100` in commit `8be87fc` precisely to keep prefixes unique), `validate`
should flag two task files whose `^T-(\d+)` prefix is equal as a duplicate-ID error. As-is, the
collision is silent until something downstream breaks.

## Impact / workaround

- Impact: silent ID collisions; a fresh `task new` can shadow a milestone task (`T-003`), and
  `validate` gives false confidence.
- Workaround used: delete the mis-numbered bare files and hand-author `T-NNN-slug.md` at the true
  next free numeric ID.

## Note (not a taskrail bug — for completeness)

`task new` correctly **quotes** titles that contain a colon in the scaffolded frontmatter
(`title: 'Some: task'`). Hand-authored task files must do the same — an unquoted YAML value
containing `:` fails `taskrail validate` with `yaml: line 2: mapping values are not allowed in this
context`. This is standard YAML behavior, mentioned only so the fix for the ID bug keeps the
existing correct quoting.
