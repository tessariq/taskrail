package taskrail

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSkillEvalRegistryCoversEveryShippedSkill(t *testing.T) {
	registry, err := loadSkillEvalRegistry(filepath.Join("testdata", "skill-evals", "v1", "cases"), shippableSkills)
	if err != nil {
		t.Fatalf("loadSkillEvalRegistry: %v", err)
	}

	for _, skill := range shippableSkills {
		assertSkillEvalCoverage(t, registry, skill)
	}
}

func TestParseSkillEvalCaseRejectsStrictMutations(t *testing.T) {
	base := `{"schema_version":1,"case_id":"autonomous-task-committed","skill":"autonomous-task","storage_mode":"committed","baseline_required":true,"prompt":"run the documented workflow","expected_observation":"it remains valid","assertions":["uses JSON"],"scenario":{"fixture":"fixture","sandbox":"autonomous-task-committed","setup":[{"id":"initialize-git","operation":"git-command","command":["git","init"]}],"actions":[{"id":"uses-json","operation":"taskrail-command","command":["taskrail","validate","--json"]}]},"oracle":{"assertions":[{"assertion":"uses JSON","action":"uses-json","predicate":"command-exit-zero"}]},"human_review_questions":["was it safe?"]}`
	for _, tc := range []struct {
		name   string
		mutate func(string) string
		want   string
	}{
		{"unknown member", func(s string) string { return s[:len(s)-1] + `,"extra":true}` }, "unknown member"},
		{"duplicate member", func(s string) string { return strings.Replace(s, `"prompt":`, `"prompt":"first","prompt":`, 1) }, "repeats member"},
		{"empty assertions", func(s string) string { return strings.Replace(s, `["uses JSON"]`, `[]`, 1) }, "assertions"},
		{"duplicate assertion", func(s string) string { return strings.Replace(s, `["uses JSON"]`, `["uses JSON","uses JSON"]`, 1) }, "repeats"},
		{"wrong baseline", func(s string) string {
			return strings.Replace(s, `"baseline_required":true`, `"baseline_required":false`, 1)
		}, "baseline requirement"},
		{"wrong storage", func(s string) string {
			return strings.Replace(s, `"storage_mode":"committed"`, `"storage_mode":"other"`, 1)
		}, "allowed value"},
		{"missing scenario", func(s string) string {
			return strings.Replace(s, `,"scenario":{"fixture":"fixture","sandbox":"autonomous-task-committed","setup":[{"id":"initialize-git","operation":"git-command","command":["git","init"]}],"actions":[{"id":"uses-json","operation":"taskrail-command","command":["taskrail","validate","--json"]}]}`, "", 1)
		}, "missing member"},
		{"unsupported oracle predicate", func(s string) string { return strings.Replace(s, `"command-exit-zero"`, `"unknown"`, 1) }, "allowed value"},
		{"generic scenario", func(s string) string {
			return strings.Replace(s, `"sandbox":"autonomous-task-committed"`, `"sandbox":"case"`, 1)
		}, "incomplete executable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseSkillEvalCase([]byte(tc.mutate(base))); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("parseSkillEvalCase error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSkillEvalTreeDigestUsesDomainSeparatedFixtureBytes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("alpha"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "nested", "b.txt"), []byte("beta"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := skillEvalTreeDigest("taskrail-skill-eval-fixtures-v1", root)
	if err != nil {
		t.Fatalf("skillEvalTreeDigest: %v", err)
	}
	const want = "5db773a558c295a028a00d61ac340c02f821857b65c0e3fd27e1134819c391d2"
	if got != want {
		t.Fatalf("digest = %s, want %s", got, want)
	}
}

func TestSkillEvalRegistryRejectsRegistryMutations(t *testing.T) {
	for _, tc := range []struct {
		name   string
		setup  func(t *testing.T, root string)
		skills []string
		want   string
	}{
		{"missing local mode", func(t *testing.T, root string) {
			writeSkillEvalFixture(t, root, "autonomous-task", "committed", "task-committed", true, true)
		}, nil, "no local case"},
		{"path mismatch", func(t *testing.T, root string) {
			writeSkillEvalFixture(t, root, "autonomous-task", "committed", "other", true, true)
			writeSkillEvalFixture(t, root, "autonomous-task", "local", "task-local", true, true)
			path := filepath.Join(root, "autonomous-task", "other", "case.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(data), "other", "different")), 0o644); err != nil {
				t.Fatal(err)
			}
		}, nil, "does not match its path"},
		{"duplicate case ID", func(t *testing.T, root string) {
			writeSkillEvalFixture(t, root, "autonomous-task", "committed", "same", true, true)
			writeSkillEvalFixture(t, root, "autonomous-verify", "committed", "same", true, true)
		}, []string{"autonomous-task", "autonomous-verify"}, "duplicated"},
		{"missing local fixture", func(t *testing.T, root string) {
			writeSkillEvalFixture(t, root, "autonomous-task", "committed", "task-committed", true, true)
			writeSkillEvalFixture(t, root, "autonomous-task", "local", "task-local", true, false)
		}, nil, "lacks concrete decoy"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			tc.setup(t, root)
			skills := tc.skills
			if skills == nil {
				skills = []string{"autonomous-task"}
			}
			if _, err := loadSkillEvalRegistry(root, skills); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("loadSkillEvalRegistry error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSkillEvalTreeDigestRejectsHardLinkedFixtures(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "first.txt")
	if err := os.WriteFile(first, []byte("fixture"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(first, filepath.Join(root, "second.txt")); err != nil {
		t.Skipf("hard links unavailable: %v", err)
	}
	if _, err := skillEvalTreeDigest("taskrail-skill-eval-fixtures-v1", root); err == nil || !strings.Contains(err.Error(), "aliases") {
		t.Fatalf("skillEvalTreeDigest error = %v, want alias refusal", err)
	}
}

func writeSkillEvalFixture(t *testing.T, root, skill, mode, caseID string, baseline, localFixtures bool) {
	t.Helper()
	caseRoot := filepath.Join(root, skill, caseID)
	if err := os.MkdirAll(caseRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	init := `["taskrail","init","--json"]`
	if mode == "local" {
		init = `["taskrail","init","--local","--json"]`
	}
	caseJSON := `{"schema_version":1,"case_id":"` + caseID + `","skill":"` + skill + `","storage_mode":"` + mode + `","baseline_required":` + strconv.FormatBool(baseline) + `,"prompt":"run","expected_observation":"observe","assertions":["assert"],"scenario":{"fixture":"fixture","sandbox":"` + caseID + `","setup":[{"id":"initialize-git","operation":"git-command","command":["git","init"]},{"id":"initialize-taskrail","operation":"taskrail-command","command":` + init + `},{"id":"create-tracked-subject-task","operation":"taskrail-command","command":["taskrail","task","new","--title","Evaluation subject task","--json"]},{"id":"stage-seeded-repository","operation":"git-command","command":["git","add","--all"]},{"id":"commit-seeded-repository","operation":"git-command","command":["git","commit","--message","Seed"]}],"actions":[{"id":"assert","operation":"taskrail-command","command":["taskrail","validate","--json"]}]},"oracle":{"assertions":[{"assertion":"assert","action":"assert","predicate":"command-exit-zero"}]},"human_review_questions":["review?"]}`
	if err := os.WriteFile(filepath.Join(caseRoot, "case.json"), []byte(caseJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(caseRoot, "fixture"), 0o755); err != nil {
		t.Fatal(err)
	}
	seed := `{"schema_version":1,"storage_mode":"` + mode + `","git_init":true,"taskrail_init":["init","--json"]}` + "\n"
	if mode == "local" {
		seed = `{"schema_version":1,"storage_mode":"local","git_init":true,"taskrail_init":["init","--local","--json"],"decoy":{"path":"decoy/planning/STATE.md","contents":"fixture decoy"},"provenance":"fixture provenance"}` + "\n"
		if !localFixtures {
			seed = `{"schema_version":1,"storage_mode":"local","git_init":true,"taskrail_init":["init","--local","--json"]}` + "\n"
		}
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "fixture", "seed.json"), []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}
	if mode != "local" || !localFixtures {
		return
	}
	decoy := filepath.Join(caseRoot, "fixture", "decoy", "planning", "STATE.md")
	if err := os.MkdirAll(filepath.Dir(decoy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(decoy, []byte("decoy"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(caseRoot, "fixture", "git-provenance.txt"), []byte("Git"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertSkillEvalCoverage(t *testing.T, registry []SkillEvalCase, skill string) {
	t.Helper()
	seen := map[string]bool{}
	for _, evaluation := range registry {
		if evaluation.Skill != skill {
			continue
		}
		seen[evaluation.StorageMode] = true
	}
	for _, mode := range []string{"committed", "local"} {
		if !seen[mode] {
			t.Errorf("%s has no %s case", skill, mode)
		}
	}
}

// skillEvalClaimedStateCase is the canonical shape a scenario must have to match
// the state its prompt claims: a real commit before agent execution and a
// concrete tracked-work subject to exercise.
const skillEvalClaimedStateCase = `{"schema_version":1,"case_id":"autonomous-task-committed","skill":"autonomous-task","storage_mode":"committed","baseline_required":true,"prompt":"run the documented workflow","expected_observation":"it remains valid","assertions":["uses JSON"],"scenario":{"fixture":"fixture","sandbox":"autonomous-task-committed","setup":[{"id":"initialize-git-worktree","operation":"git-command","command":["git","init"]},{"id":"initialize-taskrail-repository","operation":"taskrail-command","command":["taskrail","init","--json"]},{"id":"create-tracked-subject-task","operation":"taskrail-command","command":["taskrail","task","new","--title","Evaluation subject task","--json"]},{"id":"stage-seeded-repository","operation":"git-command","command":["git","add","--all"]},{"id":"commit-seeded-repository","operation":"git-command","command":["git","commit","--message","Seed"]}],"actions":[{"id":"uses-json","operation":"taskrail-command","command":["taskrail","validate","--json"]}]},"oracle":{"assertions":[{"assertion":"uses JSON","action":"uses-json","predicate":"command-exit-zero"}]},"human_review_questions":["was it safe?"]}`

func TestParseSkillEvalCaseRequiresClaimedScenarioState(t *testing.T) {
	if _, err := parseSkillEvalCase([]byte(skillEvalClaimedStateCase)); err != nil {
		t.Fatalf("parseSkillEvalCase on a complete scenario: %v", err)
	}
	// `git commit -n` skips hooks and still commits, so it must stay accepted.
	hookless := strings.Replace(skillEvalClaimedStateCase, `["git","commit","--message","Seed"]`, `["git","commit","-n","--message","Seed"]`, 1)
	if _, err := parseSkillEvalCase([]byte(hookless)); err != nil {
		t.Fatalf("parseSkillEvalCase on a hook-skipping commit: %v", err)
	}
	for _, tc := range []struct {
		name   string
		mutate func(string) string
		want   string
	}{
		{"unborn committed repository", func(s string) string {
			return strings.Replace(s, `,{"id":"stage-seeded-repository","operation":"git-command","command":["git","add","--all"]},{"id":"commit-seeded-repository","operation":"git-command","command":["git","commit","--message","Seed"]}`, "", 1)
		}, "clean committed HEAD"},
		{"commit that creates nothing", func(s string) string {
			return strings.Replace(s, `["git","commit","--message","Seed"]`, `["git","commit","--dry-run"]`, 1)
		}, "clean committed HEAD"},
		{"subject command that creates nothing", func(s string) string {
			return strings.Replace(s, `["taskrail","task","new","--title","Evaluation subject task","--json"]`, `["taskrail","task","new","--help"]`, 1)
		}, "tracked-work subject"},
		{"missing positive-path subject", func(s string) string {
			return strings.Replace(s, `{"id":"create-tracked-subject-task","operation":"taskrail-command","command":["taskrail","task","new","--title","Evaluation subject task","--json"]},`, "", 1)
		}, "tracked-work subject"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseSkillEvalCase([]byte(tc.mutate(skillEvalClaimedStateCase))); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("parseSkillEvalCase error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestSkillEvalTreeDigestIgnoresGitInternals(t *testing.T) {
	// An evaluation sandbox is a live Git repository. Git's auto maintenance
	// creates and removes files such as .git/objects/maintenance.lock in the
	// background after a commit, so walking .git races with Git itself and
	// observes bytes no case ever claimed. Worktree content is the subject here;
	// Git state is observed separately through git status digests.
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}
	before, err := skillEvalTreeDigest("taskrail-skill-eval-sandbox-v1", root)
	if err != nil {
		t.Fatalf("digest without a repository: %v", err)
	}
	objects := filepath.Join(root, ".git", "objects")
	if err := os.MkdirAll(objects, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(objects, "maintenance.lock"), []byte("lock"), 0o600); err != nil {
		t.Fatal(err)
	}
	after, err := skillEvalTreeDigest("taskrail-skill-eval-sandbox-v1", root)
	if err != nil {
		t.Fatalf("digest with a repository: %v", err)
	}
	if before != after {
		t.Fatalf("Git internals changed the sandbox digest: %s != %s", before, after)
	}
}
