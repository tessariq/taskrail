package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

type report struct {
	SchemaVersion          int      `json:"schema_version"`
	TaskID                 string   `json:"task_id"`
	TaskTitle              string   `json:"task_title"`
	Result                 string   `json:"result"`
	VerificationID         *string  `json:"verification_id,omitempty"`
	PreviousVerificationID *string  `json:"previous_verification_id,omitempty"`
	ObservedCompletionID   *string  `json:"observed_completion_id,omitempty"`
	Summary                string   `json:"summary"`
	Details                string   `json:"details,omitempty"`
	GeneratedAt            string   `json:"generated_at"`
	SpecRef                string   `json:"spec_ref"`
	Artifacts              []string `json:"artifacts"`
	FollowupTaskID         string   `json:"followup_task_id,omitempty"`
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
	if got.VerificationID == nil && (got.PreviousVerificationID != nil || got.ObservedCompletionID != nil) {
		fail(fmt.Errorf("verification predecessor or completion identity requires verification_id"))
	}
	for name, value := range map[string]*string{
		"verification_id":          got.VerificationID,
		"previous_verification_id": got.PreviousVerificationID,
		"observed_completion_id":   got.ObservedCompletionID,
	} {
		if value != nil && !lowerHex32(*value) {
			fail(fmt.Errorf("%s must be lower-case 32-hex", name))
		}
	}
	recommendation := ""
	if got.FollowupTaskID != "" {
		var err error
		if recommendation, err = parseRecommendation(got.Details); err != nil {
			fail(err)
		}
	}
	verificationID := ""
	if got.VerificationID != nil {
		verificationID = *got.VerificationID
	}
	fmt.Printf("%s\n%s\n%s\n%s\n", got.GeneratedAt, got.FollowupTaskID, recommendation, verificationID)
}

func lowerHex32(value string) bool {
	if len(value) != 32 {
		return false
	}
	for _, r := range value {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}
	return true
}

const recommendationMarker = "follow-up recommendation: "

// parseRecommendation returns the single normalized advisory the loop prompt
// requires. `taskrail verify --details` emits one paragraph, so the marker is
// matched anywhere in the details rather than only at the start of a line, and
// the rationale ends at the first line break so the runner keeps one field.
func parseRecommendation(details string) (string, error) {
	start := strings.Index(details, recommendationMarker)
	if start < 0 {
		return "", fmt.Errorf("follow-up report lacks a run/hold recommendation and rationale")
	}
	rest := details[start+len(recommendationMarker):]
	if strings.Contains(rest, recommendationMarker) {
		return "", fmt.Errorf("follow-up report carries more than one recommendation")
	}
	if end := strings.IndexAny(rest, "\r\n"); end >= 0 {
		rest = rest[:end]
	}
	rest = strings.TrimSpace(rest)
	for _, mode := range []string{"run", "hold"} {
		rationale, ok := strings.CutPrefix(rest, mode+" - ")
		if !ok {
			continue
		}
		if rationale = strings.TrimSpace(rationale); rationale != "" {
			return recommendationMarker + mode + " - " + rationale, nil
		}
	}
	return "", fmt.Errorf("follow-up recommendation must be run or hold with a rationale")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
