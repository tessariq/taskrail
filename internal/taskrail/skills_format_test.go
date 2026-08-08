package taskrail

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func skillDocument(frontmatter string) []byte {
	return []byte("---\n" + frontmatter + "---\n\n# Body\n")
}

func TestValidateAgentSkillFrontmatter(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{"required fields", "name: probe\ndescription: useful\n", ""},
		{"all optional fields", "name: probe\ndescription: useful\nlicense: MIT\ncompatibility: Taskrail v0.5\nmetadata:\n  owner: core\nallowed-tools: Bash Read\n", ""},
		{"missing name", "description: useful\n", "name"},
		{"non-string description", "name: probe\ndescription: [useful]\n", "description"},
		{"non-string metadata key", "name: probe\ndescription: useful\nmetadata:\n  1: owner\n", "metadata entries"},
		{"unknown top-level field", "name: probe\ndescription: useful\nargument-hint: '[task-id]'\n", "argument-hint"},
		{"legacy marker", "name: probe\ndescription: useful\ntaskrail_version: v0.4.0\n", "taskrail_version"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAgentSkill(skillDocument(tc.content))
			if tc.wantErr == "" && err != nil {
				t.Fatalf("validateAgentSkill: %v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("error = %v, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}

func TestPackagedAndCommittedSkillsAreAgentSkillsCompliant(t *testing.T) {
	for rel, content := range embeddedSkillFiles(t) {
		if err := validateAgentSkill([]byte(content)); err != nil {
			t.Errorf("embedded %s: %v", rel, err)
		}
		if strings.Contains(content, skillVersionKey) {
			t.Errorf("embedded %s contains a version marker", rel)
		}
	}

	for _, target := range committedSkillTargets {
		for rel := range embeddedSkillFiles(t) {
			data, err := os.ReadFile(filepath.Join(target, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatalf("read %s/%s: %v", target, rel, err)
			}
			if err := validateAgentSkill(data); err != nil {
				t.Errorf("%s/%s: %v", target, rel, err)
			}
			if strings.Contains(string(data), skillVersionKey) {
				t.Errorf("%s/%s contains a version marker", target, rel)
			}
		}
	}
}

func TestSkillVersionMarkerForms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		fm      string
		want    string
		wantErr string
	}{
		{"marker free", "name: probe\ndescription: useful\n", "", ""},
		{"nested", "name: probe\ndescription: useful\nmetadata:\n  taskrail_version: v0.5.0\n", "v0.5.0", ""},
		{"legacy", "name: probe\ndescription: useful\ntaskrail_version: v0.4.0\n", "v0.4.0", ""},
		{"matching dual", "name: probe\ndescription: useful\ntaskrail_version: v0.4.0\nmetadata:\n  taskrail_version: v0.4.0\n", "v0.4.0", ""},
		{"conflicting dual", "name: probe\ndescription: useful\ntaskrail_version: v0.4.0\nmetadata:\n  taskrail_version: v0.5.0\n", "", "conflicting"},
		{"empty nested", "name: probe\ndescription: useful\nmetadata:\n  taskrail_version: ''\n", "", "non-empty"},
		{"non-string legacy", "name: probe\ndescription: useful\ntaskrail_version: [v0.4.0]\n", "", "string"},
		{"duplicate legacy", "name: probe\ndescription: useful\ntaskrail_version: v0.4.0\ntaskrail_version: v0.5.0\n", "", "duplicate"},
		{"duplicate nested", "name: probe\ndescription: useful\nmetadata:\n  taskrail_version: v0.4.0\n  taskrail_version: v0.5.0\n", "", "duplicate"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := skillVersionOf(skillDocument(tc.fm))
			if tc.wantErr == "" && err != nil {
				t.Fatalf("skillVersionOf: %v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("error = %v, want it to mention %q", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("version = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSkillVersionMarkerRejectsConflictingCRLF(t *testing.T) {
	t.Parallel()

	data := skillDocument("name: probe\ndescription: useful\ntaskrail_version: v0.4.0\nmetadata:\n  taskrail_version: v0.5.0\n")
	data = []byte(strings.ReplaceAll(string(data), "\n", "\r\n"))
	if _, err := skillVersionOf(data); err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("skillVersionOf CRLF error = %v, want conflicting-marker refusal", err)
	}
}

func TestStampSkillVersionRejectsEmptyVersion(t *testing.T) {
	t.Parallel()

	if _, err := stampSkillVersion(skillDocument("name: probe\ndescription: useful\n"), " "); err == nil || !strings.Contains(err.Error(), "non-empty") {
		t.Fatalf("stampSkillVersion error = %v, want non-empty refusal", err)
	}
}

func TestStampSkillVersionPreservesBodyBytesAcrossLineEndings(t *testing.T) {
	t.Parallel()

	for _, newline := range []string{"\n", "\r\n", "\r"} {
		newline := newline
		t.Run(strings.ReplaceAll(newline, "\r", "CR"), func(t *testing.T) {
			body := []byte(newline + "# Body" + newline + "exact" + newline)
			data := append([]byte("---"+newline+"name: probe"+newline+"description: useful"+newline+"---"+newline), body...)
			got, err := stampSkillVersion(data, "v0.5.0")
			if err != nil {
				t.Fatalf("stampSkillVersion: %v", err)
			}
			if !strings.HasSuffix(string(got), string(body)) {
				t.Fatalf("stamp normalized body bytes:\n%q", got)
			}
		})
	}
}

func TestStampSkillVersionNormalizesAndPreservesMetadata(t *testing.T) {
	t.Parallel()

	input := skillDocument("name: probe\ndescription: useful\ntaskrail_version: v0.4.0\nmetadata:\n  owner: core\n  taskrail_version: v0.4.0\n")
	got, err := stampSkillVersion(input, "v0.5.0")
	if err != nil {
		t.Fatalf("stampSkillVersion: %v", err)
	}
	text := string(got)
	if strings.Contains(text, "\ntaskrail_version:") {
		t.Fatalf("legacy top-level marker remains:\n%s", text)
	}
	for _, want := range []string{"metadata:", "owner: core", "taskrail_version: v0.5.0", "# Body"} {
		if !strings.Contains(text, want) {
			t.Errorf("stamped skill missing %q:\n%s", want, text)
		}
	}
}
