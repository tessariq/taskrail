package durablefs

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
)

// Publish atomically creates complete staged bytes without replacing an existing
// name. The final hard-link operation is the no-replace commit point.
func (r *Root) Publish(path string, content []byte, mode fs.FileMode) (*Entry, error) {
	if err := r.authorize(); err != nil {
		return nil, err
	}
	parent, leaf, ancestors, err := r.bindParent(path)
	if err != nil {
		return nil, err
	}
	if err := exactName(parent, leaf, false); err != nil {
		parent.Close()
		return nil, err
	}
	staged, file, stagedSnapshot, err := stage(parent, content, mode)
	if err != nil {
		parent.Close()
		return nil, err
	}
	runMutationHook("publish", path)
	if testHookBeforeStageCAS != nil {
		testHookBeforeStageCAS(parent, staged)
	}
	if err := checkStaged(parent, staged, stagedSnapshot); err != nil {
		err = cleanupFailure("publish", path, parent, staged, file, err)
		parent.Close()
		return nil, err
	}
	fresh, freshLeaf, freshAncestors, err := r.bindParent(path)
	if err != nil {
		err = cleanupFailure("publish", path, parent, staged, file, err)
		parent.Close()
		return nil, err
	}
	defer fresh.Close()
	if freshLeaf != leaf || !sameIdentities(ancestors, freshAncestors) {
		err := cleanupFailure("publish", path, parent, staged, file,
			fmt.Errorf("%w: ancestors changed before publishing %s", ErrConflict, path))
		parent.Close()
		return nil, err
	}
	if err := exactName(fresh, leaf, false); err != nil {
		err = cleanupFailure("publish", path, parent, staged, file, err)
		parent.Close()
		return nil, err
	}
	if err := file.Close(); err != nil {
		err = cleanupFailure("publish", path, parent, staged, file, err)
		parent.Close()
		return nil, err
	}
	if testHookBeforeLink != nil {
		testHookBeforeLink(parent, leaf)
	}
	if err := parent.Link(staged, leaf); err != nil {
		if cleanup := removeStage(parent, staged); cleanup != nil {
			err = errors.Join(err, &MutationError{Operation: "publish", Path: path, Staging: staged, Err: cleanup})
		}
		parent.Close()
		return nil, err
	}
	if err := removeStage(parent, staged); err != nil {
		parent.Close()
		return nil, &MutationError{Operation: "publish", Path: path, Staging: staged, Committed: true, Err: err}
	}
	if err := barrier(BarrierDirectory, func() error { return syncDirectory(parent) }); err != nil {
		parent.Close()
		return nil, &MutationError{Operation: "publish", Path: path, Committed: true, Err: err}
	}
	parent.Close()
	entry, err := r.Bind(path)
	if err != nil {
		return nil, &MutationError{Operation: "publish", Path: path, Committed: true, Err: err}
	}
	return entry, nil
}

func cleanupStage(parent *os.Root, name string, file *os.File) error {
	return errors.Join(file.Close(), removeStage(parent, name))
}

func cleanupFailure(operation, path string, parent *os.Root, name string, file *os.File, cause error) error {
	cleanup := cleanupStage(parent, name, file)
	if cleanup == nil {
		return cause
	}
	return errors.Join(cause, &MutationError{
		Operation: operation,
		Path:      path,
		Staging:   name,
		Committed: false,
		Err:       cleanup,
	})
}

func removeStage(parent *os.Root, name string) error {
	if testHookRemoveStage != nil {
		return testHookRemoveStage(parent, name)
	}
	return parent.Remove(name)
}

func stage(parent *os.Root, content []byte, mode fs.FileMode) (string, *os.File, Snapshot, error) {
	name, err := randomName()
	if err != nil {
		return "", nil, Snapshot{}, err
	}
	file, err := parent.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0o600)
	if err != nil {
		return "", nil, Snapshot{}, err
	}
	failed := func(err error) (string, *os.File, Snapshot, error) {
		return "", nil, Snapshot{}, cleanupFailure("stage", "", parent, name, file, err)
	}
	if _, err := file.Write(content); err != nil {
		return failed(err)
	}
	if err := barrier(BarrierContent, func() error { return nativeSync(file) }); err != nil {
		return failed(err)
	}
	if err := file.Chmod(PortableMode(mode)); err != nil {
		return failed(err)
	}
	if err := barrier(BarrierMetadata, func() error { return nativeSync(file) }); err != nil {
		return failed(err)
	}
	snapshot, err := observeFile(parent, name)
	if err != nil {
		return failed(err)
	}
	if snapshot.Mode != PortableMode(mode) {
		return failed(fmt.Errorf("%w: requested mode %o became %o", ErrUnsupported, PortableMode(mode), snapshot.Mode))
	}
	return name, file, snapshot, nil
}

func checkStaged(parent *os.Root, name string, expected Snapshot) error {
	current, err := observeFile(parent, name)
	if err != nil || current != expected {
		return fmt.Errorf("%w: staged source %q changed before publication: %v", ErrConflict, name, err)
	}
	return nil
}

// Mkdir creates one absent directory relative to a retained parent and makes the
// new namespace entry durable before reporting success.
func (r *Root) Mkdir(path string, mode fs.FileMode) (*Directory, error) {
	if err := r.authorize(); err != nil {
		return nil, err
	}
	parent, leaf, ancestors, err := r.bindParent(path)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	if err := exactName(parent, leaf, false); err != nil {
		return nil, err
	}
	runMutationHook("mkdir", path)
	fresh, _, freshAncestors, err := r.bindParent(path)
	if err != nil {
		return nil, err
	}
	if !sameIdentities(ancestors, freshAncestors) {
		fresh.Close()
		return nil, fmt.Errorf("%w: ancestors changed before creating %s", ErrConflict, path)
	}
	if err := exactName(fresh, leaf, false); err != nil {
		fresh.Close()
		return nil, err
	}
	fresh.Close()
	if err := parent.Mkdir(leaf, PortableMode(mode)); err != nil {
		return nil, err
	}
	barrierErr := barrier(BarrierDirectory, func() error { return syncDirectory(parent) })
	if testHookAfterCommit != nil {
		testHookAfterCommit("mkdir", path)
	}
	created, err := parent.OpenRoot(leaf)
	if err != nil {
		return nil, &MutationError{Operation: "mkdir", Path: path, Committed: true, Err: err}
	}
	defer created.Close()
	identity, _, err := directoryIdentity(created)
	if err != nil {
		return nil, &MutationError{Operation: "mkdir", Path: path, Committed: true, Err: err}
	}
	directory := &Directory{Path: path, Identity: identity}
	if barrierErr != nil {
		return directory, &MutationError{Operation: "mkdir", Path: path, Committed: true, Err: barrierErr}
	}
	return directory, nil
}

// PublishDirectory stages a fixed flat file set beside path and moves the
// complete directory to that absent name at one native no-replace commit point.
// The destination parent must already exist.
func (r *Root) PublishDirectory(ctx context.Context, destination string, files []DirectoryFile) (directory *Directory, err error) {
	return r.PublishDirectoryValidated(ctx, destination, files, nil)
}

// PublishDirectoryValidated invokes validate after the staged directory has been
// re-proven intact and immediately before its no-replace namespace commit.
func (r *Root) PublishDirectoryValidated(ctx context.Context, destination string, files []DirectoryFile, validate func() error) (directory *Directory, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := r.authorize(); err != nil {
		return nil, err
	}
	stagedFiles, err := validateDirectoryFiles(files)
	if err != nil {
		return nil, err
	}
	parent, leaf, destinationAncestors, err := r.bindParent(destination)
	if err != nil {
		return nil, err
	}
	defer parent.Close()
	if err := exactName(parent, leaf, false); err != nil {
		return nil, err
	}
	stagedLeaf, err := randomName()
	if err != nil {
		return nil, err
	}
	stagedPath := path.Join(path.Dir(destination), stagedLeaf)
	staged, err := r.Mkdir(stagedPath, 0o755)
	if err != nil {
		if staged != nil {
			cleanup := r.cleanupDirectory(stagedPath, staged.Identity, nil, nil)
			return nil, errors.Join(err, cleanup)
		}
		return nil, err
	}
	committed := false
	stagedCount := 0
	snapshots := make([]Snapshot, len(stagedFiles))
	defer func() {
		if !committed {
			if cleanup := r.cleanupDirectory(stagedPath, staged.Identity, stagedFiles[:stagedCount], snapshots[:stagedCount]); cleanup != nil {
				err = errors.Join(err, &MutationError{Operation: "publish-directory", Path: destination, Staging: stagedPath, Err: cleanup})
			}
		}
	}()
	for i, file := range stagedFiles {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if testHookBeforeDirectoryFile != nil {
			if err := testHookBeforeDirectoryFile(file.Name); err != nil {
				return nil, err
			}
		}
		entry, err := r.Publish(stagedPath+"/"+file.Name, file.Content, file.Mode)
		if err != nil {
			return nil, err
		}
		if err := entry.Close(); err != nil {
			return nil, err
		}
		snapshots[i] = entry.Snapshot()
		stagedCount++
	}
	if testHookBeforeDirectoryCommit != nil {
		testHookBeforeDirectoryCommit(stagedPath, destination)
	}
	freshDestination, freshLeaf, freshAncestors, err := r.bindParent(destination)
	if err != nil {
		return nil, err
	}
	defer freshDestination.Close()
	if freshLeaf != leaf || !sameIdentities(destinationAncestors, freshAncestors) {
		return nil, fmt.Errorf("%w: destination ancestors changed before publishing %s", ErrConflict, destination)
	}
	if err := exactName(freshDestination, leaf, false); err != nil {
		return nil, err
	}
	freshStaged, stagedName, _, err := r.bindParent(stagedPath)
	if err != nil {
		return nil, err
	}
	defer freshStaged.Close()
	identity, err := plainDirectoryIdentity(freshStaged, stagedName, stagedPath)
	if err != nil || identity != staged.Identity {
		return nil, fmt.Errorf("%w: staged directory changed before publication: %v", ErrConflict, err)
	}
	if err := r.validateStagedDirectory(stagedPath, freshStaged, stagedName, stagedFiles, snapshots); err != nil {
		return nil, err
	}
	if testHookBeforeDirectoryMove != nil {
		testHookBeforeDirectoryMove(stagedPath, destination)
	}
	if validate != nil {
		if err := validate(); err != nil {
			return nil, err
		}
	}
	commitDestination, commitLeaf, commitAncestors, err := r.bindParent(destination)
	if err != nil {
		return nil, err
	}
	defer commitDestination.Close()
	if commitLeaf != leaf || !sameIdentities(destinationAncestors, commitAncestors) {
		return nil, fmt.Errorf("%w: destination ancestors changed at publication of %s", ErrConflict, destination)
	}
	if err := exactName(commitDestination, leaf, false); err != nil {
		return nil, err
	}
	commitStaged, commitStagedName, _, err := r.bindParent(stagedPath)
	if err != nil {
		return nil, err
	}
	defer commitStaged.Close()
	commitIdentity, err := plainDirectoryIdentity(commitStaged, commitStagedName, stagedPath)
	if err != nil || commitIdentity != staged.Identity {
		return nil, fmt.Errorf("%w: staged directory changed at publication: %v", ErrConflict, err)
	}
	if err := r.validateStagedDirectory(stagedPath, commitStaged, commitStagedName, stagedFiles, snapshots); err != nil {
		return nil, err
	}
	if testHookAfterDirectoryCheck != nil {
		testHookAfterDirectoryCheck(stagedPath, destination)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := moveDirectoryNoReplace(commitStaged, commitStagedName, commitDestination, leaf); err != nil {
		return nil, err
	}
	if err := barrier(BarrierDirectory, func() error { return syncDirectory(commitDestination) }); err != nil {
		if validationErr := r.validatePublishedDirectory(destination, staged.Identity, stagedFiles, snapshots); validationErr != nil {
			committed = true
			return nil, &MutationError{Operation: "publish-directory", Path: destination, Committed: true, Err: errors.Join(err, validationErr)}
		}
		if rollbackErr := moveDirectoryNoReplace(commitDestination, leaf, commitStaged, commitStagedName); rollbackErr != nil {
			committed = true
			return nil, &MutationError{Operation: "publish-directory", Path: destination, Committed: true, Err: errors.Join(err, rollbackErr)}
		}
		rollbackBarrierErr := barrier(BarrierDirectory, func() error { return syncDirectory(commitDestination) })
		return nil, errors.Join(err, rollbackBarrierErr)
	}
	if err := r.validatePublishedDirectory(destination, staged.Identity, stagedFiles, snapshots); err != nil {
		if rollbackErr := moveDirectoryNoReplace(commitDestination, leaf, commitStaged, commitStagedName); rollbackErr != nil {
			committed = true
			return nil, &MutationError{Operation: "publish-directory", Path: destination, Committed: true, Err: errors.Join(err, rollbackErr)}
		}
		barrierErr := barrier(BarrierDirectory, func() error { return syncDirectory(commitDestination) })
		return nil, errors.Join(err, barrierErr)
	}
	committed = true
	return &Directory{Path: destination, Identity: staged.Identity}, nil
}

func (r *Root) validatePublishedDirectory(path string, expected Identity, files []DirectoryFile, snapshots []Snapshot) error {
	parent, leaf, _, err := r.bindParent(path)
	if err != nil {
		return err
	}
	defer parent.Close()
	if err := exactName(parent, leaf, true); err != nil {
		return err
	}
	identity, err := plainDirectoryIdentity(parent, leaf, path)
	if err != nil {
		return err
	}
	if identity != expected {
		return fmt.Errorf("%w: directory identity changed for %s", ErrConflict, path)
	}
	return r.validateStagedDirectory(path, parent, leaf, files, snapshots)
}

func (r *Root) validateStagedDirectory(stagedPath string, parent *os.Root, leaf string, files []DirectoryFile, snapshots []Snapshot) error {
	directory, err := parent.OpenRoot(leaf)
	if err != nil {
		return err
	}
	entries, err := directory.Open(".")
	if err != nil {
		directory.Close()
		return err
	}
	listed, readErr := entries.ReadDir(-1)
	closeErr := errors.Join(entries.Close(), directory.Close())
	if readErr != nil || closeErr != nil {
		return errors.Join(readErr, closeErr)
	}
	if len(listed) != len(files) {
		return fmt.Errorf("%w: staged directory membership changed", ErrConflict)
	}
	slices.SortFunc(listed, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	for i, file := range files {
		if listed[i].Name() != file.Name {
			return fmt.Errorf("%w: staged directory membership changed", ErrConflict)
		}
		entry, err := r.Rebind(stagedPath+"/"+file.Name, snapshots[i])
		if err != nil {
			return err
		}
		if err := entry.Close(); err != nil {
			return err
		}
	}
	return nil
}

func validateDirectoryFiles(files []DirectoryFile) ([]DirectoryFile, error) {
	if len(files) == 0 {
		return nil, errors.New("directory publication has no files")
	}
	out := make([]DirectoryFile, len(files))
	seen := make(map[string]string, len(files))
	for i, file := range files {
		parts, err := splitPath(file.Name)
		if err != nil || len(parts) != 1 {
			return nil, fmt.Errorf("invalid directory member %q", file.Name)
		}
		key := aliasKey(file.Name)
		if previous, ok := seen[key]; ok {
			return nil, fmt.Errorf("%w: directory members %q and %q collide", ErrAlias, previous, file.Name)
		}
		seen[key] = file.Name
		out[i] = DirectoryFile{Name: file.Name, Content: slices.Clone(file.Content), Mode: file.Mode}
	}
	slices.SortFunc(out, func(a, b DirectoryFile) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (r *Root) cleanupDirectory(stagedPath string, expected Identity, files []DirectoryFile, snapshots []Snapshot) error {
	if err := r.validatePublishedDirectory(stagedPath, expected, files, snapshots); err != nil {
		return err
	}
	var problems []error
	for i, file := range files {
		entry, err := r.Rebind(stagedPath+"/"+file.Name, snapshots[i])
		if err != nil {
			problems = append(problems, err)
			continue
		}
		if err := entry.Remove(); err != nil {
			problems = append(problems, err)
		}
	}
	if err := r.RemoveDir(stagedPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
		problems = append(problems, err)
	}
	return errors.Join(problems...)
}

// RemoveDir removes one empty directory relative to a retained parent and makes
// the removal durable before reporting success. A name occupied by anything but
// a plain directory refuses, so a planted link never turns a cleanup into a
// removal somewhere else.
func (r *Root) RemoveDir(path string) error {
	return r.removeDir(path, nil)
}

// RemoveDirExpected removes one empty directory only when its identity still
// matches the caller's creation snapshot.
func (r *Root) RemoveDirExpected(path string, expected Identity) error {
	return r.removeDir(path, &expected)
}

func (r *Root) removeDir(path string, expected *Identity) error {
	if err := r.authorize(); err != nil {
		return err
	}
	parent, leaf, ancestors, err := r.bindParent(path)
	if err != nil {
		return err
	}
	defer parent.Close()
	boundIdentity, err := plainDirectoryIdentity(parent, leaf, path)
	if err != nil {
		return err
	}
	if expected != nil && boundIdentity != *expected {
		return fmt.Errorf("%w: directory identity changed before removing %s", ErrConflict, path)
	}
	runMutationHook("removedir", path)
	fresh, freshLeaf, freshAncestors, err := r.bindParent(path)
	if err != nil {
		return err
	}
	defer fresh.Close()
	if freshLeaf != leaf || !sameIdentities(ancestors, freshAncestors) {
		return fmt.Errorf("%w: ancestors changed before removing %s", ErrConflict, path)
	}
	freshIdentity, err := plainDirectoryIdentity(fresh, leaf, path)
	if err != nil {
		return err
	}
	if freshIdentity != boundIdentity || expected != nil && freshIdentity != *expected {
		return fmt.Errorf("%w: directory changed before removing %s", ErrConflict, path)
	}
	if err := parent.Remove(leaf); err != nil {
		return err
	}
	if err := barrier(BarrierDirectory, func() error { return syncDirectory(parent) }); err != nil {
		return &MutationError{Operation: "removedir", Path: path, Committed: true, Err: err}
	}
	return nil
}

// MoveDir atomically moves one plain directory within the bound root to an
// absent name and persists both parent namespaces. It is used to clear a
// transaction fence at one namespace commit point before best-effort cleanup.
func (r *Root) MoveDir(from, to string) error {
	if err := r.authorize(); err != nil {
		return err
	}
	source, sourceLeaf, sourceAncestors, err := r.bindParent(from)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, destinationLeaf, destinationAncestors, err := r.bindParent(to)
	if err != nil {
		return err
	}
	defer destination.Close()
	sourceIdentity, err := plainDirectoryIdentity(source, sourceLeaf, from)
	if err != nil {
		return err
	}
	if err := exactName(destination, destinationLeaf, false); err != nil {
		return err
	}
	freshSource, freshSourceLeaf, freshSourceAncestors, err := r.bindParent(from)
	if err != nil {
		return err
	}
	freshIdentity, identityErr := plainDirectoryIdentity(freshSource, freshSourceLeaf, from)
	freshSource.Close()
	if identityErr != nil || freshIdentity != sourceIdentity {
		return fmt.Errorf("%w: source directory changed before move", ErrConflict)
	}
	freshDestination, _, freshDestinationAncestors, err := r.bindParent(to)
	if err != nil {
		return err
	}
	freshDestination.Close()
	if !sameIdentities(sourceAncestors, freshSourceAncestors) || !sameIdentities(destinationAncestors, freshDestinationAncestors) {
		return fmt.Errorf("%w: directory ancestors changed before move", ErrConflict)
	}
	if err := exactName(destination, destinationLeaf, false); err != nil {
		return err
	}
	runMutationHook("movedir", from)
	if err := r.handle.Rename(from, to); err != nil {
		return err
	}
	if err := barrier(BarrierDirectory, func() error { return syncDirectory(source) }); err != nil {
		return &MutationError{Operation: "movedir", Path: from, Committed: true, Err: err}
	}
	if !sameIdentities(sourceAncestors, destinationAncestors) {
		if err := barrier(BarrierDirectory, func() error { return syncDirectory(destination) }); err != nil {
			return &MutationError{Operation: "movedir", Path: from, Committed: true, Err: err}
		}
	}
	return nil
}

func plainDirectoryIdentity(parent *os.Root, leaf, reported string) (Identity, error) {
	if err := exactName(parent, leaf, true); err != nil {
		return Identity{}, err
	}
	info, err := parent.Lstat(leaf)
	if err != nil || !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
		return Identity{}, fmt.Errorf("%w: %q is not a plain directory", ErrNotRegular, reported)
	}
	bound, err := parent.OpenRoot(leaf)
	if err != nil {
		return Identity{}, err
	}
	defer bound.Close()
	identity, _, err := directoryIdentity(bound)
	return identity, err
}

// SyncDir persists the namespace state of one bound directory.
func (r *Root) SyncDir(path string) error {
	if err := r.authorize(); err != nil {
		return err
	}
	parent, leaf, _, err := r.bindParent(path)
	if err != nil {
		return err
	}
	defer parent.Close()
	if err := exactName(parent, leaf, true); err != nil {
		return err
	}
	directory, err := parent.OpenRoot(leaf)
	if err != nil {
		return err
	}
	defer directory.Close()
	return barrier(BarrierDirectory, func() error { return syncDirectory(directory) })
}

// Replace atomically replaces a bound regular file after immediate semantic and
// identity comparison. It does not claim exclusion of an external actor racing
// after that comparison outside Taskrail's repository lock.
func (e *Entry) Replace(content []byte, mode fs.FileMode) (*Entry, error) {
	if err := e.root.authorize(); err != nil {
		return nil, err
	}
	if e.parent == nil {
		return nil, fs.ErrClosed
	}
	staged, file, stagedSnapshot, err := stage(e.parent, content, mode)
	if err != nil {
		return nil, err
	}
	runMutationHook("replace", e.path)
	if testHookBeforeStageCAS != nil {
		testHookBeforeStageCAS(e.parent, staged)
	}
	if err := checkStaged(e.parent, staged, stagedSnapshot); err != nil {
		return nil, cleanupFailure("replace", e.path, e.parent, staged, file, err)
	}
	if err := e.checkCurrent(); err != nil {
		return nil, cleanupFailure("replace", e.path, e.parent, staged, file, err)
	}
	if err := replaceStaged(e.parent, file, staged, e.leaf); err != nil {
		return nil, cleanupFailure("replace", e.path, e.parent, staged, file, err)
	}
	if err := file.Close(); err != nil {
		return nil, &MutationError{Operation: "replace", Path: e.path, Committed: true, Err: err}
	}
	if err := barrier(BarrierDirectory, func() error { return syncDirectory(e.parent) }); err != nil {
		return nil, &MutationError{Operation: "replace", Path: e.path, Committed: true, Err: err}
	}
	next, err := e.root.Bind(e.path)
	if err != nil {
		return nil, &MutationError{Operation: "replace", Path: e.path, Committed: true, Err: err}
	}
	e.Close()
	return next, nil
}

// Remove removes a bound regular file after the same immediate comparison and
// persists the parent-directory update before reporting success.
func (e *Entry) Remove() error {
	if err := e.root.authorize(); err != nil {
		return err
	}
	runMutationHook("remove", e.path)
	if err := e.checkCurrent(); err != nil {
		return err
	}
	if err := e.parent.Remove(e.leaf); err != nil {
		return err
	}
	if err := barrier(BarrierDirectory, func() error { return syncDirectory(e.parent) }); err != nil {
		return &MutationError{Operation: "remove", Path: e.path, Committed: true, Err: err}
	}
	if err := e.Close(); err != nil {
		return &MutationError{Operation: "remove", Path: e.path, Committed: true, Err: err}
	}
	return nil
}

func (e *Entry) checkCurrent() error {
	if e.parent == nil {
		return fs.ErrClosed
	}
	fresh, leaf, ancestors, err := e.root.bindParent(e.path)
	if err != nil {
		return fmt.Errorf("%w: rebind ancestors: %v", ErrConflict, err)
	}
	defer fresh.Close()
	if leaf != e.leaf || !sameIdentities(e.ancestors, ancestors) {
		return fmt.Errorf("%w: ancestors changed for %s", ErrConflict, e.path)
	}
	current, err := observeFile(fresh, leaf)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || errors.Is(err, ErrAlias) || errors.Is(err, ErrNotRegular) {
			return fmt.Errorf("%w: %s changed: %v", ErrConflict, e.path, err)
		}
		return err
	}
	if current != e.snapshot {
		return fmt.Errorf("%w: bytes, mode, links, or identity changed for %s", ErrConflict, e.path)
	}
	return nil
}
