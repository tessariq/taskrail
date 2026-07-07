package toolchain_test

import (
	"strings"
	"testing"
)

// ciActionUses returns the action reference from every `uses:` step directive in
// a workflow file, skipping YAML comment lines and trailing inline comments. It
// deliberately ignores prose so a historical comment mentioning an action cannot
// flip the mise-provisioning guard below (a bare substring search over the file
// would).
func ciActionUses(content string) []string {
	var uses []string
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		directive := strings.TrimPrefix(trimmed, "- ") // steps write `- uses:` or `uses:`
		if !strings.HasPrefix(directive, "uses:") {
			continue
		}
		ref := strings.TrimSpace(strings.TrimPrefix(directive, "uses:"))
		if i := strings.Index(ref, " #"); i >= 0 {
			ref = strings.TrimSpace(ref[:i])
		}
		uses = append(uses, ref)
	}
	return uses
}

// CI must provision its toolchain through mise (jdx/mise-action) so the pinned
// versions in mise.toml are the single source of truth for local and CI alike
// (specs/v0.2.0.md#mise-toolchain-management). A lingering actions/setup-go step
// would reintroduce a second, independently pinned Go version for CI, so its
// absence is asserted too — over actual `uses:` steps, not raw file text.
func TestCIProvisionsToolchainViaMise(t *testing.T) {
	root := repoRoot(t)
	uses := ciActionUses(readFile(t, root, ".github/workflows/ci.yml"))

	mise := false
	for _, ref := range uses {
		if strings.HasPrefix(ref, "jdx/mise-action") {
			mise = true
		}
		if strings.HasPrefix(ref, "actions/setup-go") {
			t.Errorf("ci.yml uses %q; mise is the single toolchain provisioner", ref)
		}
	}
	if !mise {
		t.Error("ci.yml must provision the toolchain via a jdx/mise-action step")
	}
}
