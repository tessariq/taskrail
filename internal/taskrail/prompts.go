package taskrail

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"unicode/utf8"

	"github.com/tessariq/taskrail/internal/durablefs"
)

const promptReplacementLimit = 256 * 1024

//go:embed prompts/v1/*.md
var builtinPrompts embed.FS

type promptDefinition struct {
	id       string
	contract string
	asset    string
}

var promptRegistry = []promptDefinition{
	{"task-implementation", "v1", "prompts/v1/task-implementation.md"},
	{"task-authoring", "v1", "prompts/v1/task-authoring.md"},
	{"task-review", "v1", "prompts/v1/task-review.md"},
	{"spec-consistency", "v1", "prompts/v1/spec-consistency.md"},
	{"spec-gaps", "v1", "prompts/v1/spec-gaps.md"},
	{"spec-additions", "v1", "prompts/v1/spec-additions.md"},
	{"spec-adversarial", "v1", "prompts/v1/spec-adversarial.md"},
	{"task-decomposition", "v1", "prompts/v1/task-decomposition.md"},
	{"task-decomposition-adversarial", "v1", "prompts/v1/task-decomposition-adversarial.md"},
	{"workflow-adversarial", "v1", "prompts/v1/workflow-adversarial.md"},
}

type PromptListEntry struct {
	ID              string  `json:"id"`
	ContractVersion string  `json:"contract_version"`
	Source          string  `json:"source"`
	ReplacementPath *string `json:"replacement_path"`
}

type PromptListResult struct {
	Prompts []PromptListEntry `json:"prompts"`
}

type PromptShowInput struct {
	ID       string
	Contract string
	Builtin  bool
}

type PromptContentResult struct {
	ID              string  `json:"id"`
	ContractVersion string  `json:"contract_version"`
	Source          string  `json:"source"`
	ReplacementPath *string `json:"replacement_path"`
	Content         string  `json:"content"`
	SHA256          string  `json:"sha256"`
	TemplateSHA256  string  `json:"template_sha256"`
}

// PromptList reports every retained embedded prompt pair in registry order. It
// validates committed replacements before reporting their source, so an invalid
// local file cannot be mistaken for an absent one.
func (s *Service) PromptList() (PromptListResult, error) {
	if err := s.paths.ensureStorageCapability(); err != nil {
		return PromptListResult{}, err
	}
	return stableRead(s.promptListSnapshot)
}

func (s *Service) promptListSnapshot() (PromptListResult, error) {
	replacements, err := s.committedPromptReplacements(false)
	if err != nil {
		return PromptListResult{}, err
	}
	result := PromptListResult{Prompts: make([]PromptListEntry, 0, len(promptRegistry))}
	for _, definition := range promptRegistry {
		entry := PromptListEntry{ID: definition.id, ContractVersion: definition.contract, Source: "builtin"}
		if replacement, ok := replacements[promptKey(definition.id, definition.contract)]; ok {
			entry.Source = "replacement"
			entry.ReplacementPath = stringPtr(replacement.logicalPath)
		}
		result.Prompts = append(result.Prompts, entry)
	}
	return result, nil
}

// PromptShow returns exact template bytes, resolving only a complete committed
// replacement. Contextual rendering is deliberately a later command surface.
func (s *Service) PromptShow(input PromptShowInput) (PromptContentResult, error) {
	if err := s.paths.ensureStorageCapability(); err != nil {
		return PromptContentResult{}, err
	}
	definition, err := promptDefinitionFor(input.ID, input.Contract)
	if err != nil {
		return PromptContentResult{}, err
	}
	return stableRead(func() (PromptContentResult, error) {
		return s.promptShowSnapshot(definition, input.Builtin)
	})
}

func (s *Service) promptShowSnapshot(definition promptDefinition, builtinOnly bool) (PromptContentResult, error) {
	builtin, err := builtinPrompts.ReadFile(definition.asset)
	if err != nil {
		return PromptContentResult{}, fmt.Errorf("read embedded prompt %s: %w", definition.id, err)
	}
	data := builtin
	source := "builtin"
	var replacementPath *string
	if !builtinOnly {
		replacements, err := s.committedPromptReplacements(true)
		if err != nil {
			return PromptContentResult{}, err
		}
		if replacement, ok := replacements[promptKey(definition.id, definition.contract)]; ok {
			data = replacement.data
			source = "replacement"
			replacementPath = stringPtr(replacement.logicalPath)
		}
	}
	digest := promptDigest(data)
	return PromptContentResult{
		ID: definition.id, ContractVersion: definition.contract, Source: source,
		ReplacementPath: replacementPath, Content: string(data), SHA256: digest,
		TemplateSHA256: digest,
	}, nil
}

func promptDefinitionFor(id, contract string) (promptDefinition, error) {
	for _, definition := range promptRegistry {
		if definition.id == id && (contract == "" || definition.contract == contract) {
			return definition, nil
		}
	}
	return promptDefinition{}, WithMachineErrorCode(MachineCodePromptNotFound,
		fmt.Errorf("prompt %q with contract %q is not available", id, contract))
}

type promptReplacement struct {
	logicalPath string
	data        []byte
}

func (s *Service) committedPromptReplacements(allowPathBlocked bool) (map[string]promptReplacement, error) {
	tree, err := durablefs.ObserveTree(s.paths.ManagedRoot, s.paths.LogicalPromptsDir)
	if err != nil {
		return nil, promptReadError("inspect prompt replacements", err, allowPathBlocked)
	}
	if !tree.Present {
		return map[string]promptReplacement{}, nil
	}
	replacements := make(map[string]promptReplacement, len(tree.Entries))
	for _, entry := range tree.Entries {
		if entry.Path == "v1" && entry.Directory {
			continue
		}
		if !entry.Directory && path.Dir(entry.Path) == "v1" {
			filename := path.Base(entry.Path)
			definition, ok := promptDefinitionForFilename(filename, "v1")
			if !ok {
				for _, known := range promptRegistry {
					if known.contract == "v1" && samePortableName(filename, known.id+".md") {
						return nil, promptPathError(allowPathBlocked, "prompt replacement path alias %q", path.Join(s.paths.LogicalPromptsDir, entry.Path))
					}
				}
				return nil, WithMachineErrorCode(MachineCodePromptInvalid, fmt.Errorf("unknown prompt replacement %q", path.Join(s.paths.LogicalPromptsDir, entry.Path)))
			}
			logical := path.Join(s.paths.LogicalPromptsDir, entry.Path)
			data, _, err := durablefs.ReadFile(s.paths.ManagedRoot, logical, promptReplacementLimit+1)
			if err != nil {
				return nil, promptReadError("read prompt replacement "+logical, err, allowPathBlocked)
			}
			if err := validatePromptReplacement(data); err != nil {
				return nil, WithMachineErrorCode(MachineCodePromptInvalid, fmt.Errorf("invalid prompt replacement %s: %w", logical, err))
			}
			replacements[promptKey(definition.id, definition.contract)] = promptReplacement{logicalPath: logical, data: data}
			continue
		}
		if entry.Path == "v1" && !entry.Directory {
			return nil, promptPathError(allowPathBlocked, "prompt contract %q is not a directory", s.paths.LogicalPromptsDir+"/v1")
		}
		if path.Dir(entry.Path) == "." && samePortableName(entry.Path, "v1") {
			return nil, promptPathError(allowPathBlocked, "prompt contract path alias %q", path.Join(s.paths.LogicalPromptsDir, entry.Path))
		}
		return nil, WithMachineErrorCode(MachineCodePromptInvalid, fmt.Errorf("unknown prompt contract entry %q", path.Join(s.paths.LogicalPromptsDir, entry.Path)))
	}
	return replacements, nil
}

func promptReadError(action string, err error, allowPathBlocked bool) error {
	code := MachineCodePromptInvalid
	switch {
	case allowPathBlocked && (errors.Is(err, durablefs.ErrAlias) || errors.Is(err, durablefs.ErrInvalidPath) || errors.Is(err, durablefs.ErrNotRegular)):
		code = MachineCodePathBlocked
	case errors.Is(err, durablefs.ErrUnsupported):
		code = MachineCodeUnsupported
	}
	return WithMachineErrorCode(code, fmt.Errorf("%s: %w", action, err))
}

func promptPathError(allowPathBlocked bool, format string, a ...any) error {
	code := MachineCodePromptInvalid
	if allowPathBlocked {
		code = MachineCodePathBlocked
	}
	return WithMachineErrorCode(code, fmt.Errorf(format, a...))
}

func promptDefinitionForFilename(filename, contract string) (promptDefinition, bool) {
	for _, definition := range promptRegistry {
		if definition.contract == contract && filename == definition.id+".md" {
			return definition, true
		}
	}
	return promptDefinition{}, false
}

func validatePromptReplacement(data []byte) error {
	switch {
	case len(data) == 0:
		return fmt.Errorf("replacement is empty")
	case len(data) > promptReplacementLimit:
		return fmt.Errorf("replacement exceeds %d bytes", promptReplacementLimit)
	case bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}):
		return fmt.Errorf("replacement has a UTF-8 BOM")
	case !utf8.Valid(data):
		return fmt.Errorf("replacement is not valid UTF-8")
	}
	return nil
}

func promptKey(id, contract string) string { return contract + "/" + id }

func promptDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func stringPtr(value string) *string { return &value }
