package taskrail

import (
	"fmt"
	"strings"
	"time"
)

func starterState(now time.Time) *State {
	frontmatter := StateFrontmatter{
		SchemaVersion:          stateSchemaVersion,
		UpdatedAt:              timestamp(now),
		ActiveSpecVersion:      "v0.1.0",
		ActiveSpecPath:         "specs/v0.1.0.md",
		CurrentTask:            "",
		CurrentTaskTitle:       "",
		StatusSummary:          "idle",
		Blockers:               []string{},
		NextAction:             "Create initial Taskrail tasks and begin tracked work",
		LastVerificationResult: "Not yet run",
		RelevantArtifacts:      []string{},
		ContinuationNotes: []string{
			"This repository is using manual Taskrail-style workflow scaffolding until the product replaces more of the bootstrap steps.",
		},
	}

	state := &State{Frontmatter: frontmatter}
	state.Body = renderStateBody(state.Frontmatter, nil)
	return state
}

func starterSpecsReadme() string {
	return strings.TrimSpace(`# Specs

	This repository uses versioned specs under `+"`specs/`"+`.

	- Add the active release spec as `+"`specs/v0.1.0.md`"+`.
	- Keep `+"`planning/tasks/`"+` linked to live spec headings.
	`) + "\n"
}

func starterSpecV010() string {
	return strings.TrimSpace(`# Taskrail v0.1.0

	## Summary

	Starter Taskrail spec created by `+"`taskrail init`"+`.
	`) + "\n"
}

func renderStateBody(state StateFrontmatter, tasks []*Task) string {
	var builder strings.Builder
	builder.WriteString("# STATE\n\n")
	builder.WriteString("## Active Spec\n\n")
	builder.WriteString(fmt.Sprintf("- `%s`\n\n", state.ActiveSpecPath))
	builder.WriteString("## Current Focus\n\n")
	if state.CurrentTask == "" {
		builder.WriteString("- Task: none\n")
	} else {
		builder.WriteString(fmt.Sprintf("- Task: `%s`\n", state.CurrentTask))
		builder.WriteString(fmt.Sprintf("- Title: %s\n", state.CurrentTaskTitle))
	}
	builder.WriteString("\n## Status\n\n")
	builder.WriteString(fmt.Sprintf("- %s\n\n", state.StatusSummary))
	builder.WriteString("## Blockers\n\n")
	if len(state.Blockers) == 0 {
		builder.WriteString("- None\n\n")
	} else {
		for _, blocker := range state.Blockers {
			builder.WriteString(fmt.Sprintf("- %s\n", blocker))
		}
		builder.WriteString("\n")
	}
	builder.WriteString("## Last Verification\n\n")
	builder.WriteString(fmt.Sprintf("- %s\n\n", state.LastVerificationResult))
	builder.WriteString("## Next Action\n\n")
	builder.WriteString(fmt.Sprintf("- %s\n\n", state.NextAction))
	builder.WriteString("## Relevant Artifacts\n\n")
	if len(state.RelevantArtifacts) == 0 {
		builder.WriteString("- None\n\n")
	} else {
		for _, artifact := range state.RelevantArtifacts {
			builder.WriteString(fmt.Sprintf("- `%s`\n", artifact))
		}
		builder.WriteString("\n")
	}
	builder.WriteString("## Notes\n\n")
	if len(state.ContinuationNotes) == 0 {
		builder.WriteString("- None\n")
	} else {
		for _, note := range state.ContinuationNotes {
			builder.WriteString(fmt.Sprintf("- %s\n", note))
		}
	}
	if len(tasks) > 0 {
		counts := taskCounts(tasks)
		builder.WriteString("\n## Task Counts\n\n")
		builder.WriteString(fmt.Sprintf("- todo: %d\n", counts["todo"]))
		builder.WriteString(fmt.Sprintf("- in_progress: %d\n", counts["in_progress"]))
		builder.WriteString(fmt.Sprintf("- completed: %d\n", counts["completed"]))
		builder.WriteString(fmt.Sprintf("- blocked: %d\n", counts["blocked"]))
		builder.WriteString(fmt.Sprintf("- cancelled: %d\n", counts["cancelled"]))
	}
	return builder.String()
}

func taskCounts(tasks []*Task) map[string]int {
	counts := map[string]int{"todo": 0, "in_progress": 0, "completed": 0, "blocked": 0, "cancelled": 0}
	for _, task := range tasks {
		counts[task.Frontmatter.Status]++
	}
	return counts
}

func timestamp(now time.Time) string {
	return now.UTC().Format(time.RFC3339)
}
