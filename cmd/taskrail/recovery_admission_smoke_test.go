package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/taskrail"
)

func TestRecoveryFenceRefusesReaderAndWriterWithStrictMachineError(t *testing.T) {
	root := setupRepo(t)
	writeTask(t, root, "T-001-fenced", "todo", "")
	statePath := filepath.Join(root, "planning", "STATE.md")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	tx := "0123456789abcdef0123456789abcdef"
	dir := filepath.Join(root, ".git", "taskrail", "transactions", tx)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	journal := `{"transaction_id":"` + tx + `","command":"start","phase":"prepared"}`
	if err := os.WriteFile(filepath.Join(dir, "journal.json"), []byte(journal), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"status", "--json"}, {"start", "T-001-fenced", "--json"}} {
		out, runErr := runRoot(t, args...)
		if runErr == nil {
			t.Fatalf("%v succeeded: %s", args, out)
		}
		env, decodeErr := taskrail.DecodeMachineEnvelope([]byte(out))
		if decodeErr != nil {
			t.Fatalf("decode %v: %v\n%s", args, decodeErr, out)
		}
		if env.Result != nil || env.Error == nil || env.Error.Code != taskrail.MachineCodeRecoveryPending {
			t.Fatalf("%v envelope = %+v", args, env)
		}
		if env.Error.Message != "repository recovery is pending" || env.Error.Details.Applied || env.Error.Details.Recovery == nil {
			t.Fatalf("%v error = %+v", args, env.Error)
		}
	}
	after, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatal("fenced writer changed STATE.md")
	}
}

func TestMalformedRecoveryFenceUsesSameRefusalWithoutPartialResult(t *testing.T) {
	root := setupRepo(t)
	dir := filepath.Join(root, ".git", "taskrail", "transactions", "malformed")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "journal.json"), []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	out, err := runRoot(t, "validate", "--json")
	if err == nil || strings.Contains(out, `"result"`) {
		t.Fatalf("validate malformed fence: err=%v out=%s", err, out)
	}
	env, decodeErr := taskrail.DecodeMachineEnvelope([]byte(out))
	if decodeErr != nil || env.Error.Code != taskrail.MachineCodeRecoveryPending || env.Error.Details.Recovery != nil {
		t.Fatalf("envelope=%+v decode=%v", env, decodeErr)
	}
}

func TestEveryConstructedSemanticCommandFamilyUsesRecoveryAdmission(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "init", args: []string{"init"}},
		{name: "retrofit", args: []string{"retrofit"}},
		{name: "validate", args: []string{"validate"}},
		{name: "repair", args: []string{"repair"}},
		{name: "coverage", args: []string{"coverage"}},
		{name: "status", args: []string{"status"}},
		{name: "stats", args: []string{"stats"}},
		{name: "next", args: []string{"next"}},
		{name: "start", args: []string{"start", "T-001-fenced"}},
		{name: "complete", args: []string{"complete", "T-001-fenced"}},
		{name: "block", args: []string{"block", "T-001-fenced", "--reason", "blocked"}},
		{name: "unblock", args: []string{"unblock", "T-001-fenced"}},
		{name: "verify", args: []string{"verify", "T-001-fenced", "--result", "pass", "--summary", "checked"}},
		{name: "task new", args: []string{"task", "new", "--title", "new", "--area", "summary"}},
		{name: "task rename", args: []string{"task", "rename", "T-001-fenced", "--slug", "renamed"}},
		{name: "task repoint", args: []string{"task", "repoint", "T-001-fenced", "--area", "summary"}},
		{name: "task show", args: []string{"task", "show", "T-001-fenced"}},
		{name: "review show", args: []string{"review", "show", "planning/reviews/spec/v0.1.0/session/report.json"}},
		{name: "spec list", args: []string{"spec", "list"}},
		{name: "spec show", args: []string{"spec", "show", "v0.1.0"}},
		{name: "spec add", args: []string{"spec", "add", "v0.2.0"}},
		{name: "spec diff", args: []string{"spec", "diff", "v0.1.0", "v0.1.0"}},
		{name: "spec activate", args: []string{"spec", "activate", "v0.1.0"}},
		{name: "import", args: []string{"import", "README.md", "--to", "tasks"}},
		{name: "lock status", args: []string{"lock", "status"}},
		{name: "lock clear", args: []string{"lock", "clear", "0123456789abcdef0123456789abcdef", "--expect-sha256", strings.Repeat("a", 64)}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := setupRepo(t)
			writeTask(t, root, "T-001-fenced", "todo", "")
			installRecoveryFence(t, root, "start")
			args := append(append([]string{}, test.args...), "--json")
			out, err := runRoot(t, args...)
			if err == nil {
				t.Fatalf("command succeeded: %s", out)
			}
			env, decodeErr := taskrail.DecodeMachineEnvelope([]byte(out))
			if decodeErr != nil || env.Result != nil || env.Error == nil || env.Error.Code != taskrail.MachineCodeRecoveryPending {
				t.Fatalf("envelope=%+v decode=%v output=%s", env, decodeErr, out)
			}
		})
	}
}

func TestGraphExportUsesRecoveryAdmission(t *testing.T) {
	root := setupRepo(t)
	installRecoveryFence(t, root, "stats")
	out, err := runRoot(t, "stats", "--format", "dot")
	if err == nil || out != "" || err.Error() != "repository recovery is pending" {
		t.Fatalf("graph fence: out=%q err=%v", out, err)
	}
}

func installRecoveryFence(t *testing.T, root, command string) {
	t.Helper()
	tx := "0123456789abcdef0123456789abcdef"
	dir := filepath.Join(root, ".git", "taskrail", "transactions", tx)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	journal := `{"transaction_id":"` + tx + `","command":"` + command + `","phase":"prepared"}`
	if err := os.WriteFile(filepath.Join(dir, "journal.json"), []byte(journal), 0o600); err != nil {
		t.Fatal(err)
	}
}
