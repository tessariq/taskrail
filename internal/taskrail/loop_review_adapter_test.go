package taskrail

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestValidateReviewAdapterResultRejectsUntrustedResponses(t *testing.T) {
	request := ReviewAdapterRequest{
		SchemaVersion:      1,
		RequestID:          "request-1",
		Operation:          "inspect_change",
		Repository:         "/repo",
		TargetRef:          "refs/heads/main",
		SourceRef:          "refs/heads/taskrail/T-001",
		ChangeID:           stringPtr("change-1"),
		ExpectedTargetHead: "base",
		CandidateHead:      "candidate",
	}
	valid := ReviewAdapterResult{
		SchemaVersion: 1,
		RequestID:     request.RequestID,
		Operation:     request.Operation,
		Applied:       true,
		ChangeID:      request.ChangeID,
		SourceRef:     request.SourceRef,
		TargetRef:     request.TargetRef,
		SourceHead:    request.CandidateHead,
		TargetHead:    request.ExpectedTargetHead,
		Checks:        "pass",
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

func TestLoopExecuteDeliversParallelReviewBatchThroughAdapter(t *testing.T) {
	clearLoopChildEnvironment(t)
	repo, svc := loopFixture(t)
	writeTask(t, repo, "T-001-ready", "First", "todo", "high", "specs/v0.1.0.md#summary", nil)
	writeTask(t, repo, "T-002-ready", "Second", "todo", "medium", "specs/v0.1.0.md#summary", nil)
	setLoopPolicy(t, repo, "T-001-ready", "allow", "independent work")
	setLoopPolicy(t, repo, "T-002-ready", "allow", "independent work")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "allow parallel tasks")

	binary := buildTaskrailExecutable(t)
	previous := loopExecutablePath
	loopExecutablePath = func() (string, error) { return binary, nil }
	t.Cleanup(func() { loopExecutablePath = previous })
	t.Setenv("GO_WANT_LOOP_CHILD", "1")
	helper, err := os.Executable()
	if err != nil {
		t.Fatalf("resolve test helper: %v", err)
	}
	child := []string{helper, "-test.run=^TestLoopLaunchChildHelper$", "--", "parallel-worker", filepath.Join(t.TempDir(), "unused")}

	report, err := svc.LoopExecute(context.Background(), LoopInvocation{MaxIterations: 2, Parallel: 2,
		Delivery: "review", DeliverySet: true, ReviewAdapter: buildReviewAdapter(t), ReviewAdapterSet: true, Child: child})
	if err != nil {
		t.Fatalf("LoopExecute: %v", err)
	}
	if report.Outcome != "batch_pass" || report.Remote != "not_checked" || report.Parallel == nil || report.Parallel.Delivery.Remote != "adapter_reported" || len(report.Parallel.Delivery.PublishedTasks) != 2 {
		encoded, _ := json.MarshalIndent(report, "", "  ")
		t.Fatalf("review delivery report = %s", encoded)
	}
	children := report.Parallel.Integration.Children
	if len(children) != 2 || children[0].Role != "conflict_resolution" || children[0].Outcome != "pass" ||
		children[0].TaskID == nil || children[0].CandidateHead == nil || children[0].WorkerEvidenceSHA256 == nil ||
		children[0].Prompt.ID != "loop-integration" || children[1].Role != "aggregate_gate" || children[1].Outcome != "pass" ||
		children[1].BoundHead != *report.Parallel.Integration.Head || children[1].Prompt.ID != "loop-integration" {
		encoded, _ := json.MarshalIndent(report.Parallel.Integration, "", "  ")
		t.Fatalf("review integration evidence = %s", encoded)
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
	adapter := buildReviewAdapter(t, "{}")
	request := ReviewAdapterRequest{SchemaVersion: 1, RequestID: "request-1", Operation: "inspect_change",
		Repository: t.TempDir(), TargetRef: "refs/heads/main", SourceRef: "refs/heads/taskrail/T-001", ChangeID: stringPtr("change-1"),
		ExpectedTargetHead: "base", CandidateHead: "candidate"}
	if _, err := runReviewAdapter(context.Background(), adapter, request, &bytes.Buffer{}); err == nil {
		t.Fatal("runReviewAdapter accepted multiple stdout documents")
	}
}

func TestDecodeReviewAdapterResultRejectsMalformedDocuments(t *testing.T) {
	valid := `{"schema_version":1,"request_id":"r","operation":"inspect_change","applied":true,"change_id":"c","source_ref":"s","target_ref":"t","source_head":"h","target_head":"b","checks":"pass","merge_head":null,"message":"ok"}`
	for name, data := range map[string][]byte{
		"duplicate":    []byte(strings.Replace(valid, `"message":"ok"`, `"message":"ok","message":"again"`, 1)),
		"missing":      []byte(strings.Replace(valid, `,"message":"ok"`, "", 1)),
		"null":         []byte(strings.Replace(valid, `"applied":true`, `"applied":null`, 1)),
		"invalid utf8": append([]byte(valid[:len(valid)-1]), 0xff, '}'),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := decodeReviewAdapterResult(data); err == nil {
				t.Fatal("decodeReviewAdapterResult succeeded")
			}
		})
	}
}

func buildReviewAdapter(t *testing.T, trailer ...string) string {
	t.Helper()
	source := `package main
import ("encoding/json"; "io"; "os")
type request struct { SchemaVersion int ` + "`json:\"schema_version\"`" + `; RequestID string ` + "`json:\"request_id\"`" + `; Operation string ` + "`json:\"operation\"`" + `; TargetRef string ` + "`json:\"target_ref\"`" + `; SourceRef string ` + "`json:\"source_ref\"`" + `; ChangeID *string ` + "`json:\"change_id\"`" + `; ExpectedTargetHead string ` + "`json:\"expected_target_head\"`" + `; CandidateHead string ` + "`json:\"candidate_head\"`" + ` }
func main() { data, _ := io.ReadAll(os.Stdin); var r request; if json.Unmarshal(data, &r) != nil { os.Exit(2) }; result := map[string]any{"schema_version": 1, "request_id": r.RequestID, "operation": r.Operation, "applied": true, "change_id": r.ChangeID, "source_ref": r.SourceRef, "target_ref": r.TargetRef, "source_head": r.CandidateHead, "target_head": r.ExpectedTargetHead, "checks": "pass", "merge_head": nil, "message": "accepted"}; if r.Operation == "open_change" { result["change_id"] = "change-1" }; if r.Operation == "merge_change" { result["merge_head"] = r.CandidateHead }; _ = json.NewEncoder(os.Stdout).Encode(result)`
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
