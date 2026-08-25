package taskrail

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestMutateTaskLoopPolicyPreviewApplyAndClear(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		operation LoopPolicyOperation
		reason    string
		seed      string
		status    string
		want      EffectiveLoopPolicy
	}{
		{"allow", LoopPolicyAllow, "bounded unattended change", "", "todo", EffectiveLoopPolicy{Source: "explicit", Policy: "allow", Reason: "bounded unattended change"}},
		{"hold", LoopPolicyHold, "operator review required", "", "blocked", EffectiveLoopPolicy{Source: "explicit", Policy: "hold", Reason: "operator review required"}},
		{"clear", LoopPolicyClear, "", "loop_policy: allow\nloop_reason: bounded unattended change\n", "todo", EffectiveLoopPolicy{Source: "default", Policy: "hold", Reason: DefaultLoopReason}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := seedFixtureRepo(t)
			writeTask(t, repo, "T-001-target", "Target", tc.status, "high", "specs/v0.1.0.md#summary", nil)
			path := filepath.Join(repo, "planning", "tasks", "T-001-target.md")
			before := readFileBytes(t, path)
			before = strings.Replace(before, "updated_at:", "unmodeled_marker: must-survive\n"+tc.seed+"updated_at:", 1)
			writeFile(t, path, before)
			svc := newTestService(t, repo, time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC))

			preview, err := svc.MutateTaskLoopPolicy(LoopPolicyMutationInput{TaskID: "T-001-target", Operation: tc.operation, Reason: tc.reason, DryRun: true})
			if err != nil {
				t.Fatalf("preview: %v", err)
			}
			if preview.Applied || preview.Candidate.EffectivePolicy != tc.want.Policy || preview.Candidate.Reason != tc.want.Reason || preview.Candidate.Source != tc.want.Source {
				t.Fatalf("preview = %+v, want %+v", preview, tc.want)
			}
			if got := readFileBytes(t, path); got != before {
				t.Fatal("dry run changed task bytes")
			}

			applied, err := svc.MutateTaskLoopPolicy(LoopPolicyMutationInput{TaskID: "T-001-target", Operation: tc.operation, Reason: tc.reason})
			if err != nil {
				t.Fatalf("apply: %v", err)
			}
			if !applied.Applied || !reflect.DeepEqual(applied.Prior, preview.Prior) || !reflect.DeepEqual(applied.Candidate, preview.Candidate) || !applied.Validation.Valid {
				t.Fatalf("apply does not match preview: preview=%+v apply=%+v", preview, applied)
			}
			after := readFileBytes(t, path)
			if !strings.Contains(after, "unmodeled_marker: must-survive") || !strings.Contains(after, "updated_at: \"2026-08-25T12:00:00Z\"") {
				t.Fatalf("unrelated fields or timestamp changed unexpectedly:\n%s", after)
			}
			for _, field := range []string{"loop_policy:", "loop_reason:"} {
				if tc.operation == LoopPolicyClear && strings.Contains(after, field) {
					t.Fatalf("clear retained %q:\n%s", field, after)
				}
				if tc.operation != LoopPolicyClear && !strings.Contains(after, field) {
					t.Fatalf("set omitted %q:\n%s", field, after)
				}
			}
			if tc.operation == LoopPolicyClear {
				want := strings.Replace(before, tc.seed, "", 1)
				want = strings.Replace(want, "updated_at: \"2026-03-31T00:00:00Z\"", "updated_at: \"2026-08-25T12:00:00Z\"", 1)
				if after != want {
					t.Fatalf("clear changed bytes outside its paired fields and timestamp:\nwant:\n%s\ngot:\n%s", want, after)
				}
			}
		})
	}
}

func TestMutateTaskLoopPolicyRefusesInvalidInputsWithoutWrites(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		operation LoopPolicyOperation
		reason    string
		status    string
		taskID    string
		code      string
	}{
		{"missing exact id", LoopPolicyAllow, "bounded", "todo", "T-001", MachineCodeTaskNotFound},
		{"in progress", LoopPolicyAllow, "bounded", "in_progress", "T-001-target", MachineCodeInvalidStatus},
		{"completed", LoopPolicyAllow, "bounded", "completed", "T-001-target", MachineCodeInvalidStatus},
		{"cancelled", LoopPolicyAllow, "bounded", "cancelled", "T-001-target", MachineCodeInvalidStatus},
		{"empty reason", LoopPolicyAllow, "", "todo", "T-001-target", MachineCodeInvalidReason},
		{"untrimmed reason", LoopPolicyHold, " bounded", "todo", "T-001-target", MachineCodeInvalidReason},
		{"control reason", LoopPolicyHold, "bounded\nchange", "todo", "T-001-target", MachineCodeInvalidReason},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := seedFixtureRepo(t)
			writeTask(t, repo, "T-001-target", "Target", tc.status, "high", "specs/v0.1.0.md#summary", nil)
			path := filepath.Join(repo, "planning", "tasks", "T-001-target.md")
			statePath := filepath.Join(repo, "planning", "STATE.md")
			before, stateBefore := readFileBytes(t, path), readFileBytes(t, statePath)
			svc := newTestService(t, repo, time.Now())

			_, err := svc.MutateTaskLoopPolicy(LoopPolicyMutationInput{TaskID: tc.taskID, Operation: tc.operation, Reason: tc.reason})
			if err == nil || MachineFailureFor(err).Code != tc.code {
				t.Fatalf("mutation error = %v (%s), want %s", err, MachineFailureFor(err).Code, tc.code)
			}
			if readFileBytes(t, path) != before || readFileBytes(t, statePath) != stateBefore {
				t.Fatal("refused mutation changed repository bytes")
			}
		})
	}
}

func TestMutateTaskLoopPolicyRefusesDelegatedInvocationWithoutWrites(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001-target", "Target", "todo", "high", "specs/v0.1.0.md#summary", nil)
	path := filepath.Join(repo, "planning", "tasks", "T-001-target.md")
	statePath := filepath.Join(repo, "planning", "STATE.md")
	before, stateBefore := readFileBytes(t, path), readFileBytes(t, statePath)
	svc := newTestService(t, repo, time.Now())
	t.Setenv("TASKRAIL_DELEGATION_ID", "0123456789abcdef0123456789abcdef")

	_, err := svc.MutateTaskLoopPolicy(LoopPolicyMutationInput{TaskID: "T-001-target", Operation: LoopPolicyAllow, Reason: "bounded"})
	if err == nil || MachineFailureFor(err).Code != MachineCodeDelegatedRefused {
		t.Fatalf("delegated mutation = %v (%s), want delegated_write_refused", err, MachineFailureFor(err).Code)
	}
	if readFileBytes(t, path) != before || readFileBytes(t, statePath) != stateBefore {
		t.Fatal("delegated mutation changed repository bytes")
	}
}

func TestMutateTaskLoopPolicyRefusesChangedPreimageWithoutOverwriting(t *testing.T) {
	repo := seedFixtureRepo(t)
	writeTask(t, repo, "T-001-target", "Target", "todo", "high", "specs/v0.1.0.md#summary", nil)
	path := filepath.Join(repo, "planning", "tasks", "T-001-target.md")
	statePath := filepath.Join(repo, "planning", "STATE.md")
	stateBefore := readFileBytes(t, statePath)
	svc := newTestService(t, repo, time.Now())
	testHookLoopPolicyCandidatePrepared = func() {
		writeFile(t, path, strings.Replace(readFileBytes(t, path), "updated_at:", "external_marker: must-survive\nupdated_at:", 1))
	}
	t.Cleanup(func() { testHookLoopPolicyCandidatePrepared = nil })

	_, err := svc.MutateTaskLoopPolicy(LoopPolicyMutationInput{TaskID: "T-001-target", Operation: LoopPolicyAllow, Reason: "bounded"})
	if err == nil || MachineFailureFor(err).Code != MachineCodeWriteConflict {
		t.Fatalf("changed preimage mutation = %v (%s), want write_conflict", err, MachineFailureFor(err).Code)
	}
	if got := readFileBytes(t, statePath); got != stateBefore {
		t.Fatal("preimage conflict changed STATE.md")
	}
	after := readFileBytes(t, path)
	if !strings.Contains(after, "external_marker: must-survive") || strings.Contains(after, "loop_policy:") {
		t.Fatalf("preimage conflict overwrote external task bytes:\n%s", after)
	}
}
