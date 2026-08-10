package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type report struct {
	SchemaVersion  int      `json:"schema_version"`
	TaskID         string   `json:"task_id"`
	TaskTitle      string   `json:"task_title"`
	Result         string   `json:"result"`
	Summary        string   `json:"summary"`
	Details        string   `json:"details,omitempty"`
	GeneratedAt    string   `json:"generated_at"`
	SpecRef        string   `json:"spec_ref"`
	Artifacts      []string `json:"artifacts"`
	FollowupTaskID string   `json:"followup_task_id,omitempty"`
}

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: check-report <report.json> <task-id> <pass|fail>")
		os.Exit(2)
	}
	f, err := os.Open(os.Args[1])
	if err != nil {
		fail(err)
	}
	defer f.Close()

	var got report
	dec := json.NewDecoder(f)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&got); err != nil {
		fail(err)
	}
	var trailing any
	if err := dec.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("trailing JSON value")
		}
		fail(err)
	}
	if got.SchemaVersion != 1 || got.TaskID != os.Args[2] || got.Result != os.Args[3] ||
		got.TaskTitle == "" || got.Summary == "" || got.GeneratedAt == "" || got.SpecRef == "" || got.Artifacts == nil {
		fail(fmt.Errorf("report fields do not match the expected verification"))
	}
	recommendation := ""
	if got.FollowupTaskID != "" {
		for line := range strings.Lines(got.Details) {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "follow-up recommendation: ") {
				recommendation = line
				break
			}
		}
		if !strings.HasPrefix(recommendation, "follow-up recommendation: run - ") &&
			!strings.HasPrefix(recommendation, "follow-up recommendation: hold - ") {
			fail(fmt.Errorf("follow-up report lacks a run/hold recommendation and rationale"))
		}
	}
	fmt.Printf("%s\n%s\n%s\n", got.GeneratedAt, got.FollowupTaskID, recommendation)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
