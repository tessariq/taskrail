# Findings: `taskrail` binary resolution and the freshness guard

Investigated 2026-07-27 while completing T-120. Feeds `T-123-contributor-binary-resolution`.
Not yet turned into tracked work — the second finding may warrant its own task.

Context: `mise run setup` had already been run in this working copy. It worked. The two
problems below are independent of it.

## Finding 1 — a shell without mise activation resolves the *installed release*

`mise.toml` exposes the working-tree build through:

```toml
[env]
_.path = ["{{config_root}}/bin"]
```

That applies only under `mise activate` (interactive shells) or an explicit `mise exec`.
A non-interactive shell — such as a coding agent's `Bash` tool — gets neither:

| Shell | `command -v taskrail` | `taskrail version` |
|---|---|---|
| non-interactive zsh (agent) | `/home/linuxbrew/.linuxbrew/bin/taskrail` | `0.3.0` |
| `mise exec -- …` | `<repo>/bin/taskrail` | `0.0.0-dev` |

Diagnostics in the agent shell: `$PATH` contains no `<repo>/bin` entry, and both
`MISE_SHELL` and `__MISE_DIFF` are unset.

So `${TASKRAIL:-taskrail}` — the fallback the packaged skills use (T-051) — silently
resolved to the shipped v0.3.0 release. This is exactly the trap `AGENTS.md` documents;
what is new is that it lands on the *agent*, in the one shell least likely to have mise
activated, and it surfaced only when a v0.4.0-era flag was rejected:

```
$ taskrail task new --title "…" --slug "…"
unknown flag: --slug
```

`~/.local/share/mise/shims/taskrail` does exist (a symlink to the `mise` binary), but the
shims directory is not on the agent shell's `PATH` either, so it never intercepts.

State written by the stale binary was re-validated afterwards with the working-tree
build and reported `state valid`; nothing was corrupted.

### Options

- Set `TASKRAIL=$PWD/bin/taskrail` in the agent environment (smallest fix).
- Route agent commands through `mise exec --` (rtk `transparent_prefixes` already has a
  mechanism for this; see the global RTK notes).
- Have the skills fail loudly on a version-capability mismatch rather than relying on
  PATH being right — closest to the spec's "resolving to an unintended binary must fail
  loudly rather than write tracked-work state".

## Finding 2 — `task taskrail:check` also fires on Go toolchain drift (likely undocumented)

After the `pre-push` hook rebuilt `bin/taskrail`, `task taskrail:check` still reported:

```
on-PATH taskrail (<repo>/bin/taskrail) is stale versus the working tree; run 'task taskrail:install'
```

The source was current. The cause is the *builder*, not the source:

```
plain shell go: /usr/local/go/bin/go                           -> go1.26.0
mise exec go:   ~/.local/share/mise/installs/go/1.26.5/bin/go  -> go1.26.5
```

Same source, same `-trimpath`, same `CGO_ENABLED=0`, different bytes:

```
af015fe77118cdefa79ad5a41f580cba9380563dbca3be9a8b3347957784f48a  built with go1.26.0
5954038c08318dc405c27a1fed5d0b040297e36a7b6b8cd3c1cee2570aa225f1  built with go1.26.5
```

`internal/toolchain/cmd/freshcheck` byte-compares, so whenever `taskrail:install` and
`taskrail:check` run under different Go toolchains the guard reports "stale" for a clean
tree. `mise.toml` pins `go = "1.26"`, which floats to 1.26.5; `/usr/local/go` here is
1.26.0. Any shell where `/usr/local/go/bin` precedes mise's path flaps the guard — the
agent shell does, and a fresh contributor with a system Go plausibly would too.

Running both halves under mise is consistent and passes:

```sh
mise exec -- task taskrail:install
mise exec -- task taskrail:check   # clean
```

### Why this matters beyond noise

The failure message sends you to `task taskrail:install`, which does **not** fix it if you
rerun in the same wrong-toolchain shell — you get an install/check loop that never
converges, with no hint that the toolchain is the variable.

### Options

- Pin `go = "1.26.5"` exactly in `mise.toml` (the pin is already described as the single
  source of truth for the developer environment).
- Have `freshcheck` distinguish "built by a different toolchain" from "stale versus the
  working tree" — e.g. compare `go version -m` build info before falling back to a byte
  comparison — so the remedy it names actually resolves what it detected.
- At minimum, document the toolchain sensitivity next to the guard.

## Relationship to existing tracked work

`T-123-contributor-binary-resolution` covers both findings. Finding 2 is a distinct
failure mode — the guard misreporting rather than the wrong binary being selected —
but it was folded into T-123 (2026-07-28) rather than split out, because both are the
same contributor-facing defect: the freshness guard reporting a condition its own
prescribed remedy cannot resolve. T-123's Description and Acceptance now carry it.
