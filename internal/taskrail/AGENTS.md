# internal/taskrail

- The packaged skills' only source is `internal/taskrail/skills/`, which is
  embedded with `//go:embed skills` and installed by `taskrail init
  --with-skills`. Never edit the committed copies under `.agents/skills/` or
  `.claude/skills/`. After changing a packaged skill, run `task skills:regen`,
  which regenerates both copies and re-runs the parity check (`task
  check:skills`). Contract: `docs/workflow/skills-productization.md`.
- Put no other file in `internal/taskrail/skills/`: everything there ships to
  adopters.
- Packaged skills call the binary through `${TASKRAIL:-taskrail}`, never
  `go run`.
- The full package test suite takes a few minutes. Iterate with
  `go test ./internal/taskrail -run '^TestName$'`, and use `task test:short`
  to skip the working-binary skill-evaluation suites, which CI runs in full.
