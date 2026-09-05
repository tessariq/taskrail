# Layout Upgrades

Taskrail stamps an adopted repository with a layout marker. A repository whose
marker sits at layout 1 upgrades to layout 2 through a gated, recoverable
`init --apply`. This page holds that contract; the
[README](../README.md) covers ordinary adoption via `init` and `retrofit`.

## The preview writes nothing

A plain `taskrail init` on a layout 1 repository reports every operator decision
the upgrade resolves before anything can apply:

- the complete candidate paths (marker, schema-2 state, preserved task files,
  notes sidecar),
- committed storage and the default broad review-round maximum,
- decoded continuation notes with their applicable `extract`/`drop` choices,
- each installed skill's classification — parity mirrors stay marker-free;
  stamped copies normalize through a forced refresh.

A blocking state refuses with actionable guidance instead of previewing: an
`AUTONOMY.tsv` legacy entry at the configured planning path, an unsafe notes
destination, or a divergent or conflicting skill copy.

## Applying is gated

`taskrail init --apply` requires:

- `--confirm-quiescent` — your assertion that every older Taskrail process able
  to touch this repository or its linked-worktree storage has stopped;
- exactly one of `--extract-continuation-notes` or `--drop-continuation-notes`
  when decoded notes exist, and neither when they do not;
- the combined `--with-skills --force` whenever stamped skill copies require
  normalization.

## Applying is durable

A fully gated apply publishes the exact previewed candidate through one
recoverable transaction: the marker is fenced as layout 2 with a
`migration_fence` transaction id **before** any task, state, note, or skill byte
changes, the complete candidate publishes and post-validates, and the strict
final marker replaces the fence as the transaction's last operation.

A handled failure rolls every candidate-written byte back before the original
marker. An interruption leaves the fence plus the retained transaction: every
other command refuses (`recovery_pending` with the transaction, or
`migration_in_progress` when only the fenced marker remains), and
`taskrail recover <transaction-id>` derives the single safe restore, accept, or
clear action — see
[commands.md](commands.md#recovering-retained-transactions).

## Compatibility and downgrade

Older binaries refuse layout 2 through the command-wide compatibility guard.
Downgrade is complete Git reversion of the upgrade — never hand-editing the
marker.

An explicit `init --with-skills` request on a layout 1 repository is served by
the current layout, so skill installation keeps working independently of the
upgrade.
