package taskrail

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestRecoverySkillsShareInstalledProcedure(t *testing.T) {
	repo := initGitRepo(t)
	svc := newTestService(t, repo, time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC))
	if _, err := svc.Init(InitInput{WithSkills: true, SkillVersion: "test"}); err != nil {
		t.Fatal(err)
	}
	for _, target := range shippableSkillTargets {
		t.Run(target, func(t *testing.T) {
			entry := filepath.Join(repo, target, "autonomous-recovery", "SKILL.md")
			data, err := os.ReadFile(entry)
			if err != nil {
				t.Fatal(err)
			}
			body := string(data)
			link := regexp.MustCompile(`\[taskrail-repair\]\(([^)]+)\)`).FindStringSubmatch(body)
			if len(link) != 2 || link[1] != "../taskrail-repair/SKILL.md" {
				t.Fatalf("recovery must delegate to its installed sibling, got %v", link)
			}
			owner, err := os.ReadFile(filepath.Join(filepath.Dir(entry), filepath.FromSlash(link[1])))
			if err != nil {
				t.Fatalf("resolve installed canonical procedure: %v", err)
			}
			if !strings.Contains(body, "Read and follow") || strings.Contains(body, "} repair") {
				t.Error("recovery must delegate, not retain a second repair recipe")
			}
			for name, text := range map[string]string{"entry": body, "owner": string(owner)} {
				for _, want := range []string{"task taskrail:check", "never hand-edit", "do not implement unrelated product changes"} {
					if !strings.Contains(text, want) {
						t.Errorf("%s lost boundary %q", name, want)
					}
				}
			}
			flow := string(owner)
			last := -1
			for _, step := range []string{"**Inspect.**", "**Dry-run.**", "**Review.**", "**Apply.**", "**Re-validate.**"} {
				at := strings.Index(flow, step)
				if at <= last {
					t.Fatalf("canonical procedure missing or misorders %s", step)
				}
				last = at
			}
			for _, want := range []string{"stop", "ambiguous", "human", "only touches STATE.md", "never advances", "invent", "same task file"} {
				if !strings.Contains(flow, want) {
					t.Errorf("canonical procedure lost safety boundary %q", want)
				}
			}
		})
	}
}
