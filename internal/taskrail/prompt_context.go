package taskrail

import (
	"fmt"
	"path"
	"path/filepath"
)

// PromptManagedContextInput identifies only managed prompt subjects. Transient
// proposal paths are intentionally resolved by the separate authorization layer.
type PromptManagedContextInput struct {
	ID             string
	Task           string
	Spec           string
	SpecReviewPath string
	MemoryPath     string
}

// PromptManagedContext contains logical placeholder values resolved through the
// active storage context. Callers combine it with authorized transient values.
type PromptManagedContext struct {
	Values map[string]string
}

type promptManagedContextRequirements struct {
	task       bool
	spec       bool
	specReview bool
	memory     bool
}

var promptManagedContextRequirementsByID = map[string]promptManagedContextRequirements{
	"task-implementation":            {task: true},
	"task-authoring":                 {task: true},
	"task-review":                    {task: true},
	"spec-consistency":               {spec: true},
	"spec-gaps":                      {spec: true},
	"spec-additions":                 {spec: true},
	"spec-adversarial":               {spec: true},
	"task-decomposition":             {spec: true, specReview: true},
	"task-decomposition-adversarial": {spec: true, specReview: true},
	"workflow-adversarial":           {spec: true, memory: true},
}

// ResolvePromptManagedContext resolves task, spec, and durable-review subjects
// through active storage and returns logical values only. It deliberately reads
// durable inputs even though their bytes are not prompt values, so a logical
// decoy cannot stand in for the active-storage subject.
func (s *Service) ResolvePromptManagedContext(input PromptManagedContextInput) (PromptManagedContext, error) {
	if err := s.paths.ensureStorageCapability(); err != nil {
		return PromptManagedContext{}, err
	}
	if _, err := promptDefinitionFor(input.ID, ""); err != nil {
		return PromptManagedContext{}, err
	}
	requirements := promptManagedContextRequirementsByID[input.ID]
	if err := validatePromptManagedContextInput(input, requirements); err != nil {
		return PromptManagedContext{}, err
	}
	return stableRead(func() (PromptManagedContext, error) {
		return s.resolvePromptManagedContext(input, requirements)
	})
}

func validatePromptManagedContextInput(input PromptManagedContextInput, requirements promptManagedContextRequirements) error {
	if requirements.task != (input.Task != "") {
		return invalidArgumentsf("prompt %q requires %s--task", input.ID, requiredPrefix(requirements.task))
	}
	if requirements.spec != (input.Spec != "") {
		return invalidArgumentsf("prompt %q requires %s--spec", input.ID, requiredPrefix(requirements.spec))
	}
	if requirements.specReview != (input.SpecReviewPath != "") {
		return invalidArgumentsf("prompt %q requires %s--spec-review", input.ID, requiredPrefix(requirements.specReview))
	}
	if requirements.memory != (input.MemoryPath != "") {
		return invalidArgumentsf("prompt %q requires %s--memory", input.ID, requiredPrefix(requirements.memory))
	}
	return nil
}

func requiredPrefix(required bool) string {
	if required {
		return ""
	}
	return "no "
}

func (s *Service) resolvePromptManagedContext(input PromptManagedContextInput, requirements promptManagedContextRequirements) (PromptManagedContext, error) {
	values := make(map[string]string)
	var spec SpecShowResult
	if requirements.task {
		task, err := s.TaskShow(input.Task)
		if err != nil {
			return PromptManagedContext{}, err
		}
		frontmatter, _, err := parseFrontmatter[TaskFrontmatter]([]byte(task.Content))
		if err != nil {
			return PromptManagedContext{}, fmt.Errorf("parse prompt task %s: %w", task.TaskPath, err)
		}
		if frontmatter.ID != task.TaskID {
			return PromptManagedContext{}, fmt.Errorf("prompt task identity changed while reading")
		}
		if _, err := s.validateSpecRef(frontmatter.SpecRef); err != nil {
			return PromptManagedContext{}, fmt.Errorf("validate prompt task spec_ref: %w", err)
		}
		specPath, _, _ := parseSpecRef(frontmatter.SpecRef)
		spec, err = s.resolvePromptSpec(filepath.ToSlash(specPath))
		if err != nil {
			return PromptManagedContext{}, err
		}
		values["TASK_ID"] = task.TaskID
		values["TASK_PATH"] = task.TaskPath
	}
	if requirements.spec {
		var err error
		spec, err = s.resolvePromptSpec(input.Spec)
		if err != nil {
			return PromptManagedContext{}, err
		}
	}
	if requirements.task && input.ID == "task-implementation" {
		active, err := s.resolveActivePromptSpec()
		if err != nil {
			return PromptManagedContext{}, err
		}
		values["ACTIVE_SPEC_VERSION"] = active.Version
		values["ACTIVE_SPEC_PATH"] = active.Path
		values["STORAGE_MODE"] = string(s.paths.Storage.Mode)
	} else if requirements.task || requirements.spec {
		values["SPEC_VERSION"] = spec.Version
		values["SPEC_PATH"] = spec.Path
	}
	if requirements.specReview {
		review, err := s.ReviewShow(input.SpecReviewPath)
		if err != nil {
			return PromptManagedContext{}, err
		}
		values["SPEC_REVIEW_PATH"] = review.Path
	}
	if requirements.memory {
		review, err := s.ReviewShow(input.MemoryPath)
		if err != nil {
			return PromptManagedContext{}, err
		}
		values["MEMORY_PATH"] = review.Path
	}
	return PromptManagedContext{Values: values}, nil
}

func (s *Service) resolvePromptSpec(subject string) (SpecShowResult, error) {
	list, err := s.SpecList()
	if err != nil {
		return SpecShowResult{}, err
	}
	for _, entry := range list.Specs {
		if subject == entry.Version || subject == entry.Path {
			return s.SpecShow(entry.Version, false)
		}
	}
	return SpecShowResult{}, invalidArgumentsf("spec %q is not a discoverable version or configured versioned path", subject)
}

func (s *Service) resolveActivePromptSpec() (SpecShowResult, error) {
	state, err := s.loadState()
	if err != nil {
		return SpecShowResult{}, err
	}
	spec, err := s.resolvePromptSpec(state.Frontmatter.ActiveSpecVersion)
	if err != nil {
		return SpecShowResult{}, err
	}
	if state.Frontmatter.ActiveSpecPath != spec.Path || path.Clean(spec.Path) != spec.Path {
		return SpecShowResult{}, fmt.Errorf("active spec path %q does not match discovered spec %q", state.Frontmatter.ActiveSpecPath, spec.Path)
	}
	return spec, nil
}
