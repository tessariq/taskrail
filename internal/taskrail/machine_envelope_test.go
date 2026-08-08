package taskrail

import (
	"slices"
	"strings"
	"testing"
)

const (
	digestA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	digestB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
)

// successGolden carries every common success member a consumer reads: the
// version, the canonical command path, an ordered mixed warning array, and one
// command-owned result payload.
const successGolden = `{
  "schema_version": 1,
  "command": "task loop list",
  "warnings": [
    {
      "code": "selected_off_spec",
      "message": "selected task is off the active spec",
      "task_id": "T-001-example",
      "spec_ref": "specs/v0.4.0.md#area",
      "active_spec_path": "specs/v0.5.0.md"
    },
    {"code": "skill_version_skew", "message": "packaged skills differ"}
  ],
  "result": {"rows": [], "violations": []}
}`

// errorGolden carries every common error member, including one violation with a
// path and one without, all three snapshot path kinds, and a recovery reference.
const errorGolden = `{
  "schema_version": 1,
  "command": "verify",
  "warnings": [],
  "error": {
    "code": "write_conflict",
    "message": "managed state changed during publication",
    "details": {
      "applied": false,
      "violations": [
        {"code": "digest_mismatch", "message": "state digest changed", "path": "planning/STATE.md"},
        {"code": "digest_mismatch", "message": "task digest changed", "path": null},
        {"code": "stale_snapshot", "message": "snapshot predates the write", "path": null}
      ],
      "paths": ["planning/STATE.md", "planning/tasks/T-001.md"],
      "snapshots": [
        {
          "path_kind": "git",
          "path": "/repo/.git/info/exclude",
          "original_sha256": "` + digestA + `",
          "candidate_sha256": null,
          "current_sha256": "` + digestB + `"
        },
        {
          "path_kind": "managed",
          "path": "planning/STATE.md",
          "original_sha256": "` + digestA + `",
          "candidate_sha256": "` + digestB + `",
          "current_sha256": "` + digestB + `"
        },
        {
          "path_kind": "worktree",
          "path": ".claude/skills/taskrail-spec/SKILL.md",
          "original_sha256": null,
          "candidate_sha256": null,
          "current_sha256": null
        }
      ],
      "recovery": {"transaction_id": "tx-0001", "command": "verify", "phase": "rolling_back"}
    }
  }
}`

func TestDecodeMachineEnvelopeSuccessGolden(t *testing.T) {
	env, err := DecodeMachineEnvelope([]byte(successGolden))
	if err != nil {
		t.Fatalf("decode success golden: %v", err)
	}
	if env.SchemaVersion != MachineSchemaVersion {
		t.Fatalf("schema version = %d, want %d", env.SchemaVersion, MachineSchemaVersion)
	}
	if env.Command != "task loop list" {
		t.Fatalf("command = %q", env.Command)
	}
	if env.Error != nil {
		t.Fatalf("success envelope decoded an error: %+v", env.Error)
	}
	if got := strings.TrimSpace(string(env.Result)); !strings.HasPrefix(got, "{") {
		t.Fatalf("result = %q, want the exact result object bytes", got)
	}
	if len(env.Warnings) != 2 {
		t.Fatalf("warnings = %d, want 2", len(env.Warnings))
	}
	selection := env.Warnings[0]
	if selection.Code != "selected_off_spec" || selection.TaskID != "T-001-example" ||
		selection.SpecRef != "specs/v0.4.0.md#area" || selection.ActiveSpecPath != "specs/v0.5.0.md" {
		t.Fatalf("selection warning = %+v", selection)
	}
	if skew := env.Warnings[1]; skew.Code != "skill_version_skew" || skew.TaskID != "" {
		t.Fatalf("skill warning = %+v", skew)
	}
}

func TestDecodeMachineEnvelopeErrorGolden(t *testing.T) {
	env, err := DecodeMachineEnvelope([]byte(errorGolden))
	if err != nil {
		t.Fatalf("decode error golden: %v", err)
	}
	if env.Result != nil {
		t.Fatalf("error envelope decoded a result: %s", env.Result)
	}
	if env.Error == nil {
		t.Fatal("error envelope decoded no error")
	}
	if env.Error.Code != "write_conflict" || env.Error.Message == "" {
		t.Fatalf("error = %+v", env.Error)
	}
	details := env.Error.Details
	if details.Applied {
		t.Fatal("applied = true, want false")
	}
	if len(details.Violations) != 3 || len(details.Paths) != 2 || len(details.Snapshots) != 3 {
		t.Fatalf("details collections = %d/%d/%d", len(details.Violations), len(details.Paths), len(details.Snapshots))
	}
	if got := details.Violations[0]; got.Path == nil || *got.Path != "planning/STATE.md" {
		t.Fatalf("first violation path = %+v", got.Path)
	}
	if details.Violations[1].Path != nil {
		t.Fatalf("second violation path = %q, want null", *details.Violations[1].Path)
	}
	git := details.Snapshots[0]
	if git.PathKind != "git" || git.CandidateSHA256 != nil || git.OriginalSHA256 == nil || *git.OriginalSHA256 != digestA {
		t.Fatalf("git snapshot = %+v", git)
	}
	if details.Recovery == nil || details.Recovery.TransactionID != "tx-0001" ||
		details.Recovery.Command != "verify" || details.Recovery.Phase != "rolling_back" {
		t.Fatalf("recovery = %+v", details.Recovery)
	}
}

func TestDecodeMachineEnvelopeAcceptsEmptyCollections(t *testing.T) {
	doc := `{"schema_version":1,"command":"status","warnings":[],"error":{"code":"not_initialized",
		"message":"repository is not initialized","details":{"applied":false,"violations":[],
		"paths":[],"snapshots":[],"recovery":null}}}`
	env, err := DecodeMachineEnvelope([]byte(doc))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Error.Details.Recovery != nil || len(env.Warnings) != 0 {
		t.Fatalf("envelope = %+v", env)
	}
}

// TestDecodeMachineWarningVariants decodes each closed warning variant once, so
// no variant is only exercised through its rejections.
func TestDecodeMachineWarningVariants(t *testing.T) {
	cases := []struct {
		name    string
		warning string
		check   func(t *testing.T, w MachineWarning)
	}{
		{
			name:    "unknown_skill_version",
			warning: `{"code":"unknown_skill_version","message":"packaged version is unknown"}`,
		},
		{
			name:    "empty_derived_slug",
			warning: `{"code":"empty_derived_slug","message":"derived slug is empty","task_id":"T-001-example"}`,
			check: func(t *testing.T, w MachineWarning) {
				if w.TaskID != "T-001-example" {
					t.Fatalf("task_id = %q", w.TaskID)
				}
			},
		},
		{
			name: "local_initialized",
			warning: `{"code":"local_initialized","message":"local storage was initialized",
				"storage_mode":"local","storage_root":".taskrail/local"}`,
			check: func(t *testing.T, w MachineWarning) {
				if w.StorageMode != "local" || w.StorageRoot != ".taskrail/local" {
					t.Fatalf("storage = %q %q", w.StorageMode, w.StorageRoot)
				}
			},
		},
		{
			name: "local_head_drift all null",
			warning: `{"code":"local_head_drift","message":"local head drifted","origin_branch":null,
				"origin_head":null,"current_branch":null,"current_head":null}`,
			check: func(t *testing.T, w MachineWarning) {
				if w.OriginBranch != nil || w.CurrentHead != nil {
					t.Fatalf("drift = %+v", w)
				}
			},
		},
		{
			name: "local_head_drift populated",
			warning: `{"code":"local_head_drift","message":"local head drifted","origin_branch":"main",
				"origin_head":"` + digestA + `","current_branch":"main","current_head":"` + digestB + `"}`,
			check: func(t *testing.T, w MachineWarning) {
				if w.OriginBranch == nil || *w.OriginBranch != "main" {
					t.Fatalf("origin_branch = %+v", w.OriginBranch)
				}
			},
		},
		{
			name: "verify_pass_before_complete",
			warning: `{"code":"verify_pass_before_complete","message":"verify precedes complete",
				"task_id":"T-001-example","status":"in_progress","expected_status":"completed"}`,
			check: func(t *testing.T, w MachineWarning) {
				if w.Status != "in_progress" || w.ExpectedStatus != "completed" {
					t.Fatalf("status = %q/%q", w.Status, w.ExpectedStatus)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			doc := `{"schema_version":1,"command":"status","warnings":[` + tc.warning + `],"result":{}}`
			env, err := DecodeMachineEnvelope([]byte(doc))
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if len(env.Warnings) != 1 {
				t.Fatalf("warnings = %d", len(env.Warnings))
			}
			if tc.check != nil {
				tc.check(t, env.Warnings[0])
			}
		})
	}
}

func TestDecodeMachineEnvelopeRejects(t *testing.T) {
	cases := []struct {
		name string
		doc  string
		want string
	}{
		// Document framing.
		{"byte order mark", "\ufeff" + `{"schema_version":1,"command":"status","warnings":[],"result":{}}`, "byte order mark"},
		{"invalid utf-8", "{\"schema_version\":1,\"command\":\"status\",\"warnings\":[],\"result\":{\"a\":\"\xff\"}}", "valid UTF-8"},
		{"trailing value", `{"schema_version":1,"command":"status","warnings":[],"result":{}} {}`, "trailing value"},
		{"not an object", `[{"schema_version":1}]`, "not a JSON object"},
		{"empty document", ``, "not a JSON object"},
		{"duplicate envelope key", `{"schema_version":1,"command":"status","warnings":[],"warnings":[],"result":{}}`, `repeats member "warnings"`},
		{"duplicate nested key", `{"schema_version":1,"command":"status","warnings":[],"result":{"a":1,"a":2}}`, `repeats member "result.a"`},
		{
			"duplicate key inside an array element",
			violationsDoc(`[{"code":"c","message":"m","path":null,"code":"c"}]`),
			`repeats member "error.details.violations.code"`,
		},

		// Version.
		{"unsupported version", `{"schema_version":2,"command":"status","warnings":[],"result":{}}`, "unsupported schema version 2"},
		{"non-integer version", `{"schema_version":1.0,"command":"status","warnings":[],"result":{}}`, "is not an integer"},
		{"string version", `{"schema_version":"1","command":"status","warnings":[],"result":{}}`, "is not an integer"},
		{"missing version", `{"command":"status","warnings":[],"result":{}}`, `missing member "schema_version"`},

		// Envelope membership and exclusivity.
		{"unknown member", `{"schema_version":1,"command":"status","warnings":[],"result":{},"extra":1}`, `unknown member "extra"`},
		{"missing command", `{"schema_version":1,"warnings":[],"result":{}}`, `missing member "command"`},
		{"missing warnings", `{"schema_version":1,"command":"status","result":{}}`, `missing member "warnings"`},
		{"result and error", `{"schema_version":1,"command":"status","warnings":[],"result":{},"error":null}`, `both "result" and "error"`},
		{"neither result nor error", `{"schema_version":1,"command":"status","warnings":[]}`, `neither "result" nor "error"`},
		{"null result", `{"schema_version":1,"command":"status","warnings":[],"result":null}`, `"result" is null`},
		{"array result", `{"schema_version":1,"command":"status","warnings":[],"result":[]}`, `"result" is not a JSON object`},
		{"null warnings", `{"schema_version":1,"command":"status","warnings":null,"result":{}}`, `"warnings" is null`},
		{"object warnings", `{"schema_version":1,"command":"status","warnings":{},"result":{}}`, `"warnings" is not an array`},

		// Canonical command path.
		{"command with flag", `{"schema_version":1,"command":"task loop list --json","warnings":[],"result":{}}`, "canonical command path"},
		{"command with executable", `{"schema_version":1,"command":"taskrail status","warnings":[],"result":{}}`, "canonical command path"},
		{"command with operand", `{"schema_version":1,"command":"start T-001","warnings":[],"result":{}}`, "canonical command path"},
		{"command double space", `{"schema_version":1,"command":"task  new","warnings":[],"result":{}}`, "canonical command path"},
		{"empty command", `{"schema_version":1,"command":"","warnings":[],"result":{}}`, `"command" is empty`},

		// Warnings.
		{"null warning", `{"schema_version":1,"command":"status","warnings":[null],"result":{}}`, "warning at index 0 is null"},
		{
			"unknown warning code",
			`{"schema_version":1,"command":"status","warnings":[{"code":"nearly_stale","message":"x"}],"result":{}}`,
			"not a registered warning code",
		},
		{
			"cross-variant warning member",
			`{"schema_version":1,"command":"status","warnings":[{"code":"skill_version_skew","message":"x","task_id":"T-001-example"}],"result":{}}`,
			`unknown member "task_id"`,
		},
		{
			"missing variant member",
			`{"schema_version":1,"command":"status","warnings":[{"code":"empty_derived_slug","message":"x"}],"result":{}}`,
			`missing member "task_id"`,
		},
		{
			"selection warning missing spec_ref",
			`{"schema_version":1,"command":"status","warnings":[{"code":"selected_off_spec","message":"x","task_id":"T-001-example","active_spec_path":"specs/v0.5.0.md"}],"result":{}}`,
			`missing member "spec_ref"`,
		},
		{
			"local_initialized wrong storage mode",
			`{"schema_version":1,"command":"next","warnings":[{"code":"local_initialized","message":"x","storage_mode":"committed","storage_root":".taskrail/local"}],"result":{}}`,
			`"storage_mode" must be "local"`,
		},
		{
			"local_initialized wrong storage root",
			`{"schema_version":1,"command":"next","warnings":[{"code":"local_initialized","message":"x","storage_mode":"local","storage_root":".taskrail"}],"result":{}}`,
			`"storage_root" must be ".taskrail/local"`,
		},
		{
			"head drift non-nullable shape",
			`{"schema_version":1,"command":"status","warnings":[{"code":"local_head_drift","message":"x","origin_branch":"main","origin_head":null,"current_branch":null}],"result":{}}`,
			`missing member "current_head"`,
		},
		{
			"verify order warning bad status",
			`{"schema_version":1,"command":"verify","warnings":[{"code":"verify_pass_before_complete","message":"x","task_id":"T-001-example","status":"completed","expected_status":"completed"}],"result":{}}`,
			`"status" is not an allowed value`,
		},
		{
			"verify order warning bad expected status",
			`{"schema_version":1,"command":"verify","warnings":[{"code":"verify_pass_before_complete","message":"x","task_id":"T-001-example","status":"todo","expected_status":"in_progress"}],"result":{}}`,
			`"expected_status" must be "completed"`,
		},
		{
			"empty warning message",
			`{"schema_version":1,"command":"status","warnings":[{"code":"skill_version_skew","message":""}],"result":{}}`,
			`"message" is empty`,
		},
		{
			"unsorted warnings",
			`{"schema_version":1,"command":"status","warnings":[{"code":"skill_version_skew","message":"x"},{"code":"empty_derived_slug","message":"y","task_id":"T-001-example"}],"result":{}}`,
			"warnings are not in contract order at index 1",
		},
		{
			"unsorted warning identifying fields",
			`{"schema_version":1,"command":"status","warnings":[{"code":"empty_derived_slug","message":"x","task_id":"T-002-example"},{"code":"empty_derived_slug","message":"y","task_id":"T-001-example"}],"result":{}}`,
			"warnings are not in contract order at index 1",
		},

		// Error envelope.
		{"null error", `{"schema_version":1,"command":"status","warnings":[],"error":null}`, `"error" is null`},
		{
			"unknown error code",
			errorDoc(`"code":"nearly_valid","message":"x","details":` + minimalDetails),
			"not a registered error code",
		},
		{
			"loop postflight code",
			errorDoc(`"code":"child_failed","message":"x","details":` + minimalDetails),
			"loop diagnostic",
		},
		{
			"unknown error member",
			errorDoc(`"code":"lock_held","message":"x","hint":"y","details":` + minimalDetails),
			`unknown member "hint"`,
		},
		{"missing details", errorDoc(`"code":"lock_held","message":"x"`), `missing member "details"`},
		{"null details", errorDoc(`"code":"lock_held","message":"x","details":null`), "details is null"},

		// Error details.
		{
			"missing recovery member",
			errorDoc(`"code":"lock_held","message":"x","details":{"applied":false,"violations":[],"paths":[],"snapshots":[]}`),
			`missing member "recovery"`,
		},
		{
			"null violations",
			errorDoc(`"code":"lock_held","message":"x","details":{"applied":false,"violations":null,"paths":[],"snapshots":[],"recovery":null}`),
			`"violations" is null`,
		},
		{
			"null paths",
			errorDoc(`"code":"lock_held","message":"x","details":{"applied":false,"violations":[],"paths":null,"snapshots":[],"recovery":null}`),
			`"paths" is null`,
		},
		{
			"null snapshots",
			errorDoc(`"code":"lock_held","message":"x","details":{"applied":false,"violations":[],"paths":[],"snapshots":null,"recovery":null}`),
			`"snapshots" is null`,
		},
		{
			"applied is not boolean",
			errorDoc(`"code":"lock_held","message":"x","details":{"applied":"false","violations":[],"paths":[],"snapshots":[],"recovery":null}`),
			`"applied" is not a boolean`,
		},
		{
			"null applied",
			errorDoc(`"code":"lock_held","message":"x","details":{"applied":null,"violations":[],"paths":[],"snapshots":[],"recovery":null}`),
			`"applied" is null`,
		},
		{
			"unsorted paths",
			pathsDoc(`["planning/tasks/T-001.md","planning/STATE.md"]`),
			"paths are not in contract order at index 1",
		},
		{
			"empty path",
			pathsDoc(`[""]`),
			"error path at index 0 is empty",
		},

		// Violations.
		{
			"violation unknown member",
			violationsDoc(`[{"code":"c","message":"m","path":null,"kind":"k"}]`),
			`unknown member "kind"`,
		},
		{
			"violation missing path member",
			violationsDoc(`[{"code":"c","message":"m"}]`),
			`missing member "path"`,
		},
		{
			"violation empty path",
			violationsDoc(`[{"code":"c","message":"m","path":""}]`),
			`"path" is empty`,
		},
		{
			"unsorted violations by code",
			violationsDoc(`[{"code":"b","message":"m","path":null},{"code":"a","message":"m","path":null}]`),
			"violations are not in contract order at index 1",
		},
		{
			"violations null path sorts last",
			violationsDoc(`[{"code":"a","message":"m","path":null},{"code":"a","message":"m","path":"planning/STATE.md"}]`),
			"violations are not in contract order at index 1",
		},
		{
			"unsorted violations by message",
			violationsDoc(`[{"code":"a","message":"n","path":null},{"code":"a","message":"m","path":null}]`),
			"violations are not in contract order at index 1",
		},

		// Snapshots.
		{
			"snapshot unknown path kind",
			snapshotsDoc(`[` + nullDigestSnapshot("local", "planning/STATE.md") + `]`),
			`"path_kind" is not an allowed value`,
		},
		{
			"git snapshot with relative path",
			snapshotsDoc(`[` + nullDigestSnapshot("git", ".git/info/exclude") + `]`),
			`canonical absolute path for path_kind "git"`,
		},
		{
			"managed snapshot with absolute path",
			snapshotsDoc(`[` + nullDigestSnapshot("managed", "/repo/planning/STATE.md") + `]`),
			`canonical relative path for path_kind "managed"`,
		},
		{
			"worktree snapshot with dot segment",
			snapshotsDoc(`[` + nullDigestSnapshot("worktree", ".claude/../.agents/skills") + `]`),
			`canonical relative path for path_kind "worktree"`,
		},
		{
			"upper-case digest",
			snapshotsDoc(`[{"path_kind":"managed","path":"planning/STATE.md","original_sha256":"` + strings.ToUpper(digestA) + `","candidate_sha256":null,"current_sha256":null}]`),
			`"original_sha256" is not a lower-case 64-hex digest`,
		},
		{
			"short digest",
			snapshotsDoc(`[{"path_kind":"managed","path":"planning/STATE.md","original_sha256":null,"candidate_sha256":"abc","current_sha256":null}]`),
			`"candidate_sha256" is not a lower-case 64-hex digest`,
		},
		{
			"snapshot missing digest member",
			snapshotsDoc(`[{"path_kind":"managed","path":"planning/STATE.md","original_sha256":null,"candidate_sha256":null}]`),
			`missing member "current_sha256"`,
		},
		{
			"unsorted snapshots by path kind",
			snapshotsDoc(`[{"path_kind":"managed","path":"planning/STATE.md","original_sha256":null,"candidate_sha256":null,"current_sha256":null},{"path_kind":"git","path":"/repo/.git/info/exclude","original_sha256":null,"candidate_sha256":null,"current_sha256":null}]`),
			"snapshots are not in contract order at index 1",
		},
		{
			"unsorted snapshots by path",
			snapshotsDoc(`[{"path_kind":"managed","path":"planning/tasks/T-001.md","original_sha256":null,"candidate_sha256":null,"current_sha256":null},{"path_kind":"managed","path":"planning/STATE.md","original_sha256":null,"candidate_sha256":null,"current_sha256":null}]`),
			"snapshots are not in contract order at index 1",
		},

		// Recovery.
		{
			"unknown recovery phase",
			recoveryDoc(`{"transaction_id":"tx-1","command":"verify","phase":"finished"}`),
			`"phase" is not an allowed value`,
		},
		{
			"recovery unknown member",
			recoveryDoc(`{"transaction_id":"tx-1","command":"verify","phase":"prepared","attempt":1}`),
			`unknown member "attempt"`,
		},
		{
			"recovery non-canonical command",
			recoveryDoc(`{"transaction_id":"tx-1","command":"verify --json","phase":"prepared"}`),
			"canonical command path",
		},
		{
			"recovery missing transaction id",
			recoveryDoc(`{"command":"verify","phase":"prepared"}`),
			`missing member "transaction_id"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env, err := DecodeMachineEnvelope([]byte(tc.doc))
			if err == nil {
				t.Fatalf("decoded a rejected document: %+v", env)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q does not contain %q", err, tc.want)
			}
			if env.SchemaVersion != 0 || env.Command != "" || env.Result != nil || env.Error != nil || env.Warnings != nil {
				t.Fatalf("rejection returned a partial document: %+v", env)
			}
		})
	}
}

// TestDecodeMachineEnvelopeAcceptsEveryRegisteredErrorCode keeps the decoder's
// accepted codes tied to the closed registry rather than a private copy.
func TestDecodeMachineEnvelopeAcceptsEveryRegisteredErrorCode(t *testing.T) {
	for _, code := range MachineErrorCodes() {
		doc := errorDoc(`"code":"` + code + `","message":"x","details":` + minimalDetails)
		_, err := DecodeMachineEnvelope([]byte(doc))
		if slices.Contains(loopPostflightErrors, code) {
			if err == nil || !strings.Contains(err.Error(), "loop diagnostic") {
				t.Fatalf("code %q: want a loop-diagnostic rejection, got %v", code, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("code %q: %v", code, err)
		}
	}
}

const minimalDetails = `{"applied":false,"violations":[],"paths":[],"snapshots":[],"recovery":null}`

func errorDoc(errorMembers string) string {
	return `{"schema_version":1,"command":"verify","warnings":[],"error":{` + errorMembers + `}}`
}

func detailsDoc(detailMembers string) string {
	return errorDoc(`"code":"lock_held","message":"x","details":{` + detailMembers + `}`)
}

// The one-collection helpers fill the minimal details skeleton, so a case shows
// only the bytes it exercises.
func violationsDoc(violations string) string {
	return detailsDoc(`"applied":false,"violations":` + violations + `,"paths":[],"snapshots":[],"recovery":null`)
}

func pathsDoc(paths string) string {
	return detailsDoc(`"applied":false,"violations":[],"paths":` + paths + `,"snapshots":[],"recovery":null`)
}

func snapshotsDoc(snapshots string) string {
	return detailsDoc(`"applied":false,"violations":[],"paths":[],"snapshots":` + snapshots + `,"recovery":null`)
}

func recoveryDoc(recovery string) string {
	return detailsDoc(`"applied":false,"violations":[],"paths":[],"snapshots":[],"recovery":` + recovery)
}

// nullDigestSnapshot is a snapshot whose digests are all null, so a path-class
// case shows only the path_kind and path it exercises.
func nullDigestSnapshot(pathKind, path string) string {
	return `{"path_kind":"` + pathKind + `","path":"` + path + `","original_sha256":null,"candidate_sha256":null,"current_sha256":null}`
}
