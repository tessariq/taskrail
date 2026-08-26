package taskrail

import (
	"encoding/hex"
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

// ActiveSpecDependencyGraph exports the active-spec cohort plus the dependency
// context required to make its edges meaningful. It refuses an incoherent active
// state before rendering any partial graph.
func (s *Service) ActiveSpecDependencyGraph(format string) (string, error) {
	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return "", err
	}
	scope, err := s.activeStatsScope(state, tasks)
	if err != nil {
		return "", err
	}
	return renderActiveSpecDependencyGraph(scope, format)
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

func renderActiveSpecDependencyGraph(scope activeStatsScope, format string) (string, error) {
	switch format {
	case "dot":
		return renderActiveSpecDOT(scope), nil
	case "mermaid":
		return renderActiveSpecMermaid(scope), nil
	default:
		return "", fmt.Errorf("unknown graph format %q: want dot or mermaid", format)
	}
}

func renderActiveSpecDOT(scope activeStatsScope) string {
	var b strings.Builder
	b.WriteString("digraph taskrail {\n")
	b.WriteString("  rankdir=LR;\n")
	writeDOTScopeComments(&b, scope.report)
	for _, task := range scope.graph {
		label := dotLabel(task)
		if !scope.subjectID[task.Frontmatter.ID] {
			label += `\n[off-spec context]`
			fmt.Fprintf(&b, "  %q [label=\"%s\", style=\"dashed\", color=\"gray\"];\n", task.Frontmatter.ID, label)
			continue
		}
		fmt.Fprintf(&b, "  %q [label=\"%s\"];\n", task.Frontmatter.ID, label)
	}
	for _, ref := range scope.missing {
		id := "missing:" + ref
		fmt.Fprintf(&b, "  %q [label=\"%s\", style=\"dashed\", color=\"red\"];\n", id, dotEscape(id))
	}
	for _, task := range scope.graph {
		for _, dep := range sortedDeps(task) {
			if _, ok := scope.byID[dep]; !ok {
				dep = "missing:" + dep
			}
			fmt.Fprintf(&b, "  %q -> %q;\n", task.Frontmatter.ID, dep)
		}
	}
	b.WriteString("}\n")
	return b.String()
}

func writeDOTScopeComments(b *strings.Builder, scope StatsScope) {
	fmt.Fprintf(b, "  // active-spec scope: %s; subjects %d; excluded %d; dependency context %d; malformed subjects %d; malformed ledger %d\n",
		scope.ActiveSpecPath, scope.SubjectTaskCount, scope.ExcludedTaskCount, scope.DependencyContextTaskCount, scope.MalformedSubjectCount, scope.MalformedLedgerCount)
	for _, issue := range scope.SpecRefIssues {
		fmt.Fprintf(b, "  // spec-ref issue: %s | %s | %s\n", dotEscape(issue.TaskID), dotEscape(issue.SpecRef), dotEscape(issue.Classification))
	}
}

func renderActiveSpecMermaid(scope activeStatsScope) string {
	var b strings.Builder
	contextIDs := make([]string, 0)
	missingIDs := make([]string, 0, len(scope.missing))
	b.WriteString("graph LR\n")
	fmt.Fprintf(&b, "  %%%% active-spec scope: %s; subjects %d; excluded %d; dependency context %d; malformed subjects %d; malformed ledger %d\n",
		scope.report.ActiveSpecPath, scope.report.SubjectTaskCount, scope.report.ExcludedTaskCount, scope.report.DependencyContextTaskCount, scope.report.MalformedSubjectCount, scope.report.MalformedLedgerCount)
	for _, issue := range scope.report.SpecRefIssues {
		fmt.Fprintf(&b, "  %%%% spec-ref issue: %s | %s | %s\n", mermaidText(issue.TaskID), mermaidText(issue.SpecRef), mermaidText(issue.Classification))
	}
	for _, task := range scope.graph {
		label := mermaidLabel(task)
		if !scope.subjectID[task.Frontmatter.ID] {
			label += "<br/>[off-spec context]"
			contextIDs = append(contextIDs, scopedMermaidTaskID(task.Frontmatter.ID))
		}
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", scopedMermaidTaskID(task.Frontmatter.ID), label)
	}
	for _, ref := range scope.missing {
		id := "missing:" + ref
		missingID := missingMermaidID(ref)
		missingIDs = append(missingIDs, missingID)
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", missingID, mermaidText(id))
	}
	if len(contextIDs) > 0 {
		b.WriteString("  classDef offSpec fill:#f3f3f3,stroke:#888,stroke-dasharray: 5 5;\n")
		fmt.Fprintf(&b, "  class %s offSpec\n", strings.Join(contextIDs, ","))
	}
	if len(missingIDs) > 0 {
		b.WriteString("  classDef missing fill:#fee,stroke:#c00,stroke-width:2px,stroke-dasharray: 5 5;\n")
		fmt.Fprintf(&b, "  class %s missing\n", strings.Join(missingIDs, ","))
	}
	for _, task := range scope.graph {
		for _, dep := range sortedDeps(task) {
			if _, ok := scope.byID[dep]; !ok {
				fmt.Fprintf(&b, "  %s --> %s\n", scopedMermaidTaskID(task.Frontmatter.ID), missingMermaidID(dep))
				continue
			}
			fmt.Fprintf(&b, "  %s --> %s\n", scopedMermaidTaskID(task.Frontmatter.ID), scopedMermaidTaskID(dep))
		}
	}
	return b.String()
}

func scopedMermaidTaskID(id string) string {
	return "task_" + hex.EncodeToString([]byte(id))
}

// missingMermaidID encodes the entire missing reference to avoid collapsing two
// distinct dependency strings through Mermaid's lossy identifier normalization.
func missingMermaidID(ref string) string {
	return "missing_" + hex.EncodeToString([]byte(ref))
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
