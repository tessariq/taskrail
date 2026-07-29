package toolchain_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGoReleaserVersionLdflagRetainsVPrefix(t *testing.T) {
	cfg := loadGoReleaser(t)
	if len(cfg.Builds) != 1 {
		t.Fatalf(".goreleaser.yaml must declare exactly one build; got %d", len(cfg.Builds))
	}
	if !hasExactVersionLdflag(cfg.Builds[0].Ldflags) {
		t.Errorf("GoReleaser ldflags = %q, want exactly one main.version assignment set to v{{.Version}}", cfg.Builds[0].Ldflags)
	}
}

func TestExactGoReleaserVersionLdflag(t *testing.T) {
	tests := []struct {
		name    string
		ldflags []string
		want    bool
	}{
		{name: "linker optimizations", ldflags: []string{"-s -w", "-X main.version=v{{.Version}}"}, want: true},
		{name: "suffix", ldflags: []string{"-s -w -X main.version=v{{.Version}}-dirty"}},
		{name: "prefix", ldflags: []string{"-s -w -X main.version=release-v{{.Version}}"}},
		{name: "conflicting duplicate", ldflags: []string{"-X main.version=v{{.Version}}", "-X main.version=v{{.Version}}-dirty"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasExactVersionLdflag(tt.ldflags); got != tt.want {
				t.Errorf("hasExactVersionLdflag(%q) = %v, want %v", tt.ldflags, got, tt.want)
			}
		})
	}
}

func hasExactVersionLdflag(ldflags []string) bool {
	found := false
	for _, ldflag := range ldflags {
		fields := strings.Fields(ldflag)
		for i := 0; i+1 < len(fields); i++ {
			if fields[i] != "-X" || !strings.HasPrefix(fields[i+1], "main.version=") {
				continue
			}
			if found || fields[i+1] != "main.version=v{{.Version}}" {
				return false
			}
			found = true
		}
	}
	return found
}

func TestChangelogReleaseGuards(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("release workflow and scripts run on Linux")
	}

	tests := []struct {
		name      string
		changelog string
		wantNotes string
		wantOK    bool
	}{
		{
			name: "missing section",
			changelog: "# Changelog\n\n" +
				"## v0.3.0 - 2026-07-14\n\nOlder notes.\n",
		},
		{
			name: "empty section",
			changelog: "# Changelog\n\n" +
				"## v0.4.0 - 2026-07-29\n\n\n## v0.3.0 - 2026-07-14\n\nOlder notes.\n",
		},
		{
			name: "whitespace-only section",
			changelog: "# Changelog\n\n" +
				"## v0.4.0 - 2026-07-29\n \t\n\t\n## v0.3.0 - 2026-07-14\n\nOlder notes.\n",
		},
		{
			name: "populated dated section",
			changelog: "# Changelog\n\n" +
				"## v0.4.0 - 2026-07-29\n \t\nRelease summary.\n \t\n- Fixed publishing.\n\t \n" +
				"## v0.3.0 - 2026-07-14\n\nOlder notes.\n",
			wantNotes: "Release summary.\n \t\n- Fixed publishing.\n",
			wantOK:    true,
		},
	}

	root := repoRoot(t)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			changelog := filepath.Join(t.TempDir(), "CHANGELOG.md")
			if err := os.WriteFile(changelog, []byte(tt.changelog), 0o600); err != nil {
				t.Fatal(err)
			}

			guard := exec.Command("bash", filepath.Join(root, "scripts/check-changelog-version.sh"), "v0.4.0", changelog)
			guardOutput, guardErr := guard.CombinedOutput()
			if (guardErr == nil) != tt.wantOK {
				t.Errorf("heading guard success = %v, want %v; output: %s", guardErr == nil, tt.wantOK, guardOutput)
			}

			extract := exec.Command("bash", filepath.Join(root, "scripts/changelog-release-notes.sh"), "v0.4.0", changelog)
			notes, extractErr := extract.Output()
			if (extractErr == nil) != tt.wantOK {
				t.Errorf("notes extraction success = %v, want %v", extractErr == nil, tt.wantOK)
			}
			if got := string(notes); got != tt.wantNotes {
				t.Errorf("release notes:\n%q\nwant:\n%q", got, tt.wantNotes)
			}
		})
	}
}

func TestChangelogWhitespaceTrimmingIsPortable(t *testing.T) {
	root := repoRoot(t)
	script, err := os.ReadFile(filepath.Join(root, "scripts/changelog-release-notes.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(script), `\s`) {
		t.Fatal(`changelog trimming must use POSIX character classes, not the non-portable \s escape`)
	}
}
