package taskrail

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParseLoopCheck(t *testing.T) {
	for _, args := range [][]string{
		{"--check", "gate", "--", "child"},
		{"--dry-run", "--check=gate"},
		{"--max-iterations=2", "--parallel=2", "--check=gate", "--", "child"},
	} {
		if _, err := ParseLoopInvocation(args); err != nil {
			t.Fatalf("ParseLoopInvocation(%v): %v", args, err)
		}
	}
	for _, args := range [][]string{
		{"--check=", "--", "child"},
		{"--check=gate", "--check=other", "--", "child"},
	} {
		if _, err := ParseLoopInvocation(args); MachineFailureFor(err).Code != MachineCodeInvalidArguments {
			t.Fatalf("ParseLoopInvocation(%v): %v, want invalid_arguments", args, err)
		}
	}
}

func TestLoopCheckExecution(t *testing.T) {
	clearLoopChildEnvironment(t)
	t.Setenv("GO_WANT_LOOP_CHILD", "1")
	binary := buildTaskrailExecutable(t)
	previous := loopExecutablePath
	loopExecutablePath = func() (string, error) { return binary, nil }
	t.Cleanup(func() { loopExecutablePath = previous })
	check := buildLoopCheck(t)
	for _, mode := range []string{"pass", "next", "fail", "timeout", "close-output", "signal", "worktree", "ignored", "type", "config", "ref", "head", "rootref", "input", "child-fail", "child-config"} {
		t.Run(mode, func(t *testing.T) {
			if mode == "signal" && runtime.GOOS == "windows" {
				t.Skip("Windows does not support sending os.Interrupt to oneself")
			}
			if mode == "type" && runtime.GOOS == "windows" {
				t.Skip("Unix file/symlink mode collision")
			}
			t.Setenv("LOOP_CHECK_MODE", mode)
			record := filepath.Join(t.TempDir(), "check-heads")
			t.Setenv("LOOP_CHECK_RECORD", record)
			t.Setenv("LOOP_CHILD_STOP_TASK", "T-002-ready")
			t.Setenv("LOOP_CHILD_STOP_RECORD", record+".second")
			repo, svc := loopFixture(t)
			writeFile(t, filepath.Join(repo, ".gitignore"), string(readBytes(t, filepath.Join(repo, ".gitignore")))+"\nignored-check.txt\n")
			if mode == "type" {
				file := filepath.Join(repo, "ignored-check.txt")
				if err := os.WriteFile(file, []byte("same-target"), 0o777); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(file, 0o777); err != nil {
					t.Fatal(err)
				}
				if info, err := os.Lstat(file); err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o777 || readBytes(t, file) != "same-target" {
					t.Fatalf("incorrect type-collision fixture: %v", err)
				}
			}
			for _, id := range []string{"T-001-ready", "T-002-ready"} {
				writeTask(t, repo, id, "Check delivery", "todo", "high", "specs/v0.1.0.md#summary", nil)
				if _, err := svc.MutateTaskLoopPolicy(LoopPolicyMutationInput{TaskID: id, Operation: LoopPolicyAllow, Reason: "check delivery"}); err != nil {
					t.Fatal(err)
				}
			}
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-m", "allow tasks")
			input := LoopInvocation{MaxIterations: 2, Check: check, Child: []string{os.Args[0], "-test.run=^TestLoopLaunchChildHelper$", "--", "parallel-worker", "unused"}}
			if mode == "pass" {
				input.MaxIterations = 1
			}
			if mode == "timeout" || mode == "close-output" {
				timeout := 3 * time.Second
				input.Timeout = &timeout
			}
			report, err := svc.LoopExecute(context.Background(), input)
			if err != nil {
				t.Fatalf("LoopExecute: %v", err)
			}
			if mode == "next" {
				if report.Outcome != "child_failed" || report.IterationsCompleted != 2 || report.LastIteration.TaskID != "T-002-ready" || report.LastIteration.Check != nil || readBytes(t, record+".second") != "T-002-ready" || len(strings.Fields(readBytes(t, record))) != 1 {
					t.Fatalf("passing check did not permit exactly one next launch: %+v", report)
				}
				return
			}
			if _, err := os.Stat(record + ".second"); !os.IsNotExist(err) {
				t.Fatalf("unexpected second child launch: %v", err)
			}
			if mode == "child-fail" || mode == "child-config" {
				want := "child_failed"
				if mode == "child-config" {
					want = "invalid_postflight"
				}
				if report.Outcome != want || valueOrEmpty(report.LastIteration.LifecycleCandidate) != "completed_pass" || report.LastIteration.Check != nil {
					t.Fatalf("check ran despite final child outcome: %+v", report)
				}
				if _, err := os.Stat(record); !os.IsNotExist(err) {
					t.Fatalf("check marker exists: %v", err)
				}
				return
			}
			want, count := "check_failed", 1
			if mode == "pass" {
				want = "iteration_limit"
			}
			if report.Outcome != want || report.IterationsCompleted != count || report.LastIteration == nil || valueOrEmpty(report.LastIteration.LifecycleCandidate) != "completed_pass" {
				t.Fatalf("report = %+v", report)
			}
			evidence := report.LastIteration.Check
			if evidence == nil || evidence.Path != check || evidence.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(readBytes(t, check)))) || evidence.DurationMS < 0 {
				t.Fatalf("check evidence = %+v", evidence)
			}
			if evidence.StdoutSHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte("gate stdout\n"))) || evidence.StderrSHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte("gate stderr\n"))) {
				t.Fatalf("stream digests = %+v", evidence)
			}
			if evidence.MutationViolations == nil || evidence.ProcessViolations == nil || (evidence.ExitCode == nil) == (evidence.Signal == nil) {
				t.Fatalf("incomplete evidence = %+v", evidence)
			}
			if evidence.TimedOut != (mode == "timeout" || mode == "close-output") || (mode == "fail" && (evidence.ExitCode == nil || *evidence.ExitCode != 7)) {
				t.Fatalf("process evidence = %+v", evidence)
			}
			if mode == "signal" && evidence.Signal == nil {
				t.Fatalf("missing signal: %+v", evidence)
			}
			if mode == "type" {
				target, err := os.Readlink(filepath.Join(repo, "ignored-check.txt"))
				if err != nil || target != "same-target" || evidence.ExitCode == nil || *evidence.ExitCode != 0 {
					t.Fatalf("type check did not replace file successfully: %s %v %+v", target, err, evidence)
				}
			}
			if mode == "worktree" || mode == "ignored" || mode == "type" || mode == "config" || mode == "ref" || mode == "head" || mode == "rootref" || mode == "input" {
				if len(evidence.MutationViolations) == 0 {
					t.Fatalf("mutation accepted: %+v", evidence)
				}
			}
			heads := strings.Fields(string(readBytes(t, record)))
			head := strings.TrimSpace(gitOutput(t, repo, "rev-parse", "HEAD"))
			delivered := heads[len(heads)-1]
			if mode == "head" && head == delivered {
				t.Fatal("check did not move HEAD")
			}
			if len(heads) != count || report.Git.HeadAfter != delivered || delivered == report.Git.HeadBefore || len(report.Git.Commits) != 1 || report.Git.Commits[0] != delivered || !report.Git.Clean {
				t.Fatalf("check count or delivered HEAD changed: heads=%v git=%+v", heads, report.Git)
			}
			var cause error
			if mode != "pass" {
				cause = WithMachineErrorCode("check_failed", fmt.Errorf("check failed"))
			}
			data, err := EncodeLoopResultFileDocument(&report, cause, nil)
			if err != nil {
				t.Fatal(err)
			}
			var envelope struct {
				Result json.RawMessage `json:"result"`
				Error  *struct {
					Code    string          `json:"code"`
					Message string          `json:"message"`
					Details json.RawMessage `json:"details"`
				} `json:"error"`
				SchemaVersion int               `json:"schema_version"`
				Command       string            `json:"command"`
				Warnings      []json.RawMessage `json:"warnings"`
			}
			decoder := json.NewDecoder(bytes.NewReader(data))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&envelope); err != nil {
				t.Fatal(err)
			}
			payload := envelope.Result
			if mode != "pass" {
				payload = envelope.Error.Details
			}
			var decoded LoopDiagnostic
			decoder = json.NewDecoder(bytes.NewReader(payload))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&decoded); err != nil {
				t.Fatal(err)
			}
			if decoded.LastIteration.Check.StdoutSHA256 != evidence.StdoutSHA256 || decoded.LastIteration.Check.Path != check || decoded.Git.HeadAfter != delivered {
				t.Fatalf("result file lost evidence: %s", data)
			}
			var diagnostic struct {
				Last struct {
					Check map[string]json.RawMessage `json:"check"`
				} `json:"last_iteration"`
			}
			if err := json.Unmarshal(payload, &diagnostic); err != nil {
				t.Fatal(err)
			}
			keys := strings.Fields("path sha256 exit_code signal timed_out duration_ms stdout_sha256 stderr_sha256 mutation_violations process_violations")
			if len(diagnostic.Last.Check) != len(keys) {
				t.Fatalf("check shape = %s", payload)
			}
			for _, key := range keys {
				if _, ok := diagnostic.Last.Check[key]; !ok {
					t.Fatalf("missing check member %s", key)
				}
			}
		})
	}
}

func passingLoopCheck(t *testing.T) string {
	t.Helper()
	t.Setenv("LOOP_CHECK_MODE", "pass")
	t.Setenv("LOOP_CHECK_RECORD", filepath.Join(t.TempDir(), "heads"))
	return buildLoopCheck(t)
}

func buildLoopCheck(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	program := filepath.Join(dir, "check.go")
	writeFile(t, program, `package main
import ("fmt"; "os"; "os/exec"; "time")
func main() {
 if len(os.Args) != 1 || os.Getenv("TASKRAIL_DELEGATION_TOKEN") != "" || os.Getenv("LOOP_CHECK_MODE") == "" { os.Exit(91) }
 head, err := exec.Command("git", "rev-parse", "HEAD").Output(); if err != nil { os.Exit(92) }
 f, err := os.OpenFile(os.Getenv("LOOP_CHECK_RECORD"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600); if err != nil { os.Exit(93) }; f.Write(head); f.Close()
 fmt.Fprint(os.Stdout, "gate stdout\n"); fmt.Fprint(os.Stderr, "gate stderr\n")
 switch os.Getenv("LOOP_CHECK_MODE") {
 case "fail": os.Exit(7)
 case "build": c := exec.Command("go", "test", "./..."); c.Stdout = os.Stdout; c.Stderr = os.Stderr; if c.Run() != nil { os.Exit(7) }
 case "final-fail": data, _ := os.ReadFile(os.Getenv("LOOP_CHECK_RECORD")); n := 0; for _, b := range data { if b == '\n' { n++ } }; if n == 4 { os.Exit(7) }
 case "timeout": time.Sleep(30*time.Second)
 case "close-output": os.Stdout.Close(); os.Stderr.Close(); time.Sleep(6*time.Second)
 case "signal": p, _ := os.FindProcess(os.Getpid()); p.Signal(os.Interrupt); time.Sleep(time.Second); os.Exit(90)
 case "worktree": os.WriteFile("check-write.txt", []byte("changed"), 0600)
 case "ignored": os.WriteFile("ignored-check.txt", []byte("changed"), 0600)
 case "type": os.Remove("ignored-check.txt"); if os.Symlink("same-target", "ignored-check.txt") != nil { os.Exit(94) }
 case "config": exec.Command("git", "config", "check.changed", "yes").Run()
 case "ref": exec.Command("git", "update-ref", "refs/tags/check-created", "HEAD").Run()
 case "head": if exec.Command("git", "update-ref", "HEAD", "HEAD^").Run() != nil { os.Exit(94) }
 case "rootref": os.WriteFile(".git/CHECK_REF", []byte("changed"), 0600)
 case "input": os.WriteFile("specs/v0.1.0.md", []byte("changed"), 0600)
 }
 if record := os.Getenv("REVIEW_EVENT_RECORD"); record != "" { f, err := os.OpenFile(record, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600); if err != nil { os.Exit(93) }; fmt.Fprintf(f, "check %s", head); f.Close() }
}
`)
	binary := filepath.Join(dir, "check")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	if output, err := exec.Command("go", "build", "-o", binary, program).CombinedOutput(); err != nil {
		t.Fatalf("build check: %v\n%s", err, output)
	}
	return binary
}

func loopCheckChildOutcome() int {
	switch os.Getenv("LOOP_CHECK_MODE") {
	case "child-fail":
		return 7
	case "child-config":
		if exec.Command("git", "config", "check.childchanged", "yes").Run() != nil {
			return 93
		}
	}
	return 0
}

func TestLoopCheckPreviewAndRefusals(t *testing.T) {
	clearLoopChildEnvironment(t)
	repo, svc := loopFixture(t)
	check := buildLoopCheck(t)
	caller := filepath.Dir(check)
	t.Chdir(caller)
	t.Setenv("PATH", caller+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, name := range []string{check, filepath.Base(check), "." + string(filepath.Separator) + filepath.Base(check)} {
		report, err := svc.LoopDryRun(LoopInvocation{DryRun: true, MaxIterations: 1, Check: name})
		if err != nil || report.Check == nil || !filepath.IsAbs(report.Check.Path) || report.Check.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(readBytes(t, check)))) {
			t.Fatalf("dry-run check %q = %+v, %v", name, report.Check, err)
		}
		want, wantErr := os.Stat(check)
		got, gotErr := os.Stat(report.Check.Path)
		if wantErr != nil || gotErr != nil || !os.SameFile(want, got) {
			t.Fatalf("check resolved to another file: %+v", report.Check)
		}
		data, err := json.Marshal(report.Check)
		if err != nil {
			t.Fatal(err)
		}
		var descriptor struct {
			Path   string `json:"path"`
			SHA256 string `json:"sha256"`
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&descriptor); err != nil || descriptor.Path != report.Check.Path {
			t.Fatalf("descriptor: %s %v", data, err)
		}
	}
	for _, name := range []string{filepath.Join(caller, "missing"), repo} {
		_, err := svc.LoopExecute(context.Background(), LoopInvocation{MaxIterations: 1, Check: name, Child: []string{"must-not-launch"}})
		if MachineFailureFor(err).Code != MachineCodeInvalidArguments {
			t.Fatalf("invalid check %q: %v", name, err)
		}
	}
	if runtime.GOOS != "windows" {
		noExec := filepath.Join(caller, "no-exec")
		if err := os.WriteFile(noExec, []byte("no executable bit"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := svc.LoopDryRun(LoopInvocation{DryRun: true, MaxIterations: 1, Check: noExec})
		if MachineFailureFor(err).Code != MachineCodeInvalidArguments {
			t.Fatalf("non-executable: %v", err)
		}
	}
	report, err := svc.LoopDryRun(LoopInvocation{DryRun: true, MaxIterations: 1})
	if err != nil || report.Check != nil {
		t.Fatalf("omitted check = %+v, %v", report.Check, err)
	}
	writeTask(t, repo, "T-001-ready", "No progress", "todo", "high", "specs/v0.1.0.md#summary", nil)
	if _, err := svc.MutateTaskLoopPolicy(LoopPolicyMutationInput{TaskID: "T-001-ready", Operation: LoopPolicyAllow, Reason: "skip check on child failure"}); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "allow task")
	t.Setenv("GO_WANT_LOOP_CHILD", "1")
	result, err := svc.LoopExecute(context.Background(), LoopInvocation{MaxIterations: 1, Check: check, Child: []string{os.Args[0], "-test.run=^TestLoopLaunchChildHelper$", "--", "nonzero", filepath.Join(caller, "record")}})
	if err != nil || result.Outcome != "child_failed" || result.LastIteration.Check != nil {
		t.Fatalf("failed child ran check: %+v, %v", result, err)
	}
}

func TestLoopCheckSpawnFailure(t *testing.T) {
	clearLoopChildEnvironment(t)
	t.Setenv("GO_WANT_LOOP_CHILD", "1")
	binary := buildTaskrailExecutable(t)
	previous := loopExecutablePath
	loopExecutablePath = func() (string, error) { return binary, nil }
	t.Cleanup(func() { loopExecutablePath = previous })
	repo, svc := loopFixture(t)
	writeTask(t, repo, "T-001-ready", "Deliver", "todo", "high", "specs/v0.1.0.md#summary", nil)
	if _, err := svc.MutateTaskLoopPolicy(LoopPolicyMutationInput{TaskID: "T-001-ready", Operation: LoopPolicyAllow, Reason: "check spawn failure"}); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-m", "allow task")
	check := filepath.Join(t.TempDir(), "invalid.exe")
	if err := os.WriteFile(check, []byte("not a native executable"), 0o755); err != nil {
		t.Fatal(err)
	}
	report, err := svc.LoopExecute(context.Background(), LoopInvocation{MaxIterations: 1, Check: check, Child: []string{os.Args[0], "-test.run=^TestLoopLaunchChildHelper$", "--", "parallel-worker", "unused"}})
	if err != nil || report.LastIteration == nil {
		t.Fatalf("execute: %+v, %v", report, err)
	}
	evidence := report.LastIteration.Check
	emptyDigest := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if report.Outcome != "check_failed" || evidence == nil || evidence.ExitCode != nil || evidence.Signal != nil || evidence.StdoutSHA256 != emptyDigest || evidence.StderrSHA256 != emptyDigest || len(evidence.ProcessViolations) == 0 {
		t.Fatalf("spawn failure evidence: %+v report=%+v", evidence, report)
	}
}
