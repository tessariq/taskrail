package taskrail

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// The committed-mode artifact ignore is one marked block in the worktree-root
// .gitignore. Committed bytes are the only ignore state that travels with a
// clone, which is what the gitignored-artifacts contract requires
// (specs/v0.2.0.md "Portable Committed State"); local mode keeps its own
// marked block in .git/info/exclude instead.
const (
	artifactsIgnoreBegin = "# taskrail artifacts begin"
	artifactsIgnoreEnd   = "# taskrail artifacts end"
	gitignoreFile        = ".gitignore"
)

// artifactIgnorePlan is init's decision for the worktree-root .gitignore: the
// reported write-entry action, the candidate bytes when init appends, and
// whether the transaction can digest the file at all.
type artifactIgnorePlan struct {
	action    string
	original  []byte
	candidate []byte
	// bindable reports whether the init transaction may bind the file: an
	// absent or regular .gitignore can be digest-bound, while a linked or
	// special file cannot and only a no-write preserve may proceed unbound.
	bindable bool
}

// planArtifactIgnore decides how one committed-mode init outcome treats the
// worktree-root .gitignore. Only committed storage manages it — local mode
// keeps the worktree clean through its own .git/info/exclude block — and
// outside a Git worktree there is no ignore state at all, so both report no
// entry and write nothing. A .gitignore that already addresses the artifacts
// directory — through Taskrail's own block, an equivalent user rule, or a
// deliberate negation — is preserved byte-for-byte; otherwise init appends
// its marked block without touching unrelated user lines.
func (s *Service) planArtifactIgnore() (artifactIgnorePlan, error) {
	if s.paths.WorktreeRoot == "" || s.paths.Storage.Mode != StorageCommitted {
		return artifactIgnorePlan{}, nil
	}
	entry := s.artifactsIgnoreEntry()
	physical := filepath.Join(s.paths.WorktreeRoot, gitignoreFile)
	original, err := readGitignore(physical)
	if err != nil {
		return artifactIgnorePlan{}, err
	}
	if original != nil && gitignoreManagesArtifacts(string(original), entry) {
		return artifactIgnorePlan{action: writeActionPreserve, original: original, bindable: regularFile(physical)}, nil
	}
	// A non-regular .gitignore (a symlink, as in stow/dotfiles setups) is read
	// through for the decision above but never written through: appending
	// would modify its target, so init refuses and names the rule to add.
	if original != nil && !regularFile(physical) {
		return artifactIgnorePlan{}, WithMachineErrorCode(MachineCodePathBlocked,
			fmt.Errorf("%s is not a regular file: add the line %s to it by hand and re-run init", gitignoreFile, entry))
	}
	action := writeActionRefresh
	if original == nil {
		action = writeActionCreate
	}
	return artifactIgnorePlan{
		action:    action,
		original:  original,
		candidate: appendArtifactsIgnore(original, entry),
		bindable:  true,
	}, nil
}

// artifactsIgnoreEntry is the anchored .gitignore line for the configured
// artifacts directory, derived from the logical planning directory so a
// relocated planning tree is ignored at its real location.
func (s *Service) artifactsIgnoreEntry() string {
	return "/" + path.Join(s.paths.LogicalPlanningDir, "artifacts") + "/"
}

// readGitignore reads the worktree-root .gitignore for the ignore decision.
// Reading follows a symlink so an already-managed linked file still reports
// preserve; the write decision in planArtifactIgnore refuses before any
// non-regular file is rewritten. An absent file is no error.
func readGitignore(physical string) ([]byte, error) {
	data, err := os.ReadFile(physical)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf("read %s: %w", gitignoreFile, err))
	}
	return data, nil
}

func regularFile(path string) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode().IsRegular()
}

// gitignoreManagesArtifacts reports whether existing .gitignore content
// already addresses the artifacts directory: Taskrail's own block, an exact
// rule that ignores it, or an exact negation that deliberately re-includes
// it. In every one of those shapes init appends nothing — the last matching
// pattern wins in .gitignore, so a later append could only override an
// explicit user choice. Leading whitespace is significant to Git, so only
// trailing whitespace is ignored here, matching Git's own stripping.
func gitignoreManagesArtifacts(text, entry string) bool {
	equivalent := strings.Trim(entry, "/")
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimRight(raw, " \t")
		if line == artifactsIgnoreBegin || line == artifactsIgnoreEnd {
			return true
		}
		if strings.Trim(strings.TrimPrefix(line, "!"), "/") == equivalent {
			return true
		}
	}
	return false
}

// appendArtifactsIgnore appends the marked block after the existing bytes,
// inserting a separating newline only when user content lacks one so the last
// user line is never merged into the block and no user byte is dropped.
func appendArtifactsIgnore(original []byte, entry string) []byte {
	text := string(original)
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return []byte(text + artifactsIgnoreBegin + "\n" + entry + "\n" + artifactsIgnoreEnd + "\n")
}
