package taskrail

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestEncodeLoopResultFileDocumentUsesDiagnosticForPostflightFailure(t *testing.T) {
	diagnostic := LoopDiagnostic{
		Outcome:             "child_failed",
		IterationsCompleted: 1,
		Validation:          LoopValidation{Valid: true, Violations: []MachineViolation{}},
		Git:                 LoopGitDiagnostic{Ref: "refs/heads/main", HeadBefore: "before", HeadAfter: "after", Clean: false, Descendant: true, Commits: []string{}},
		Storage:             LoopStorageSnapshot{Mode: "committed", Root: "."},
		Review:              LoopReviewPolicy{ConfiguredMaxRounds: 1, EffectiveMaxRounds: 1, MaxReviewersPerRound: 3, FinalDiffReviewRequiredOnChange: true, Source: "config"},
		Execution:           LoopExecutionBudget{TimeoutSource: "none"},
		Executable:          LoopExecutable{InvocationID: "invocation", Path: "/staged", SHA256: "digest"},
		MutationViolations:  []MachineViolation{},
		ProcessViolations:   []MachineViolation{},
		Remote:              "not_checked",
		NextAction:          "Inspect the child failure before another loop invocation.",
	}
	document, err := EncodeLoopResultFileDocument(&diagnostic, WithMachineErrorCode("child_failed", errors.New("child failed")), nil)
	if err != nil {
		t.Fatalf("encode result file: %v", err)
	}
	var envelope struct {
		SchemaVersion int    `json:"schema_version"`
		Command       string `json:"command"`
		Warnings      []any  `json:"warnings"`
		Error         struct {
			Code    string         `json:"code"`
			Details LoopDiagnostic `json:"details"`
		} `json:"error"`
	}
	if err := json.Unmarshal(document, &envelope); err != nil {
		t.Fatalf("decode result file: %v", err)
	}
	if envelope.SchemaVersion != 1 || envelope.Command != "loop" || envelope.Error.Code != "child_failed" || envelope.Error.Details.Outcome != diagnostic.Outcome {
		t.Fatalf("postflight result-file envelope = %+v", envelope)
	}
}
