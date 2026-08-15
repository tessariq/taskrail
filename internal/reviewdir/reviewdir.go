// Package reviewdir publishes validated review bundles through one no-clobber
// destination-directory commit point.
package reviewdir

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/tessariq/taskrail/internal/durablefs"
	"github.com/tessariq/taskrail/internal/repolock"
)

// Type identifies one fixed review directory inventory and destination class.
type Type string

const (
	TypeTask          Type = "task"
	TypeSpec          Type = "spec"
	TypeDecomposition Type = "decomposition"
)

// File is one exact proposal file selected for publication.
type File struct {
	Name    string
	Content []byte
}

// Request is a complete validated typed bundle and its absent destination.
type Request struct {
	Type        Type
	ReviewsRoot string
	Destination string
	Files       []File
	Validate    func(Type, []File) error
}

// PublishedFile reports one deterministic final path and exact-byte digest.
type PublishedFile struct {
	Destination string
	SHA256      string
}

// Result is returned only after the complete destination commits successfully.
type Result struct {
	Type  Type
	Files []PublishedFile
}

type ownership interface {
	Owner() repolock.Owner
	Repository() repolock.Repository
	Capability() repolock.Capability
	Authorize(command string, fields ...string) error
}

// Publish validates and atomically publishes one typed directory without
// replacing or merging an existing destination.
func Publish(ctx context.Context, own ownership, request Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if err := own.Authorize("review publish"); err != nil {
		return Result{}, err
	}
	files := cloneFiles(request.Files)
	if err := validateInventory(request.Type, files); err != nil {
		return Result{}, err
	}
	if err := validateDestination(request.Type, request.ReviewsRoot, request.Destination); err != nil {
		return Result{}, err
	}
	if request.Validate == nil {
		return Result{}, errors.New("review directory publication requires validation")
	}
	if err := request.Validate(request.Type, cloneFiles(files)); err != nil {
		return Result{}, err
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	repository := own.Repository()
	root, err := durablefs.OpenAt(repository.StorageRoot(), repository, own)
	if err != nil {
		return Result{}, err
	}
	defer root.Close()
	candidates := make([]durablefs.DirectoryFile, len(files))
	result := Result{Type: request.Type, Files: make([]PublishedFile, len(files))}
	writes := make([]string, len(files))
	for i, file := range files {
		candidates[i] = durablefs.DirectoryFile{Name: file.Name, Content: file.Content, Mode: 0o644}
		result.Files[i] = PublishedFile{Destination: request.Destination + "/" + file.Name, SHA256: digest(file.Content)}
		writes[i] = result.Files[i].Destination
	}
	if err := own.Capability().AllowsWrites(writes); err != nil {
		return Result{}, err
	}
	if _, err := root.PublishDirectory(ctx, request.Destination, candidates); err != nil {
		return Result{}, err
	}
	return result, nil
}

func validateDestination(bundleType Type, reviewsRoot, destination string) error {
	rootParts := strings.Split(reviewsRoot, "/")
	parts := strings.Split(destination, "/")
	if len(rootParts) == 0 || rootParts[len(rootParts)-1] != "reviews" || len(parts) != len(rootParts)+3 ||
		!slices.Equal(parts[:len(rootParts)], rootParts) || parts[len(rootParts)] != string(bundleType) {
		return fmt.Errorf("%s review directory has cross-type or invalid destination %q", bundleType, destination)
	}
	for _, part := range append(slices.Clone(rootParts), parts...) {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("%s review directory has invalid destination %q", bundleType, destination)
		}
	}
	return nil
}

func validateInventory(bundleType Type, files []File) error {
	want := expectedNames(bundleType, bundleType == TypeDecomposition && len(files) == 5)
	if want == nil {
		return fmt.Errorf("unsupported review directory type %q", bundleType)
	}
	got := make([]string, len(files))
	for i, file := range files {
		if strings.Contains(file.Name, "/") || strings.Contains(file.Name, `\`) {
			return fmt.Errorf("review directory member %q is not a basename", file.Name)
		}
		got[i] = file.Name
	}
	slices.Sort(got)
	sortedWant := slices.Clone(want)
	slices.Sort(sortedWant)
	if !slices.Equal(got, sortedWant) {
		return fmt.Errorf("%s review directory inventory is %v, want %v", bundleType, got, sortedWant)
	}
	slices.SortFunc(files, func(a, b File) int {
		return slices.Index(want, a.Name) - slices.Index(want, b.Name)
	})
	return nil
}

func expectedNames(bundleType Type, secondPass bool) []string {
	switch bundleType {
	case TypeTask:
		return []string{"review.json"}
	case TypeSpec:
		return []string{"consistency.json", "gaps.json", "additions.json", "adversarial.json", "manifest.json"}
	case TypeDecomposition:
		names := []string{"draft.json", "trace.json", "review-1.json"}
		if secondPass {
			names = append(names, "review-2.json")
		}
		return append(names, "manifest.json")
	default:
		return nil
	}
}

func cloneFiles(files []File) []File {
	out := make([]File, len(files))
	for i, file := range files {
		out[i] = File{Name: file.Name, Content: slices.Clone(file.Content)}
	}
	return out
}

func digest(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}
