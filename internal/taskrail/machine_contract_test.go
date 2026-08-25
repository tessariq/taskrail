package taskrail

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

var (
	backtickedToken = regexp.MustCompile("`([^`]+)`")
	// warningCodeMember matches the discriminant of one companion warning
	// variant, whose value may list alternatives such as `a | b`.
	warningCodeMember = regexp.MustCompile(`code:\s*([^,}\n]+)`)
)

// backtickedTokens returns every `token` in s, in source order.
func backtickedTokens(s string) []string {
	var tokens []string
	for _, match := range backtickedToken.FindAllStringSubmatch(s, -1) {
		tokens = append(tokens, match[1])
	}
	return tokens
}

func readSpecFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{"..", ".."}, parts...)...)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func readCompanion(t *testing.T) string {
	t.Helper()
	return readSpecFile(t, "specs", "contracts", "v0.5.0-machine-api.md")
}

func readFeatureSpec(t *testing.T) string {
	t.Helper()
	return readSpecFile(t, "specs", "v0.5.0.md")
}

// specLine returns the single feature-spec bullet starting with prefix.
func specLine(t *testing.T, body, prefix string) string {
	t.Helper()
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
	t.Fatalf("specs/v0.5.0.md is missing the bullet %q", prefix)
	return ""
}

type companionRegistryRow struct {
	command string
	results []string
	nonzero string
}

// parseCompanionRegistry reads the co-normative command registry table so the
// inventory is compared against the companion's own bytes rather than a
// hand-copied restatement of it.
func parseCompanionRegistry(t *testing.T) []companionRegistryRow {
	t.Helper()
	lines := strings.Split(readCompanion(t), "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "| `command` | Result | Nonzero result |") {
			start = i + 2 // i is the header row, so +2 lands on the first data row
			break
		}
	}
	if start < 0 {
		t.Fatal("companion is missing the command registry table header")
	}
	var rows []companionRegistryRow
	for _, line := range lines[start:] {
		if !strings.HasPrefix(line, "|") {
			break
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) != 3 {
			t.Fatalf("registry row %q does not have three cells", line)
		}
		rows = append(rows, companionRegistryRow{
			command: strings.TrimSpace(cells[0]),
			results: backtickedTokens(strings.TrimSpace(cells[1])),
			nonzero: strings.TrimSpace(cells[2]),
		})
	}
	if len(rows) == 0 {
		t.Fatal("companion command registry table is empty")
	}
	return rows
}

func TestMachineInventoryMatchesCompanionRegistry(t *testing.T) {
	rows := parseCompanionRegistry(t)
	entries := MachineCommandInventory()
	if len(entries) != len(rows) {
		t.Fatalf("inventory has %d entries, companion registry has %d rows", len(entries), len(rows))
	}
	for i, row := range rows {
		entry := entries[i]
		if entry.CompanionRow != row.command {
			t.Errorf("entry %d row is %q, companion says %q", i, entry.CompanionRow, row.command)
			continue
		}
		if !reflect.DeepEqual(entry.Results, row.results) {
			t.Errorf("%s results are %v, companion says %v", entry.CompanionRow, entry.Results, row.results)
		}
		if entry.NonzeroResult != row.nonzero {
			t.Errorf("%s nonzero result is %q, companion says %q", entry.CompanionRow, entry.NonzeroResult, row.nonzero)
		}
		tokens := backtickedTokens(row.command)
		if len(tokens) == 0 {
			t.Errorf("companion row %q names no command path", row.command)
			continue
		}
		if entry.Command != tokens[0] {
			t.Errorf("%s canonical command is %q, companion says %q", entry.CompanionRow, entry.Command, tokens[0])
		}
	}
}

func TestMachineInventoryLoopSurfacesAreDistinguished(t *testing.T) {
	dryRun, ok := MachineCommandEntryFor("loop", MachineSurfaceStdout)
	if !ok {
		t.Fatal("inventory is missing the loop dry-run stdout entry")
	}
	if dryRun.NonzeroResult == "never" {
		t.Errorf("loop dry-run must keep its report-result exit exception, got %q", dryRun.NonzeroResult)
	}
	resultFile, ok := MachineCommandEntryFor("loop", MachineSurfaceResultFile)
	if !ok {
		t.Fatal("inventory is missing the loop result-file entry")
	}
	if !reflect.DeepEqual(resultFile.Results, []string{"LoopDiagnostic"}) {
		t.Errorf("loop result file results are %v, want [LoopDiagnostic]", resultFile.Results)
	}
	for _, code := range []string{"blocked_fail", "rework_fail", "completed_unverified", "completed_audit_fail", "child_failed", "no_progress", "invalid_postflight", "result_file_publish_failed"} {
		if !slices.Contains(resultFile.Errors, code) {
			t.Errorf("loop result file must name postflight code %q", code)
		}
	}
	if slices.Contains(dryRun.Errors, "invalid_postflight") {
		t.Error("loop dry-run must not claim postflight-only error codes")
	}
}

// parseSpecErrorRegistry reads the closed v0.5 error-code registry sentence.
func parseSpecErrorRegistry(t *testing.T) []string {
	t.Helper()
	return backtickedTokens(specLine(t, readFeatureSpec(t), "- The closed v0.5 error-code registry is"))
}

// parseCompanionWarningCodes reads the closed warning union from the companion's
// warning block, where each variant declares its `code:` alternatives.
func parseCompanionWarningCodes(t *testing.T) []string {
	t.Helper()
	_, rest, ok := strings.Cut(readCompanion(t), "\n## Warnings\n")
	if !ok {
		t.Fatal("companion is missing its warnings section")
	}
	block, _, ok := strings.Cut(rest, "\n## ")
	if !ok {
		t.Fatal("companion warnings section is unterminated")
	}
	var codes []string
	for _, match := range warningCodeMember.FindAllStringSubmatch(block, -1) {
		for _, alternative := range strings.Split(match[1], "|") {
			codes = append(codes, strings.TrimSpace(alternative))
		}
	}
	if len(codes) == 0 {
		t.Fatal("companion warnings section declares no codes")
	}
	return codes
}

func TestMachineRegistriesMatchNormativeSources(t *testing.T) {
	if got, want := sortedCopy(MachineErrorCodes()), sortedCopy(parseSpecErrorRegistry(t)); !reflect.DeepEqual(got, want) {
		t.Errorf("error registry is %v, spec says %v", got, want)
	}
	if got, want := sortedCopy(MachineWarningCodes()), sortedCopy(parseCompanionWarningCodes(t)); !reflect.DeepEqual(got, want) {
		t.Errorf("warning union is %v, companion says %v", got, want)
	}
}

func TestMachineInventoryIsClosedAndComplete(t *testing.T) {
	if err := ValidateMachineInventory(); err != nil {
		t.Fatalf("inventory is invalid: %v", err)
	}
	claimedErrors := map[string]bool{}
	claimedWarnings := map[string]bool{}
	for _, entry := range MachineCommandInventory() {
		for _, code := range entry.Errors {
			claimedErrors[code] = true
		}
		for _, code := range entry.Warnings {
			claimedWarnings[code] = true
		}
	}
	for _, code := range MachineErrorCodes() {
		if !claimedErrors[code] {
			t.Errorf("no inventory entry claims error code %q", code)
		}
	}
	for _, code := range MachineWarningCodes() {
		if !claimedWarnings[code] {
			t.Errorf("no inventory entry claims warning code %q", code)
		}
	}
}

// companionSection returns one "## <name>" section body of the companion.
func companionSection(t *testing.T, name string) string {
	t.Helper()
	_, rest, ok := strings.Cut(readCompanion(t), "\n## "+name+"\n")
	if !ok {
		t.Fatalf("companion is missing its %s section", name)
	}
	block, _, ok := strings.Cut(rest, "\n## ")
	if !ok {
		t.Fatalf("companion section %s is unterminated", name)
	}
	return block
}

// parseReusableErrorSets reads the companion's named error sets, whose right-hand
// sides may reference an earlier set by name.
func parseReusableErrorSets(t *testing.T, section string) map[string][]string {
	t.Helper()
	_, block, ok := strings.Cut(section, "Exact reusable error sets:\n\n```text\n")
	if !ok {
		t.Fatal("companion declares no reusable error sets")
	}
	block, _, ok = strings.Cut(block, "\n```")
	if !ok {
		t.Fatal("companion reusable error sets are unterminated")
	}
	sets := map[string][]string{}
	// A definition starts at column zero and may wrap onto indented lines, and
	// the companion separates some definitions by a single newline.
	var definitions []string
	for _, line := range strings.Split(block, "\n") {
		switch {
		case strings.TrimSpace(line) == "":
		case strings.HasPrefix(line, " ") && len(definitions) > 0:
			definitions[len(definitions)-1] += " " + strings.TrimSpace(line)
		default:
			definitions = append(definitions, strings.TrimSpace(line))
		}
	}
	for _, definition := range definitions {
		name, rhs, ok := strings.Cut(definition, " = ")
		if !ok {
			t.Fatalf("reusable set %q has no definition", definition)
		}
		sets[name] = expandErrorTokens(t, sets, strings.Split(rhs, "|"))
	}
	if len(sets) == 0 {
		t.Fatal("companion reusable error sets are empty")
	}
	return sets
}

// expandErrorTokens resolves each token to either a named set or a single code.
func expandErrorTokens(t *testing.T, sets map[string][]string, tokens []string) []string {
	t.Helper()
	var codes []string
	for _, token := range tokens {
		token = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(token), "or "))
		if token == "" {
			continue
		}
		if resolved, ok := sets[token]; ok {
			codes = append(codes, resolved...)
			continue
		}
		codes = append(codes, token)
	}
	if len(codes) == 0 {
		t.Fatalf("error token list %v resolves to no codes", tokens)
	}
	return codes
}

// machineDocument keys the assignment tables by the document an entry publishes,
// because the companion gives `loop` several rows across two surfaces.
type machineDocument struct {
	command string
	surface MachineSurface
}

// loopDocument routes the companion's qualified `loop` rows. Only the dry-run row
// describes stdout; execution preflight, publication, and postflight all describe
// the result file.
func loopDocument(row string) machineDocument {
	if strings.HasSuffix(row, "dry-run") {
		return machineDocument{"loop", MachineSurfaceStdout}
	}
	return machineDocument{"loop", MachineSurfaceResultFile}
}

// parseErrorAssignment reads the companion's exact (command, error.code) table.
func parseErrorAssignment(t *testing.T, section string, sets map[string][]string) map[machineDocument][]string {
	t.Helper()
	lines := strings.Split(section, "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, "| Commands | Allowed `error.code` set |") {
			start = i + 2 // i is the header row, so +2 lands on the first data row
			break
		}
	}
	if start < 0 {
		t.Fatal("companion is missing the command/error assignment table")
	}
	assignment := map[machineDocument][]string{}
	for _, line := range lines[start:] {
		if !strings.HasPrefix(line, "|") {
			break
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) != 2 {
			t.Fatalf("assignment row %q does not have two cells", line)
		}
		// A cell is either a set expression joined by `+` or a literal code list.
		cell := strings.ReplaceAll(strings.TrimSpace(cells[1]), "`", "")
		codes := expandErrorTokens(t, sets, strings.FieldsFunc(cell, func(r rune) bool { return r == '+' || r == ',' }))
		for _, label := range strings.Split(strings.TrimSpace(cells[0]), ",") {
			label = strings.TrimSpace(label)
			document := machineDocument{strings.Trim(label, "`"), MachineSurfaceStdout}
			if strings.HasPrefix(label, "`loop`") {
				document = loopDocument(label)
			}
			assignment[document] = append(assignment[document], codes...)
		}
	}
	if len(assignment) == 0 {
		t.Fatal("companion command/error assignment table is empty")
	}
	return assignment
}

func TestMachineInventoryErrorSubsetsMatchCompanionAssignment(t *testing.T) {
	section := companionSection(t, "Error Registry")
	assignment := parseErrorAssignment(t, section, parseReusableErrorSets(t, section))
	for _, entry := range MachineCommandInventory() {
		document := machineDocument{entry.Command, entry.Surface}
		want, ok := assignment[document]
		if !ok {
			t.Errorf("%s has no companion error assignment", entry.CompanionRow)
			continue
		}
		delete(assignment, document)
		if got := sortedCopy(entry.Errors); !reflect.DeepEqual(got, uniqueSorted(want)) {
			t.Errorf("%s errors are %v, companion assigns %v", entry.CompanionRow, got, uniqueSorted(want))
		}
	}
	for document := range assignment {
		t.Errorf("companion assigns error codes to %v with no inventory entry", document)
	}
}

// companionBullets splits a companion section into whole bullets, joining the
// continuation lines the source wraps.
func companionBullets(section string) []string {
	var bullets []string
	for _, line := range strings.Split(section, "\n") {
		switch {
		case strings.HasPrefix(line, "- "):
			bullets = append(bullets, strings.TrimPrefix(line, "- "))
		case strings.HasPrefix(line, "  ") && len(bullets) > 0:
			bullets[len(bullets)-1] += " " + strings.TrimSpace(line)
		}
	}
	return bullets
}

// parseWarningAssignment reads the companion's exact warning-applicability rules
// into the warning subset each published document may carry.
func parseWarningAssignment(t *testing.T, documents []machineDocument) map[machineDocument][]string {
	t.Helper()
	assignment := map[machineDocument][]string{}
	add := func(document machineDocument, codes ...string) {
		assignment[document] = append(assignment[document], codes...)
	}
	rules := 0
	for _, bullet := range companionBullets(companionSection(t, "Error Registry")) {
		codes := backtickedTokens(bullet)
		switch {
		case strings.HasPrefix(bullet, "Every registry command may emit"):
			rules++
			for _, document := range documents {
				add(document, codes...)
			}
		case strings.Contains(bullet, " limited to "):
			rules++
			scope, targets, _ := strings.Cut(bullet, " limited to ")
			limited := backtickedTokens(scope)
			for _, command := range backtickedTokens(targets) {
				for _, document := range documents {
					if document.command != command || !emitsLimitedWarning(document, command) {
						continue
					}
					add(document, limited...)
				}
			}
		case strings.Contains(bullet, "may accompany any initialized local-mode command except"):
			rules++
			_, exceptions, _ := strings.Cut(bullet, "except")
			excluded := backtickedTokens(exceptions)
			for _, document := range documents {
				if slices.Contains(excluded, document.command) {
					continue
				}
				add(document, codes[0])
			}
		}
	}
	if rules != 6 {
		t.Fatalf("companion warning assignment has %d rules, want 6", rules)
	}
	return assignment
}

// emitsLimitedWarning resolves the one command the companion scopes by form
// rather than by path: only executing `loop` bootstraps, and execution publishes
// the result file, so its preview surface is excluded.
func emitsLimitedWarning(document machineDocument, command string) bool {
	if command != "loop" {
		return true
	}
	return document.surface == MachineSurfaceResultFile
}

func TestMachineInventoryWarningSubsetsMatchCompanionAssignment(t *testing.T) {
	entries := MachineCommandInventory()
	documents := make([]machineDocument, 0, len(entries))
	for _, entry := range entries {
		documents = append(documents, machineDocument{entry.Command, entry.Surface})
	}
	assignment := parseWarningAssignment(t, documents)
	for _, entry := range entries {
		want := assignment[machineDocument{entry.Command, entry.Surface}]
		if got := sortedCopy(entry.Warnings); !reflect.DeepEqual(got, uniqueSorted(want)) {
			t.Errorf("%s warnings are %v, companion assigns %v", entry.CompanionRow, got, uniqueSorted(want))
		}
	}
}

func TestMachineInventoryClassifiesConstructedCommands(t *testing.T) {
	origins := map[string]MachineCommandOrigin{}
	for _, entry := range MachineCommandInventory() {
		origins[entry.CompanionRow] = entry.Origin
	}
	for _, row := range []string{"`validate`", "`status`", "`task new`", "`task author`", "`task loop allow`", "`task loop hold`", "`task loop clear`", "`task loop list`", "`spec activate`", "`local status`", "`local path`", "`prompt list`", "`prompt show`", "`prompt render`", "`review publish`"} {
		if origins[row] != MachineOriginConstructed {
			t.Errorf("%s is built by the current CLI, inventory says %q", row, origins[row])
		}
	}
	for _, row := range []string{"`local promote`"} {
		if origins[row] != MachineOriginPlanned {
			t.Errorf("%s is not built by the current CLI, inventory says %q", row, origins[row])
		}
	}
	if origins["`loop` dry-run"] != MachineOriginConstructed {
		t.Errorf("`loop` dry-run is built by the current CLI, inventory says %q", origins["`loop` dry-run"])
	}
}

func TestMachineInventoryIsDeterministicAndDefensivelyCopied(t *testing.T) {
	first := MachineCommandInventory()
	second := MachineCommandInventory()
	if !reflect.DeepEqual(first, second) {
		t.Fatal("inventory is not deterministic across calls")
	}
	first[0].Command = "mutated"
	first[0].Results[0] = "mutated"
	first[0].Errors[0] = "mutated"
	if third := MachineCommandInventory(); !reflect.DeepEqual(second, third) {
		t.Error("mutating a returned entry changed the inventory")
	}
	codes := MachineErrorCodes()
	codes[0] = "mutated"
	if !slices.Contains(MachineErrorCodes(), "invalid_arguments") {
		t.Error("mutating a returned registry slice changed the registry")
	}
}

func TestMachineCommandEntryForUnknownSurface(t *testing.T) {
	if _, ok := MachineCommandEntryFor("version", MachineSurfaceStdout); ok {
		t.Error("version is not a v0.5 JSON-capable command")
	}
	if _, ok := MachineCommandEntryFor("status", MachineSurfaceResultFile); ok {
		t.Error("only loop publishes a result file")
	}
	entry, ok := MachineCommandEntryFor("verify", MachineSurfaceStdout)
	if !ok {
		t.Fatal("verify must be inventoried")
	}
	if !slices.Contains(entry.Warnings, "verify_pass_before_complete") {
		t.Errorf("verify warnings are %v, want the verify-order warning", entry.Warnings)
	}
}

// decodeEnvelope is a representative strict consumer. It is written against the
// inventory alone: no implementation struct, example, or skill contributes to the
// decision, which is exactly the property later decoders and drift enforcement
// depend on.
func decodeEnvelope(command string, surface MachineSurface, document map[string]any) error {
	entry, ok := MachineCommandEntryFor(command, surface)
	if !ok {
		return fmt.Errorf("no schema-1 contract for %q on %s", command, surface)
	}
	for key := range document {
		switch key {
		case "schema_version", "command", "warnings", "result", "error":
		default:
			return fmt.Errorf("unknown top-level field %q", key)
		}
	}
	if version, ok := document["schema_version"].(float64); !ok || int(version) != MachineSchemaVersion {
		return fmt.Errorf("unsupported schema version %v", document["schema_version"])
	}
	if document["command"] != entry.Command {
		return fmt.Errorf("command is %v, want %q", document["command"], entry.Command)
	}
	warnings, ok := document["warnings"].([]any)
	if !ok {
		return errors.New("warnings must be a non-null array")
	}
	for _, warning := range warnings {
		code, _ := warning.(map[string]any)["code"].(string)
		if !slices.Contains(entry.Warnings, code) {
			return fmt.Errorf("warning %q is outside the %s subset", code, entry.Command)
		}
	}
	_, hasResult := document["result"]
	failure, hasError := document["error"].(map[string]any)
	if hasResult == hasError {
		return errors.New("exactly one of result or error is required")
	}
	if hasError {
		code, _ := failure["code"].(string)
		if !slices.Contains(entry.Errors, code) {
			return fmt.Errorf("error %q is outside the %s subset", code, entry.Command)
		}
	}
	return nil
}

func TestInventoryAloneDrivesStrictEnvelopeDecoding(t *testing.T) {
	cases := []struct {
		name     string
		command  string
		surface  MachineSurface
		document map[string]any
		wantErr  string
	}{
		{
			name:    "start success",
			command: "start",
			surface: MachineSurfaceStdout,
			document: map[string]any{
				"schema_version": float64(1),
				"command":        "start",
				"warnings":       []any{map[string]any{"code": "local_initialized"}},
				"result":         map[string]any{"task_id": "T-230"},
			},
		},
		{
			name:    "loop postflight failure on the result file",
			command: "loop",
			surface: MachineSurfaceResultFile,
			document: map[string]any{
				"schema_version": float64(1),
				"command":        "loop",
				"warnings":       []any{},
				"error":          map[string]any{"code": "blocked_fail"},
			},
		},
		{
			name:    "postflight code on an ordinary command",
			command: "start",
			surface: MachineSurfaceStdout,
			document: map[string]any{
				"schema_version": float64(1),
				"command":        "start",
				"warnings":       []any{},
				"error":          map[string]any{"code": "blocked_fail"},
			},
			wantErr: `error "blocked_fail" is outside the start subset`,
		},
		{
			name:    "verify-order warning on a command that cannot emit it",
			command: "status",
			surface: MachineSurfaceStdout,
			document: map[string]any{
				"schema_version": float64(1),
				"command":        "status",
				"warnings":       []any{map[string]any{"code": "verify_pass_before_complete"}},
				"result":         map[string]any{},
			},
			wantErr: `warning "verify_pass_before_complete" is outside the status subset`,
		},
		{
			name:    "unsupported schema version",
			command: "status",
			surface: MachineSurfaceStdout,
			document: map[string]any{
				"schema_version": float64(2),
				"command":        "status",
				"warnings":       []any{},
				"result":         map[string]any{},
			},
			wantErr: "unsupported schema version 2",
		},
		{
			name:    "result and error together",
			command: "status",
			surface: MachineSurfaceStdout,
			document: map[string]any{
				"schema_version": float64(1),
				"command":        "status",
				"warnings":       []any{},
				"result":         map[string]any{},
				"error":          map[string]any{"code": "invalid_arguments"},
			},
			wantErr: "exactly one of result or error is required",
		},
		{
			name:    "unknown top-level field",
			command: "status",
			surface: MachineSurfaceStdout,
			document: map[string]any{
				"schema_version": float64(1),
				"command":        "status",
				"warnings":       []any{},
				"result":         map[string]any{},
				"partial":        true,
			},
			wantErr: `unknown top-level field "partial"`,
		},
		{
			name:     "command outside the machine API",
			command:  "version",
			surface:  MachineSurfaceStdout,
			document: map[string]any{},
			wantErr:  `no schema-1 contract for "version" on stdout`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := decodeEnvelope(tc.command, tc.surface, tc.document)
			switch {
			case tc.wantErr == "" && err != nil:
				t.Fatalf("decode failed: %v", err)
			case tc.wantErr != "" && err == nil:
				t.Fatalf("decode accepted the document, want %q", tc.wantErr)
			case tc.wantErr != "" && err.Error() != tc.wantErr:
				t.Fatalf("decode error is %q, want %q", err, tc.wantErr)
			}
		})
	}
}

func uniqueSorted(values []string) []string {
	return slices.Compact(sortedCopy(values))
}

func sortedCopy(values []string) []string {
	out := slices.Clone(values)
	slices.Sort(out)
	return out
}
