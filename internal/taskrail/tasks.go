package taskrail

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// numericPrefixPattern extracts a task id's numeric identity. Ids may be bare
// (`T-085`) or slug-suffixed (`T-076-ingestion-commands`); the `T-<number>`
// prefix — not the full string — is what id allocation and collision detection
// key on, matching how dependencies and renumbering treat task identity. The
// digits must end at the id or a `-` slug boundary, so a malformed id like
// `T-1abc` is not mistaken for prefix `1`.
var numericPrefixPattern = regexp.MustCompile(`^T-(\d+)(?:-.*)?$`)

// taskNumericPrefix returns the numeric prefix of a task id and whether the id
// has one.
func taskNumericPrefix(id string) (int, bool) {
	m := numericPrefixPattern.FindStringSubmatch(id)
	if m == nil {
		return 0, false
	}
	num, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return num, true
}

// taskIDPrefix returns the literal `T-<digits>` identity segment of a task id,
// preserving the digits exactly as written. Rename keeps this segment fixed and
// swaps only the slug, so it must not reformat the number (which would change the
// id it claims to preserve).
func taskIDPrefix(id string) (string, bool) {
	m := numericPrefixPattern.FindStringSubmatch(id)
	if m == nil {
		return "", false
	}
	return "T-" + m[1], true
}

func taskByID(tasks []*Task, id string) (*Task, bool) {
	for _, task := range tasks {
		if task.Frontmatter.ID == id {
			return task, true
		}
	}
	return nil, false
}

// exactTaskByID resolves only the complete persisted identifier. Numeric-prefix,
// slug, and whitespace normalization are deliberately not identity semantics.
func exactTaskByID(tasks []*Task, id string) (*Task, error) {
	if task, ok := taskByID(tasks, id); ok {
		return task, nil
	}
	return nil, fmt.Errorf("task %q not found: identity must be an exact full persisted task ID", id)
}

func eligibleTasks(tasks []*Task) []*Task {
	eligible := make([]*Task, 0)
	for _, task := range tasks {
		if task.Frontmatter.Status != "todo" {
			continue
		}
		if dependenciesResolved(task, tasks) {
			eligible = append(eligible, task)
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		left := priorityRank[eligible[i].Frontmatter.Priority]
		right := priorityRank[eligible[j].Frontmatter.Priority]
		if left != right {
			return left < right
		}
		return eligible[i].Frontmatter.ID < eligible[j].Frontmatter.ID
	})
	return eligible
}

// inProgressTasks returns the tasks currently marked in_progress, preserving
// input order. Both validation (which reports a current_task disagreement) and
// repair (which reconciles it) derive the current_task invariant from this set,
// so the rule lives in one place.
func inProgressTasks(tasks []*Task) []*Task {
	active := make([]*Task, 0, 1)
	for _, task := range tasks {
		if task.Frontmatter.Status == "in_progress" {
			active = append(active, task)
		}
	}
	return active
}

func dependenciesResolved(task *Task, tasks []*Task) bool {
	for _, dep := range task.Frontmatter.Dependencies {
		found, ok := taskByID(tasks, dep)
		if !ok || found.Frontmatter.Status != "completed" {
			return false
		}
	}
	return true
}

// verificationNoteLine renders the committed, path-free verification note that
// `verify` appends to a task body. It is the single source of the note's wording
// so gap analysis (taskVerificationRecorded) can detect a recorded verification
// from committed content without the two drifting apart.
func verificationNoteLine(ts, result string) string {
	return fmt.Sprintf("- %s: verification %s", ts, result)
}

// implementationNotesHeading is the section verify and block append their notes to.
const implementationNotesHeading = "## Implementation Notes"

// hasImplementationNotesHeading reports whether body already carries the section.
// It matches a whole line, so neither a heading that merely starts with the same
// words nor a mention inside prose counts as the section.
func hasImplementationNotesHeading(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimRight(line, " \t\r") == implementationNotesHeading {
			return true
		}
	}
	return false
}

// appendTaskNote adds one note line to the task's Implementation Notes section,
// scaffolding the section only when the body genuinely lacks it. Matching the
// heading line itself matters: renderNewTaskBody ends the scaffold *at* the heading
// with no blank line after it, so a probe for "heading + blank line" misses every
// fresh task and stamps a duplicate heading into a committed file.
func appendTaskNote(task *Task, line string) {
	body := strings.TrimRight(task.Body, "\n")
	if !hasImplementationNotesHeading(body) {
		body += "\n\n" + implementationNotesHeading
	}
	// A body that ends at the heading — including one just scaffolded above — needs
	// the section's blank line before its first note; one that already has notes
	// just continues the list.
	separator := "\n"
	if strings.HasSuffix(body, implementationNotesHeading) {
		separator = "\n\n"
	}
	task.Body = body + separator + line + "\n"
}

func nextTaskID(tasks []*Task) string {
	max := 0
	for _, task := range tasks {
		if num, ok := taskNumericPrefix(task.Frontmatter.ID); ok && num > max {
			max = num
		}
	}
	return fmt.Sprintf("T-%03d", max+1)
}
