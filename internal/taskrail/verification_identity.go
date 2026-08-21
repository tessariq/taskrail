package taskrail

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const verificationIDAttempts = 16

func randomVerificationID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate verification id: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

// freshVerificationID reserves no state; the mutation lock taken by Verify
// immediately afterward closes the gap before publication.
func (s *Service) freshVerificationID(state *State, tasks []*Task) (string, error) {
	used, err := s.usedVerificationIDs(state, tasks)
	if err != nil {
		return "", err
	}
	if delegatedInvocation() {
		id := os.Getenv("TASKRAIL_DELEGATION_ID")
		if !lowerHex32.MatchString(id) {
			return "", fmt.Errorf("generate verification id: delegation id must be lower-case 32-hex")
		}
		if _, exists := used[id]; exists {
			return "", fmt.Errorf("generate verification id: delegation id is already present in preflight evidence")
		}
		return id, nil
	}
	generator := s.verificationID
	if generator == nil {
		generator = randomVerificationID
	}
	for range verificationIDAttempts {
		id, err := generator()
		if err != nil {
			return "", fmt.Errorf("generate verification id: %w", err)
		}
		if !lowerHex32.MatchString(id) {
			return "", fmt.Errorf("generate verification id: generated value must be lower-case 32-hex")
		}
		if _, exists := used[id]; !exists {
			return id, nil
		}
	}
	return "", fmt.Errorf("generate verification id: could not produce an identity absent from preflight evidence")
}

func (s *Service) usedVerificationIDs(state *State, tasks []*Task) (map[string]struct{}, error) {
	used := make(map[string]struct{}, len(tasks)*2)
	for _, task := range tasks {
		meta := task.Frontmatter.CompletionVerificationMetadata
		for _, id := range []string{meta.LastVerificationID, meta.LastVerificationPreviousID} {
			if id != "" {
				used[id] = struct{}{}
			}
		}
	}
	for _, id := range []string{state.Frontmatter.LastVerificationID, state.Frontmatter.LastVerificationPreviousID} {
		if id != "" {
			used[id] = struct{}{}
		}
	}
	for _, id := range verificationIDsInText(state.Frontmatter.LastVerificationResult) {
		used[id] = struct{}{}
	}
	if !dirExists(s.paths.VerifyDir) {
		return used, nil
	}
	err := filepath.WalkDir(s.paths.VerifyDir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if len(name) >= 32 {
				if id := name[len(name)-32:]; lowerHex32.MatchString(id) {
					used[id] = struct{}{}
				}
			}
			return nil
		}
		if entry.Name() != "report.json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, id := range verificationIDsInText(string(data)) {
			used[id] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan verification artifacts: %w", fsCause(err))
	}
	return used, nil
}

func verificationIDsInText(text string) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return (r < '0' || r > '9') && (r < 'a' || r > 'f')
	})
	ids := make([]string, 0, len(fields))
	for _, field := range fields {
		if lowerHex32.MatchString(field) {
			ids = append(ids, field)
		}
	}
	return ids
}

func optionalVerificationID(id string) *string {
	if id == "" {
		return nil
	}
	return &id
}
