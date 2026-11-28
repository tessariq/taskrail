# Go Code Conventions

Applies to all Go code in this repository, including `cmd/`.

- Run `gofmt` on changed files. `task lint` (golangci-lint, `.golangci.yml`)
  sets a hard ceiling on function size and complexity; aim well below it with
  focused functions and early returns.
- Keep new files to about 200–400 lines. When a change grows a file that is
  already over 800 lines, extract the responsibility you are touching or ask
  first; do not split files you are not otherwise changing.
- Comments and godoc explain why, not what. Cite the task ID for code that
  exists because of an incident (for example `T-123`).
- Prefer the standard library; ask before adding a dependency.
- Start behavior changes with a failing test. Keep unit tests dominant, use
  temporary directories for filesystem tests, and keep test infrastructure
  light. A bug fix gets one regression test that fails without the fix.
- CI runs native Linux (amd64 and arm64), Windows, and macOS. Build paths with
  `filepath`, expect CRLF input, route tracked planning writes through the
  existing `repotx`/`durablefs` transaction code instead of new raw `os` writes,
  and run `task test:filesystem` when you touch file I/O.
- `internal/toolchain` holds no production code: its tests guard the
  repository's CI, Taskfile, hooks, and toolchain pins. When you change one of
  those files, run `go test ./internal/toolchain/...`.
