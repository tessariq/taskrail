package durablefs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
)

var testHookObserveTree func()

// TreeSnapshot is a stable, no-follow observation of a directory membership and
// every regular file beneath it. Ancestors are retained as substitution evidence
// even when the requested directory is absent.
type TreeSnapshot struct {
	Present   bool
	Ancestors []Identity
	Identity  Identity
	Mode      fs.FileMode
	Modified  int64
	Entries   []TreeEntry
}

type TreeEntry struct {
	Path      string
	Directory bool
	Identity  Identity
	Mode      fs.FileMode
	Modified  int64
	Snapshot  Snapshot
}

func (s TreeSnapshot) Same(other TreeSnapshot) bool {
	return reflect.DeepEqual(s, other)
}

// ObserveTree binds base and each existing path component without following
// links, then reads the complete target tree twice through retained handles. A
// disagreement is a conflict rather than a potentially mixed snapshot.
func ObserveTree(base, path string) (TreeSnapshot, error) {
	abs, err := filepath.Abs(base)
	if err != nil {
		return TreeSnapshot{}, err
	}
	if filepath.Clean(abs) != abs {
		return TreeSnapshot{}, fmt.Errorf("%w: non-canonical root %q", ErrInvalidPath, base)
	}
	root, identity, err := openInspectionRoot(abs)
	if err != nil {
		return TreeSnapshot{}, err
	}
	defer root.Close()

	first, err := observeTree(root, identity, path)
	if err != nil {
		return TreeSnapshot{}, err
	}
	if testHookObserveTree != nil {
		testHookObserveTree()
	}
	second, err := observeTree(root, identity, path)
	if err != nil || !first.Same(second) {
		return TreeSnapshot{}, fmt.Errorf("%w: %s changed while inspecting", ErrConflict, path)
	}
	return first, nil
}

// ObserveRoot returns the same stable tree evidence for base itself. It exists
// for root-level transaction members such as Git's index, whose parent cannot be
// expressed as a non-empty relative path to ObserveTree.
func ObserveRoot(base string) (TreeSnapshot, error) {
	abs, err := filepath.Abs(base)
	if err != nil || filepath.Clean(abs) != abs {
		return TreeSnapshot{}, fmt.Errorf("%w: non-canonical root %q", ErrInvalidPath, base)
	}
	root, identity, err := openInspectionRoot(abs)
	if err != nil {
		return TreeSnapshot{}, err
	}
	defer root.Close()
	read := func() (TreeSnapshot, error) {
		file, err := root.Open(".")
		if err != nil {
			return TreeSnapshot{}, err
		}
		info, statErr := file.Stat()
		file.Close()
		if statErr != nil {
			return TreeSnapshot{}, statErr
		}
		entries, err := inspectDirectory(root, "")
		if err != nil {
			return TreeSnapshot{}, err
		}
		return TreeSnapshot{Present: true, Identity: identity, Mode: PortableMode(info.Mode()), Modified: info.ModTime().UnixNano(), Entries: entries}, nil
	}
	first, err := read()
	if err != nil {
		return TreeSnapshot{}, err
	}
	if testHookObserveTree != nil {
		testHookObserveTree()
	}
	second, err := read()
	if err != nil || !first.Same(second) {
		return TreeSnapshot{}, fmt.Errorf("%w: root changed while inspecting", ErrConflict)
	}
	return first, nil
}

// ReadFile returns bounded bytes from one no-follow regular file only when its
// semantic and identity snapshot is unchanged across the read.
func ReadFile(base, path string, maxBytes int64) ([]byte, Snapshot, error) {
	if maxBytes <= 0 {
		return nil, Snapshot{}, fmt.Errorf("%w: maxBytes must be positive", ErrInvalidPath)
	}
	abs, err := filepath.Abs(base)
	if err != nil || filepath.Clean(abs) != abs {
		return nil, Snapshot{}, fmt.Errorf("%w: non-canonical root %q", ErrInvalidPath, base)
	}
	root, _, err := openInspectionRoot(abs)
	if err != nil {
		return nil, Snapshot{}, err
	}
	defer root.Close()
	parts, err := splitPath(path)
	if err != nil {
		return nil, Snapshot{}, err
	}
	current, err := root.OpenRoot(".")
	if err != nil {
		return nil, Snapshot{}, err
	}
	defer func() { current.Close() }()
	for _, part := range parts[:len(parts)-1] {
		if err := exactName(current, part, true); err != nil {
			return nil, Snapshot{}, err
		}
		next, _, err := openInspectionDirectory(current, part)
		if err != nil {
			return nil, Snapshot{}, err
		}
		current.Close()
		current = next
	}
	leaf := parts[len(parts)-1]
	before, err := observeFile(current, leaf)
	if err != nil {
		return nil, Snapshot{}, err
	}
	file, err := openObserved(current, leaf, false)
	if err != nil {
		return nil, Snapshot{}, err
	}
	content, readErr := io.ReadAll(io.LimitReader(file, maxBytes+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, Snapshot{}, readErr
	}
	if closeErr != nil {
		return nil, Snapshot{}, closeErr
	}
	if int64(len(content)) > maxBytes {
		return nil, Snapshot{}, fmt.Errorf("%w: %s exceeds stable read limit", ErrUnsupported, path)
	}
	after, err := observeFile(current, leaf)
	if err != nil || before != after {
		return nil, Snapshot{}, fmt.Errorf("%w: %s changed while reading", ErrConflict, path)
	}
	return content, before, nil
}

func openInspectionRoot(path string) (*os.Root, Identity, error) {
	path = nativeRootPath(path)
	observed, err := openRootObserved(path)
	if err != nil {
		return nil, Identity{}, err
	}
	observedIdentity, _, identityErr := nativeIdentity(observed)
	observed.Close()
	if identityErr != nil {
		return nil, Identity{}, identityErr
	}
	root, err := os.OpenRoot(path)
	if err != nil {
		return nil, Identity{}, err
	}
	identity, _, err := directoryIdentity(root)
	if err != nil || identity != observedIdentity {
		root.Close()
		return nil, Identity{}, fmt.Errorf("%w: inspection root changed while binding", ErrConflict)
	}
	return root, identity, nil
}

func observeTree(root *os.Root, rootIdentity Identity, path string) (TreeSnapshot, error) {
	parts, err := splitPath(path)
	if err != nil {
		return TreeSnapshot{}, err
	}
	current, err := root.OpenRoot(".")
	if err != nil {
		return TreeSnapshot{}, err
	}
	defer func() { current.Close() }()
	ancestors := []Identity{rootIdentity}

	for _, part := range parts {
		if err := exactName(current, part, true); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return TreeSnapshot{Ancestors: ancestors, Entries: []TreeEntry{}}, nil
			}
			return TreeSnapshot{}, err
		}
		info, err := current.Lstat(part)
		if err != nil || !info.IsDir() || info.Mode()&fs.ModeSymlink != 0 {
			return TreeSnapshot{}, fmt.Errorf("%w: %q is not a plain directory", ErrNotRegular, part)
		}
		next, identity, err := openInspectionDirectory(current, part)
		if err != nil {
			return TreeSnapshot{}, err
		}
		current.Close()
		current = next
		ancestors = append(ancestors, identity)
	}

	info, err := current.Open(".")
	if err != nil {
		return TreeSnapshot{}, err
	}
	stat, statErr := info.Stat()
	info.Close()
	if statErr != nil {
		return TreeSnapshot{}, statErr
	}
	entries, err := inspectDirectory(current, "")
	if err != nil {
		return TreeSnapshot{}, err
	}
	return TreeSnapshot{
		Present: true, Ancestors: ancestors[:len(ancestors)-1], Identity: ancestors[len(ancestors)-1],
		Mode: PortableMode(stat.Mode()), Modified: stat.ModTime().UnixNano(), Entries: entries,
	}, nil
}

func openInspectionDirectory(parent *os.Root, name string) (*os.Root, Identity, error) {
	observed, err := openObserved(parent, name, true)
	if err != nil {
		return nil, Identity{}, err
	}
	want, _, identityErr := nativeIdentity(observed)
	observed.Close()
	if identityErr != nil {
		return nil, Identity{}, identityErr
	}
	root, err := parent.OpenRoot(name)
	if err != nil {
		return nil, Identity{}, err
	}
	got, _, err := directoryIdentity(root)
	if err != nil || got != want {
		root.Close()
		return nil, Identity{}, fmt.Errorf("%w: directory %q changed while binding", ErrConflict, name)
	}
	return root, got, nil
}

func inspectDirectory(root *os.Root, prefix string) ([]TreeEntry, error) {
	dir, err := root.Open(".")
	if err != nil {
		return nil, err
	}
	names, readErr := dir.ReadDir(-1)
	closeErr := dir.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	slices.SortFunc(names, func(a, b fs.DirEntry) int { return compareNames(a.Name(), b.Name()) })
	for i := 1; i < len(names); i++ {
		if aliasKey(names[i-1].Name()) == aliasKey(names[i].Name()) {
			return nil, fmt.Errorf("%w: %q collides with %q", ErrAlias, names[i-1].Name(), names[i].Name())
		}
	}

	entries := make([]TreeEntry, 0, len(names))
	for _, named := range names {
		name := named.Name()
		if err := exactName(root, name, true); err != nil {
			return nil, err
		}
		info, err := root.Lstat(name)
		if err != nil || info.Mode()&fs.ModeSymlink != 0 {
			return nil, fmt.Errorf("%w: entry %q changed while inspecting", ErrConflict, name)
		}
		entryPath := filepath.ToSlash(filepath.Join(prefix, name))
		if info.IsDir() {
			child, identity, err := openInspectionDirectory(root, name)
			if err != nil {
				return nil, err
			}
			entries = append(entries, TreeEntry{Path: entryPath, Directory: true, Identity: identity, Mode: PortableMode(info.Mode()), Modified: info.ModTime().UnixNano()})
			nested, err := inspectDirectory(child, entryPath)
			child.Close()
			if err != nil {
				return nil, err
			}
			entries = append(entries, nested...)
			continue
		}
		if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("%w: entry %q is not regular", ErrNotRegular, name)
		}
		snapshot, err := observeFile(root, name)
		if err != nil {
			return nil, err
		}
		entries = append(entries, TreeEntry{Path: entryPath, Identity: snapshot.Identity, Mode: snapshot.Mode, Modified: info.ModTime().UnixNano(), Snapshot: snapshot})
	}
	return entries, nil
}

func compareNames(a, b string) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
