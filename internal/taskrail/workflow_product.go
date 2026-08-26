package taskrail

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os/exec"
	"slices"
	"strconv"
	"strings"
)

// WorkflowProductSHA256 hashes the recorded Git tree, not mutable worktree
// bytes. Review memory is deliberately outside the product boundary because it
// is evidence about the product rather than part of the reviewed subject.
func WorkflowProductSHA256(repoRoot, head, reviewsRoot string) (string, error) {
	if repoRoot == "" || !workflowObjectID.MatchString(head) {
		return "", fmt.Errorf("workflow product hash requires a repository and full Git object ID")
	}
	if absolutePathStart.MatchString(reviewsRoot) || strings.ContainsRune(reviewsRoot, 0) || !canonicalPathSegments(reviewsRoot) {
		return "", fmt.Errorf("workflow product hash review root is not a canonical repository-relative path")
	}
	entries, err := workflowProductEntries(repoRoot, head, reviewsRoot)
	if err != nil {
		return "", err
	}
	slices.SortFunc(entries, func(a, b workflowProductEntry) int { return strings.Compare(a.path, b.path) })

	hash := sha256.New()
	_, _ = hash.Write([]byte("taskrail-workflow-product-v1\x00"))
	for _, entry := range entries {
		_, _ = hash.Write([]byte(entry.path))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(entry.mode))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(strconv.Itoa(len(entry.content))))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(entry.content)
		_, _ = hash.Write([]byte{0})
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

type workflowProductEntry struct {
	path, mode string
	content    []byte
}

func workflowProductEntries(repoRoot, head, reviewsRoot string) ([]workflowProductEntry, error) {
	command := exec.Command("git", "-C", repoRoot, "ls-tree", "-rz", head)
	command.Env = readOnlyGitEnvironment()
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("list workflow product tree: %w", err)
	}
	var entries []workflowProductEntry
	for _, record := range bytes.Split(bytes.TrimSuffix(output, []byte{0}), []byte{0}) {
		if len(record) == 0 {
			continue
		}
		entry, err := decodeWorkflowProductEntry(repoRoot, record)
		if err != nil {
			return nil, err
		}
		if entry.path == reviewsRoot || strings.HasPrefix(entry.path, reviewsRoot+"/") {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func decodeWorkflowProductEntry(repoRoot string, record []byte) (workflowProductEntry, error) {
	metadata, name, found := bytes.Cut(record, []byte{'\t'})
	if !found || len(name) == 0 {
		return workflowProductEntry{}, fmt.Errorf("list workflow product tree: malformed tree entry")
	}
	fields := bytes.Fields(metadata)
	if len(fields) != 3 {
		return workflowProductEntry{}, fmt.Errorf("list workflow product tree: malformed tree metadata")
	}
	mode, kind, objectID := string(fields[0]), string(fields[1]), string(fields[2])
	entry := workflowProductEntry{path: string(name), mode: mode}
	switch {
	case kind == "blob" && (mode == "100644" || mode == "100755" || mode == "120000"):
		blob := exec.Command("git", "-C", repoRoot, "cat-file", "blob", objectID)
		blob.Env = readOnlyGitEnvironment()
		data, err := blob.Output()
		if err != nil {
			return workflowProductEntry{}, fmt.Errorf("read workflow product blob %q: %w", entry.path, err)
		}
		entry.content = data
	case kind == "commit" && mode == "160000" && workflowObjectID.MatchString(objectID):
		entry.content = []byte(objectID)
	default:
		return workflowProductEntry{}, fmt.Errorf("list workflow product tree: unsupported entry %q (%s %s)", entry.path, mode, kind)
	}
	return entry, nil
}
