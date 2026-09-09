package taskrail

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

var skillEvalCaseID = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// skillEvalBaselineSkills is the v0.4.0 package at the fixed release commit.
// The evaluation registry cannot infer whether a case needs a baseline from the
// candidate tree because skills added after that release have no comparison arm.
var skillEvalBaselineSkills = map[string]bool{
	"autonomous-backlog":     true,
	"autonomous-manual-test": true,
	"autonomous-recovery":    true,
	"autonomous-task":        true,
	"autonomous-verify":      true,
	"taskrail-decompose":     true,
	"taskrail-gap":           true,
	"taskrail-import":        true,
	"taskrail-repair":        true,
	"taskrail-retrofit":      true,
	"taskrail-spec":          true,
}

// skillEvalBaselineSurfaces is the v0.4.0 command surface a baseline-required
// scenario may use: command path to its accepted flags, each mapped to whether
// the flag takes a value. Like the skill inventory above it is maintainer-owned
// rather than inferred, because the candidate tree cannot say what the released
// binary understood. It must stay truthful about absence: v0.4.0 answers an
// unknown subcommand by printing its parent's help and exiting zero, so a probe
// that reached past this set would grade `pass` while reading nothing at all.
// It registers only the paths the scenarios use: `task` also had rename and
// repoint in v0.4.0, but an unregistered path is refused loudly rather than
// accepted, so adding one is a deliberate act. `task show` arrived in v0.5.
var skillEvalBaselineSurfaces = map[string]map[string]bool{
	"init":     {"--json": false, "--with-skills": false, "--apply": false, "--force": false},
	"validate": {"--json": false},
	"task new": {
		"--json": false, "--title": true, "--slug": true, "--spec-ref": true,
		"--priority": true, "--dep": true, "--follow-up": true, "--area": true,
	},
}

// skillEvalBaselineExecutable reports whether the v0.4.0 binary can run the
// command. Git invocations are external to the release; the case validator has
// already established that a non-taskrail command is a git one. Registered paths
// take no positional argument, so the whole word sequence must name one: a
// trailing word would otherwise be dropped, and v0.4.0 refuses those as an
// unknown command.
func skillEvalBaselineExecutable(command []string) error {
	if len(command) == 0 || command[0] != "taskrail" {
		return nil
	}
	words, rest, err := skillEvalCommandWords(command[1:])
	if err != nil {
		return err
	}
	path := strings.Join(words, " ")
	accepted, known := skillEvalBaselineSurfaces[path]
	if !known {
		return fmt.Errorf("command %q is not in the v0.4.0 command surface", strings.Join(command, " "))
	}
	return skillEvalBaselineFlags(path, accepted, rest)
}

// skillEvalCommandWords splits the command path from the arguments that follow
// it. v0.4.0 registers every flag on its subcommand rather than on the root, so
// a flag ahead of the path is refused instead of skipped: which flags it would
// even belong to is unknowable before the path is read.
func skillEvalCommandWords(arguments []string) ([]string, []string, error) {
	words := []string{}
	for index, argument := range arguments {
		if argument == "--" {
			return words, arguments[index:], nil
		}
		if strings.HasPrefix(argument, "-") {
			if len(words) == 0 {
				return nil, nil, fmt.Errorf("flag %q precedes the command path, which v0.4.0 registers per subcommand", argument)
			}
			return words, arguments[index:], nil
		}
		words = append(words, argument)
	}
	return words, nil, nil
}

func skillEvalBaselineFlags(path string, accepted map[string]bool, arguments []string) error {
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		// Every registered path takes no positional argument, so operands after
		// the end-of-flags separator are refused exactly like bare ones.
		if argument == "--" {
			if index+1 < len(arguments) {
				return fmt.Errorf("positional argument %q of %q is not in the v0.4.0 command surface", arguments[index+1], path)
			}
			return nil
		}
		if !strings.HasPrefix(argument, "-") {
			return fmt.Errorf("positional argument %q of %q is not in the v0.4.0 command surface", argument, path)
		}
		name, _, hasInline := strings.Cut(argument, "=")
		takesValue, ok := accepted[name]
		if !ok {
			return fmt.Errorf("flag %q of %q is not in the v0.4.0 command surface", name, path)
		}
		if takesValue && !hasInline {
			index++
		}
	}
	return nil
}

// SkillEvalCase is the maintainer-owned deterministic input for one behavioral
// skill evaluation. It intentionally contains no provider or run result data.
type SkillEvalCase struct {
	CaseID               string
	Skill                string
	StorageMode          string
	BaselineRequired     bool
	Prompt               string
	ExpectedObservation  string
	Assertions           []string
	HumanReviewQuestions []string
	Scenario             SkillEvalScenario
	Oracle               SkillEvalOracle
	FixtureSHA256        string
	fixtureRoot          string
}

// SkillEvalScenario is an adapter-neutral recipe for an isolated evaluation.
// It gives a caller-owned adapter exact fixture, sandbox, and command inputs.
type SkillEvalScenario struct {
	Fixture string
	Sandbox string
	Setup   []SkillEvalScenarioAction
	Actions []SkillEvalScenarioAction
}

type SkillEvalScenarioAction struct {
	ID        string
	Operation string
	Command   []string
}

// SkillEvalOracle maps every authored assertion to one independently observed
// command predicate. Adapters provide facts, never assertion identities or grades.
type SkillEvalOracle struct {
	Assertions []SkillEvalAssertionOracle
}

type SkillEvalAssertionOracle struct {
	Assertion string
	Action    string
	Predicate string
}

func loadSkillEvalRegistry(root string, shippedSkills []string) ([]SkillEvalCase, error) {
	shipped := make(map[string]struct{}, len(shippedSkills))
	for _, skill := range shippedSkills {
		if skill == "" {
			return nil, fmt.Errorf("shipped skill name is empty")
		}
		if _, exists := shipped[skill]; exists {
			return nil, fmt.Errorf("shipped skill %q is duplicated", skill)
		}
		shipped[skill] = struct{}{}
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read skill evaluation registry: %w", err)
	}
	seenSkills := make(map[string]bool, len(entries))
	seenModes := make(map[string]map[string]bool, len(entries))
	seenCases := map[string]bool{}
	var registry []SkillEvalCase
	for _, entry := range entries {
		if !entry.IsDir() {
			return nil, fmt.Errorf("registry entry %q is not a skill directory", entry.Name())
		}
		skill := entry.Name()
		if _, ok := shipped[skill]; !ok {
			return nil, fmt.Errorf("registry contains unshipped skill %q", skill)
		}
		seenSkills[skill] = true
		caseEntries, err := os.ReadDir(filepath.Join(root, skill))
		if err != nil {
			return nil, fmt.Errorf("read cases for %s: %w", skill, err)
		}
		if len(caseEntries) == 0 {
			return nil, fmt.Errorf("shipped skill %q has no cases", skill)
		}
		for _, caseEntry := range caseEntries {
			if !caseEntry.IsDir() {
				return nil, fmt.Errorf("case entry %s/%s is not a directory", skill, caseEntry.Name())
			}
			caseRoot := filepath.Join(root, skill, caseEntry.Name())
			evaluation, err := loadSkillEvalCase(caseRoot)
			if err != nil {
				return nil, err
			}
			if evaluation.Skill != skill || evaluation.CaseID != caseEntry.Name() {
				return nil, fmt.Errorf("case %s/%s does not match its path", skill, caseEntry.Name())
			}
			if seenCases[evaluation.CaseID] {
				return nil, fmt.Errorf("case ID %q is duplicated", evaluation.CaseID)
			}
			seenCases[evaluation.CaseID] = true
			if seenModes[skill] == nil {
				seenModes[skill] = map[string]bool{}
			}
			seenModes[skill][evaluation.StorageMode] = true
			registry = append(registry, evaluation)
		}
	}
	for skill := range shipped {
		if !seenSkills[skill] {
			return nil, fmt.Errorf("shipped skill %q has no registry directory", skill)
		}
		for _, mode := range []string{"committed", "local"} {
			if !seenModes[skill][mode] {
				return nil, fmt.Errorf("shipped skill %q has no %s case", skill, mode)
			}
		}
	}
	slices.SortFunc(registry, func(a, b SkillEvalCase) int {
		if a.Skill == b.Skill {
			return strings.Compare(a.CaseID, b.CaseID)
		}
		return strings.Compare(a.Skill, b.Skill)
	})
	return registry, nil
}

func loadSkillEvalCase(root string) (SkillEvalCase, error) {
	data, err := os.ReadFile(filepath.Join(root, "case.json"))
	if err != nil {
		return SkillEvalCase{}, fmt.Errorf("read case %s: %w", root, err)
	}
	evaluation, err := parseSkillEvalCase(data)
	if err != nil {
		return SkillEvalCase{}, fmt.Errorf("parse case %s: %w", root, err)
	}
	if err := validateSkillEvalFixtureTree(root); err != nil {
		return SkillEvalCase{}, err
	}
	fixtureInfo, err := os.Stat(filepath.Join(root, evaluation.Scenario.Fixture))
	if err != nil {
		return SkillEvalCase{}, fmt.Errorf("read scenario fixture %s: %w", evaluation.Scenario.Fixture, err)
	}
	if !fixtureInfo.IsDir() {
		return SkillEvalCase{}, fmt.Errorf("scenario fixture %s is not a directory", evaluation.Scenario.Fixture)
	}
	seed, err := validateSkillEvalSeed(filepath.Join(root, evaluation.Scenario.Fixture, "seed.json"), evaluation.StorageMode)
	if err != nil {
		return SkillEvalCase{}, err
	}
	if !skillEvalScenarioUsesSeed(evaluation.Scenario, seed) {
		return SkillEvalCase{}, fmt.Errorf("scenario does not execute its fixture seed initialization")
	}
	digest, err := skillEvalTreeDigest("taskrail-skill-eval-case-v1", root)
	if err != nil {
		return SkillEvalCase{}, err
	}
	evaluation.FixtureSHA256 = digest
	evaluation.fixtureRoot = root
	return evaluation, nil
}

func parseSkillEvalCase(data []byte) (SkillEvalCase, error) {
	if err := checkDocumentFraming(data); err != nil {
		return SkillEvalCase{}, err
	}
	var raw json.RawMessage = data
	object, err := strictObject(raw, "case")
	if err != nil {
		return SkillEvalCase{}, err
	}
	if err := exactMembers(object, "case", []string{
		"schema_version", "case_id", "skill", "storage_mode", "baseline_required",
		"prompt", "expected_observation", "assertions", "human_review_questions", "scenario", "oracle",
	}); err != nil {
		return SkillEvalCase{}, err
	}
	schemaVersion, ok := decodeJSONInteger(object["schema_version"])
	if !ok || schemaVersion != 1 {
		return SkillEvalCase{}, fmt.Errorf("case member %q must be integer 1", "schema_version")
	}
	caseID, err := stringMember(object, "case", "case_id")
	if err != nil {
		return SkillEvalCase{}, err
	}
	if len(caseID) > 64 || !skillEvalCaseID.MatchString(caseID) {
		return SkillEvalCase{}, fmt.Errorf("case member %q is not a portable case ID", "case_id")
	}
	skill, err := stringMember(object, "case", "skill")
	if err != nil {
		return SkillEvalCase{}, err
	}
	storageMode, err := enumMember(object, "case", "storage_mode", []string{"committed", "local"})
	if err != nil {
		return SkillEvalCase{}, err
	}
	baselineRequired, err := boolMember(object, "case", "baseline_required")
	if err != nil {
		return SkillEvalCase{}, err
	}
	// Local storage is a v0.5 mode, so a v0.4.0 arm cannot even seed the
	// repository; those cases are candidate-only rather than missing a baseline.
	if baselineRequired != (skillEvalBaselineSkills[skill] && storageMode == "committed") {
		return SkillEvalCase{}, fmt.Errorf("case %q baseline requirement does not match the v0.4.0 skill and storage inventory", caseID)
	}
	prompt, err := stringMember(object, "case", "prompt")
	if err != nil {
		return SkillEvalCase{}, err
	}
	expectedObservation, err := stringMember(object, "case", "expected_observation")
	if err != nil {
		return SkillEvalCase{}, err
	}
	assertions, err := skillEvalStringArray(object, "assertions")
	if err != nil {
		return SkillEvalCase{}, err
	}
	questions, err := skillEvalStringArray(object, "human_review_questions")
	if err != nil {
		return SkillEvalCase{}, err
	}
	scenario, err := parseSkillEvalScenario(object["scenario"])
	if err != nil {
		return SkillEvalCase{}, err
	}
	oracle, err := parseSkillEvalOracle(object["oracle"])
	if err != nil {
		return SkillEvalCase{}, err
	}
	evaluation := SkillEvalCase{
		CaseID:               caseID,
		Skill:                skill,
		StorageMode:          storageMode,
		BaselineRequired:     baselineRequired,
		Prompt:               prompt,
		ExpectedObservation:  expectedObservation,
		Assertions:           assertions,
		HumanReviewQuestions: questions,
		Scenario:             scenario,
		Oracle:               oracle,
	}
	if err := validateSkillEvalCaseDefinition(evaluation); err != nil {
		return SkillEvalCase{}, err
	}
	if evaluation.BaselineRequired {
		for _, action := range slices.Concat(evaluation.Scenario.Setup, evaluation.Scenario.Actions) {
			if err := skillEvalBaselineExecutable(action.Command); err != nil {
				return SkillEvalCase{}, fmt.Errorf("case %q action %q: %w", caseID, action.ID, err)
			}
		}
	}
	return evaluation, nil
}

func parseSkillEvalScenario(raw json.RawMessage) (SkillEvalScenario, error) {
	object, err := strictObject(raw, "scenario")
	if err != nil {
		return SkillEvalScenario{}, err
	}
	if err := exactMembers(object, "scenario", []string{"fixture", "sandbox", "setup", "actions"}); err != nil {
		return SkillEvalScenario{}, err
	}
	fixture, err := stringMember(object, "scenario", "fixture")
	if err != nil {
		return SkillEvalScenario{}, err
	}
	sandbox, err := stringMember(object, "scenario", "sandbox")
	if err != nil {
		return SkillEvalScenario{}, err
	}
	setup, err := parseSkillEvalScenarioActions(object["setup"], "setup")
	if err != nil || len(setup) == 0 {
		return SkillEvalScenario{}, fmt.Errorf("scenario member %q must be non-empty", "setup")
	}
	actions, err := parseSkillEvalScenarioActions(object["actions"], "actions")
	if err != nil || len(actions) == 0 {
		return SkillEvalScenario{}, fmt.Errorf("scenario member %q must be non-empty", "actions")
	}
	return SkillEvalScenario{Fixture: fixture, Sandbox: sandbox, Setup: setup, Actions: actions}, nil
}

func parseSkillEvalScenarioActions(raw json.RawMessage, name string) ([]SkillEvalScenarioAction, error) {
	elements, err := arrayMember(raw, "scenario", name)
	if err != nil {
		return nil, err
	}
	actions := make([]SkillEvalScenarioAction, 0, len(elements))
	for _, element := range elements {
		action, err := strictObject(element, "scenario action")
		if err != nil {
			return nil, err
		}
		if err := exactMembers(action, "scenario action", []string{"id", "operation", "command"}); err != nil {
			return nil, err
		}
		id, err := stringMember(action, "scenario action", "id")
		if err != nil {
			return nil, err
		}
		operation, err := enumMember(action, "scenario action", "operation", []string{"git-command", "taskrail-command"})
		if err != nil {
			return nil, err
		}
		command, err := skillEvalStringArray(action, "command")
		if err != nil {
			return nil, err
		}
		actions = append(actions, SkillEvalScenarioAction{ID: id, Operation: operation, Command: command})
	}
	return actions, nil
}

func parseSkillEvalOracle(raw json.RawMessage) (SkillEvalOracle, error) {
	object, err := strictObject(raw, "oracle")
	if err != nil {
		return SkillEvalOracle{}, err
	}
	if err := exactMembers(object, "oracle", []string{"assertions"}); err != nil {
		return SkillEvalOracle{}, err
	}
	elements, err := arrayMember(object["assertions"], "oracle", "assertions")
	if err != nil || len(elements) == 0 {
		return SkillEvalOracle{}, fmt.Errorf("oracle member %q must be non-empty", "assertions")
	}
	assertions := make([]SkillEvalAssertionOracle, 0, len(elements))
	for _, element := range elements {
		assertion, err := strictObject(element, "oracle assertion")
		if err != nil {
			return SkillEvalOracle{}, err
		}
		if err := exactMembers(assertion, "oracle assertion", []string{"assertion", "action", "predicate"}); err != nil {
			return SkillEvalOracle{}, err
		}
		name, err := stringMember(assertion, "oracle assertion", "assertion")
		if err != nil {
			return SkillEvalOracle{}, err
		}
		action, err := stringMember(assertion, "oracle assertion", "action")
		if err != nil {
			return SkillEvalOracle{}, err
		}
		predicate, err := enumMember(assertion, "oracle assertion", "predicate", []string{"command-exit-zero", "taskrail-validation-pass", "git-worktree-clean", "git-publication-only"})
		if err != nil {
			return SkillEvalOracle{}, err
		}
		assertions = append(assertions, SkillEvalAssertionOracle{Assertion: name, Action: action, Predicate: predicate})
	}
	return SkillEvalOracle{Assertions: assertions}, nil
}

func skillEvalStringArray(object map[string]json.RawMessage, name string) ([]string, error) {
	elements, err := arrayMember(object[name], "case", name)
	if err != nil {
		return nil, err
	}
	if len(elements) == 0 {
		return nil, fmt.Errorf("case member %q is empty", name)
	}
	values := make([]string, 0, len(elements))
	seen := map[string]bool{}
	for _, element := range elements {
		var value string
		if err := json.Unmarshal(element, &value); err != nil || value == "" || !utf8.ValidString(value) {
			return nil, fmt.Errorf("case member %q contains a non-empty UTF-8 string", name)
		}
		if seen[value] {
			return nil, fmt.Errorf("case member %q repeats %q", name, value)
		}
		seen[value] = true
		values = append(values, value)
	}
	return values, nil
}

// skillEvalGitInternals reports whether an entry is a repository's own .git
// directory. An evaluation sandbox is a live Git repository whose internals Git
// rewrites on its own schedule: auto maintenance after a commit creates and then
// removes .git/objects/maintenance.lock, so a walk of .git races with Git and
// fails on a path that existed a moment earlier. Sandbox content is what a case
// claims; Git state is observed separately through git status digests. Fixture,
// skill, and registry trees contain no .git, so this changes nothing for them.
func skillEvalGitInternals(entry fs.DirEntry) bool {
	return entry.IsDir() && entry.Name() == ".git"
}

func validateSkillEvalFixtureTree(root string) error {
	var files []fs.FileInfo
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if skillEvalGitInternals(entry) {
			return fs.SkipDir
		}
		if entry.Type()&fs.ModeSymlink != 0 || !entry.Type().IsRegular() && !entry.IsDir() {
			return fmt.Errorf("fixture %s is not a regular file or directory", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel != "." && (!utf8.ValidString(rel) || strings.ContainsRune(rel, '\x00')) {
			return fmt.Errorf("fixture path %q is not valid UTF-8", rel)
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		for _, other := range files {
			if os.SameFile(info, other) {
				return fmt.Errorf("fixture %s aliases another regular file", path)
			}
		}
		files = append(files, info)
		return nil
	})
}

type skillEvalSeed struct {
	SchemaVersion int      `json:"schema_version"`
	StorageMode   string   `json:"storage_mode"`
	GitInit       bool     `json:"git_init"`
	TaskrailInit  []string `json:"taskrail_init"`
	Decoy         *struct {
		Path     string `json:"path"`
		Contents string `json:"contents"`
	} `json:"decoy,omitempty"`
	Provenance string `json:"provenance,omitempty"`
}

func validateSkillEvalSeed(path, mode string) (skillEvalSeed, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return skillEvalSeed{}, fmt.Errorf("read fixture seed: %w", err)
	}
	if err := checkDocumentFraming(data); err != nil {
		return skillEvalSeed{}, fmt.Errorf("decode fixture seed: %w", err)
	}
	var seed skillEvalSeed
	if err := json.Unmarshal(data, &seed); err != nil || seed.SchemaVersion != 1 || seed.StorageMode != mode || !seed.GitInit {
		return skillEvalSeed{}, fmt.Errorf("fixture seed %s is not a runnable %s repository seed", path, mode)
	}
	want := []string{"init", "--json"}
	if mode == "local" {
		want = []string{"init", "--local", "--json"}
		if seed.Decoy == nil || seed.Decoy.Path == "" || seed.Decoy.Contents == "" || seed.Provenance == "" {
			return skillEvalSeed{}, fmt.Errorf("local fixture seed %s lacks concrete decoy or provenance", path)
		}
	}
	if !slices.Equal(seed.TaskrailInit, want) {
		return skillEvalSeed{}, fmt.Errorf("fixture seed %s has unsupported Taskrail initialization", path)
	}
	return seed, nil
}

func skillEvalScenarioUsesSeed(scenario SkillEvalScenario, seed skillEvalSeed) bool {
	gitInitialized := false
	for _, setup := range scenario.Setup {
		if setup.Operation == "git-command" && slices.Equal(setup.Command, []string{"git", "init"}) {
			gitInitialized = true
			continue
		}
		if setup.Operation == "taskrail-command" && slices.Equal(setup.Command, append([]string{"taskrail"}, seed.TaskrailInit...)) {
			return gitInitialized
		}
	}
	return false
}

func skillEvalTreeDigest(domain, root string) (string, error) {
	if err := validateSkillEvalFixtureTree(root); err != nil {
		return "", err
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if skillEvalGitInternals(entry) {
			return fs.SkipDir
		}
		if entry.Type().IsRegular() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	slices.SortFunc(files, func(a, b string) int {
		aRel, _ := filepath.Rel(root, a)
		bRel, _ := filepath.Rel(root, b)
		return strings.Compare(filepath.ToSlash(aRel), filepath.ToSlash(bRel))
	})
	hash := sha256.New()
	hash.Write([]byte(domain))
	hash.Write([]byte{0})
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return "", err
		}
		rel, err := filepath.Rel(root, file)
		if err != nil {
			return "", err
		}
		hash.Write([]byte(filepath.ToSlash(rel)))
		hash.Write([]byte{0})
		hash.Write([]byte(strconv.Itoa(len(data))))
		hash.Write([]byte{0})
		hash.Write(data)
		hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}
