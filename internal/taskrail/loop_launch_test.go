package taskrail

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestLoopLaunchChildTransportsExactPromptAndIdentity(t *testing.T) {
	t.Setenv("GO_WANT_LOOP_CHILD", "1")
	caller := t.TempDir()
	repository := t.TempDir()
	bin := filepath.Join(caller, "bin")
	if err := os.Mkdir(bin, 0o755); err != nil {
		t.Fatalf("create bin: %v", err)
	}
	name := "loop-child"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	copyLoopHelper(t, filepath.Join(bin, name))
	t.Chdir(caller)
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("LOOP_CALLER_VALUE", "preserved")
	t.Setenv("TASKRAIL", "caller-owned-value")

	identity := loopChildIdentity{
		Executable: "/staged/taskrail", SHA256: strings.Repeat("a", 64),
		InvocationID: "invocation", Token: "secret-token",
	}
	for _, prompt := range [][]byte{nil, []byte("final newline\n"), []byte("no final newline"), []byte("Gruesse \xe2\x98\x83")} {
		t.Run(fmt.Sprintf("prompt %q", prompt), func(t *testing.T) {
			record := filepath.Join(t.TempDir(), "record")
			var stdout, stderr bytes.Buffer
			execution, err := launchLoopChild(loopChildLaunch{
				Command: []string{name, "-test.run=^TestLoopLaunchChildHelper$", "--", "observe", record, "; touch should-not-exist", "two words"},
				Prompt:  prompt, RepositoryRoot: repository, Identity: identity,
				Stdout: &stdout, Stderr: &stderr,
			})
			if err != nil {
				t.Fatalf("launchLoopChild: %v", err)
			}
			if execution.Failed() || execution.ExitCode == nil || *execution.ExitCode != 0 {
				t.Fatalf("execution = %+v", execution)
			}
			if got := string(readBytes(t, record+".stdin")); !bytes.Equal([]byte(got), prompt) {
				t.Fatalf("stdin = %q, want %q", got, prompt)
			}
			if got := string(readBytes(t, record+".cwd")); got != repository {
				t.Fatalf("cwd = %q, want %q", got, repository)
			}
			if got := string(readBytes(t, record+".pid")); got != strconv.Itoa(execution.PID) || execution.PID <= 0 {
				t.Fatalf("child pid = %q, launch pid = %d", got, execution.PID)
			}
			if got := string(readBytes(t, record+".env")); got != strings.Join([]string{
				"/staged/taskrail", strings.Repeat("a", 64), "invocation", "secret-token", "preserved",
			}, "\n") {
				t.Fatalf("environment = %q", got)
			}
			var args []string
			if err := json.Unmarshal([]byte(readBytes(t, record+".args")), &args); err != nil {
				t.Fatalf("decode arguments: %v", err)
			}
			wantArgs := []string{"observe", record, "; touch should-not-exist", "two words"}
			if !reflect.DeepEqual(args, wantArgs) {
				t.Fatalf("arguments = %#v, want %#v", args, wantArgs)
			}
			if got := stdout.String(); got != "stdout\n" {
				t.Fatalf("stdout = %q", got)
			}
			if got := stderr.String(); got != "stderr\n" {
				t.Fatalf("stderr = %q", got)
			}
			if _, err := os.Stat(filepath.Join(caller, "should-not-exist")); !os.IsNotExist(err) {
				t.Fatalf("shell metacharacter was evaluated: %v", err)
			}
		})
	}
}

func TestLoopLaunchChildResolvesSeparatorPathBeforeRepositoryCWD(t *testing.T) {
	t.Setenv("GO_WANT_LOOP_CHILD", "1")
	caller := t.TempDir()
	repository := t.TempDir()
	child := filepath.Join(caller, "child")
	copyLoopHelper(t, child)
	t.Chdir(caller)
	record := filepath.Join(t.TempDir(), "record")

	execution, err := launchLoopChild(loopChildLaunch{
		Command:        []string{"./child", "-test.run=^TestLoopLaunchChildHelper$", "--", "observe", record},
		RepositoryRoot: repository, Identity: loopChildIdentity{Executable: "/staged/taskrail", SHA256: "digest", InvocationID: "id", Token: "token"},
	})
	if err != nil || execution.Failed() {
		t.Fatalf("launchLoopChild = %+v, %v", execution, err)
	}
	if got := string(readBytes(t, record+".cwd")); got != repository {
		t.Fatalf("cwd = %q, want %q", got, repository)
	}
}

func TestLoopLaunchChildReportsExecutionFailures(t *testing.T) {
	t.Run("spawn", func(t *testing.T) {
		execution, err := launchLoopChild(loopChildLaunch{Command: []string{filepath.Join(t.TempDir(), "missing")}, RepositoryRoot: t.TempDir()})
		if err != nil || execution.SpawnError == nil || !execution.Failed() {
			t.Fatalf("execution = %+v, %v", execution, err)
		}
	})
	t.Run("stdin", func(t *testing.T) {
		input := loopHelperLaunch(t, "close-stdin", bytes.Repeat([]byte("x"), 1<<20))
		execution, err := launchLoopChild(input)
		if err != nil || execution.StdinError == nil || !execution.Failed() {
			t.Fatalf("execution = %+v, %v", execution, err)
		}
		assertLoopLaunchCount(t, input, 1)
	})
	t.Run("stdout", func(t *testing.T) {
		input := loopHelperLaunch(t, "observe", nil)
		input.Stdout = failingLoopWriter{}
		execution, err := launchLoopChild(input)
		if err != nil || execution.StdoutError == nil || !execution.Failed() {
			t.Fatalf("execution = %+v, %v", execution, err)
		}
		assertLoopLaunchCount(t, input, 1)
	})
	t.Run("stderr", func(t *testing.T) {
		input := loopHelperLaunch(t, "observe", nil)
		input.Stderr = failingLoopWriter{}
		execution, err := launchLoopChild(input)
		if err != nil || execution.StderrError == nil || !execution.Failed() {
			t.Fatalf("execution = %+v, %v", execution, err)
		}
		assertLoopLaunchCount(t, input, 1)
	})
	t.Run("nonzero", func(t *testing.T) {
		input := loopHelperLaunch(t, "nonzero", nil)
		execution, err := launchLoopChild(input)
		if err != nil || execution.WaitError == nil || execution.ExitCode == nil || *execution.ExitCode != 7 {
			t.Fatalf("execution = %+v, %v", execution, err)
		}
		assertLoopLaunchCount(t, input, 1)
	})
	t.Run("stream failure stops a running child", func(t *testing.T) {
		input := loopHelperLaunch(t, "linger-output", nil)
		input.Stdout = failingLoopWriter{}
		started := time.Now()
		execution, err := launchLoopChild(input)
		if err != nil || execution.StdoutError == nil || time.Since(started) > time.Second {
			t.Fatalf("execution = %+v, %v after %s", execution, err, time.Since(started))
		}
		assertLoopLaunchCount(t, input, 1)
	})
}

func TestLoopLaunchChildDrainsBothStreamsConcurrently(t *testing.T) {
	input := loopHelperLaunch(t, "duplex", nil)
	var stdout, stderr bytes.Buffer
	input.Stdout, input.Stderr = &stdout, &stderr
	execution, err := launchLoopChild(input)
	if err != nil || execution.Failed() {
		t.Fatalf("execution = %+v, %v", execution, err)
	}
	if stdout.Len() != 1<<20 || stderr.Len() != 1<<20 {
		t.Fatalf("stream lengths = %d, %d", stdout.Len(), stderr.Len())
	}
	assertLoopLaunchCount(t, input, 1)
}

type failingLoopWriter struct{}

func (failingLoopWriter) Write([]byte) (int, error) { return 0, fmt.Errorf("stream failed") }

func loopHelperLaunch(t *testing.T, mode string, prompt []byte) loopChildLaunch {
	t.Helper()
	t.Setenv("GO_WANT_LOOP_CHILD", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	return loopChildLaunch{
		Command: []string{executable, "-test.run=^TestLoopLaunchChildHelper$", "--", mode, filepath.Join(t.TempDir(), "record")},
		Prompt:  prompt, RepositoryRoot: t.TempDir(),
		Identity: loopChildIdentity{Executable: "/staged/taskrail", SHA256: "digest", InvocationID: "id", Token: "token"},
	}
}

func assertLoopLaunchCount(t *testing.T, input loopChildLaunch, want int) {
	t.Helper()
	record := input.Command[len(input.Command)-1]
	got, err := strconv.Atoi(string(readBytes(t, record+".launches")))
	if err != nil || got != want {
		t.Fatalf("launch count = %d, %v; want %d", got, err, want)
	}
}

func copyLoopHelper(t *testing.T, destination string) {
	t.Helper()
	source, err := os.Executable()
	if err != nil {
		t.Fatalf("executable: %v", err)
	}
	data, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read helper: %v", err)
	}
	if err := os.WriteFile(destination, data, 0o755); err != nil {
		t.Fatalf("write helper: %v", err)
	}
}

func TestLoopLaunchChildHelper(t *testing.T) {
	if os.Getenv("GO_WANT_LOOP_CHILD") != "1" {
		return
	}
	args := os.Args
	separator := -1
	for i, arg := range args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator < 0 || len(args) < separator+3 {
		os.Exit(99)
	}
	mode, record := args[separator+1], args[separator+2]
	count := 0
	if previous, err := os.ReadFile(record + ".launches"); err == nil {
		count, _ = strconv.Atoi(string(previous))
	}
	if err := os.WriteFile(record+".launches", []byte(strconv.Itoa(count+1)), 0o600); err != nil {
		os.Exit(93)
	}
	switch mode {
	case "close-stdin":
		_ = os.Stdin.Close()
		time.Sleep(50 * time.Millisecond)
		os.Exit(0)
	case "nonzero":
		os.Exit(7)
	case "linger-output":
		fmt.Fprint(os.Stdout, "stdout\n")
		time.Sleep(5 * time.Second)
		os.Exit(0)
	case "duplex":
		done := make(chan struct{}, 2)
		for _, stream := range []io.Writer{os.Stdout, os.Stderr} {
			go func(stream io.Writer) {
				_, _ = stream.Write(bytes.Repeat([]byte("x"), 1<<20))
				done <- struct{}{}
			}(stream)
		}
		<-done
		<-done
		os.Exit(0)
	case "observe":
		prompt, err := io.ReadAll(os.Stdin)
		if err != nil {
			os.Exit(98)
		}
		cwd, err := os.Getwd()
		if err != nil {
			os.Exit(97)
		}
		env := strings.Join([]string{
			os.Getenv("TASKRAIL"), os.Getenv("TASKRAIL_EXECUTABLE_SHA256"),
			os.Getenv("TASKRAIL_DELEGATION_ID"), os.Getenv("TASKRAIL_DELEGATION_TOKEN"), os.Getenv("LOOP_CALLER_VALUE"),
		}, "\n")
		encoded, err := json.Marshal(args[separator+1:])
		if err != nil {
			os.Exit(96)
		}
		for path, data := range map[string][]byte{record + ".stdin": prompt, record + ".cwd": []byte(cwd), record + ".pid": []byte(strconv.Itoa(os.Getpid())), record + ".env": []byte(env), record + ".args": encoded} {
			if err := os.WriteFile(path, data, 0o600); err != nil {
				os.Exit(95)
			}
		}
		fmt.Fprint(os.Stdout, "stdout\n")
		fmt.Fprint(os.Stderr, "stderr\n")
		os.Exit(0)
	default:
		os.Exit(94)
	}
}
