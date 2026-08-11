package taskrail

// The human-owned notes sidecar. `STATE.md` is a bounded machine-managed
// snapshot, so repository-wide human context needs somewhere else to live that
// Taskrail creates once and then never parses, rewrites, or reads back as state
// (specs/v0.5.0.md#layout-compatibility-and-upgrade). Every helper here takes the
// planning directory it should act in rather than deriving one from a repository
// root, so a caller holding a different storage context — the local overlay
// T-222 constructs — resolves the same no-clobber destination without a second
// implementation.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const notesFileName = "NOTES.md"

// notesImportedHeading labels the section a layout upgrade fills with the
// continuation notes it removes from state, so a reader can tell imported prose
// from anything the human wrote themselves.
const notesImportedHeading = "## Imported Continuation Notes"

func notesPath(planningDir string) string {
	return filepath.Join(planningDir, notesFileName)
}

// classifyNotesDestination reports whether a sidecar is already there, and
// refuses outright whenever the destination is not plainly absent-or-regular: a
// symlink or reparse point would publish the template somewhere else entirely, a
// directory or device cannot hold it, and a case-only sibling (`notes.md`) is
// the same entry as `NOTES.md` on a case-insensitive filesystem, so accepting it
// would make the outcome depend on which machine ran init. A caller therefore
// only ever sees the two states a no-clobber writer can act on. root only
// anchors the repo-relative paths named in errors (T-088).
func classifyNotesDestination(root, planningDir string) (bool, error) {
	if err := refuseNotesAlias(root, planningDir); err != nil {
		return false, err
	}

	path := notesPath(planningDir)
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", relPath(root, path), fsCause(err))
	}
	if !info.Mode().IsRegular() {
		return false, WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf(
			"notes sidecar %s is a %s, not a regular file; remove or move it before initializing",
			relPath(root, path), describeMode(info.Mode())))
	}
	return true, nil
}

// refuseNotesAlias rejects a sibling that differs from `NOTES.md` only by case.
// Comparison is ASCII case folding rather than full Unicode normalization: the
// filename is a fixed ASCII constant, so no other form of it can collide, and
// this keeps the check in the standard library.
func refuseNotesAlias(root, planningDir string) error {
	entries, err := os.ReadDir(planningDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read planning directory %s: %w", relPath(root, planningDir), fsCause(err))
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.EqualFold(name, notesFileName) && name != notesFileName {
			return WithMachineErrorCode(MachineCodePathBlocked, fmt.Errorf(
				"%s aliases the notes sidecar %s; rename it before initializing",
				relPath(root, filepath.Join(planningDir, name)), notesFileName))
		}
	}
	return nil
}

func describeMode(mode fs.FileMode) string {
	switch {
	case mode&fs.ModeSymlink != 0:
		return "symlink"
	case mode.IsDir():
		return "directory"
	default:
		return "special file"
	}
}

// testHookBeforeNotesCreate opens the window between classification and creation
// so a test can substitute the destination there. The exclusive create is the
// only thing standing between that substitution and a write through it, and
// nothing else can make that guarantee observable (repotx uses the same seam for
// its own publication race).
var testHookBeforeNotesCreate func(path string)

// ensureNotesTemplate creates the sidecar when, and only when, its destination
// is absent; an existing one is human-owned content left exactly as it is. O_EXCL
// is what makes the classification above it binding: it neither follows a symlink
// planted since nor truncates a file created since, so the no-clobber promise
// does not depend on nothing having changed in between. Losing that race means
// someone else's sidecar is there now, which is the outcome no-clobber wanted.
func ensureNotesTemplate(root, planningDir string) error {
	exists, err := classifyNotesDestination(root, planningDir)
	if err != nil || exists {
		return err
	}

	path := notesPath(planningDir)
	if testHookBeforeNotesCreate != nil {
		testHookBeforeNotesCreate(path)
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if errors.Is(err, fs.ErrExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("write %s: %w", relPath(root, path), fsCause(err))
	}
	if _, err := file.Write([]byte(starterNotes())); err != nil {
		file.Close()
		return fmt.Errorf("write %s: %w", relPath(root, path), fsCause(err))
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("write %s: %w", relPath(root, path), fsCause(err))
	}
	return nil
}

// notesExtractionCandidate builds the exact bytes an upgrade would publish when
// the operator chooses to keep legacy `continuation_notes` as human context. It
// writes nothing: the layout-2 transaction (T-157) publishes the candidate
// inside its own snapshot boundary, and validating here means an impossible
// extraction is refused before that transaction starts. Note text is copied
// verbatim in decoded order — Taskrail never reformats prose it is preserving.
func notesExtractionCandidate(root, planningDir string, notes []string) ([]byte, error) {
	if len(notes) == 0 {
		return nil, invalidArgumentsf("no continuation notes to extract into %s", notesFileName)
	}
	exists, err := classifyNotesDestination(root, planningDir)
	if err != nil {
		return nil, err
	}
	if exists {
		// Appending or replacing would put Taskrail inside a file it does not own,
		// so the merge is the operator's and the notes are dropped afterwards.
		return nil, WithMachineErrorCode(MachineCodeDestinationExists, fmt.Errorf(
			"notes sidecar %s already exists; merge the continuation notes manually, then re-run with --drop-continuation-notes",
			relPath(root, notesPath(planningDir))))
	}

	var builder strings.Builder
	builder.WriteString(starterNotes())
	builder.WriteString("\n" + notesImportedHeading + "\n\n")
	builder.WriteString("<!--\nImported verbatim from the removed `continuation_notes` state field, in the\norder Taskrail decoded them.\n-->\n")
	for i, note := range notes {
		fmt.Fprintf(&builder, "\n### Note %d\n\n%s\n", i+1, note)
	}
	return []byte(builder.String()), nil
}

// starterNotes is the short commented template a fresh layout gets. It says what
// the file is for, because it is the only place an operator who never read the
// documentation will encounter the rule.
func starterNotes() string {
	return `# Repository Notes

<!--
Human-owned repository context. Taskrail creates this file once when it is
absent, and never parses, rewrites, or reads it as state. Agents may read it,
but should edit it only when a human explicitly asks. Task-specific history and
evidence belong in task notes, blocker reasons, verification reports, or
follow-up tasks.
-->
`
}
