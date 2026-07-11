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

// scaffoldSpec renders the standard section skeleton for a spec authored by
// `taskrail spec add <version>`. It lives beside starterSpecV010 because both
// author spec bodies; unlike that fixed v0.1.0 starter, this one is
// version-parameterized. The Potential Features section is intentionally
// area-free (no `###` headings) so the fresh spec has zero coverable areas and
// coverage reports N/A; each section carries a TODO prompting the author.
func scaffoldSpec(version string) string {
	return strings.TrimSpace("# Taskrail "+version+`

## Summary

_TODO: one-paragraph summary of what this spec version proves or adds._

## Goals

_TODO: enumerate the goals this spec version commits to._

## Potential Features

_TODO: add `+"`### Feature Area`"+` headings here to define coverable areas.
Until an area is added, this spec has zero coverable areas and `+"`taskrail coverage`"+`
reports N/A for it._

## Caution

_TODO: note the risks, sharp edges, and scope traps for this version._

## Recommendation About LLM Support

_TODO: state the stance on built-in LLM/provider integration for this version._

## Explicitly Excluded

_TODO: list what this version deliberately does not do._
`) + "\n"
}

// renderNewTaskBody produces the placeholder body for a scaffolded task: the
// four sections agents and humans fill in before starting work. A non-empty
// provenance line is appended to the Description so a follow-up records in its
// durable file that it derives from a parent, not only in the dependency list.
func renderNewTaskBody(id, title, provenance string) string {
	description := "TODO: describe the work and link the spec section."
	if provenance != "" {
		description += "\n\n" + provenance
	}
	return fmt.Sprintf(`# %s %s

## Description

%s

## Acceptance

- TODO: define acceptance criteria.

## Verification Notes

- TODO: record verification evidence paths.

## Implementation Notes
`, id, title, description)
}

// renderFollowupTaskBody produces the body for a verify-created follow-up task.
// Unlike renderNewTaskBody its Description is evidence-populated from the
// verification finding and it deliberately omits an Implementation Notes section,
// so the two task-body scaffolds stay distinct while living side by side
// (T-028/T-038).
func renderFollowupTaskBody(id, title, description string) string {
	return fmt.Sprintf(`# %s %s

## Description

%s

## Acceptance

- The follow-up issue described by verification is resolved.
- Verification evidence is updated.

## Verification Notes

- Re-run task-scoped verification after implementing the fix.
`, id, title, description)
}

func renderVerificationPlan(task *Task, input VerifyInput, followupTaskID string) string {
	var builder strings.Builder
	builder.WriteString("# Verification Plan\n\n")
	builder.WriteString(fmt.Sprintf("- Task: `%s`\n", task.Frontmatter.ID))
	builder.WriteString(fmt.Sprintf("- Title: %s\n", task.Frontmatter.Title))
	builder.WriteString(fmt.Sprintf("- Requested result: %s\n", input.Result))
	builder.WriteString(fmt.Sprintf("- Summary: %s\n", input.Summary))
	if input.Details != "" {
		builder.WriteString(fmt.Sprintf("- Details: %s\n", input.Details))
	}
	if followupTaskID != "" {
		builder.WriteString(fmt.Sprintf("- Follow-up task to create: `%s`\n", followupTaskID))
	}
	return builder.String()
}

func renderVerificationReportMarkdown(report VerificationArtifact) string {
	var builder strings.Builder
	builder.WriteString("# Verification Report\n\n")
	builder.WriteString(fmt.Sprintf("- Task: `%s`\n", report.TaskID))
	builder.WriteString(fmt.Sprintf("- Title: %s\n", report.TaskTitle))
	builder.WriteString(fmt.Sprintf("- Result: %s\n", report.Result))
	builder.WriteString(fmt.Sprintf("- Summary: %s\n", report.Summary))
	if report.Details != "" {
		builder.WriteString(fmt.Sprintf("- Details: %s\n", report.Details))
	}
	builder.WriteString(fmt.Sprintf("- Generated at: %s\n", report.GeneratedAt))
	builder.WriteString(fmt.Sprintf("- Spec ref: `%s`\n", report.SpecRef))
	for _, artifact := range report.Artifacts {
		builder.WriteString(fmt.Sprintf("- Artifact: `%s`\n", artifact))
	}
	if report.FollowupTaskID != "" {
		builder.WriteString(fmt.Sprintf("- Follow-up task: `%s`\n", report.FollowupTaskID))
	}
	return builder.String()
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
