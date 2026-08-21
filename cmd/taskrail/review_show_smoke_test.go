package main

import (
	"crypto/sha256"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/tessariq/taskrail/internal/taskrail"
)

func TestReviewShowPublishesExactReadOnlyDurableReview(t *testing.T) {
	root := setupRepo(t)
	logicalPath := "planning/reviews/spec/v0.1.0/session-1/report.json"
	physicalPath := filepath.Join(root, filepath.FromSlash(logicalPath))
	const content = "review bytes\nwith exact trailing newline\n"
	if err := os.MkdirAll(filepath.Dir(physicalPath), 0o755); err != nil {
		t.Fatalf("create review directory: %v", err)
	}
	if err := os.WriteFile(physicalPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write review: %v", err)
	}
	before := snapshotTree(t, root)

	text, stderr, err := runRootSplit(t, "review", "show", logicalPath)
	if err != nil {
		t.Fatalf("review show text: %v (stderr %q)", err, stderr)
	}
	if text != content {
		t.Fatalf("review show text = %q, want exact bytes %q", text, content)
	}

	stdout, stderr, err := runRootSplit(t, "review", "show", logicalPath, "--json")
	if err != nil {
		t.Fatalf("review show json: %v (stderr %q)", err, stderr)
	}
	envelope := decodeEnvelope(t, stdout)
	if envelope.Command != "review show" || envelope.Error != nil {
		t.Fatalf("review show envelope = %+v", envelope)
	}
	var result struct {
		Path    string `json:"path"`
		Content string `json:"content"`
		SHA256  string `json:"sha256"`
	}
	decodeMachineResult(t, stdout, &result)
	if result.Path != logicalPath || result.Content != content || result.SHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(content))) {
		t.Fatalf("review show result = %+v", result)
	}
	if got := snapshotTree(t, root); !maps.Equal(got, before) {
		t.Fatal("review show changed repository bytes")
	}
}

func TestReviewShowClassifiesMissingAndBlockedPaths(t *testing.T) {
	setupRepo(t)
	for _, test := range []struct {
		path string
		code string
	}{
		{path: "planning/reviews/spec/v0.1.0/session-1/missing.json", code: taskrail.MachineCodeReviewNotFound},
		{path: "planning/artifacts/review-proposals/spec/p-1/report.json", code: taskrail.MachineCodePathBlocked},
	} {
		t.Run(test.path, func(t *testing.T) {
			stdout, _, err := runRootSplit(t, "review", "show", test.path, "--json")
			if err == nil {
				t.Fatal("review show succeeded")
			}
			if failure := decodeMachineError(t, stdout); failure.Code != test.code {
				t.Fatalf("review show error = %+v, want %q", failure, test.code)
			}
		})
	}
}
