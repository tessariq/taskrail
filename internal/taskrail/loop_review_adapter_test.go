package taskrail

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCheckedCombinedReviewProtocolOptIn(t *testing.T) {
	input, err := ParseLoopInvocation([]string{"--parallel", "2", "--max-iterations", "2", "--delivery", "review", "--review-adapter", "adapter", "--review-protocol", "checked-combined-v1", "--check", "check", "--timeout", "1m", "--result-file", "result", "--", "worker"})
	if err != nil {
		t.Fatalf("approved protocol opt-in: %v", err)
	}
	if input.ReviewProtocol != "checked-combined-v1" || !input.ReviewProtocolSet {
		t.Fatalf("protocol not captured: %+v", input)
	}
	for name, mutate := range map[string]func(*LoopInvocation){
		"unknown":   func(in *LoopInvocation) { in.ReviewProtocol = "other" },
		"empty":     func(in *LoopInvocation) { in.ReviewProtocol = "" },
		"local":     func(in *LoopInvocation) { in.Delivery = "local" },
		"width-one": func(in *LoopInvocation) { in.MaxIterations = 1 },
		"unbounded": func(in *LoopInvocation) { in.Timeout = nil },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := input
			mutate(&candidate)
			if err := ValidateLoopInvocation(candidate); MachineFailureFor(err).Code != MachineCodeInvalidArguments {
				t.Fatalf("unsafe protocol invocation accepted: %v", err)
			}
		})
	}
}

func TestValidateReviewAdapterResultRejectsUntrustedResponses(t *testing.T) {
	zero := 0
	request := ReviewAdapterRequest{
		SchemaVersion:      2,
		RequestID:          "request-1",
		Operation:          "inspect_change",
		Repository:         "/repo",
		TargetID:           "target-1",
		TargetRef:          "refs/heads/main",
		SourceRef:          "refs/heads/taskrail/T-001",
		ChangeID:           stringPtr("change-1"),
		ExpectedTargetHead: "base",
		CandidateHead:      "candidate",
		Check:              &ParallelCheck{Head: "candidate", Evidence: LoopCheckEvidence{ExitCode: &zero}},
	}
	valid := ReviewAdapterResult{
		SchemaVersion: 2,
		RequestID:     request.RequestID,
		Operation:     request.Operation,
		State:         "applied",
		TargetID:      request.TargetID,
		ChangeID:      request.ChangeID,
		SourceRef:     request.SourceRef,
		TargetRef:     request.TargetRef,
		SourceHead:    request.CandidateHead,
		TargetHead:    request.ExpectedTargetHead,
		Checks:        "pass",
		Approval:      "approved",
		ApprovalToken: stringPtr("approval-1"),
		Message:       "ready",
	}
	if err := validateReviewAdapterResult(request, valid); err != nil {
		t.Fatalf("validate valid result: %v", err)
	}

	for name, mutate := range map[string]func(*ReviewAdapterResult){
		"request id":  func(result *ReviewAdapterResult) { result.RequestID = "other" },
		"operation":   func(result *ReviewAdapterResult) { result.Operation = "merge_change" },
		"source ref":  func(result *ReviewAdapterResult) { result.SourceRef = "refs/heads/other" },
		"source head": func(result *ReviewAdapterResult) { result.SourceHead = "other" },
		"stale target": func(result *ReviewAdapterResult) {
			result.TargetHead = "stale"
		},
		"unknown checks": func(result *ReviewAdapterResult) { result.Checks = "approved" },
	} {
		t.Run(name, func(t *testing.T) {
			result := valid
			mutate(&result)
			if err := validateReviewAdapterResult(request, result); err == nil {
				t.Fatal("validateReviewAdapterResult succeeded")
			}
		})
	}
}

func TestLoopExecuteRefusesParallelReviewBatchBeforeAdapter(t *testing.T) {
	clearLoopChildEnvironment(t)
	repo, svc := loopFixture(t)
	writeTask(t, repo, "T-001-ready", "First", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002-ready", "Second", "todo", "medium", "specs/v0.1.0.md#summary", nil)
	setLoopPolicy(t, repo, "T-001-ready", "allow", "independent work")
	setLoopPolicy(t, repo, "T-002-ready", "allow", "independent work")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "allow parallel tasks")

	result, err := PrepareLoopResultFile(filepath.Join(t.TempDir(), "loop-result.json"))
	if err != nil {
		t.Fatalf("prepare result file: %v", err)
	}
	_, err = svc.LoopExecuteWithPreparedResultFile(context.Background(), LoopInvocation{MaxIterations: 2, Parallel: 2, Check: passingLoopCheck(t),
		Delivery: "review", DeliverySet: true, ReviewAdapter: "must-not-resolve", ReviewAdapterSet: true,
		ResultFile: filepath.Join(t.TempDir(), "loop-result.json"), Child: []string{"must-not-launch"}}, result)
	if MachineFailureFor(err).Code != MachineCodeUnsupported {
		t.Fatalf("review delivery refusal: %v", err)
	}
	for _, taskID := range []string{"T-001-ready", "T-002-ready"} {
		tasks, err := svc.loadTasks()
		task, found := taskByIDFromSlice(tasks, taskID)
		if err != nil || !found || task.Frontmatter.Status != "todo" {
			t.Fatalf("review delivery changed source task %s: %+v, %v", taskID, task, err)
		}
	}
}

func TestExecutableReviewAdapterUsesPlatformSemantics(t *testing.T) {
	tests := []struct {
		name string
		mode os.FileMode
		goos string
		want bool
	}{
		{name: "windows regular file without execute bits", mode: 0o600, goos: "windows", want: true},
		{name: "unix regular file with execute bit", mode: 0o700, goos: "linux", want: true},
		{name: "unix regular file without execute bits", mode: 0o600, goos: "linux", want: false},
		{name: "windows directory", mode: os.ModeDir | 0o700, goos: "windows", want: false},
		{name: "windows non-regular file", mode: os.ModeNamedPipe | 0o700, goos: "windows", want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := executableReviewAdapter(test.mode, test.goos); got != test.want {
				t.Fatalf("executableReviewAdapter(%v, %q) = %t, want %t", test.mode, test.goos, got, test.want)
			}
		})
	}
}

func TestResolveReviewAdapterRejectsUnresolvedPath(t *testing.T) {
	if _, err := resolveReviewAdapter(filepath.Join(t.TempDir(), "missing-adapter")); err == nil {
		t.Fatal("resolveReviewAdapter accepted an unresolved path")
	}
}

func TestRunReviewAdapterRejectsExtraStdout(t *testing.T) {
	zero := 0
	request := ReviewAdapterRequest{SchemaVersion: 2, RequestID: "request-1", Operation: "inspect_change",
		Repository: t.TempDir(), TargetID: "target", TargetRef: "refs/heads/main", SourceRef: "refs/heads/taskrail/T-001", ChangeID: stringPtr("change-1"),
		ExpectedTargetHead: "base", CandidateHead: "candidate", Check: &ParallelCheck{Head: "candidate", Evidence: LoopCheckEvidence{ExitCode: &zero}}}
	if _, err := runReviewAdapter(context.Background(), buildReviewAdapter(t), request, &bytes.Buffer{}); err != nil {
		t.Fatalf("valid adapter: %v", err)
	}
	adapter := buildReviewAdapter(t, "{}")
	if _, err := runReviewAdapter(context.Background(), adapter, request, &bytes.Buffer{}); err == nil {
		t.Fatal("runReviewAdapter accepted multiple stdout documents")
	}
}

func TestCancelledReviewDoesNotLaunchAdapter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := runReviewDocument(ctx, filepath.Join(t.TempDir(), "must-not-launch"), t.TempDir(), ReviewCapabilityRequest{}, &bytes.Buffer{})
	if !errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "launch adapter") {
		t.Fatalf("cancelled invocation reached spawn: %v", err)
	}
}

func TestDecodeReviewAdapterResultRejectsMalformedDocuments(t *testing.T) {
	valid := `{"schema_version":2,"request_id":"r","operation":"inspect_change","state":"applied","target_id":"target","change_id":"c","source_ref":"s","target_ref":"t","source_head":"h","target_head":"b","checks":"pass","approval":"approved","approval_token":"a","merge_head":null,"message":"ok"}`
	if _, err := decodeReviewAdapterResult([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"duplicate":    []byte(strings.Replace(valid, `"message":"ok"`, `"message":"ok","message":"again"`, 1)),
		"missing":      []byte(strings.Replace(valid, `,"message":"ok"`, "", 1)),
		"null":         []byte(strings.Replace(valid, `"state":"applied"`, `"state":null`, 1)),
		"invalid utf8": append([]byte(valid[:len(valid)-1]), 0xff, '}'),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeReviewAdapterResult(data); err == nil {
				t.Fatal("decodeReviewAdapterResult succeeded")
			}
		})
	}
}

func TestReviewCapabilityRejectsNullArrayElement(t *testing.T) {
	data := []byte(`{"schema_version":2,"request_id":"r","operation":"capabilities","target_id":"t","capabilities":[null,"checked_combined_ff_v1"],"message":"ok"}`)
	var result ReviewCapabilityResult
	if err := decodeReviewDocument(data, []string{"schema_version", "request_id", "operation", "target_id", "capabilities", "message"}, nil, &result); err == nil {
		t.Fatal("capability accepted a null string element")
	}
}

func buildReviewAdapter(t *testing.T, trailer ...string) string {
	t.Helper()
	source := `package main
import ("encoding/json"; "io"; "os")
func main() { data, _ := io.ReadAll(os.Stdin); var r map[string]any; if json.Unmarshal(data, &r) != nil { os.Exit(2) }
 result := map[string]any{"schema_version": 2, "request_id": r["request_id"], "operation": r["operation"], "state": "applied", "target_id":r["target_id"], "change_id": r["change_id"], "source_ref": r["source_ref"], "target_ref": r["target_ref"], "source_head": r["candidate_head"], "target_head": r["expected_target_head"], "checks": "pass", "approval":"approved", "approval_token":"approval", "merge_head": nil, "message": "accepted"}; _ = json.NewEncoder(os.Stdout).Encode(result)`
	if len(trailer) != 0 {
		source += `; _, _ = os.Stdout.WriteString("` + trailer[0] + `")`
	}
	source += ` }`
	dir := t.TempDir()
	program := filepath.Join(dir, "adapter.go")
	if err := os.WriteFile(program, []byte(source), 0o600); err != nil {
		t.Fatalf("write adapter source: %v", err)
	}
	adapter := filepath.Join(dir, "review-adapter")
	if runtime.GOOS == "windows" {
		adapter += ".exe"
	}
	command := exec.Command("go", "build", "-o", adapter, program)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build adapter: %v\n%s", err, output)
	}
	return adapter
}
