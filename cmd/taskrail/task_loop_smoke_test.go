package main

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/tessariq/taskrail/internal/taskrail"
)

func TestTaskLoopListPublishesRowsAndStaysReadOnly(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001-held", "todo", "")
	writeTask(t, root, "T-002-ready", "todo", "")
	before := readAllFiles(t, root)

	stdout, err := runRoot(t, "task", "loop", "list", "--json")
	if err != nil {
		t.Fatalf("task loop list: %v (stdout %q)", err, stdout)
	}
	envelope, err := taskrail.DecodeMachineEnvelope([]byte(stdout))
	if err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if envelope.Command != "task loop list" {
		t.Fatalf("command = %q", envelope.Command)
	}
	var report taskrail.TaskLoopListResult
	if err := json.Unmarshal(envelope.Result, &report); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if len(report.Tasks) != 2 || report.Tasks[0].Source != "default" || report.Tasks[0].Disposition != "held" {
		t.Fatalf("report rows = %+v", report.Tasks)
	}
	if after := readAllFiles(t, root); !reflect.DeepEqual(after, before) {
		t.Fatal("task loop list changed repository files")
	}
}

func TestTaskLoopListReturnsGatedReportForInvalidTask(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001-invalid", "wat", "")

	stdout, err := runRoot(t, "task", "loop", "list", "--json")
	if err == nil {
		t.Fatal("invalid list must exit non-zero")
	}
	envelope, decodeErr := taskrail.DecodeMachineEnvelope([]byte(stdout))
	if decodeErr != nil {
		t.Fatalf("decode gated envelope: %v (stdout %q)", decodeErr, stdout)
	}
	var report taskrail.TaskLoopListResult
	if decodeErr := json.Unmarshal(envelope.Result, &report); decodeErr != nil {
		t.Fatalf("decode gated result: %v", decodeErr)
	}
	if len(report.Violations) == 0 || len(report.Tasks) != 1 || report.Tasks[0].Disposition != "invalid" {
		t.Fatalf("gated report = %+v", report)
	}
}
