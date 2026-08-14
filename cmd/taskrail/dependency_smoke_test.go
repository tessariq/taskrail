package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func seedDependencyTasks(t *testing.T) string {
	t.Helper()
	root := setupRepo(t)
	writeTask(t, root, "T-100-target", "todo", "")
	writeTask(t, root, "T-101-dependency", "todo", "")
	return root
}

func TestTaskDependencyJSONUsesExactCommonResult(t *testing.T) {
	root := seedDependencyTasks(t)
	before, err := os.ReadFile(filepath.Join(root, "planning", "tasks", "T-100-target.md"))
	if err != nil {
		t.Fatal(err)
	}
	out, _, err := runRootSplit(t, "task", "dependency", "add", "T-100-target", "T-101-dependency", "--dry-run", "--json")
	if err != nil {
		t.Fatalf("dependency preview: %v (%s)", err, out)
	}
	envelope := decodeEnvelope(t, out)
	if envelope.Command != "task dependency add" || envelope.Error != nil {
		t.Fatalf("unexpected envelope: %+v", envelope)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(envelope.Result, &fields); err != nil {
		t.Fatal(err)
	}
	want := []string{"applied", "dependencies_after", "dependencies_before", "dependency_id", "operation", "task_id", "validation"}
	got := make([]string, 0, len(fields))
	for key := range fields {
		got = append(got, key)
	}
	slicesSort(got)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result fields = %v, want %v", got, want)
	}
	var dependenciesBefore, dependenciesAfter []string
	if err := json.Unmarshal(fields["dependencies_before"], &dependenciesBefore); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(fields["dependencies_after"], &dependenciesAfter); err != nil {
		t.Fatal(err)
	}
	if dependenciesBefore == nil || !reflect.DeepEqual(dependenciesAfter, []string{"T-101-dependency"}) {
		t.Fatalf("dependency arrays are not exact: %s", envelope.Result)
	}
	after, _ := os.ReadFile(filepath.Join(root, "planning", "tasks", "T-100-target.md"))
	if !reflect.DeepEqual(after, before) {
		t.Fatal("dry run wrote the task")
	}
}

func TestTaskDependencyHelpAndFuzzyRefusal(t *testing.T) {
	seedDependencyTasks(t)
	out, err := runRoot(t, "task", "dependency", "--help")
	if err != nil || !strings.Contains(out, "add") || !strings.Contains(out, "remove") {
		t.Fatalf("dependency help = %q err=%v", out, err)
	}
	out, _, err = runRootSplit(t, "task", "dependency", "add", "T-100", "T-101-dependency", "--json")
	if err == nil {
		t.Fatal("fuzzy target unexpectedly succeeded")
	}
	if envelope := decodeEnvelope(t, out); envelope.Error == nil || envelope.Error.Code != "task_not_found" {
		t.Fatalf("unexpected refusal: %+v", envelope)
	}
}

func slicesSort(values []string) {
	for i := range values {
		for j := i + 1; j < len(values); j++ {
			if values[j] < values[i] {
				values[i], values[j] = values[j], values[i]
			}
		}
	}
}
