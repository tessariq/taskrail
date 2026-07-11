package main

import (
	"strings"
	"testing"
)

func TestStatsFormatDotExportsDAGReadOnly(t *testing.T) {
	root := setupRepo(t)
	seedStatsDAG(t, root)

	before := readAllFiles(t, root)

	out, err := runRoot(t, "stats", "--format", "dot")
	if err != nil {
		t.Fatalf("stats --format dot: %v (output %q)", err, out)
	}
	for _, want := range []string{
		"digraph taskrail {",
		`"T-4" [label="T-4`,
		`"T-2" -> "T-1";`,
		`"T-4" -> "T-3";`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("dot export missing %q:\n%s", want, out)
		}
	}
	// The graph export is not the default stats table.
	if strings.Contains(out, "blocked ratio") {
		t.Errorf("dot export leaked the stats table:\n%s", out)
	}

	after := readAllFiles(t, root)
	if len(before) != len(after) {
		t.Fatalf("stats --format changed the file set")
	}
	for path, content := range before {
		if after[path] != content {
			t.Errorf("stats --format mutated %s", path)
		}
	}
}

func TestStatsFormatMermaidExportsDAG(t *testing.T) {
	root := setupRepo(t)
	seedStatsDAG(t, root)

	out, err := runRoot(t, "stats", "--format", "mermaid")
	if err != nil {
		t.Fatalf("stats --format mermaid: %v (output %q)", err, out)
	}
	for _, want := range []string{"graph LR", "T_2 --> T_1", "T_4 --> T_3"} {
		if !strings.Contains(out, want) {
			t.Errorf("mermaid export missing %q:\n%s", want, out)
		}
	}
}

func TestStatsDefaultUnchangedWhenFormatOmitted(t *testing.T) {
	root := setupRepo(t)
	seedStatsDAG(t, root)

	out, err := runRoot(t, "stats")
	if err != nil {
		t.Fatalf("stats: %v (output %q)", err, out)
	}
	if !strings.Contains(out, "tasks: 4 total") || strings.Contains(out, "digraph") {
		t.Errorf("default stats output changed by the --format addition:\n%s", out)
	}
}

func TestStatsRejectsUnknownFormat(t *testing.T) {
	setupRepo(t)
	if _, err := runRoot(t, "stats", "--format", "svg"); err == nil {
		t.Errorf("expected error for unknown --format value")
	}
}

func TestStatsRejectsFormatWithJSON(t *testing.T) {
	setupRepo(t)
	if _, err := runRoot(t, "stats", "--format", "dot", "--json"); err == nil {
		t.Errorf("expected error combining --format with --json")
	}
}
