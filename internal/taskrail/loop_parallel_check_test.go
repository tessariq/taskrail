package taskrail

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParallelCheckRequired(t *testing.T) {
	_, svc := loopFixture(t)
	input := LoopInvocation{MaxIterations: 2, Parallel: 2, Child: []string{"must-not-launch"}}
	if _, err := svc.LoopExecute(context.Background(), input); MachineFailureFor(err).Code != MachineCodeInvalidArguments {
		t.Fatalf("missing check: %v", err)
	}
	input.DryRun = true
	report, err := svc.LoopDryRun(input)
	if err != nil || report.Action != "invalid" || !strings.Contains(report.Reason, "--check") {
		t.Fatalf("missing-check preview: %+v, %v", report, err)
	}
}

func TestParallelReviewCheckRefusalBoundaries(t *testing.T) {
	clearLoopChildEnvironment(t)
	repo, svc := loopFixture(t)
	check := passingLoopCheck(t)
	workspace := t.TempDir()
	before := snapshotTreeWithoutGitHousekeeping(t, repo)
	for _, dry := range []bool{false, true} {
		input := LoopInvocation{DryRun: dry, MaxIterations: 2, Parallel: 2, Delivery: "review", DeliverySet: true,
			ReviewAdapter: "must-not-resolve-or-run", ReviewAdapterSet: true, ResultFile: filepath.Join(workspace, "result"),
			Child: []string{"must-not-launch"}}
		if dry {
			input.ResultFile = ""
			report, err := svc.LoopDryRun(input)
			if err != nil || report.Action != "invalid" || !strings.Contains(report.Reason, "--check") {
				t.Fatalf("missing-check precedence: %+v %v", report, err)
			}
		} else if _, err := svc.LoopExecute(context.Background(), input); MachineFailureFor(err).Code != MachineCodeInvalidArguments {
			t.Fatalf("missing check: %v", err)
		}
		input.Check = check
		_, err := svc.LoopPreflight(input)
		if MachineFailureFor(err).Code != MachineCodeUnsupported {
			t.Fatalf("review check refusal: %v", err)
		}
		input.MaxIterations = 1
		if _, err := svc.LoopPreflight(input); MachineFailureFor(err).Code != MachineCodeInvalidArguments {
			t.Fatalf("width-one review flags: %v", err)
		}
	}
	for _, gate := range []string{"", check} {
		report, err := svc.LoopDryRun(LoopInvocation{DryRun: true, MaxIterations: 1, Parallel: 3, Check: gate})
		if err != nil || report.Action == "invalid" || report.Parallel != nil {
			t.Fatalf("effective one: %+v %v", report, err)
		}
	}
	if after := snapshotTreeWithoutGitHousekeeping(t, repo); !reflect.DeepEqual(before, after) {
		t.Fatal("refusal changed repository")
	}
	if entries, err := os.ReadDir(workspace); err != nil || len(entries) != 0 {
		t.Fatalf("refusal allocated output: %v %v", entries, err)
	}
	if fileExists(os.Getenv("LOOP_CHECK_RECORD")) {
		t.Fatal("refusal ran check")
	}
}

func TestParallelChecksGateIntegrationAndPublication(t *testing.T) {
	clearLoopChildEnvironment(t)
	binary := buildTaskrailExecutable(t)
	previous := loopExecutablePath
	loopExecutablePath = func() (string, error) { return binary, nil }
	t.Cleanup(func() { loopExecutablePath = previous })
	check := buildLoopCheck(t)
	for _, mode := range []string{"build", "final-fail", "worktree", "config", "ref", "input", "spawn"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("GO_WANT_LOOP_CHILD", "1")
			t.Setenv("LOOP_CHECK_MODE", mode)
			record := filepath.Join(t.TempDir(), "heads")
			t.Setenv("LOOP_CHECK_RECORD", record)
			repo, svc := loopFixture(t)
			writeFile(t, filepath.Join(repo, "go.mod"), "module example.com/checkfixture\n\ngo 1.26\n")
			writeFile(t, filepath.Join(repo, "api.go"), "package fixture\nfunc Old() {}\n")
			writeFile(t, filepath.Join(repo, ".gitattributes"), "*.go text eol=lf\nindependent.txt text eol=lf\n")
			for _, id := range []string{"T-001-ready", "T-002-ready", "T-003-ready"} {
				writeTask(t, repo, id, id, "todo", "high", "specs/v0.1.0.md#summary", nil)
				setLoopPolicy(t, repo, id, "allow", "independent work")
			}
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-m", "prepare checked batch")
			base, _ := gitCommand(repo, "rev-parse", "HEAD")
			workspace := t.TempDir()
			for _, key := range []string{"TMPDIR", "TMP", "TEMP"} {
				t.Setenv(key, workspace)
			}
			childMode := "parallel-worker"
			childArg := "unused"
			if mode == "build" {
				childMode, childArg = "parallel-conflict-worker", "semantic"
			}
			checkPath := check
			if mode == "spawn" {
				checkPath = filepath.Join(t.TempDir(), "invalid.exe")
				if err := os.WriteFile(checkPath, []byte("not a native executable"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			report, err := svc.LoopExecute(context.Background(), LoopInvocation{MaxIterations: 3, Parallel: 3, Check: checkPath,
				Child: []string{os.Args[0], "-test.run=^TestLoopLaunchChildHelper$", "--", childMode, childArg}})
			if err != nil {
				t.Fatal(err)
			}
			data, _ := json.MarshalIndent(report, "", "  ")
			batch := report.Parallel
			if batch == nil {
				t.Fatalf("no batch: %s", data)
			}
			if mode == "spawn" {
				emptyDigest := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
				for _, worker := range batch.Workers {
					if worker.Check == nil || worker.Check.Head == "" || worker.CandidateCommit == nil || worker.Outcome != "integration_failed" || worker.Integrated || worker.IntegratedCommit != nil {
						t.Fatalf("spawn binding: %s", data)
					}
					evidence := worker.Check.Evidence
					if evidence.ExitCode != nil || evidence.Signal != nil || len(evidence.ProcessViolations) == 0 || evidence.StdoutSHA256 != emptyDigest || evidence.StderrSHA256 != emptyDigest {
						t.Fatalf("spawn evidence: %s", data)
					}
				}
				if batch.Integration.AggregatePass || len(batch.Delivery.PublishedTasks) != 0 {
					t.Fatalf("spawn failure published: %s", data)
				}
				return
			}
			observedHeads := strings.Fields(readBytes(t, record))
			expectedHeads := []string{}
			for _, worker := range batch.Workers {
				if worker.Check != nil {
					expectedHeads = append(expectedHeads, worker.Check.Head)
				}
			}
			if batch.Integration.Check != nil {
				expectedHeads = append(expectedHeads, batch.Integration.Check.Head)
			}
			if !reflect.DeepEqual(observedHeads, expectedHeads) {
				t.Fatalf("checks observed %v; diagnostic heads %v", observedHeads, expectedHeads)
			}
			if mode == "build" {
				if report.Outcome != "batch_partial" || strings.Join(batch.Delivery.PublishedTasks, ",") != "T-001-ready,T-003-ready" {
					t.Fatalf("semantic conflict: %s", data)
				}
				worker := batch.Workers[1]
				if worker.Outcome != "integration_failed" || worker.Integrated || worker.IntegratedCommit != nil || worker.Check == nil || worker.Check.Head == *worker.CandidateCommit || worker.Check.Evidence.ExitCode == nil || *worker.Check.Evidence.ExitCode == 0 {
					t.Fatalf("rejected candidate binding: %s", data)
				}
				if batch.Integration.Check == nil || !batch.Integration.AggregatePass || batch.Integration.Check.Head != *batch.Integration.Head || batch.Integration.Children[len(batch.Integration.Children)-1].BoundHead != batch.Integration.Check.Head {
					t.Fatalf("final binding: %s", data)
				}
				if strings.Contains(readBytes(t, filepath.Join(repo, "api.go")), "Old") || fileExists(filepath.Join(repo, "caller.go")) || readBytes(t, filepath.Join(repo, "independent.txt")) != "T-003-ready\n" {
					t.Fatalf("published incorrect product: %s", data)
				}
			} else {
				if batch.Integration.AggregatePass || len(batch.Delivery.PublishedTasks) != 0 {
					t.Fatalf("failed gate published: %s", data)
				}
				head, _ := gitCommand(repo, "rev-parse", "HEAD")
				if head != base || batch.Integration.Workspace == nil {
					t.Fatalf("failure identity/retention: %s", data)
				}
				if info, err := os.Stat(*batch.Integration.Workspace); err != nil || !info.IsDir() {
					t.Fatalf("failure workspace removed: %v", err)
				}
				if mode == "final-fail" {
					if batch.Integration.Check == nil || batch.Integration.Check.Evidence.ExitCode == nil || *batch.Integration.Check.Evidence.ExitCode != 7 || len(strings.Fields(readBytes(t, record))) != 4 {
						t.Fatalf("final gate: %s", data)
					}
				} else if batch.Workers[0].Check == nil || len(batch.Workers[0].Check.Evidence.MutationViolations) == 0 {
					t.Fatalf("mutation evidence: %s", data)
				}
			}
		})
	}
}
