package taskrail

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	"github.com/tessariq/taskrail/internal/durablefs"
)

var maxReviewShowBytes = int64(^uint64(0)>>1) - 1

// ReviewShowResult is the exact persisted content of one durable review, named
// only by its logical path so active local storage stays an implementation detail.
type ReviewShowResult struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
}

// ReviewShow reads one historical durable review through the active storage
// context. It deliberately validates only containment and filesystem safety, not
// the review's current schema, prompts, or selected product roots.
func (s *Service) ReviewShow(logicalPath string) (ReviewShowResult, error) {
	if err := s.paths.ensureStorageCapability(); err != nil {
		return ReviewShowResult{}, err
	}
	if err := validateDurableReviewPath(logicalPath, s.paths.LogicalPlanningDir); err != nil {
		return ReviewShowResult{}, WithMachineErrorCode(MachineCodePathBlocked, err)
	}
	return stableRead(func() (ReviewShowResult, error) {
		data, _, err := durablefs.ReadFile(s.paths.StorageRoot, logicalPath, maxReviewShowBytes)
		if err != nil {
			code := MachineCodePathBlocked
			if errors.Is(err, fs.ErrNotExist) {
				code = MachineCodeReviewNotFound
			} else if errors.Is(err, durablefs.ErrConflict) {
				code = MachineCodeWriteConflict
			}
			return ReviewShowResult{}, WithMachineErrorCode(code,
				fmt.Errorf("read durable review %s: %w", logicalPath, fsCause(err)))
		}
		digest := sha256.Sum256(data)
		return ReviewShowResult{Path: logicalPath, Content: string(data), SHA256: fmt.Sprintf("%x", digest)}, nil
	})
}

func validateDurableReviewPath(logicalPath, planningDir string) error {
	if filepath.ToSlash(logicalPath) != logicalPath || path.Clean(logicalPath) != logicalPath {
		return fmt.Errorf("review path must be canonical")
	}
	prefix := path.Join(planningDir, "reviews") + "/"
	if !strings.HasPrefix(logicalPath, prefix) {
		return fmt.Errorf("review path is outside configured durable review roots")
	}
	parts := strings.Split(strings.TrimPrefix(logicalPath, prefix), "/")
	if len(parts) == 0 {
		return fmt.Errorf("review path is outside configured durable review roots")
	}
	switch parts[0] {
	case "spec", "task", "decomposition":
		if len(parts) >= 4 && parts[1] != "" && parts[2] != "" {
			return nil
		}
	case "workflow-adversarial":
		if len(parts) >= 2 {
			return nil
		}
	}
	return fmt.Errorf("review path is outside configured durable review roots")
}
