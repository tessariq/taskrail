package taskrail

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"strings"
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
	{"loop-integration", "v1", "prompts/v1/loop-integration.md"},
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

// PromptRenderCommandInput contains the complete command context before it is
// resolved into strictly declared template values.
type PromptRenderCommandInput struct {
	ID                   string
	Contract             string
	Task                 string
	Spec                 string
	SpecReviewPath       string
	DraftPath            string
	TracePath            string
	MemoryPath           string
	ReviewPath           string
	MaxReviewRounds      int
	MaxReviewRoundsIsSet bool
}

type promptTransientRequirement struct {
	role         string
	proposalType string
}

var promptTokenDeclarations = map[string][]string{
	"task-implementation":            {"TASK_ID", "TASK_PATH", "ACTIVE_SPEC_VERSION", "ACTIVE_SPEC_PATH", "IMPLEMENTATION_REVIEW_MAX_ROUNDS", "STORAGE_MODE"},
	"task-authoring":                 {"TASK_ID", "TASK_PATH", "SPEC_VERSION", "SPEC_PATH"},
	"task-review":                    {"TASK_ID", "TASK_PATH", "SPEC_VERSION", "SPEC_PATH", PromptContextReviewPath},
	"spec-consistency":               {"SPEC_VERSION", "SPEC_PATH", PromptContextReviewPath},
	"spec-gaps":                      {"SPEC_VERSION", "SPEC_PATH", PromptContextReviewPath},
	"spec-additions":                 {"SPEC_VERSION", "SPEC_PATH", PromptContextReviewPath},
	"spec-adversarial":               {"SPEC_VERSION", "SPEC_PATH", PromptContextReviewPath},
	"task-decomposition":             {"SPEC_VERSION", "SPEC_PATH", "SPEC_REVIEW_PATH", PromptContextTracePath, PromptContextDraftPath},
	"task-decomposition-adversarial": {"SPEC_VERSION", "SPEC_PATH", "SPEC_REVIEW_PATH", PromptContextTracePath, PromptContextDraftPath, PromptContextReviewPath},
	"workflow-adversarial":           {"SPEC_VERSION", "SPEC_PATH", "MEMORY_PATH", PromptContextReviewPath},
	"loop-integration":               {"INTEGRATION_ROLE", "TASK_ID", "TASK_PATH", "SPEC_VERSION", "SPEC_PATH", "BASE_HEAD", "CURRENT_HEAD", "CANDIDATE_HEAD", "CONFLICT_PATHS", "WORKER_EVIDENCE_PATH", "STORAGE_MODE"},
}

var promptTransientRequirementsByID = map[string][]promptTransientRequirement{
	"task-review":                    {{PromptContextReviewPath, "task"}},
	"spec-consistency":               {{PromptContextReviewPath, "spec"}},
	"spec-gaps":                      {{PromptContextReviewPath, "spec"}},
	"spec-additions":                 {{PromptContextReviewPath, "spec"}},
	"spec-adversarial":               {{PromptContextReviewPath, "spec"}},
	"task-decomposition":             {{PromptContextTracePath, "decomposition"}, {PromptContextDraftPath, "decomposition"}},
	"task-decomposition-adversarial": {{PromptContextTracePath, "decomposition"}, {PromptContextDraftPath, "decomposition"}, {PromptContextReviewPath, "decomposition"}},
	"workflow-adversarial":           {{PromptContextReviewPath, "workflow-adversarial"}},
}

// PromptRenderInput is the storage-neutral data needed to render one template.
type PromptRenderInput struct {
	Template       []byte
	DeclaredTokens []string
	Values         map[string]string
}

// PromptRenderResult contains the rendered content and hashes of both stages.
type PromptRenderResult struct {
	Content        string
	SHA256         string
	TemplateSHA256 string
}

// RenderPrompt validates a v1 prompt template before making one non-recursive
// substitution pass. Keeping this independent of prompt resolution lets every
// future context loader share the same strict rendering contract.
func RenderPrompt(input PromptRenderInput) (PromptRenderResult, error) {
	if err := validatePromptTemplate(input.Template); err != nil {
		return PromptRenderResult{}, err
	}
	declared := make(map[string]struct{}, len(input.DeclaredTokens))
	for _, name := range input.DeclaredTokens {
		if !validPromptTokenName(name) {
			return PromptRenderResult{}, fmt.Errorf("invalid declared prompt token %q", name)
		}
		if _, exists := declared[name]; exists {
			return PromptRenderResult{}, fmt.Errorf("duplicate declared prompt token %q", name)
		}
		declared[name] = struct{}{}
	}
	if len(input.Values) != len(declared) {
		return PromptRenderResult{}, fmt.Errorf("prompt values do not exactly match declared tokens")
	}
	for name, value := range input.Values {
		if _, ok := declared[name]; !ok {
			return PromptRenderResult{}, fmt.Errorf("value supplied for undeclared prompt token %q", name)
		}
		if !utf8.ValidString(value) {
			return PromptRenderResult{}, fmt.Errorf("value for prompt token %q is not valid UTF-8", name)
		}
	}
	if err := validatePromptTokenReferences(input.Template, declared); err != nil {
		return PromptRenderResult{}, err
	}

	var rendered strings.Builder
	for offset := 0; offset < len(input.Template); {
		start := bytes.Index(input.Template[offset:], []byte("{{"))
		if start < 0 {
			rendered.Write(input.Template[offset:])
			break
		}
		start += offset
		rendered.Write(input.Template[offset:start])
		end := bytes.Index(input.Template[start+2:], []byte("}}"))
		end += start + 2
		rendered.WriteString(input.Values[string(input.Template[start+2:end])])
		offset = end + 2
	}

	content := rendered.String()
	return PromptRenderResult{
		Content: content, SHA256: promptDigest([]byte(content)), TemplateSHA256: promptDigest(input.Template),
	}, nil
}

func validatePromptTemplate(template []byte) error {
	switch {
	case len(template) > promptReplacementLimit:
		return fmt.Errorf("prompt template exceeds %d bytes", promptReplacementLimit)
	case bytes.HasPrefix(template, []byte{0xef, 0xbb, 0xbf}):
		return fmt.Errorf("prompt template has a UTF-8 BOM")
	case !utf8.Valid(template):
		return fmt.Errorf("prompt template is not valid UTF-8")
	}
	return nil
}

func validatePromptTokenReferences(template []byte, declared map[string]struct{}) error {
	for offset := 0; offset < len(template); {
		start := bytes.Index(template[offset:], []byte("{{"))
		if start < 0 {
			return nil
		}
		start += offset
		end := bytes.Index(template[start+2:], []byte("}}"))
		if end < 0 {
			return fmt.Errorf("unterminated prompt token %q", string(template[start:]))
		}
		end += start + 2
		name := string(template[start+2 : end])
		if !validPromptTokenName(name) {
			return fmt.Errorf("malformed prompt token %q", string(template[start:end+2]))
		}
		if _, ok := declared[name]; !ok {
			return fmt.Errorf("undeclared prompt token %q", name)
		}
		offset = end + 2
	}
	return nil
}

func validPromptTokenName(name string) bool {
	if len(name) == 0 || name[0] < 'A' || name[0] > 'Z' {
		return false
	}
	for i := 1; i < len(name); i++ {
		if (name[i] < 'A' || name[i] > 'Z') && (name[i] < '0' || name[i] > '9') && name[i] != '_' {
			return false
		}
	}
	return true
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
	replacements, err := s.promptReplacements(false)
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

// PromptRender resolves every managed and transient subject twice before
// publishing, so one rendered prompt cannot combine mismatched snapshots.
func (s *Service) PromptRender(input PromptRenderCommandInput) (PromptContentResult, error) {
	if err := s.paths.ensureStorageCapability(); err != nil {
		return PromptContentResult{}, err
	}
	definition, err := promptDefinitionFor(input.ID, input.Contract)
	if err != nil {
		return PromptContentResult{}, err
	}
	if definition.id == "loop-integration" {
		return PromptContentResult{}, invalidArgumentsf("prompt %q is coordinator-only", definition.id)
	}
	managed := PromptManagedContextInput{
		ID: definition.id, Task: input.Task, Spec: input.Spec,
		SpecReviewPath: input.SpecReviewPath, MemoryPath: input.MemoryPath,
	}
	transient, rounds, err := validatePromptRenderInput(input, managed)
	if err != nil {
		return PromptContentResult{}, WithMachineErrorCode(MachineCodePromptInvalid, err)
	}
	type snapshot struct {
		result    PromptContentResult
		managed   PromptManagedContext
		transient TransientPromptPathAuthorization
	}
	resolved, err := stableRead(func() (snapshot, error) {
		template, err := s.promptShowSnapshot(definition, false)
		if err != nil {
			return snapshot{}, err
		}
		context, err := s.resolvePromptManagedContext(managed, promptManagedContextRequirementsByID[definition.id])
		if err != nil {
			return snapshot{}, err
		}
		authorization := TransientPromptPathAuthorization{}
		if len(transient) > 0 {
			authorization, err = s.AuthorizeTransientPromptPaths(transient)
			if err != nil {
				return snapshot{}, err
			}
		}
		values := maps.Clone(context.Values)
		if definition.id == "task-implementation" {
			values["IMPLEMENTATION_REVIEW_MAX_ROUNDS"] = fmt.Sprintf("%d", rounds)
		}
		for _, authorized := range authorization.Paths {
			values[authorized.Role] = authorized.Path
		}
		rendered, err := RenderPrompt(PromptRenderInput{
			Template: []byte(template.Content), DeclaredTokens: promptTokenDeclarations[definition.id], Values: values,
		})
		if err != nil {
			return snapshot{}, WithMachineErrorCode(MachineCodePromptInvalid, err)
		}
		template.Content = rendered.Content
		template.SHA256 = rendered.SHA256
		template.TemplateSHA256 = rendered.TemplateSHA256
		return snapshot{result: template, managed: context, transient: authorization}, nil
	})
	if err != nil {
		return PromptContentResult{}, normalizePromptRenderError(err)
	}
	return resolved.result, nil
}

func normalizePromptRenderError(err error) error {
	switch MachineFailureFor(err).Code {
	case MachineCodeReviewNotFound, MachineCodeWriteConflict:
		return WithMachineErrorCode(MachineCodePromptInvalid, err)
	default:
		return err
	}
}

func validatePromptRenderInput(input PromptRenderCommandInput, managed PromptManagedContextInput) ([]TransientPromptPath, int, error) {
	if err := validatePromptManagedContextInput(managed, promptManagedContextRequirementsByID[input.ID]); err != nil {
		return nil, 0, err
	}
	rounds := 1
	if input.ID != "task-implementation" && input.MaxReviewRoundsIsSet {
		return nil, 0, invalidArgumentsf("--max-review-rounds is only accepted for task-implementation")
	}
	if input.ID == "task-implementation" && input.MaxReviewRoundsIsSet {
		if input.MaxReviewRounds < 1 || input.MaxReviewRounds > 2 {
			return nil, 0, invalidArgumentsf("--max-review-rounds must be between 1 and 2")
		}
		rounds = input.MaxReviewRounds
	}
	values := map[string]string{
		PromptContextReviewPath: input.ReviewPath,
		PromptContextDraftPath:  input.DraftPath,
		PromptContextTracePath:  input.TracePath,
	}
	requirements := promptTransientRequirementsByID[input.ID]
	paths := make([]TransientPromptPath, 0, len(requirements))
	for _, requirement := range requirements {
		value := values[requirement.role]
		if value == "" {
			return nil, 0, invalidArgumentsf("prompt %q requires --%s", input.ID, strings.ToLower(strings.TrimSuffix(requirement.role, "_PATH")))
		}
		paths = append(paths, TransientPromptPath{Role: requirement.role, ProposalType: requirement.proposalType, Path: value})
		delete(values, requirement.role)
	}
	for role, value := range values {
		if value != "" {
			return nil, 0, invalidArgumentsf("prompt %q does not accept --%s", input.ID, strings.ToLower(strings.TrimSuffix(role, "_PATH")))
		}
	}
	return paths, rounds, nil
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
		replacements, err := s.promptReplacements(true)
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

func (s *Service) promptReplacements(allowPathBlocked bool) (map[string]promptReplacement, error) {
	root, replacementDir := s.paths.ManagedRoot, s.paths.LogicalPromptsDir
	if s.paths.Storage.Mode == StorageLocal {
		if err := s.validateLocalPromptReplacementSource(allowPathBlocked); err != nil {
			return nil, err
		}
		root, replacementDir = s.paths.StorageRoot, "prompts"
	}
	tree, err := durablefs.ObserveTree(root, replacementDir)
	if err != nil {
		return nil, promptReadError("inspect prompt replacements", err, allowPathBlocked)
	}
	if !tree.Present {
		return map[string]promptReplacement{}, nil
	}
	replacements := make(map[string]promptReplacement, len(tree.Entries))
	for _, entry := range tree.Entries {
		if s.paths.Storage.Mode == StorageLocal && !entry.Directory {
			physical := filepath.Join(s.paths.PromptsDir, filepath.FromSlash(entry.Path))
			if err := s.validateLocalPromptReplacementPath(filepath.ToSlash(relPath(s.paths.WorktreeRoot, physical)), allowPathBlocked); err != nil {
				return nil, err
			}
		}
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
			data, _, err := durablefs.ReadFile(root, path.Join(replacementDir, entry.Path), promptReplacementLimit+1)
			if err != nil {
				if errors.Is(err, durablefs.ErrUnsupported) {
					return nil, WithMachineErrorCode(MachineCodePromptInvalid, fmt.Errorf("invalid prompt replacement %s: replacement exceeds %d bytes", logical, promptReplacementLimit))
				}
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

// validateLocalPromptReplacementSource keeps overlay ownership distinct from the
// committed namespace before local bytes can affect a prompt binding.
func (s *Service) validateLocalPromptReplacementSource(allowPathBlocked bool) error {
	committed, err := durablefs.ObserveTree(s.paths.ManagedRoot, s.paths.LogicalPromptsDir)
	if err != nil {
		return promptReadError("inspect committed prompt replacements", err, allowPathBlocked)
	}
	if committed.Present {
		return promptPathError(allowPathBlocked, "committed prompt replacements conflict with local storage")
	}
	localPath := filepath.ToSlash(relPath(s.paths.WorktreeRoot, s.paths.StorageRoot))
	if output, err := gitCommand(s.paths.WorktreeRoot, "ls-files", "--error-unmatch", "--", localPath); err == nil && strings.TrimSpace(output) != "" {
		return promptPathError(allowPathBlocked, "Git tracks local prompt replacements")
	}
	if _, err := gitCommand(s.paths.WorktreeRoot, "diff", "--cached", "--quiet", "--", localPath); err != nil {
		return promptPathError(allowPathBlocked, "Git index contains local prompt replacements")
	}
	return nil
}

func (s *Service) validateLocalPromptReplacementPath(localPath string, allowPathBlocked bool) error {
	if _, err := gitCommand(s.paths.WorktreeRoot, "check-ignore", "-q", "--no-index", localPath); err != nil {
		return promptPathError(allowPathBlocked, "local prompt replacement %q is not effectively ignored", localPath)
	}
	return nil
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
