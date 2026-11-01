// This deterministic adapter only touches a caller-supplied disposable local
// bare repository. It never contacts a hosted provider.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type request struct {
	SchemaVersion      int             `json:"schema_version"`
	RequestID          string          `json:"request_id"`
	Operation          string          `json:"operation"`
	Repository         string          `json:"repository"`
	TargetID           string          `json:"target_id"`
	TargetRef          string          `json:"target_ref"`
	SourceRef          string          `json:"source_ref"`
	ChangeID           *string         `json:"change_id"`
	ExpectedTargetHead string          `json:"expected_target_head"`
	CandidateHead      string          `json:"candidate_head"`
	Check              json.RawMessage `json:"check"`
	ApprovalToken      *string         `json:"approval_token"`
}

func git(args ...string) string {
	args = append([]string{"--git-dir", os.Getenv("REVIEW_TARGET")}, args...)
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		fmt.Fprintln(os.Stderr, string(out), err)
		os.Exit(3)
	}
	return strings.TrimSpace(string(out))
}

func main() {
	data, _ := io.ReadAll(os.Stdin)
	var r request
	if json.Unmarshal(data, &r) != nil || r.SchemaVersion != 2 {
		os.Exit(2)
	}
	f, err := os.OpenFile(os.Getenv("REVIEW_LOG"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		os.Exit(2)
	}
	f.Write(append(data, '\n'))
	f.Close()
	mode := os.Getenv("REVIEW_MODE")
	if r.Operation == "capabilities" {
		if mode == "legacy" {
			fmt.Print(`{"schema_version":1}`)
			return
		}
		capabilities := []string{"checked_combined_ff_v1"}
		if mode == "unsupported" {
			capabilities = []string{}
		}
		json.NewEncoder(os.Stdout).Encode(map[string]any{"schema_version": 2, "request_id": r.RequestID, "operation": r.Operation,
			"target_id": os.Getenv("REVIEW_TARGET"), "capabilities": capabilities, "message": "disposable local adapter"})
		return
	}
	// The token represents one approval/check revision of the entire bound tuple.
	binding, _ := json.Marshal([]any{r.TargetID, r.TargetRef, r.ExpectedTargetHead, r.SourceRef, r.CandidateHead, r.Check})
	token := fmt.Sprintf("%x", sha256.Sum256(binding))
	result := map[string]any{"schema_version": 2, "request_id": r.RequestID, "operation": r.Operation, "state": "applied",
		"target_id": r.TargetID, "change_id": r.ChangeID, "source_ref": r.SourceRef, "target_ref": r.TargetRef,
		"source_head": r.CandidateHead, "target_head": r.ExpectedTargetHead, "checks": "pass", "approval": "approved",
		"approval_token": token, "merge_head": nil, "message": "disposable local result"}
	switch r.Operation {
	case "publish_branch":
		if record := os.Getenv("REVIEW_EVENT_RECORD"); record != "" {
			f, err := os.OpenFile(record, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
			if err != nil {
				os.Exit(2)
			}
			fmt.Fprintln(f, "publish", r.CandidateHead)
			f.Close()
		}
		if mode == "schema-zero" {
			result["schema_version"] = 0
			break
		}
		if mode == "oversize" {
			fmt.Print(strings.Repeat("x", (1<<20)+1))
			time.Sleep(90 * time.Second)
			return
		}
		git("fetch", "--no-tags", r.Repository, r.CandidateHead)
		git("update-ref", r.SourceRef, r.CandidateHead, strings.Repeat("0", len(r.CandidateHead)))
		if mode == "publish-lost" {
			os.Exit(9)
		}
	case "open_change":
		result["change_id"] = "combined-change"
	case "inspect_change":
		log, _ := os.ReadFile(os.Getenv("REVIEW_LOG"))
		second := strings.Count(string(log), `"operation":"inspect_change"`) == 2
		if mode == "pending" {
			result["approval"], result["approval_token"] = "pending", nil
		}
		if second {
			switch mode {
			case "target-drift":
				git("update-ref", r.TargetRef, r.CandidateHead, r.ExpectedTargetHead)
			case "candidate-drift":
				git("update-ref", r.SourceRef, r.ExpectedTargetHead, r.CandidateHead)
			case "approval-drift":
				result["approval_token"] = token + "-revoked"
			case "checks-drift":
				result["checks"] = "fail"
			case "identity-drift":
				result["target_id"] = "another-target"
			case "local-worktree":
				os.WriteFile(filepath.Join(r.Repository, "adapter-drift.txt"), []byte("retain me"), 0o600)
			case "local-config":
				exec.Command("git", "-C", r.Repository, "config", "adapter.drift", "yes").Run()
			case "local-ref":
				exec.Command("git", "-C", r.Repository, "update-ref", "refs/tags/adapter-drift", r.CandidateHead).Run()
			case "source-drift":
				os.WriteFile(filepath.Join(os.Getenv("REVIEW_SOURCE"), "adapter-drift.txt"), []byte("retain me"), 0o600)
			case "check-drift":
				var check struct{ Evidence struct{ Path string } }
				json.Unmarshal(r.Check, &check)
				f, _ := os.OpenFile(check.Evidence.Path, os.O_APPEND|os.O_WRONLY, 0)
				if f != nil {
					f.Write([]byte("drift"))
					f.Close()
				}
			}
		}
		result["target_head"] = git("rev-parse", r.TargetRef)
		result["source_head"] = git("rev-parse", r.SourceRef)
	case "merge_change":
		if mode == "merge-approval-drift" {
			token += "-revoked"
		}
		if mode == "merge-candidate-drift" {
			git("update-ref", r.SourceRef, r.ExpectedTargetHead, r.CandidateHead)
		}
		if mode == "merge-target-drift" {
			git("update-ref", r.TargetRef, r.CandidateHead, r.ExpectedTargetHead)
		}
		if mode == "merge-refused" || r.ApprovalToken == nil || *r.ApprovalToken != token || git("rev-parse", r.TargetRef) != r.ExpectedTargetHead || git("rev-parse", r.SourceRef) != r.CandidateHead {
			result["state"] = "refused"
			break
		}
		git("update-ref", r.TargetRef, r.CandidateHead, r.ExpectedTargetHead)
		result["merge_head"] = r.CandidateHead
		switch mode {
		case "unknown", "cli-unknown", "cli-unknown-output":
			result["state"] = "unknown"
		case "lost":
			os.Exit(9)
		case "malformed":
			fmt.Print(`{`)
			return
		case "wrong-merge-head":
			result["merge_head"] = r.ExpectedTargetHead
		case "timeout":
			time.Sleep(90 * time.Second)
		case "after-merge-drift":
			os.WriteFile(filepath.Join(os.Getenv("REVIEW_SOURCE"), "adapter-drift.txt"), []byte("retain me"), 0o600)
		}
		if mode == "cli-applied-output" || mode == "cli-unknown-output" {
			os.WriteFile(os.Getenv("REVIEW_RESULT"), []byte("collision"), 0o600)
		}
		if mode == "cli-recovery" {
			root := filepath.Join(os.Getenv("REVIEW_SOURCE"), ".git", "taskrail", "transactions", strings.Repeat("a", 32))
			os.MkdirAll(root, 0o700)
			os.WriteFile(filepath.Join(root, "journal.json"), []byte("{}"), 0o600)
		}
	default:
		os.Exit(2)
	}
	json.NewEncoder(os.Stdout).Encode(result)
}
