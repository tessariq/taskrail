package taskrail

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// parseSpecReportGatedCommands reads the feature spec's closed list of commands
// whose completed report may exit non-zero, so the derived exit policy is
// compared against the normative sentence rather than against itself.
func parseSpecReportGatedCommands(t *testing.T) []string {
	t.Helper()
	sentence := specLine(t, readFeatureSpec(t), "- An envelope contains `result` when")
	var commands []string
	for _, token := range backtickedTokens(sentence) {
		command := commandPathPrefix(token)
		if _, ok := MachineCommandEntryFor(command, MachineSurfaceStdout); !ok {
			continue
		}
		commands = append(commands, command)
	}
	if len(commands) == 0 {
		t.Fatal("the report-result exception sentence names no inventoried command")
	}
	return uniqueSorted(commands)
}

// commandPathPrefix keeps the leading command tokens of a spec mention such as
// "coverage --min" or "loop --dry-run", dropping the flags that qualify it.
func commandPathPrefix(token string) string {
	var words []string
	for _, word := range strings.Fields(token) {
		if strings.HasPrefix(word, "-") {
			break
		}
		words = append(words, word)
	}
	return strings.Join(words, " ")
}

func TestMachineExitPoliciesMatchTheSpecGatingList(t *testing.T) {
	var gated []string
	for _, entry := range MachineCommandInventory() {
		if entry.Surface == MachineSurfaceStdout && entry.ExitPolicy() == MachineExitReportGated {
			gated = append(gated, entry.Command)
		}
	}
	if got, want := uniqueSorted(gated), parseSpecReportGatedCommands(t); !reflect.DeepEqual(got, want) {
		t.Errorf("report-gated commands are %v, the spec gates %v", got, want)
	}
	resultFile, ok := MachineCommandEntryFor("loop", MachineSurfaceResultFile)
	if !ok {
		t.Fatal("inventory is missing the loop result-file entry")
	}
	if got := resultFile.ExitPolicy(); got != MachineExitNotApplicable {
		t.Errorf("loop result-file exit policy is %q, want %q", got, MachineExitNotApplicable)
	}
}

func TestCheckMachineEntryPolicyRejectsPerturbedEntries(t *testing.T) {
	entry := func(command string) MachineCommandEntry {
		t.Helper()
		found, ok := MachineCommandEntryFor(command, MachineSurfaceStdout)
		if !ok {
			t.Fatalf("inventory is missing %q", command)
		}
		return found
	}
	cases := []struct {
		name    string
		entry   MachineCommandEntry
		wantErr string
	}{
		{
			name: "planned command claiming implemented coverage",
			entry: func() MachineCommandEntry {
				e := entry("task loop allow")
				e.JSONState = MachineJSONInherited
				return e
			}(),
			wantErr: `command "task loop allow" publishes "inherited" but is not constructed`,
		},
		{
			name: "ungated command claiming a report-result exit exception",
			entry: func() MachineCommandEntry {
				e := entry("status")
				e.NonzeroResult = "non-empty `violations`"
				return e
			}(),
			wantErr: `command "status" claims report-result exit exception`,
		},
		{
			name: "gated command dropping its report-result exit exception",
			entry: func() MachineCommandEntry {
				e := entry("validate")
				e.NonzeroResult = "never"
				return e
			}(),
			wantErr: `command "validate" gates its report but claims no report-result exit exception`,
		},
		{
			name: "result file waiving an exit exception it never had",
			entry: func() MachineCommandEntry {
				e, ok := MachineCommandEntryFor("loop", MachineSurfaceResultFile)
				if !ok {
					t.Fatal("inventory is missing the loop result-file entry")
				}
				e.NonzeroResult = "never"
				return e
			}(),
			wantErr: `command "loop" publishes a result file`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkMachineEntryPolicy(tc.entry)
			if err == nil {
				t.Fatalf("perturbed entry was accepted, want %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("entry error is %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// migratedCommands is the exact set of commands whose producer publishes the
// common envelope today: every command the CLI constructs, since T-271 moved the
// last of the inherited writers over. The planned commands publish nothing yet.
var migratedCommands = []string{
	"block", "complete", "coverage", "import", "init", "lock clear", "lock status",
	"local path", "local status", "next", "recover", "repair", "retrofit",
	"prompt list", "prompt render", "prompt show", "review publish", "review show", "spec activate", "spec add", "spec diff", "spec list", "spec show", "start",
	"stats", "status", "task new", "task rename", "task repoint", "task release", "task show", "task loop list", "unblock",
	"task dependency add", "task dependency remove", "validate", "verify",
	"loop",
}

// TestOnlyMigratedCommandsPublishTheCommonEnvelope pins the migration boundary,
// so no entry can be counted as schema-1 coverage until a migration moves its
// producer onto the common envelope and flips its inventory constructor.
func TestOnlyMigratedCommandsPublishTheCommonEnvelope(t *testing.T) {
	for _, entry := range MachineCommandInventory() {
		want := (slices.Contains(migratedCommands, entry.Command) && entry.Surface == MachineSurfaceStdout) || entry.Surface == MachineSurfaceResultFile
		if got := entry.JSONState == MachineJSONEnvelope; got != want {
			t.Errorf("%s publishes the common envelope = %v, want %v", entry.CompanionRow, got, want)
		}
	}
}

// registrationsFor is the registration set a CLI that publishes exactly the
// inventoried machine documents would report.
func registrationsFor(t *testing.T) []MachineRegistration {
	t.Helper()
	var registrations []MachineRegistration
	for _, entry := range MachineCommandInventory() {
		if entry.JSONState == MachineJSONAbsent {
			continue
		}
		registrations = append(registrations, MachineRegistration{Command: entry.Command, Surface: entry.Surface})
	}
	if len(registrations) == 0 {
		t.Fatal("the inventory declares no published machine document")
	}
	return registrations
}

func TestCheckMachineRegistrationsAcceptsTheInventoriedDocuments(t *testing.T) {
	if err := CheckMachineRegistrations(registrationsFor(t)); err != nil {
		t.Fatalf("the inventoried machine documents drifted: %v", err)
	}
}

func TestCheckMachineRegistrationsRejectsDrift(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func([]MachineRegistration) []MachineRegistration
		wantErr string
	}{
		{
			name: "missing registration",
			mutate: func(registrations []MachineRegistration) []MachineRegistration {
				return registrations[1:]
			},
			wantErr: `is inventoried as publishing "envelope"`,
		},
		{
			name: "duplicate registration",
			mutate: func(registrations []MachineRegistration) []MachineRegistration {
				return append(registrations, registrations[0])
			},
			wantErr: `registers document "init stdout" twice`,
		},
		{
			name: "uninventoried command",
			mutate: func(registrations []MachineRegistration) []MachineRegistration {
				return append(registrations, MachineRegistration{Command: "version", Surface: MachineSurfaceStdout})
			},
			wantErr: `publishes "version stdout" with no v0.5 machine inventory entry`,
		},
		{
			name: "planned command masquerading as implemented",
			mutate: func(registrations []MachineRegistration) []MachineRegistration {
				return append(registrations, MachineRegistration{Command: "task loop allow", Surface: MachineSurfaceStdout})
			},
			wantErr: `"task loop allow stdout" publishes no machine document yet`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckMachineRegistrations(tc.mutate(registrationsFor(t)))
			if err == nil {
				t.Fatalf("drift was accepted, want %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("drift error is %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestCheckMachineRegistrationsIsDeterministic(t *testing.T) {
	broken := append(registrationsFor(t)[1:],
		MachineRegistration{Command: "version", Surface: MachineSurfaceStdout},
		MachineRegistration{Command: "prompt render", Surface: MachineSurfaceStdout},
		MachineRegistration{Command: "status", Surface: MachineSurfaceStdout},
	)
	first := CheckMachineRegistrations(broken)
	second := CheckMachineRegistrations(broken)
	if first == nil || second == nil {
		t.Fatal("drift was accepted")
	}
	if first.Error() != second.Error() {
		t.Fatalf("diagnostics are not stable:\n%v\n%v", first, second)
	}
}

// machineDocumentBytes builds one common envelope. Warnings and the payload are
// supplied as encoded members so a test can publish exactly the bytes it means.
func machineDocumentBytes(command, warnings, payload string) []byte {
	return fmt.Appendf(nil, `{"schema_version":1,"command":%q,"warnings":%s,%s}`, command, warnings, payload)
}

func machineErrorDocument(command, code string) []byte {
	payload := fmt.Sprintf(`"error":{"code":%q,"message":"refused","details":`+
		`{"applied":false,"violations":[],"paths":[],"snapshots":[],"recovery":null}}`, code)
	return machineDocumentBytes(command, "[]", payload)
}

func TestCheckMachinePublicationAcceptsContractualDocuments(t *testing.T) {
	cases := []struct {
		name        string
		publication MachinePublication
	}{
		{
			name: "result envelope exiting zero",
			publication: MachinePublication{
				Command:  "status",
				Surface:  MachineSurfaceStdout,
				Result:   "StatusResult",
				Document: machineDocumentBytes("status", "[]", `"result":{"status_summary":"idle"}`),
			},
		},
		{
			name: "report-gated result envelope exiting non-zero",
			publication: MachinePublication{
				Command:  "validate",
				Surface:  MachineSurfaceStdout,
				Result:   "ValidateResult",
				ExitCode: 1,
				Document: machineDocumentBytes("validate", "[]", `"result":{"valid":false}`),
			},
		},
		{
			name: "error envelope exiting non-zero",
			publication: MachinePublication{
				Command:  "status",
				Surface:  MachineSurfaceStdout,
				ExitCode: 1,
				Document: machineErrorDocument("status", "not_initialized"),
			},
		},
		{
			name: "contractual warning alongside a result",
			publication: MachinePublication{
				Command: "next",
				Surface: MachineSurfaceStdout,
				Result:  "NextResult",
				Document: machineDocumentBytes("next",
					`[{"code":"local_initialized","message":"local","storage_mode":"local","storage_root":".taskrail/local"}]`,
					`"result":{"task_id":"T-269"}`),
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := CheckMachinePublication(tc.publication); err != nil {
				t.Fatalf("contractual publication was rejected: %v", err)
			}
		})
	}
}

func TestCheckMachinePublicationRejectsDrift(t *testing.T) {
	cases := []struct {
		name        string
		publication MachinePublication
		wantErr     string
	}{
		{
			name: "result shape outside the command's contract",
			publication: MachinePublication{
				Command:  "status",
				Surface:  MachineSurfaceStdout,
				Result:   "StatsResult",
				Document: machineDocumentBytes("status", "[]", `"result":{}`),
			},
			wantErr: `command "status" publishes result shape "StatsResult", which its contract does not name`,
		},
		{
			name: "result envelope naming no shape",
			publication: MachinePublication{
				Command:  "status",
				Surface:  MachineSurfaceStdout,
				Document: machineDocumentBytes("status", "[]", `"result":{}`),
			},
			wantErr: `command "status" publishes a result envelope naming no result shape`,
		},
		{
			name: "warning outside the command's subset",
			publication: MachinePublication{
				Command: "status",
				Surface: MachineSurfaceStdout,
				Result:  "StatusResult",
				Document: machineDocumentBytes("status",
					`[{"code":"verify_pass_before_complete","message":"order","task_id":"T-269","status":"todo","expected_status":"completed"}]`,
					`"result":{}`),
			},
			wantErr: `command "status" publishes warning "verify_pass_before_complete", which its contract does not allow`,
		},
		{
			name: "error code outside the command's subset",
			publication: MachinePublication{
				Command:  "status",
				Surface:  MachineSurfaceStdout,
				ExitCode: 1,
				Document: machineErrorDocument("status", "task_not_found"),
			},
			wantErr: `command "status" publishes error "task_not_found", which its contract does not allow`,
		},
		{
			name: "result envelope exiting non-zero without a gating contract",
			publication: MachinePublication{
				Command:  "status",
				Surface:  MachineSurfaceStdout,
				Result:   "StatusResult",
				ExitCode: 1,
				Document: machineDocumentBytes("status", "[]", `"result":{}`),
			},
			wantErr: `command "status" exits 1 with a result envelope, which its contract never gates`,
		},
		{
			name: "error envelope exiting zero",
			publication: MachinePublication{
				Command:  "status",
				Surface:  MachineSurfaceStdout,
				Document: machineErrorDocument("status", "not_initialized"),
			},
			wantErr: `command "status" exits 0 with an error envelope`,
		},
		{
			name: "error envelope naming a result shape",
			publication: MachinePublication{
				Command:  "status",
				Surface:  MachineSurfaceStdout,
				Result:   "StatusResult",
				ExitCode: 1,
				Document: machineErrorDocument("status", "not_initialized"),
			},
			wantErr: `command "status" publishes an error envelope naming result shape "StatusResult"`,
		},
		{
			name: "envelope command disagreeing with the publisher",
			publication: MachinePublication{
				Command:  "status",
				Surface:  MachineSurfaceStdout,
				Result:   "StatusResult",
				Document: machineDocumentBytes("stats", "[]", `"result":{}`),
			},
			wantErr: `command "status" publishes a document naming command "stats"`,
		},
		{
			name: "document the strict decoder rejects",
			publication: MachinePublication{
				Command:  "status",
				Surface:  MachineSurfaceStdout,
				Result:   "StatusResult",
				Document: machineDocumentBytes("status", "null", `"result":{}`),
			},
			wantErr: `command "status" publishes an invalid schema-1 document`,
		},
		{
			name: "command outside the machine API",
			publication: MachinePublication{
				Command:  "version",
				Surface:  MachineSurfaceStdout,
				Result:   "VersionResult",
				Document: machineDocumentBytes("version", "[]", `"result":{}`),
			},
			wantErr: `no schema-1 machine contract for "version stdout"`,
		},
		{
			name: "planned command publishing early",
			publication: MachinePublication{
				Command:  "task loop allow",
				Surface:  MachineSurfaceStdout,
				Result:   "LoopPolicyMutationResult",
				Document: machineDocumentBytes("task loop allow", "[]", `"result":{}`),
			},
			wantErr: `"task loop allow stdout" publishes no machine document yet`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckMachinePublication(tc.publication)
			if err == nil {
				t.Fatalf("drifting publication was accepted, want %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("publication error is %q, want it to contain %q", err, tc.wantErr)
			}
			if CheckMachinePublication(tc.publication).Error() != err.Error() {
				t.Fatal("publication diagnostics are not stable across runs")
			}
		})
	}
}
