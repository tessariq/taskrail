package repotx

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// This file is the transaction's filesystem half: how one path is read, proven
// to be inside the repository it claims, and replaced atomically. Everything
// here is deliberately plain — the normal transaction promises nothing about
// abrupt death, so there is no journal, no fsync of parent directories, and no
// cleverness beyond stage-then-rename.

// fsCause unwraps a filesystem error to its underlying cause so a reported
// message names the path the caller already spelled rather than the absolute
// physical location a *fs.PathError carries. Classification through errors.Is
// and errors.As is unaffected.
func fsCause(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}

// resolveRoot is the repository root with every symlink resolved. Publication
// compares real directories against it, so a symlinked component cannot make a
// path that passed the lexical containment check land somewhere else.
func resolveRoot(root string) (string, error) {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", failure(KindOutsideRepository, nil,
			fmt.Errorf("resolve repository root %s: %w", root, err))
	}
	return resolved, nil
}

func inside(root, path string) bool {
	relative, err := filepath.Rel(root, filepath.Clean(path))
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// publishTo creates the candidate's parent directory and then proves the real
// directory — every symlink resolved — is still inside the locked repository.
// The lexical containment check at authorization cannot see a symlinked
// component, so without this a planted link would publish outside the
// repository. Git metadata is exempt for the same reason it is exempt there.
// Errors name the reported path, never the caller's absolute repository
// location, so a failure stays portable across machines (the T-088 contract).
func publishTo(root string, p Path, content []byte, mode fs.FileMode) error {
	directory := filepath.Dir(p.Physical)
	// Prove containment before creating anything: a planted link partway down the
	// path would otherwise have directories made through it, outside the
	// repository, before any check could refuse the write.
	if err := proveContained(root, p, directory); err != nil {
		return err
	}
	if testHookBeforeMkdir != nil {
		testHookBeforeMkdir(p)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return fmt.Errorf("create directory for %s: %w", p.Reported, fsCause(err))
	}
	// Prove it again, now that the directory itself resolves: the first check
	// could only see its nearest existing ancestor, so a link planted at the leaf
	// while the directories were being created is only visible here. It narrows
	// the race rather than closing it — the stage-and-rename that follows has a
	// window of its own, which a normal transaction does not promise to cover.
	if err := proveContained(root, p, directory); err != nil {
		return err
	}
	return writeFile(p.Reported, p.Physical, content, mode)
}

// proveContained refuses a directory that really lives outside the repository,
// following every symlink. The lexical containment check at authorization cannot
// see a symlinked component. Git metadata is exempt for the same reason it is
// exempt there: its canonical location is legitimately outside a worktree.
func proveContained(root string, p Path, directory string) error {
	if p.Kind == Git {
		return nil
	}
	resolved, err := existingAncestor(p.Reported, directory)
	if err != nil {
		return err
	}
	if !inside(root, resolved) {
		return failure(KindOutsideRepository, nil, fmt.Errorf(
			"%s path %q resolves through %s, outside repository %s",
			p.Kind, p.Reported, resolved, root))
	}
	return nil
}

// existingAncestor is the nearest ancestor of path that exists, with every
// symlink resolved. Resolving what is already there is the only way to judge a
// directory that has not been created yet.
func existingAncestor(reported, path string) (string, error) {
	for current := path; ; {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			return resolved, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", failure(KindUnreadable, nil,
				fmt.Errorf("resolve directory of %s: %w", reported, fsCause(err)))
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", failure(KindUnreadable, nil,
				fmt.Errorf("resolve directory of %s: no ancestor of it exists", reported))
		}
		current = parent
	}
}

// withEvidence attaches the per-path observations collected so far to a failure
// raised mid-scan, so a transaction that stopped on one unusable path still
// reports what it saw of the others.
func withEvidence(err error, entries []*entry) error {
	var txErr *Error
	if errors.As(err, &txErr) && txErr.snapshots == nil {
		txErr.snapshots = evidence(entries)
	}
	return err
}

// readState reads one path as the transaction is allowed to see it. An absent
// path is a legitimate state and reads as nil; a symlink or directory is not,
// because publishing through either would write somewhere other than the path
// the snapshot names.
func readState(path Path) (*fileState, error) {
	info, err := os.Lstat(path.Physical)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, failure(KindUnreadable, nil, fmt.Errorf("snapshot %s: %w", path.Reported, fsCause(err)))
	}
	if !info.Mode().IsRegular() {
		return nil, failure(KindNotRegularFile, nil,
			fmt.Errorf("%s path %q is not a regular file", path.Kind, path.Reported))
	}
	content, err := os.ReadFile(path.Physical)
	if err != nil {
		return nil, failure(KindUnreadable, nil, fmt.Errorf("snapshot %s: %w", path.Reported, fsCause(err)))
	}
	return &fileState{content: content, mode: info.Mode().Perm(), digest: digestOf(content)}, nil
}

// observe re-reads one path and records what it found. A read that failed leaves
// no current digest at all: "I could not read this" and "these exact bytes are
// there" are different claims, and keeping the previous phase's digest would
// publish the second while meaning the first.
func observe(e *entry) (*fileState, error) {
	state, err := readState(e.path)
	if err != nil {
		e.current = nil
		return nil, err
	}
	e.current = digestPointer(state)
	return state, nil
}

// sameState compares exactly what the machine contract reports: presence and
// content digest. Mode is deliberately not part of it — a snapshot carries no
// mode, so a mode-only difference could be neither reported as a conflict nor
// explained by the evidence, and rollback's own compare has always been
// digest-only.
func sameState(a, b *fileState) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.digest == b.digest
}

// writeFile replaces a path atomically: the bytes are complete and flushed in a
// private sibling before the rename makes them visible, so a reader sees either
// the whole previous file or the whole new one. reported is the portable
// spelling every error names; the staged sibling's own name is an
// implementation detail the caller never needs.
func writeFile(reported, physical string, content []byte, mode fs.FileMode) error {
	directory := filepath.Dir(physical)
	file, err := os.CreateTemp(directory, "."+filepath.Base(physical)+".repotx-*")
	if err != nil {
		return fmt.Errorf("stage %s: %w", reported, fsCause(err))
	}
	staged := file.Name()
	removeStaged := func() error { return fsCause(os.Remove(staged)) }
	if err := stage(file, content, mode); err != nil {
		return errors.Join(err, removeStaged())
	}
	if err := os.Rename(staged, physical); err != nil {
		return errors.Join(fmt.Errorf("publish %s: %w", reported, fsCause(err)), removeStaged())
	}
	return nil
}

func stage(file *os.File, content []byte, mode fs.FileMode) error {
	if _, err := file.Write(content); err != nil {
		file.Close()
		return fmt.Errorf("stage bytes: %w", fsCause(err))
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("sync staged bytes: %w", fsCause(err))
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close staged bytes: %w", fsCause(err))
	}
	if err := os.Chmod(file.Name(), mode); err != nil {
		return fmt.Errorf("stage mode: %w", fsCause(err))
	}
	return nil
}
