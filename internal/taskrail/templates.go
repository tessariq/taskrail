package taskrail

import (
	"fmt"
	"strings"
	"time"
)

// starterState renders the fresh state a newly initialized repository publishes.
// schema is the layout's implied state schema, so a fresh layout-2 repository
// writes schema 2 directly and seeds no continuation prose.
func starterState(now time.Time, schema int) *State {
	notes := []string{}
	if schema >= stateSchemaVersionLayout2 {
		notes = nil
	}
	frontmatter := StateFrontmatter{
		SchemaVersion:          schema,
		UpdatedAt:              timestamp(now),
		ActiveSpecVersion:      "v0.1.0",
		ActiveSpecPath:         "specs/v0.1.0.md",
		CurrentTask:            "",
		CurrentTaskTitle:       "",
		StatusSummary:          statusSummaryIdle,
		Blockers:               []string{},
		NextAction:             "Create initial Taskrail tasks and begin tracked work",
		LastVerificationResult: "Not yet run",
		RelevantArtifacts:      []string{},
		ContinuationNotes:      notes,
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

// starterSpecV010 renders the starter specs/v0.1.0.md a fresh init writes. Its
// lines are flush left so the markdown a new repository receives carries no
// raw-string indentation artifact, and its `### Summary` area under
// `## Potential Features` makes the `summary` anchor — the one `spec show
// --anchors` advertises and `task new --spec-ref specs/v0.1.0.md#summary`
// accepts — a coverable area, so anchor listing and coverage agree from the
// first command in a fresh repository.
func starterSpecV010() string {
	return `# Taskrail v0.1.0

## Summary

Starter Taskrail spec created by ` + "`taskrail init`" + `.

## Potential Features

### Summary

Starter coverable area for a fresh repository's first tracked work; link
tasks with ` + "`--spec-ref specs/v0.1.0.md#summary`" + ` until real areas
are authored.
`
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

// renderNewTaskBody produces the shared body contract for native task creation.
// A non-empty provenance line records an implementation follow-up's parent in
// its durable file, not only in the dependency list.
func renderNewTaskBody(id, title, provenance string) string {
	description := "TODO: describe the outcome's invariant and relevant spec section."
	if provenance != "" {
		description += "\n\n" + provenance
	}
	return renderOutcomeFocusedTaskBody(id, title, description)
}

func renderOutcomeFocusedTaskBody(id, title, description string) string {
	// A bare scaffold may carry no title; keep the heading free of a trailing space.
	heading := strings.TrimRight(fmt.Sprintf("# %s %s", id, title), " ")
	return fmt.Sprintf(`%s

## Description

%s

TODO: state one independently meaningful outcome. Do not bundle independently valuable outcomes or create a fragment without independent value.

## Acceptance

- TODO: define observable acceptance criteria for the outcome.

## Verification Notes

- TODO: map each criterion to setup, action, expected observation, and the cheapest sufficient evidence.
- TODO: record later evidence paths after verification.

## Implementation Notes
`, heading, description)
}

// renderFollowupTaskBody records the deferred outcome and makes the newly
// created task its integrated owner while retaining the native body contract.
func renderFollowupTaskBody(id, title, description, sourceID string) string {
	description = fmt.Sprintf("Deferred independently meaningful outcome: %s\n\nThis task owns integrated delivery of the deferred outcome and its invariant after %s's verification.", description, sourceID)
	return renderOutcomeFocusedTaskBody(id, title, description)
}

func renderVerificationPlan(task *Task, input VerifyInput, verificationID, previousVerificationID, followupTaskID string) string {
	var builder strings.Builder
	builder.WriteString("# Verification Plan\n\n")
	fmt.Fprintf(&builder, "- Task: `%s`\n", task.Frontmatter.ID)
	fmt.Fprintf(&builder, "- Title: %s\n", task.Frontmatter.Title)
	fmt.Fprintf(&builder, "- Requested result: %s\n", input.Result)
	fmt.Fprintf(&builder, "- Verification ID: %s\n", verificationID)
	if task.Frontmatter.Status == "completed" && input.Result == "pass" {
		fmt.Fprintf(&builder, "- Observed completion ID: %s\n", task.Frontmatter.CompletionID)
	} else {
		builder.WriteString("- Observed completion ID: none\n")
	}
	if previousVerificationID == "" {
		previousVerificationID = "none"
	}
	fmt.Fprintf(&builder, "- Previous verification ID: %s\n", previousVerificationID)
	fmt.Fprintf(&builder, "- Summary: %s\n", input.Summary)
	if input.Details != "" {
		fmt.Fprintf(&builder, "- Details: %s\n", input.Details)
	}
	if followupTaskID != "" {
		fmt.Fprintf(&builder, "- Follow-up task to create: `%s`\n", followupTaskID)
	}
	return builder.String()
}

func renderVerificationReportMarkdown(report VerificationArtifact) string {
	var builder strings.Builder
	builder.WriteString("# Verification Report\n\n")
	fmt.Fprintf(&builder, "- Task: `%s`\n", report.TaskID)
	fmt.Fprintf(&builder, "- Title: %s\n", report.TaskTitle)
	fmt.Fprintf(&builder, "- Result: %s\n", report.Result)
	fmt.Fprintf(&builder, "- Verification ID: %s\n", report.VerificationID)
	if report.ObservedCompletionID == nil {
		builder.WriteString("- Observed completion ID: none\n")
	} else {
		fmt.Fprintf(&builder, "- Observed completion ID: %s\n", *report.ObservedCompletionID)
	}
	if report.PreviousVerificationID == nil {
		builder.WriteString("- Previous verification ID: none\n")
	} else {
		fmt.Fprintf(&builder, "- Previous verification ID: %s\n", *report.PreviousVerificationID)
	}
	fmt.Fprintf(&builder, "- Summary: %s\n", report.Summary)
	if report.Details != "" {
		fmt.Fprintf(&builder, "- Details: %s\n", report.Details)
	}
	fmt.Fprintf(&builder, "- Generated at: %s\n", report.GeneratedAt)
	fmt.Fprintf(&builder, "- Spec ref: `%s`\n", report.SpecRef)
	for _, artifact := range report.Artifacts {
		fmt.Fprintf(&builder, "- Artifact: `%s`\n", artifact)
	}
	if report.FollowupTaskID != "" {
		fmt.Fprintf(&builder, "- Follow-up task: `%s`\n", report.FollowupTaskID)
	}
	return builder.String()
}

func renderStateBody(state StateFrontmatter, tasks []*Task) string {
	body := renderStateBodySchema1(state, tasks)
	if state.SchemaVersion >= stateSchemaVersionLayout2 {
		return stripStateNotesSection(body)
	}
	return body
}

func renderStateBodySchema1(state StateFrontmatter, tasks []*Task) string {
	var builder strings.Builder
	builder.WriteString("# STATE\n\n")
	builder.WriteString("## Active Spec\n\n")
	fmt.Fprintf(&builder, "- `%s`\n\n", state.ActiveSpecPath)
	builder.WriteString("## Current Focus\n\n")
	if state.CurrentTask == "" {
		builder.WriteString("- Task: none\n")
	} else {
		fmt.Fprintf(&builder, "- Task: `%s`\n", state.CurrentTask)
		fmt.Fprintf(&builder, "- Title: %s\n", state.CurrentTaskTitle)
	}
	builder.WriteString("\n## Status\n\n")
	fmt.Fprintf(&builder, "- %s\n\n", state.StatusSummary)
	builder.WriteString("## Blockers\n\n")
	if len(state.Blockers) == 0 {
		builder.WriteString("- None\n\n")
	} else {
		for _, blocker := range state.Blockers {
			fmt.Fprintf(&builder, "- %s\n", blocker)
		}
		builder.WriteString("\n")
	}
	builder.WriteString("## Last Verification\n\n")
	fmt.Fprintf(&builder, "- %s\n\n", state.LastVerificationResult)
	builder.WriteString("## Next Action\n\n")
	fmt.Fprintf(&builder, "- %s\n\n", state.NextAction)
	builder.WriteString("## Relevant Artifacts\n\n")
	if len(state.RelevantArtifacts) == 0 {
		builder.WriteString("- None\n\n")
	} else {
		for _, artifact := range state.RelevantArtifacts {
			fmt.Fprintf(&builder, "- `%s`\n", artifact)
		}
		builder.WriteString("\n")
	}
	builder.WriteString("## Notes\n\n")
	if len(state.ContinuationNotes) == 0 {
		builder.WriteString("- None\n")
	} else {
		for _, note := range state.ContinuationNotes {
			fmt.Fprintf(&builder, "- %s\n", note)
		}
	}
	if len(tasks) > 0 {
		counts := taskCounts(tasks)
		builder.WriteString("\n## Task Counts\n\n")
		fmt.Fprintf(&builder, "- todo: %d\n", counts["todo"])
		fmt.Fprintf(&builder, "- in_progress: %d\n", counts["in_progress"])
		fmt.Fprintf(&builder, "- completed: %d\n", counts["completed"])
		fmt.Fprintf(&builder, "- blocked: %d\n", counts["blocked"])
		fmt.Fprintf(&builder, "- cancelled: %d\n", counts["cancelled"])
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
