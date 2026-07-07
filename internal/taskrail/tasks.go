package taskrail

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func taskByID(tasks []*Task, id string) (*Task, bool) {
	for _, task := range tasks {
		if task.Frontmatter.ID == id {
			return task, true
		}
	}
	return nil, false
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

func appendTaskNote(task *Task, line string) {
	section := "## Implementation Notes\n\n"
	if strings.Contains(task.Body, section) {
		task.Body = strings.TrimRight(task.Body, "\n") + "\n" + line + "\n"
		return
	}
	task.Body = strings.TrimRight(task.Body, "\n") + "\n\n" + section + line + "\n"
}

func nextTaskID(tasks []*Task) string {
	max := 0
	for _, task := range tasks {
		if !strings.HasPrefix(task.Frontmatter.ID, "T-") {
			continue
		}
		num, err := strconv.Atoi(strings.TrimPrefix(task.Frontmatter.ID, "T-"))
		if err == nil && num > max {
			max = num
		}
	}
	return fmt.Sprintf("T-%03d", max+1)
}
