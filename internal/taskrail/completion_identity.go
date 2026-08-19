package taskrail

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

const completionIDAttempts = 16

func randomCompletionID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate completion id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

func (s *Service) freshCompletionID(tasks []*Task) (string, error) {
	used := make(map[string]struct{}, len(tasks)*4)
	for _, task := range tasks {
		meta := task.Frontmatter.CompletionVerificationMetadata
		for _, id := range []string{meta.CompletionID, meta.LastVerificationID, meta.LastVerificationPreviousID, meta.LastVerifiedCompletionID} {
			if id != "" {
				used[id] = struct{}{}
			}
		}
	}
	generator := s.completionID
	if generator == nil {
		generator = randomCompletionID
	}
	for range completionIDAttempts {
		id, err := generator()
		if err != nil {
			return "", fmt.Errorf("generate completion id: %w", err)
		}
		if !lowerHex32.MatchString(id) {
			return "", fmt.Errorf("generate completion id: generated value must be lower-case 32-hex")
		}
		if _, exists := used[id]; !exists {
			return id, nil
		}
	}
	return "", fmt.Errorf("generate completion id: could not produce an identity absent from preflight metadata")
}
