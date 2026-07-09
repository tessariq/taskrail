# Backlog

Durable home for post-review ideas that were deliberately deferred out of the
current tracked version. These are candidates, not committed work: promote one
to a task (`taskrail task new`) against the version whose spec adopts it.

## Deferred to v0.4.0

- **`taskrail spec diff <v1> <v2>`** — mechanical anchor-set diff between two
  spec versions (areas added / removed / renamed), reusing the existing
  `collectHeadingAnchors` slug logic. Supports migration: when `spec activate`
  advances the active spec, the diff shows which areas are new (need tasks) and
  which vanished (orphan existing tasks). Read-only, deterministic, fits the
  `spec` command family and the drift theme. Deferred from the v0.3.0 task review
  because it is not core to the v0.3.0 threads and `specs/v0.4.0.md` does not
  exist yet; raise it when authoring the v0.4.0 spec.
