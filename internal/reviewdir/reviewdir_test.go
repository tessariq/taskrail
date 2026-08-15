package reviewdir

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/repolock"
)

func TestPublishTypedBundlesPreservesExactBytesAndReportsDeterministicFiles(t *testing.T) {
	for _, test := range []struct {
		type_ Type
		files []File
	}{
		{TypeTask, files("review.json")},
		{TypeSpec, files("adversarial.json", "manifest.json", "gaps.json", "consistency.json", "additions.json")},
		{TypeDecomposition, files("trace.json", "review-1.json", "manifest.json", "draft.json")},
		{TypeDecomposition, files("review-2.json", "trace.json", "review-1.json", "manifest.json", "draft.json")},
	} {
		t.Run(string(test.type_)+"-"+test.files[len(test.files)-1].Name, func(t *testing.T) {
			root := t.TempDir()
			destination := typedDestination(test.type_)
			if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(destination))), 0o755); err != nil {
				t.Fatal(err)
			}
			lock := acquire(t, root)
			defer release(t, lock)
			validated := false
			result, err := Publish(context.Background(), lock, Request{
				Type:        test.type_,
				ReviewsRoot: "reviews",
				Destination: destination,
				Files:       test.files,
				Validate: func(_ Type, candidate []File) error {
					validated = true
					candidate[0].Content[0] = 'X'
					return nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			if !validated || result.Type != test.type_ {
				t.Fatalf("validated=%t result type=%q", validated, result.Type)
			}
			wantNames := expectedNames(test.type_, len(test.files) == 5 && test.type_ == TypeDecomposition)
			if len(result.Files) != len(wantNames) {
				t.Fatalf("result files = %+v", result.Files)
			}
			for i, name := range wantNames {
				wantContent := []byte("exact:" + name)
				got, readErr := os.ReadFile(filepath.Join(root, filepath.FromSlash(destination), name))
				if readErr != nil || !slices.Equal(got, wantContent) {
					t.Fatalf("%s bytes = %q, err=%v", name, got, readErr)
				}
				if result.Files[i].Destination != destination+"/"+name || result.Files[i].SHA256 != digest(wantContent) {
					t.Fatalf("file[%d] = %+v", i, result.Files[i])
				}
			}
		})
	}
}

func TestPublishRefusesInvalidBundlesBeforeCreatingDestination(t *testing.T) {
	for _, test := range []struct {
		name string
		req  Request
	}{
		{name: "unknown type", req: request("reviews", "workflow", "reviews/workflow/v1/session", files("report.json"))},
		{name: "missing review root", req: Request{Type: TypeTask, Destination: typedDestination(TypeTask), Files: files("review.json"), Validate: valid}},
		{name: "outside review root", req: request("planning/reviews", TypeTask, "artifacts/reviews/task/T-1/session", files("review.json"))},
		{name: "wrong inventory", req: request("reviews", TypeTask, typedDestination(TypeTask), files("manifest.json"))},
		{name: "nested member", req: request("reviews", TypeTask, typedDestination(TypeTask), files("nested/review.json"))},
		{name: "duplicate member", req: request("reviews", TypeTask, typedDestination(TypeTask), append(files("review.json"), files("review.json")...))},
		{name: "cross type inventory", req: request("reviews", TypeSpec, typedDestination(TypeSpec), files("review.json"))},
		{name: "cross type destination", req: request("reviews", TypeTask, typedDestination(TypeSpec), files("review.json"))},
		{name: "missing validation", req: Request{Type: TypeTask, ReviewsRoot: "reviews", Destination: typedDestination(TypeTask), Files: files("review.json")}},
		{name: "validation", req: Request{Type: TypeTask, ReviewsRoot: "reviews", Destination: typedDestination(TypeTask), Files: files("review.json"), Validate: func(Type, []File) error { return errors.New("invalid bundle") }}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			lock := acquire(t, root)
			defer release(t, lock)
			_, err := Publish(context.Background(), lock, test.req)
			if err == nil {
				t.Fatal("Publish succeeded")
			}
			if _, statErr := os.Lstat(filepath.Join(root, filepath.FromSlash(test.req.Destination))); !errors.Is(statErr, os.ErrNotExist) {
				t.Fatalf("destination exists after refusal: %v", statErr)
			}
		})
	}
}

func TestPublishDoesNotClobberExistingDestination(t *testing.T) {
	root := t.TempDir()
	reported := typedDestination(TypeTask)
	destination := filepath.Join(root, filepath.FromSlash(reported))
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "sentinel"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	lock := acquire(t, root)
	defer release(t, lock)
	_, err := Publish(context.Background(), lock, request("reviews", TypeTask, reported, files("review.json")))
	if err == nil {
		t.Fatal("Publish succeeded")
	}
	got, readErr := os.ReadFile(filepath.Join(destination, "sentinel"))
	if readErr != nil || string(got) != "existing" {
		t.Fatalf("existing bytes = %q, err=%v", got, readErr)
	}
}

func TestPublishRefusesBlockedDestinationClasses(t *testing.T) {
	for _, test := range []struct {
		name  string
		build func(*testing.T, string)
	}{
		{name: "regular file", build: func(t *testing.T, destination string) {
			if err := os.WriteFile(destination, []byte("existing"), 0o644); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "directory", build: func(t *testing.T, destination string) {
			if err := os.Mkdir(destination, 0o755); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "symlink", build: func(t *testing.T, destination string) {
			if runtime.GOOS == "windows" {
				t.Skip("symlink creation may require elevated privileges")
			}
			if err := os.Symlink(t.TempDir(), destination); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "alias", build: func(t *testing.T, destination string) {
			if runtime.GOOS == "windows" {
				t.Skip("case-insensitive filesystems cannot create a distinct alias")
			}
			if err := os.Mkdir(filepath.Join(filepath.Dir(destination), "Session"), 0o755); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			reported := typedDestination(TypeTask)
			destination := filepath.Join(root, filepath.FromSlash(reported))
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				t.Fatal(err)
			}
			test.build(t, destination)
			lock := acquire(t, root)
			defer release(t, lock)
			if _, err := Publish(context.Background(), lock, request("reviews", TypeTask, reported, files("review.json"))); err == nil {
				t.Fatal("Publish succeeded")
			}
		})
	}
}

func TestPublishRefusesSymlinkedDestinationAncestor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation may require elevated privileges")
	}
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "reviews"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "reviews", "task")); err != nil {
		t.Fatal(err)
	}
	lock := acquire(t, root)
	defer release(t, lock)
	if _, err := Publish(context.Background(), lock, request("reviews", TypeTask, typedDestination(TypeTask), files("review.json"))); err == nil {
		t.Fatal("Publish succeeded")
	}
	entries, err := os.ReadDir(outside)
	if err != nil || len(entries) != 0 {
		t.Fatalf("outside entries = %v, err=%v", entries, err)
	}
}

func TestPublishHonorsBoundedWriteCapability(t *testing.T) {
	root := t.TempDir()
	destination := typedDestination(TypeTask)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(destination))), 0o755); err != nil {
		t.Fatal(err)
	}
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: repolock.Repository{Root: root, Mode: repolock.ModeCommitted},
		Command:    "review publish",
		Capability: repolock.Capability{Commands: []string{"review publish"}, Writes: []string{"reviews/task/T-2/other/review.json"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer release(t, lock)
	_, err = Publish(context.Background(), lock, request("reviews", TypeTask, destination, files("review.json")))
	if !errors.Is(err, repolock.ErrRefused) {
		t.Fatalf("Publish = %v, want delegated write refusal", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(destination))); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination exists: %v", statErr)
	}
}

func TestPublishRefusesOwnershipForAnotherCommand(t *testing.T) {
	root := t.TempDir()
	destination := typedDestination(TypeTask)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(destination))), 0o755); err != nil {
		t.Fatal(err)
	}
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: repolock.Repository{Root: root, Mode: repolock.ModeCommitted},
		Command:    "start",
		Capability: repolock.Capability{Commands: []string{"start"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer release(t, lock)
	if _, err := Publish(context.Background(), lock, request("reviews", TypeTask, destination, files("review.json"))); !errors.Is(err, repolock.ErrRefused) {
		t.Fatalf("Publish = %v, want command refusal", err)
	}
}

func TestPublishHonorsCancellationBeforeDirectoryCommit(t *testing.T) {
	root := t.TempDir()
	destination := typedDestination(TypeTask)
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, filepath.FromSlash(destination))), 0o755); err != nil {
		t.Fatal(err)
	}
	lock := acquire(t, root)
	defer release(t, lock)
	ctx, cancel := context.WithCancel(context.Background())
	own := &cancelOwnership{Lock: lock, cancel: cancel}
	_, err := Publish(ctx, own, request("reviews", TypeTask, destination, files("review.json")))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Publish = %v, want context cancellation", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(destination))); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination exists: %v", statErr)
	}
}

func TestPublishUsesActiveLocalStorageWithoutExposingPhysicalPaths(t *testing.T) {
	repoRoot := t.TempDir()
	gitCommon := filepath.Join(repoRoot, ".git")
	storageRoot := filepath.Join(repoRoot, ".taskrail", "local")
	destination := "planning/reviews/task/T-1/session"
	if err := os.MkdirAll(gitCommon, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Join(storageRoot, filepath.FromSlash(destination))), 0o755); err != nil {
		t.Fatal(err)
	}
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: repolock.Repository{Root: repoRoot, GitCommonDir: gitCommon, Mode: repolock.ModeLocal},
		Command:    "review publish",
		Capability: repolock.Capability{Commands: []string{"review publish"}, Writes: []string{destination + "/review.json"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer release(t, lock)
	result, err := Publish(context.Background(), lock, request("planning/reviews", TypeTask, destination, files("review.json")))
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Files[0].Destination; got != destination+"/review.json" || filepath.IsAbs(got) || strings.Contains(got, ".taskrail/local") {
		t.Fatalf("reported destination = %q", got)
	}
	if got, err := os.ReadFile(filepath.Join(storageRoot, filepath.FromSlash(destination), "review.json")); err != nil || string(got) != "exact:review.json" {
		t.Fatalf("local published bytes = %q, err=%v", got, err)
	}
}

type cancelOwnership struct {
	*repolock.Lock
	cancel context.CancelFunc
	calls  int
}

func (o *cancelOwnership) Authorize(command string, fields ...string) error {
	o.calls++
	if o.calls == 2 {
		o.cancel()
	}
	return o.Lock.Authorize(command, fields...)
}

func typedDestination(bundleType Type) string {
	if bundleType == TypeTask {
		return "reviews/task/T-1/session"
	}
	return "reviews/" + string(bundleType) + "/v0.5.0/session"
}

func valid(Type, []File) error { return nil }

func request(root string, bundleType Type, destination string, bundle []File) Request {
	return Request{Type: bundleType, ReviewsRoot: root, Destination: destination, Files: bundle, Validate: valid}
}

func files(names ...string) []File {
	out := make([]File, 0, len(names))
	for _, name := range names {
		out = append(out, File{Name: name, Content: []byte("exact:" + name)})
	}
	return out
}

func acquire(t *testing.T, root string) *repolock.Lock {
	t.Helper()
	lock, err := repolock.Acquire(context.Background(), repolock.Request{
		Repository: repolock.Repository{Root: root, Mode: repolock.ModeCommitted},
		Command:    "review publish",
		Capability: repolock.Capability{Commands: []string{"review publish"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return lock
}

func release(t *testing.T, lock *repolock.Lock) {
	t.Helper()
	if err := lock.Release(); err != nil {
		t.Fatal(err)
	}
}
