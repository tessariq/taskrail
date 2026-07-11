package taskrail

import (
	"fmt"
	"sort"
	"strings"
)

// DependencyGraph renders the current task dependency DAG as Graphviz DOT or
// Mermaid text. It is strictly read-only and snapshot-only: it loads the current
// task files and never writes STATE.md or task files. Nodes are tasks (labelled
// id/title/status), edges point from a task to each of its dependencies, matching
// how a task file's `dependencies:` list reads ("this task depends on these").
func (s *Service) DependencyGraph(format string) (string, error) {
	tasks, err := s.loadTasks()
	if err != nil {
		return "", err
	}
	return renderDependencyGraph(tasks, format)
}

// renderDependencyGraph is the IO-free core: it turns an already-loaded task set
// into graph text. Node order follows the caller's task order (loadTasks sorts by
// id) and each task's edges are sorted by dependency id, so the output is stable
// and diffable regardless of dependency authoring order.
func renderDependencyGraph(tasks []*Task, format string) (string, error) {
	switch format {
	case "dot":
		return renderDOT(tasks), nil
	case "mermaid":
		return renderMermaid(tasks), nil
	default:
		return "", fmt.Errorf("unknown graph format %q: want dot or mermaid", format)
	}
}

func renderDOT(tasks []*Task) string {
	var b strings.Builder
	b.WriteString("digraph taskrail {\n")
	b.WriteString("  rankdir=LR;\n")
	for _, task := range tasks {
		fmt.Fprintf(&b, "  %q [label=\"%s\"];\n", task.Frontmatter.ID, dotLabel(task))
	}
	for _, task := range tasks {
		for _, dep := range sortedDeps(task) {
			fmt.Fprintf(&b, "  %q -> %q;\n", task.Frontmatter.ID, dep)
		}
	}
	b.WriteString("}\n")
	return b.String()
}

func renderMermaid(tasks []*Task) string {
	var b strings.Builder
	b.WriteString("graph LR\n")
	for _, task := range tasks {
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", mermaidID(task.Frontmatter.ID), mermaidLabel(task))
	}
	for _, task := range tasks {
		for _, dep := range sortedDeps(task) {
			fmt.Fprintf(&b, "  %s --> %s\n", mermaidID(task.Frontmatter.ID), mermaidID(dep))
		}
	}
	return b.String()
}

// sortedDeps returns a task's dependency ids in a stable order without mutating
// the underlying frontmatter slice.
func sortedDeps(task *Task) []string {
	deps := append([]string(nil), task.Frontmatter.Dependencies...)
	sort.Strings(deps)
	return deps
}

// dotLabel builds an escaped `id\ntitle\n(status)` label; the `\n` are literal
// two-character escapes Graphviz renders as line breaks.
func dotLabel(task *Task) string {
	return strings.Join([]string{
		dotEscape(task.Frontmatter.ID),
		dotEscape(task.Frontmatter.Title),
		"(" + dotEscape(task.Frontmatter.Status) + ")",
	}, `\n`)
}

// dotEscape escapes backslashes and double-quotes so a title never breaks out of
// its quoted DOT label, and collapses any literal newline (valid in a YAML title)
// to the same two-character `\n` escape used between label fields — a raw newline
// would split the quoted string across physical lines and yield invalid DOT.
func dotEscape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\r", "")
	return strings.ReplaceAll(s, "\n", `\n`)
}

// mermaidLabel builds an `id<br/>title<br/>(status)` label; Mermaid renders
// `<br/>` as a line break inside the quoted node text.
func mermaidLabel(task *Task) string {
	return strings.Join([]string{
		mermaidText(task.Frontmatter.ID),
		mermaidText(task.Frontmatter.Title),
		"(" + mermaidText(task.Frontmatter.Status) + ")",
	}, "<br/>")
}

// mermaidText neutralizes the double-quote that would otherwise close a Mermaid
// `["..."]` node label, and collapses any literal newline (valid in a YAML title)
// to `<br/>` — a raw newline would split the node statement across lines and break
// Mermaid's line-oriented parsing.
func mermaidText(s string) string {
	s = strings.ReplaceAll(s, `"`, "'")
	s = strings.ReplaceAll(s, "\r", "")
	return strings.ReplaceAll(s, "\n", "<br/>")
}

// mermaidID turns a task id into a safe Mermaid node identifier: Mermaid node ids
// are bare tokens, so any character outside [A-Za-z0-9_] (notably the hyphen in
// `T-1`) is replaced with `_`. Task ids differ in their safe characters, so this
// stays collision-free.
func mermaidID(id string) string {
	var b strings.Builder
	for _, r := range id {
		switch {
		case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
