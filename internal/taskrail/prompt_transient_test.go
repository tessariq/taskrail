package taskrail

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/tessariq/taskrail/internal/durablefs"
)

func TestAuthorizeTransientPromptPathsAcceptsDeclaredProposalFiles(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), "planning/artifacts/\n")
	svc := transientPromptService(repo)
	paths := []TransientPromptPath{
		{Role: PromptContextReviewPath, ProposalType: "task", Path: "planning/artifacts/review-proposals/task/task-1/review.json"},
		{Role: PromptContextReviewPath, ProposalType: "spec", Path: "planning/artifacts/review-proposals/spec/spec-1/consistency.json"},
		{Role: PromptContextReviewPath, ProposalType: "decomposition", Path: "planning/artifacts/review-proposals/decomposition/decomposition-1/review-1.json"},
		{Role: PromptContextReviewPath, ProposalType: "workflow-adversarial", Path: "planning/artifacts/review-proposals/workflow-adversarial/workflow-1/review.json"},
		{Role: PromptContextDraftPath, ProposalType: "decomposition", Path: "planning/artifacts/review-proposals/decomposition/decomposition-1/draft.json"},
		{Role: PromptContextTracePath, ProposalType: "decomposition", Path: "planning/artifacts/review-proposals/decomposition/decomposition-1/trace.json"},
	}
	for _, candidate := range paths {
		if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(path.Dir(candidate.Path))), 0o755); err != nil {
			t.Fatalf("create proposal directory: %v", err)
		}
	}
	before := transientPromptGitSnapshot(t, repo)
	beforeTree, err := durablefs.ObserveTree(repo, "planning/artifacts")
	if err != nil {
		t.Fatalf("snapshot transient artifacts: %v", err)
	}

	for _, candidate := range paths {
		authorization, err := svc.AuthorizeTransientPromptPaths([]TransientPromptPath{candidate})
		if err != nil {
			t.Fatalf("authorize %s: %v", candidate.Role, err)
		}
		if len(authorization.Paths) != 1 || authorization.Paths[0] != candidate {
			t.Fatalf("authorized paths = %+v, want %v", authorization.Paths, candidate)
		}
		if err := svc.RecheckTransientPromptPaths(authorization); err != nil {
			t.Fatalf("recheck %s: %v", candidate.Role, err)
		}
	}
	if after := transientPromptGitSnapshot(t, repo); after != before {
		t.Fatalf("authorization mutated Git state:\n before %q\n after  %q", before, after)
	}
	afterTree, err := durablefs.ObserveTree(repo, "planning/artifacts")
	if err != nil {
		t.Fatalf("resnapshot transient artifacts: %v", err)
	}
	if !beforeTree.Same(afterTree) {
		t.Fatal("authorization mutated transient filesystem state")
	}
}

func TestAuthorizeTransientPromptPathsRefusesUnsafeOrUndeclaredInput(t *testing.T) {
	tests := []struct {
		name  string
		git   bool
		path  TransientPromptPath
		setup func(t *testing.T, repo string)
		code  string
	}{
		{"undeclared role", true, TransientPromptPath{Role: "MEMORY_PATH", ProposalType: "task", Path: "planning/artifacts/review-proposals/task/task-1/review.json"}, nil, MachineCodeInvalidArguments},
		{"review type not allowed", true, TransientPromptPath{Role: PromptContextReviewPath, ProposalType: "other", Path: "planning/artifacts/review-proposals/other/task-1/review.json"}, nil, MachineCodeInvalidArguments},
		{"draft outside decomposition", true, TransientPromptPath{Role: PromptContextDraftPath, ProposalType: "task", Path: "planning/artifacts/review-proposals/task/task-1/draft.json"}, nil, MachineCodeInvalidArguments},
		{"outside artifacts", true, TransientPromptPath{Role: PromptContextReviewPath, ProposalType: "task", Path: "planning/reviews/task-1/review.json"}, nil, MachineCodePathBlocked},
		{"wrong proposal subtree", true, TransientPromptPath{Role: PromptContextReviewPath, ProposalType: "task", Path: "planning/artifacts/review-proposals/spec/task-1/review.json"}, nil, MachineCodePathBlocked},
		{"nested proposal file", true, TransientPromptPath{Role: PromptContextReviewPath, ProposalType: "task", Path: "planning/artifacts/review-proposals/task/task-1/nested/review.json"}, nil, MachineCodePathBlocked},
		{"backslash path", true, TransientPromptPath{Role: PromptContextReviewPath, ProposalType: "task", Path: "planning\\artifacts\\review-proposals\\task\\task-1\\review.json"}, nil, MachineCodePathBlocked},
		{"missing proposal ancestor", true, TransientPromptPath{Role: PromptContextReviewPath, ProposalType: "task", Path: "planning/artifacts/review-proposals/task/task-1/review.json"}, nil, MachineCodePathBlocked},
		{"not ignored", true, TransientPromptPath{Role: PromptContextReviewPath, ProposalType: "task", Path: "planning/artifacts/review-proposals/task/task-1/review.json"}, func(t *testing.T, repo string) { writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), "") }, MachineCodePathBlocked},
		{"tracked", true, TransientPromptPath{Role: PromptContextReviewPath, ProposalType: "task", Path: "planning/artifacts/review-proposals/task/task-1/review.json"}, func(t *testing.T, repo string) {
			writeFile(t, filepath.Join(repo, "planning", "artifacts", "review-proposals", "task", "task-1", "review.json"), "{}")
			runLocalGit(t, repo, "add", "-f", ".")
		}, MachineCodePathBlocked},
		{"staged", true, TransientPromptPath{Role: PromptContextReviewPath, ProposalType: "task", Path: "planning/artifacts/review-proposals/task/task-1/review.json"}, func(t *testing.T, repo string) {
			writeFile(t, filepath.Join(repo, "planning", "artifacts", "review-proposals", "task", "task-1", "review.json"), "{}")
			runLocalGit(t, repo, "add", "-f", ".")
			runLocalGit(t, repo, "reset", "--", ".")
			runLocalGit(t, repo, "add", "-f", "planning/artifacts/review-proposals/task/task-1/review.json")
		}, MachineCodePathBlocked},
		{"alias", true, TransientPromptPath{Role: PromptContextReviewPath, ProposalType: "task", Path: "planning/artifacts/review-proposals/task/task-1/review.json"}, func(t *testing.T, repo string) {
			writeFile(t, filepath.Join(repo, "planning", "artifacts", "review-proposals", "task", "task-1", "Review.json"), "{}")
		}, MachineCodePathBlocked},
		{"special entry", true, TransientPromptPath{Role: PromptContextReviewPath, ProposalType: "task", Path: "planning/artifacts/review-proposals/task/task-1/review.json"}, func(t *testing.T, repo string) {
			if err := os.MkdirAll(filepath.Join(repo, "planning", "artifacts", "review-proposals", "task", "task-1"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(t.TempDir(), filepath.Join(repo, "planning", "artifacts", "review-proposals", "task", "task-1", "review.json")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
		}, MachineCodePathBlocked},
		{"non-git containment only", false, TransientPromptPath{Role: PromptContextReviewPath, ProposalType: "task", Path: "planning/artifacts/review-proposals/task/task-1/review.json"}, func(t *testing.T, repo string) {
			if err := os.MkdirAll(filepath.Join(repo, "planning", "artifacts", "review-proposals", "task", "task-1"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := t.TempDir()
			if test.git {
				initLocalGitRepo(t, repo)
				writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), "planning/artifacts/\n")
			}
			if test.setup != nil {
				test.setup(t, repo)
			}
			svc := transientPromptService(repo)
			before := transientPromptGitSnapshot(t, repo)
			_, err := svc.AuthorizeTransientPromptPaths([]TransientPromptPath{test.path})
			if test.code == "" {
				if err != nil {
					t.Fatalf("authorize non-Git path: %v", err)
				}
			} else if MachineFailureFor(err).Code != test.code {
				t.Fatalf("authorization error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, test.code)
			}
			if after := transientPromptGitSnapshot(t, repo); after != before {
				t.Fatalf("authorization mutated Git state:\n before %q\n after  %q", before, after)
			}
		})
	}
}

func TestAuthorizeTransientPromptPathsUsesLocalArtifactsDirectory(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), ".taskrail/local/\n")
	localPath := ".taskrail/local/planning/artifacts/review-proposals/task/task-1/review.json"
	if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(path.Dir(localPath))), 0o755); err != nil {
		t.Fatalf("create local proposal directory: %v", err)
	}
	svc := transientPromptService(repo)
	svc.paths.ArtifactsDir = filepath.Join(repo, ".taskrail", "local", "planning", "artifacts")

	if _, err := svc.AuthorizeTransientPromptPaths([]TransientPromptPath{{Role: PromptContextReviewPath, ProposalType: "task", Path: localPath}}); err != nil {
		t.Fatalf("authorize local transient path: %v", err)
	}
	if _, err := svc.AuthorizeTransientPromptPaths([]TransientPromptPath{{Role: PromptContextReviewPath, ProposalType: "task", Path: "planning/artifacts/review-proposals/task/task-1/review.json"}}); MachineFailureFor(err).Code != MachineCodePathBlocked {
		t.Fatalf("logical path error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, MachineCodePathBlocked)
	}
}

func TestAuthorizeTransientPromptPathsRejectsRequestedPathAliases(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), "planning/artifacts/\n")
	proposal := filepath.Join(repo, "planning", "artifacts", "review-proposals", "decomposition", "decomposition-1")
	if err := os.MkdirAll(proposal, 0o755); err != nil {
		t.Fatalf("create proposal directory: %v", err)
	}
	svc := transientPromptService(repo)
	_, err := svc.AuthorizeTransientPromptPaths([]TransientPromptPath{
		{Role: PromptContextReviewPath, ProposalType: "decomposition", Path: "planning/artifacts/review-proposals/decomposition/decomposition-1/Draft.json"},
		{Role: PromptContextDraftPath, ProposalType: "decomposition", Path: "planning/artifacts/review-proposals/decomposition/decomposition-1/draft.json"},
	})
	if MachineFailureFor(err).Code != MachineCodePathBlocked {
		t.Fatalf("authorization error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, MachineCodePathBlocked)
	}
}

func TestRecheckTransientPromptPathsRefusesProposalAncestorSwap(t *testing.T) {
	repo := t.TempDir()
	initLocalGitRepo(t, repo)
	writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), "planning/artifacts/\n")
	proposalPath := "planning/artifacts/review-proposals/task/task-1/review.json"
	if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(path.Dir(proposalPath))), 0o755); err != nil {
		t.Fatalf("create proposal directory: %v", err)
	}
	svc := transientPromptService(repo)
	authorization, err := svc.AuthorizeTransientPromptPaths([]TransientPromptPath{{Role: PromptContextReviewPath, ProposalType: "task", Path: proposalPath}})
	if err != nil {
		t.Fatalf("authorize transient path: %v", err)
	}
	writeFile(t, filepath.Join(repo, filepath.FromSlash(proposalPath)), "{}")
	if err := svc.RecheckTransientPromptPaths(authorization); err != nil {
		t.Fatalf("recheck expected proposal output: %v", err)
	}
	proposalDir := filepath.Join(repo, "planning", "artifacts", "review-proposals", "task", "task-1")
	replacement := filepath.Join(repo, "planning", "artifacts", "review-proposals", "task", "replacement")
	writeFile(t, filepath.Join(replacement, "review.json"), "{}")
	if err := os.Rename(proposalDir, proposalDir+"-old"); err != nil {
		t.Fatalf("move original proposal: %v", err)
	}
	if err := os.Rename(replacement, proposalDir); err != nil {
		t.Fatalf("replace proposal: %v", err)
	}
	if err := svc.RecheckTransientPromptPaths(authorization); MachineFailureFor(err).Code != MachineCodePathBlocked {
		t.Fatalf("recheck error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, MachineCodePathBlocked)
	}
}

func TestRecheckTransientPromptPathsRefusesChangedGitAdmission(t *testing.T) {
	t.Run("ignore removed", func(t *testing.T) {
		repo := t.TempDir()
		initLocalGitRepo(t, repo)
		writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), "planning/artifacts/\n")
		proposalPath := "planning/artifacts/review-proposals/task/task-1/review.json"
		if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(path.Dir(proposalPath))), 0o755); err != nil {
			t.Fatalf("create proposal directory: %v", err)
		}
		svc := transientPromptService(repo)
		authorization, err := svc.AuthorizeTransientPromptPaths([]TransientPromptPath{{Role: PromptContextReviewPath, ProposalType: "task", Path: proposalPath}})
		if err != nil {
			t.Fatalf("authorize transient path: %v", err)
		}
		writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), "")
		if err := svc.RecheckTransientPromptPaths(authorization); MachineFailureFor(err).Code != MachineCodePathBlocked {
			t.Fatalf("recheck error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, MachineCodePathBlocked)
		}
	})

	t.Run("path staged", func(t *testing.T) {
		repo := t.TempDir()
		initLocalGitRepo(t, repo)
		writeFile(t, filepath.Join(repo, ".git", "info", "exclude"), "planning/artifacts/\n")
		proposalPath := "planning/artifacts/review-proposals/task/task-1/review.json"
		writeFile(t, filepath.Join(repo, filepath.FromSlash(proposalPath)), "{}")
		svc := transientPromptService(repo)
		authorization, err := svc.AuthorizeTransientPromptPaths([]TransientPromptPath{{Role: PromptContextReviewPath, ProposalType: "task", Path: proposalPath}})
		if err != nil {
			t.Fatalf("authorize transient path: %v", err)
		}
		runLocalGit(t, repo, "add", "-f", proposalPath)
		if err := svc.RecheckTransientPromptPaths(authorization); MachineFailureFor(err).Code != MachineCodePathBlocked {
			t.Fatalf("recheck error = %v, code = %q, want %q", err, MachineFailureFor(err).Code, MachineCodePathBlocked)
		}
	})
}

func transientPromptService(repo string) *Service {
	paths := Paths{RepoRoot: repo, ArtifactsDir: filepath.Join(repo, "planning", "artifacts")}
	if _, err := os.Stat(filepath.Join(repo, ".git")); err == nil {
		paths.WorktreeRoot = repo
	}
	return &Service{paths: paths}
}

func transientPromptGitSnapshot(t *testing.T, repo string) string {
	t.Helper()
	if _, err := os.Stat(filepath.Join(repo, ".git")); os.IsNotExist(err) {
		return ""
	}
	exclude, err := os.ReadFile(filepath.Join(repo, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("read Git exclusions: %v", err)
	}
	return gitOutput(t, repo, "status", "--porcelain=v1", "--untracked-files=all") + "\n" + gitOutput(t, repo, "diff", "--cached", "--binary") + "\n" + string(exclude)
}
