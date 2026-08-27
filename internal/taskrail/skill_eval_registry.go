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
	FixtureSHA256        string
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
	if evaluation.StorageMode == "local" {
		for _, fixture := range []string{
			filepath.Join("fixture", "decoy", "planning", "STATE.md"),
			filepath.Join("fixture", "git-provenance.txt"),
		} {
			if _, err := os.ReadFile(filepath.Join(root, fixture)); err != nil {
				return SkillEvalCase{}, fmt.Errorf("read local fixture %s: %w", fixture, err)
			}
		}
	}
	digest, err := skillEvalTreeDigest("taskrail-skill-eval-case-v1", root)
	if err != nil {
		return SkillEvalCase{}, err
	}
	evaluation.FixtureSHA256 = digest
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
		"prompt", "expected_observation", "assertions", "human_review_questions",
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
	if baselineRequired != skillEvalBaselineSkills[skill] {
		return SkillEvalCase{}, fmt.Errorf("case %q baseline requirement does not match v0.4.0 skill inventory", caseID)
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
	return SkillEvalCase{
		CaseID:               caseID,
		Skill:                skill,
		StorageMode:          storageMode,
		BaselineRequired:     baselineRequired,
		Prompt:               prompt,
		ExpectedObservation:  expectedObservation,
		Assertions:           assertions,
		HumanReviewQuestions: questions,
	}, nil
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

func validateSkillEvalFixtureTree(root string) error {
	var files []fs.FileInfo
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
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

func skillEvalTreeDigest(domain, root string) (string, error) {
	if err := validateSkillEvalFixtureTree(root); err != nil {
		return "", err
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
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
