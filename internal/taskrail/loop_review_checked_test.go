package taskrail

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCheckedCombinedReviewPublication(t *testing.T) {
	clearLoopChildEnvironment(t)
	binary := buildTaskrailExecutable(t)
	previous := loopExecutablePath
	loopExecutablePath = func() (string, error) { return binary, nil }
	t.Cleanup(func() { loopExecutablePath = previous })
	t.Setenv("GO_WANT_LOOP_CHILD", "1")
	check := buildLoopCheck(t)
	adapter := buildCheckedReviewAdapter(t)
	for _, mode := range []string{"pass", "final-fail", "candidate-fail", "unsupported", "legacy", "pending", "target-drift", "candidate-drift", "identity-drift",
		"approval-drift", "checks-drift", "local-worktree", "local-config", "local-ref", "source-drift", "check-drift",
		"merge-refused", "merge-approval-drift", "merge-candidate-drift", "merge-target-drift", "unknown", "lost", "malformed", "wrong-merge-head", "publish-lost", "timeout", "after-merge-drift",
		"preflight-drift", "schema-zero", "oversize", "cli-pass", "cli-unknown", "cli-applied-output", "cli-unknown-output", "cli-recovery"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("REVIEW_MODE", mode)
			t.Setenv("LOOP_CHECK_MODE", "pass")
			t.Setenv("LOOP_CHECK_RECORD", filepath.Join(t.TempDir(), "checks"))
			t.Setenv("REVIEW_EVENT_RECORD", filepath.Join(t.TempDir(), "events"))
			repo, svc := loopFixture(t)
			writeFile(t, filepath.Join(repo, "go.mod"), "module example.com/reviewfixture\n\ngo 1.26\n")
			writeFile(t, filepath.Join(repo, "api.go"), "package fixture\nfunc Old() {}\n")
			writeFile(t, filepath.Join(repo, ".gitattributes"), "*.go text eol=lf\nindependent.txt text eol=lf\n")
			for _, id := range []string{"T-001-ready", "T-002-ready", "T-003-ready"} {
				writeTask(t, repo, id, id, "todo", "high", "specs/v0.1.0.md#summary", nil)
				setLoopPolicy(t, repo, id, "allow", "independent work")
			}
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-m", "allow checked review batch")
			remote := filepath.Join(t.TempDir(), "target.git")
			if output, err := exec.Command("git", "clone", "--bare", repo, remote).CombinedOutput(); err != nil {
				t.Fatalf("disposable target: %v %s", err, output)
			}
			t.Setenv("REVIEW_TARGET", remote)
			t.Setenv("REVIEW_SOURCE", repo)
			t.Setenv("REVIEW_LOG", filepath.Join(t.TempDir(), "requests.jsonl"))
			if mode == "preflight-drift" {
				loopExecutablePath = func() (string, error) {
					writeFile(t, filepath.Join(repo, "api.go"), "package fixture\nfunc Changed() {}\n")
					return binary, nil
				}
				defer func() { loopExecutablePath = func() (string, error) { return binary, nil } }()
			}
			before := snapshotTreeWithoutGitHousekeeping(t, repo)
			workspace := t.TempDir()
			for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(key, workspace)
			}
			args := []string{"--parallel", "3", "--max-iterations", "3", "--check", check, "--delivery", "review", "--review-adapter", adapter,
				"--review-protocol", "checked-combined-v1", "--timeout", "1m",
				"--keep-workspaces", "always", "--result-file", filepath.Join(t.TempDir(), "result.json"), "--", "worker"}
			input, err := ParseLoopInvocation(args)
			if err != nil {
				t.Fatal(err)
			}
			if mode != "pass" && mode != "candidate-fail" {
				input.KeepWorkspaces = "failure"
			}
			if mode == "timeout" {
				duration := 30 * time.Second
				input.Timeout = &duration
			}
			if mode == "oversize" {
				duration := 10 * time.Second
				input.Timeout = &duration
			}
			if mode == "check-drift" {
				input.Check = filepath.Join(t.TempDir(), "check.exe")
				if err := os.WriteFile(input.Check, []byte(readBytes(t, check)), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "final-fail" {
				t.Setenv("LOOP_CHECK_MODE", "final-fail")
			}
			if mode == "pass" {
				preview := input
				preview.DryRun, preview.Child, preview.ResultFile = true, nil, ""
				previewReport, err := svc.LoopDryRun(preview)
				if err != nil || previewReport.Action != "run" || previewReport.Parallel.ReviewProtocol == nil || fileExists(os.Getenv("REVIEW_LOG")) {
					t.Fatalf("read-only preview: %+v %v", previewReport, err)
				}
			}
			input.Child = []string{os.Args[0], "-test.run=^TestLoopLaunchChildHelper$", "--", "parallel-worker", "unused"}
			if mode == "candidate-fail" {
				t.Setenv("LOOP_CHECK_MODE", "build")
				input.Child = []string{os.Args[0], "-test.run=^TestLoopLaunchChildHelper$", "--", "parallel-conflict-worker", "semantic"}
			}
			result, err := PrepareLoopResultFile(input.ResultFile)
			if err != nil {
				t.Fatal(err)
			}
			var report LoopDiagnostic
			if strings.HasPrefix(mode, "cli-") {
				t.Setenv("REVIEW_RESULT", input.ResultFile)
				command := exec.Command(binary, append([]string{"loop"}, append(args[:len(args)-1], input.Child...)...)...)
				command.Dir = repo
				output, commandErr := command.CombinedOutput()
				document, readErr := os.ReadFile(input.ResultFile)
				if strings.HasSuffix(mode, "-output") {
					if commandErr == nil || string(document) != "collision" || !strings.Contains(string(output), "publish result file") {
						t.Fatalf("late output failure: %v %s", commandErr, output)
					}
					parts := strings.SplitN(string(output), "TASKRAIL_LOOP_RESULT_RECOVERY_BEGIN\n", 2)
					if len(parts) != 2 {
						t.Fatalf("remote receipt lost after output failure: %s", output)
					}
					document = []byte(strings.SplitN(parts[1], "\nTASKRAIL_LOOP_RESULT_RECOVERY_END", 2)[0])
				} else if readErr != nil {
					t.Fatal(readErr)
				}
				var envelope struct {
					Result LoopDiagnostic
					Error  *struct {
						Code    string
						Details LoopDiagnostic
					}
				}
				if err := json.Unmarshal(document, &envelope); err != nil {
					t.Fatalf("CLI document: %v %s", err, document)
				}
				report = envelope.Result
				if envelope.Error != nil {
					report = envelope.Error.Details
				}
				wantCode := ""
				if mode == "cli-unknown" || mode == "cli-unknown-output" {
					wantCode = "batch_failed"
				}
				if mode == "cli-recovery" {
					wantCode = "invalid_postflight"
				}
				if wantCode != "" && (commandErr == nil || envelope.Error == nil || envelope.Error.Code != wantCode) {
					t.Fatalf("CLI failure classification: want %s, exit=%v document=%s", wantCode, commandErr, document)
				}
				if mode == "cli-pass" && commandErr != nil {
					t.Fatalf("CLI success: %v %s", commandErr, output)
				}
			} else {
				report, err = svc.LoopExecuteWithPreparedResultFile(context.Background(), input, result)
			}
			data, _ := json.MarshalIndent(report, "", "  ")
			if err != nil || report.Parallel == nil {
				t.Fatalf("checked review: %v\n%s", err, data)
			}
			batch := report.Parallel
			if mode == "preflight-drift" {
				entries, _ := os.ReadDir(workspace)
				if fileExists(os.Getenv("REVIEW_LOG")) || len(entries) != 0 || len(batch.Workers) != 0 || batch.Delivery.Publication != "refused" {
					t.Fatalf("preflight drift reached adapter/workers: %s", data)
				}
				return
			}
			requests := readReviewRequests(t, os.Getenv("REVIEW_LOG"))
			if mode == "unsupported" || mode == "legacy" {
				entries, _ := os.ReadDir(workspace)
				if len(requests) != 1 || len(entries) != 0 || len(batch.Workers) != 0 || report.Outcome != "batch_failed" || batch.Delivery.Capability.Error == nil {
					t.Fatalf("pre-worker refusal: %s", data)
				}
				return
			}
			if mode == "source-drift" || mode == "after-merge-drift" {
				if readBytes(t, filepath.Join(repo, "adapter-drift.txt")) != "retain me" {
					t.Fatal("source drift was lost")
				}
				os.Remove(filepath.Join(repo, "adapter-drift.txt"))
			}
			if mode == "cli-recovery" {
				journal := filepath.Join(repo, ".git", "taskrail", "transactions", strings.Repeat("a", 32), "journal.json")
				if readBytes(t, journal) != "{}" {
					t.Fatal("recovery state was lost")
				}
				os.RemoveAll(filepath.Dir(journal))
			}
			if after := snapshotTreeWithoutGitHousekeeping(t, repo); !reflect.DeepEqual(before, after) {
				t.Fatal("review publication mutated source")
			}
			publication, outcome, operations := "refused", "batch_failed", 4
			switch mode {
			case "pass", "cli-pass", "cli-applied-output":
				publication, outcome, operations = "applied", "batch_pass", 5
			case "cli-recovery":
				publication, outcome, operations = "applied", "invalid_postflight", 5
			case "candidate-fail", "after-merge-drift":
				publication, outcome, operations = "applied", "batch_partial", 5
			case "final-fail":
				operations = 0
			case "pending":
				operations = 3
			case "merge-refused", "merge-approval-drift", "merge-candidate-drift", "merge-target-drift":
				operations = 5
			case "unknown", "lost", "malformed", "wrong-merge-head", "timeout", "cli-unknown", "cli-unknown-output":
				publication, operations = "unknown", 5
			case "publish-lost":
				publication, operations = "unknown", 1
			case "schema-zero", "oversize":
				publication, operations = "unknown", 1
			}
			if batch.Delivery.Publication != publication || report.Outcome != outcome || len(batch.Delivery.Operations) != operations || len(requests) != operations+1 {
				t.Fatalf("publication/order: %s", data)
			}
			if mode == "schema-zero" && (batch.Delivery.Operations[0].Result == nil || batch.Delivery.Operations[0].Result.SchemaVersion != 0) {
				t.Fatalf("structurally decoded unsafe result was lost: %s", data)
			}
			if mode == "oversize" && (batch.Delivery.Operations[0].Error == nil || !strings.Contains(*batch.Delivery.Operations[0].Error, "1 MiB")) {
				t.Fatalf("stdout limit was not enforced: %s", data)
			}
			if mode == "final-fail" {
				if batch.Integration.Check == nil || batch.Integration.Check.Evidence.ExitCode == nil || *batch.Integration.Check.Evidence.ExitCode != 7 {
					t.Fatalf("final-check receipt: %s", data)
				}
			}
			for i, receipt := range batch.Delivery.Operations {
				want := []string{"publish_branch", "open_change", "inspect_change", "inspect_change", "merge_change"}[i]
				if receipt.Request.Operation != want || receipt.Request.Check == nil || !reflect.DeepEqual(receipt.Request.Check, batch.Integration.Check) ||
					receipt.Request.CandidateHead != *batch.Integration.Head || receipt.Request.ExpectedTargetHead != batch.Plan.BaseHead {
					t.Fatalf("operation binding: %s", data)
				}
			}
			head, err := exec.Command("git", "--git-dir", remote, "rev-parse", batch.Plan.BaseRef).Output()
			wantHead := batch.Plan.BaseHead
			if publication == "applied" || mode == "unknown" || mode == "cli-unknown" || mode == "cli-unknown-output" || mode == "lost" || mode == "malformed" || mode == "wrong-merge-head" || mode == "timeout" || mode == "target-drift" || mode == "merge-target-drift" {
				wantHead = *batch.Integration.Head
			}
			if err != nil || strings.TrimSpace(string(head)) != wantHead {
				t.Fatalf("target = %s %v; want %s", head, err, wantHead)
			}
			if publication != "applied" {
				if len(batch.Delivery.PublishedTasks) != 0 || batch.Delivery.HeadAfter != nil || len(batch.Delivery.PendingTasks) != 3 || batch.Integration.Workspace == nil {
					t.Fatalf("unconfirmed publication/retention: %s", data)
				}
				for _, worker := range batch.Workers {
					if worker.Workspace == nil {
						t.Fatalf("unpublished worker removed: %s", data)
					}
				}
				if publication == "unknown" {
					last := batch.Delivery.Operations[len(batch.Delivery.Operations)-1]
					if last.Outcome != "unknown" {
						t.Fatalf("unknown outcome lost: %s", data)
					}
				}
				return
			}
			if batch.Integration.Check.Head != *batch.Delivery.HeadAfter || batch.Integration.Children[len(batch.Integration.Children)-1].BoundHead != *batch.Delivery.HeadAfter {
				t.Fatalf("applied/check binding: %s", data)
			}
			if mode == "candidate-fail" && (!reflect.DeepEqual(batch.Delivery.PublishedTasks, []string{"T-001-ready", "T-003-ready"}) ||
				!reflect.DeepEqual(batch.Delivery.PendingTasks, []string{"T-002-ready"}) || batch.Workers[1].Check == nil || *batch.Workers[1].Check.Evidence.ExitCode == 0) {
				t.Fatalf("wrong independent candidate subset: %s", data)
			}
			if mode == "pass" || mode == "cli-pass" {
				heads := strings.Fields(readBytes(t, os.Getenv("LOOP_CHECK_RECORD")))
				want := []string{}
				events := []string{}
				for _, worker := range batch.Workers {
					want = append(want, *worker.IntegratedCommit)
					events = append(events, "check "+*worker.IntegratedCommit)
				}
				want = append(want, *batch.Integration.Head)
				events = append(events, "aggregate "+*batch.Integration.Head, "check "+*batch.Integration.Head, "publish "+*batch.Integration.Head)
				if !reflect.DeepEqual(heads, want) || strings.TrimSpace(readBytes(t, os.Getenv("REVIEW_EVENT_RECORD"))) != strings.Join(events, "\n") {
					t.Fatalf("check/aggregate/publication execution order: heads=%v events=%s", heads, readBytes(t, os.Getenv("REVIEW_EVENT_RECORD")))
				}
			}
			integrated, err := NewService(*batch.Integration.Workspace)
			if err != nil {
				t.Fatal(err)
			}
			tasks, err := integrated.loadTasks()
			if err != nil {
				t.Fatal(err)
			}
			for _, worker := range batch.Workers {
				task, found := taskByIDFromSlice(tasks, worker.Task.TaskID)
				if !found || (worker.Integrated && (task.Frontmatter.Status != "completed" || task.Frontmatter.LastVerificationResult != "pass")) ||
					(!worker.Integrated && task.Frontmatter.Status != "todo") {
					t.Fatalf("integrated lifecycle: %s", data)
				}
				if worker.Integrated && (worker.Check == nil || worker.Check.Head != *worker.IntegratedCommit) {
					t.Fatalf("candidate binding: %s", data)
				}
			}
			commits := strings.Fields(gitOutput(t, *batch.Integration.Workspace, "rev-list", "--reverse", batch.Plan.BaseHead+"..HEAD"))
			if len(commits) != len(batch.Integration.AcceptedTasks) || !reflect.DeepEqual(commits, parallelIntegratedCommits(batch.Workers)) {
				t.Fatalf("separate task commits lost: %s", data)
			}
		})
	}
}

func readReviewRequests(t *testing.T, file string) []ReviewAdapterRequest {
	t.Helper()
	data := strings.TrimSpace(readBytes(t, file))
	requests := []ReviewAdapterRequest{}
	for _, line := range strings.Split(data, "\n") {
		var request ReviewAdapterRequest
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			t.Fatal(err)
		}
		requests = append(requests, request)
	}
	return requests
}

func buildCheckedReviewAdapter(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "adapter.exe")
	if output, err := exec.Command("go", "build", "-o", binary, "./testdata/checked-review-adapter").CombinedOutput(); err != nil {
		t.Fatalf("build checked adapter: %v %s", err, output)
	}
	return binary
}
