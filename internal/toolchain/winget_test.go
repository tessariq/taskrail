package toolchain_test

import (
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// goReleaserConfig captures just the fields these guards assert on: the Windows
// build target, the per-OS archive format override that yields the zip winget
// needs, and the winget publisher block. Everything else in .goreleaser.yaml is
// left to `goreleaser check`.
type goReleaserConfig struct {
	Builds []struct {
		Goos []string `yaml:"goos"`
	} `yaml:"builds"`
	Archives []struct {
		FormatOverrides []struct {
			Goos    string   `yaml:"goos"`
			Formats []string `yaml:"formats"`
		} `yaml:"format_overrides"`
	} `yaml:"archives"`
	Winget []struct {
		PackageIdentifier string `yaml:"package_identifier"`
		Repository        struct {
			Owner       string `yaml:"owner"`
			Name        string `yaml:"name"`
			Token       string `yaml:"token"`
			PullRequest struct {
				Enabled bool `yaml:"enabled"`
				Base    struct {
					Owner string `yaml:"owner"`
					Name  string `yaml:"name"`
				} `yaml:"base"`
			} `yaml:"pull_request"`
		} `yaml:"repository"`
	} `yaml:"winget"`
}

func loadGoReleaser(t *testing.T) goReleaserConfig {
	t.Helper()
	var cfg goReleaserConfig
	if err := yaml.Unmarshal([]byte(readFile(t, repoRoot(t), ".goreleaser.yaml")), &cfg); err != nil {
		t.Fatalf("parse .goreleaser.yaml: %v", err)
	}
	return cfg
}

// The release pipeline must build the Windows target so WinGet has an artifact
// to publish (T-058, specs/v0.3.0.md#windows-distribution-via-winget).
func TestGoReleaserBuildsWindows(t *testing.T) {
	cfg := loadGoReleaser(t)
	if len(cfg.Builds) == 0 {
		t.Fatal(".goreleaser.yaml declares no builds")
	}
	for _, b := range cfg.Builds {
		if slices.Contains(b.Goos, "windows") {
			return
		}
	}
	t.Error(".goreleaser.yaml must build a windows target for WinGet distribution")
}

// WinGet requires a zip installer (zip+portable); GoReleaser only emits the
// winget manifest as such when the Windows archive is a zip, so the archive must
// carry a windows->zip format override rather than the default tar.gz.
func TestGoReleaserWindowsArchiveIsZip(t *testing.T) {
	cfg := loadGoReleaser(t)
	for _, a := range cfg.Archives {
		for _, o := range a.FormatOverrides {
			if o.Goos == "windows" && slices.Contains(o.Formats, "zip") {
				return
			}
		}
	}
	t.Error(".goreleaser.yaml must override the windows archive format to zip so winget ships zip+portable")
}

// The winget block must produce a Tessariq.Taskrail manifest and open a PR from
// the Tessariq/winget-pkgs fork against microsoft/winget-pkgs.
func TestGoReleaserWingetBlock(t *testing.T) {
	cfg := loadGoReleaser(t)
	// Exactly one entry: winget is a list and GoReleaser publishes every element,
	// so a stray/duplicate block would silently open a second, wrong PR.
	if len(cfg.Winget) != 1 {
		t.Fatalf(".goreleaser.yaml must declare exactly one winget block; got %d", len(cfg.Winget))
	}
	w := cfg.Winget[0]
	if w.PackageIdentifier != "Tessariq.Taskrail" {
		t.Errorf("winget package_identifier = %q, want Tessariq.Taskrail", w.PackageIdentifier)
	}
	if w.Repository.Owner != "Tessariq" || w.Repository.Name != "winget-pkgs" {
		t.Errorf("winget repository = %s/%s, want Tessariq/winget-pkgs", w.Repository.Owner, w.Repository.Name)
	}
	// The winget block must consume the same WINGET_TOKEN the workflow passes, so
	// a rename in one file without the other is caught here (default GITHUB_TOKEN
	// cannot open the cross-repo PR).
	if !strings.Contains(w.Repository.Token, "WINGET_TOKEN") {
		t.Errorf("winget repository.token = %q, want a {{ .Env.WINGET_TOKEN }} reference", w.Repository.Token)
	}
	if !w.Repository.PullRequest.Enabled {
		t.Error("winget repository.pull_request.enabled must be true to open the cross-repo PR")
	}
	if w.Repository.PullRequest.Base.Owner != "microsoft" || w.Repository.PullRequest.Base.Name != "winget-pkgs" {
		t.Errorf("winget PR base = %s/%s, want microsoft/winget-pkgs",
			w.Repository.PullRequest.Base.Owner, w.Repository.PullRequest.Base.Name)
	}
}

// workflowStepBlocks splits a workflow's steps into blocks, one per `- name:`
// directive: each block spans its `- name:` line up to (not including) the next
// one. This lets a guard assert that a key belongs to a *specific* step rather
// than appearing anywhere in the file.
func workflowStepBlocks(content string) [][]string {
	var blocks [][]string
	var cur []string
	started := false
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "- name:") {
			if started {
				blocks = append(blocks, cur)
			}
			cur = nil
			started = true
		}
		if started {
			cur = append(cur, line)
		}
	}
	if started {
		blocks = append(blocks, cur)
	}
	return blocks
}

// isGoReleaserPublishStep reports whether a step block runs GoReleaser to
// publish (not the `--snapshot` dry run): it uses goreleaser-action and its args
// do not carry `--snapshot`. This is the only step whose env must carry the
// cross-repo winget token.
func isGoReleaserPublishStep(block []string) bool {
	usesGoReleaser, snapshot := false, false
	for _, line := range block {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "goreleaser/goreleaser-action") {
			usesGoReleaser = true
		}
		if strings.Contains(trimmed, "--snapshot") {
			snapshot = true
		}
	}
	return usesGoReleaser && !snapshot
}

func blockMapsWingetSecret(block []string) bool {
	for _, line := range block {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		if i := strings.Index(trimmed, " #"); i >= 0 {
			trimmed = strings.TrimSpace(trimmed[:i])
		}
		if strings.HasPrefix(trimmed, "WINGET_TOKEN:") && strings.Contains(trimmed, "secrets.WINGET_TOKEN") {
			return true
		}
	}
	return false
}

// The default GITHUB_TOKEN cannot open a cross-repository PR, so the GoReleaser
// *publish* step (not the snapshot dry run) must receive a WINGET_TOKEN secret.
// Scoped to that step's block so WINGET_TOKEN attached to an unrelated step, or
// mentioned only in prose, cannot satisfy the guard.
func TestReleaseWorkflowPassesWingetToken(t *testing.T) {
	content := readFile(t, repoRoot(t), ".github/workflows/release.yml")
	for _, block := range workflowStepBlocks(content) {
		if !isGoReleaserPublishStep(block) {
			continue
		}
		if !blockMapsWingetSecret(block) {
			t.Error("GoReleaser publish step must map WINGET_TOKEN to secrets.WINGET_TOKEN in its env")
		}
		return
	}
	t.Error("release.yml has no GoReleaser publish step to carry WINGET_TOKEN")
}
