package taskrail

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

// SkillEvalArtifactAnchor binds producer-local evidence to its original
// namespace without disclosing a producer-local path in the stage.
type SkillEvalArtifactAnchor struct {
	PathSHA256 string                  `json:"path_sha256"`
	Ancestors  []SkillEvalAncestorFact `json:"ancestors"`
}

// SkillEvalAncestorFact records the path's own type and the identity it resolves
// to. This admits stable caller-chosen symlinks but detects a renamed directory
// re-exposed through a link even when the target's identity is unchanged.
type SkillEvalAncestorFact struct {
	Type   uint32 `json:"type"`
	Device uint64 `json:"device"`
	Inode  uint64 `json:"inode"`
}

func skillEvalCaptureArtifactAnchor(root string) (*SkillEvalArtifactAnchor, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve artifact root: %w", err)
	}
	digest := sha256.Sum256([]byte("taskrail-skill-eval-anchor-v1\x00" + absolute))
	anchor := &SkillEvalArtifactAnchor{PathSHA256: hex.EncodeToString(digest[:])}
	for current := absolute; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return nil, fmt.Errorf("inspect artifact root ancestor: %w", err)
		}
		device, inode, err := skillEvalPathIdentity(current)
		if err != nil {
			return nil, fmt.Errorf("identify artifact root ancestor: %w", err)
		}
		anchor.Ancestors = append(anchor.Ancestors, SkillEvalAncestorFact{Type: uint32(info.Mode().Type()), Device: device, Inode: inode})
		if filepath.Dir(current) == current {
			return anchor, nil
		}
	}
}

func skillEvalValidateArtifactAnchor(root string, sealed *SkillEvalArtifactAnchor) error {
	// Legacy stages have no historical namespace observation. Preserve their
	// old component/digest validation, never fabricate a replacement pin.
	if sealed == nil {
		return nil
	}
	current, err := skillEvalCaptureArtifactAnchor(root)
	if err != nil {
		return err
	}
	if sealed.PathSHA256 != current.PathSHA256 || !slices.Equal(sealed.Ancestors, current.Ancestors) {
		return fmt.Errorf("artifact root confinement changed: sealed namespace or ancestor identity was replaced")
	}
	return nil
}

func skillEvalValidateExecutionAnchor(input SkillEvalRunInput) error {
	current, err := filepath.Abs(input.artifactRootInput)
	if err != nil || current != input.ArtifactRoot {
		return fmt.Errorf("artifact root confinement changed: relative root rebased during adapter run")
	}
	return skillEvalValidateArtifactAnchor(input.ArtifactRoot, input.artifactAnchor)
}
