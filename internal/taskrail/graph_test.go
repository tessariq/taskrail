package taskrail

import "testing"

func TestRenderDependencyGraphDOTOnSeededDAG(t *testing.T) {
	tasks := []*Task{
		depTask("T-1", "completed"),
		depTask("T-2", "todo", "T-1"),
	}
	got, err := renderDependencyGraph(tasks, "dot")
	if err != nil {
		t.Fatalf("dot: %v", err)
	}
	want := "digraph taskrail {\n" +
		"  rankdir=LR;\n" +
		`  "T-1" [label="T-1\nTask T-1\n(completed)"];` + "\n" +
		`  "T-2" [label="T-2\nTask T-2\n(todo)"];` + "\n" +
		`  "T-2" -> "T-1";` + "\n" +
		"}\n"
	if got != want {
		t.Errorf("dot mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestRenderDependencyGraphMermaidOnSeededDAG(t *testing.T) {
	tasks := []*Task{
		depTask("T-1", "completed"),
		depTask("T-2", "todo", "T-1"),
	}
	got, err := renderDependencyGraph(tasks, "mermaid")
	if err != nil {
		t.Fatalf("mermaid: %v", err)
	}
	want := "graph LR\n" +
		`  T_1["T-1<br/>Task T-1<br/>(completed)"]` + "\n" +
		`  T_2["T-2<br/>Task T-2<br/>(todo)"]` + "\n" +
		"  T_2 --> T_1\n"
	if got != want {
		t.Errorf("mermaid mismatch:\n got %q\nwant %q", got, want)
	}
}

// Edges are stable regardless of dependency authoring order: a task listing its
// dependencies out of sorted order still emits them sorted.
func TestRenderDependencyGraphSortsEdgesDeterministically(t *testing.T) {
	tasks := []*Task{
		depTask("T-1", "todo"),
		depTask("T-2", "todo"),
		depTask("T-3", "todo", "T-2", "T-1"),
	}
	got, err := renderDependencyGraph(tasks, "dot")
	if err != nil {
		t.Fatalf("dot: %v", err)
	}
	first := `  "T-3" -> "T-1";`
	second := `  "T-3" -> "T-2";`
	i := indexOf(got, first)
	j := indexOf(got, second)
	if i < 0 || j < 0 || i > j {
		t.Errorf("edges not sorted (T-1 before T-2):\n%s", got)
	}
}

func TestRenderDependencyGraphEscapesLabels(t *testing.T) {
	tasks := []*Task{
		{Frontmatter: TaskFrontmatter{ID: "T-1", Title: `a "quote" \ slash`, Status: "todo", Priority: "high"}},
	}
	dot, err := renderDependencyGraph(tasks, "dot")
	if err != nil {
		t.Fatalf("dot: %v", err)
	}
	if !contains(dot, `\"quote\"`) || !contains(dot, `\\ slash`) {
		t.Errorf("dot did not escape quotes/backslashes: %q", dot)
	}
	mermaid, err := renderDependencyGraph(tasks, "mermaid")
	if err != nil {
		t.Fatalf("mermaid: %v", err)
	}
	if contains(mermaid, `"quote"`) {
		t.Errorf("mermaid left a raw double-quote that breaks the label: %q", mermaid)
	}
}

// A title with a literal newline (valid YAML) must not split a node statement
// across physical lines: DOT collapses it to the two-char `\n` escape, Mermaid to
// `<br/>`. Otherwise the emitted graph is not parseable.
func TestRenderDependencyGraphEscapesNewlinesInLabels(t *testing.T) {
	tasks := []*Task{
		{Frontmatter: TaskFrontmatter{ID: "T-1", Title: "line one\nline two", Status: "todo", Priority: "high"}},
	}
	dot, err := renderDependencyGraph(tasks, "dot")
	if err != nil {
		t.Fatalf("dot: %v", err)
	}
	if contains(dot, "line one\nline two") {
		t.Errorf("dot left a raw newline splitting the label: %q", dot)
	}
	if !contains(dot, `line one\nline two`) {
		t.Errorf("dot did not collapse the newline to an escape: %q", dot)
	}
	mermaid, err := renderDependencyGraph(tasks, "mermaid")
	if err != nil {
		t.Fatalf("mermaid: %v", err)
	}
	if contains(mermaid, "line one\nline two") {
		t.Errorf("mermaid left a raw newline splitting the label: %q", mermaid)
	}
	if !contains(mermaid, "line one<br/>line two") {
		t.Errorf("mermaid did not collapse the newline to <br/>: %q", mermaid)
	}
}

func TestRenderDependencyGraphEmptyGraph(t *testing.T) {
	dot, err := renderDependencyGraph(nil, "dot")
	if err != nil {
		t.Fatalf("dot: %v", err)
	}
	if dot != "digraph taskrail {\n  rankdir=LR;\n}\n" {
		t.Errorf("empty dot = %q", dot)
	}
	mermaid, err := renderDependencyGraph(nil, "mermaid")
	if err != nil {
		t.Fatalf("mermaid: %v", err)
	}
	if mermaid != "graph LR\n" {
		t.Errorf("empty mermaid = %q", mermaid)
	}
}

func TestRenderDependencyGraphRejectsUnknownFormat(t *testing.T) {
	if _, err := renderDependencyGraph(nil, "svg"); err == nil {
		t.Errorf("expected error for unknown format, got nil")
	}
}

// small local string helpers to avoid pulling strings into every assertion.
func contains(haystack, needle string) bool { return indexOf(haystack, needle) >= 0 }

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
