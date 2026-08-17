package taskrail

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tessariq/taskrail/internal/repolock"
	"github.com/tessariq/taskrail/internal/repotx"
)

type DependencyOperation string

const (
	DependencyAdd    DependencyOperation = "add"
	DependencyRemove DependencyOperation = "remove"
)

type EditDependencyInput struct {
	TaskID       string
	DependencyID string
	Operation    DependencyOperation
	DryRun       bool
}

type DependencyResult struct {
	TaskID             string              `json:"task_id"`
	DependencyID       string              `json:"dependency_id"`
	Operation          DependencyOperation `json:"operation"`
	Applied            bool                `json:"applied"`
	DependenciesBefore []string            `json:"dependencies_before"`
	DependenciesAfter  []string            `json:"dependencies_after"`
	Validation         *ValidationResult   `json:"validation"`
}

// EditDependency changes exactly one dependency edge under the repository lock.
// The task candidate replaces only the dependencies field bytes; STATE.md is a
// generated projection and is published with it through one normal transaction.
func (s *Service) EditDependency(input EditDependencyInput) (result DependencyResult, err error) {
	if input.Operation != DependencyAdd && input.Operation != DependencyRemove {
		return result, invalidArgumentsf("dependency operation must be add or remove")
	}
	if input.TaskID == "" || input.DependencyID == "" {
		return result, invalidArgumentsf("task id and dependency id are required")
	}
	writer := dependencyTaskWriter(input.Operation)
	own, release, err := s.beginTaskWriterWrite(writer)
	if err != nil {
		return result, err
	}
	defer func() {
		if releaseErr := release(); releaseErr != nil && err == nil {
			err = releaseErr
		}
	}()

	state, tasks, err := s.loadStateAndTasks()
	if err != nil {
		return result, err
	}
	corpus, err := snapshotTaskCorpus(tasks)
	if err != nil {
		return result, err
	}
	baseline := s.validateInMemory(state, tasks)
	target, err := exactTaskByID(tasks, input.TaskID)
	if err != nil {
		return result, err
	}
	dependency, err := exactTaskByID(tasks, input.DependencyID)
	if err != nil {
		return result, err
	}
	if !isOpenStatus(target.Frontmatter.Status) {
		return result, WithMachineErrorCode(MachineCodeInvalidStatus,
			fmt.Errorf("task %s is %s: dependency editing requires live open work", input.TaskID, target.Frontmatter.Status))
	}

	before := slices.Clone(target.Frontmatter.Dependencies)
	after, err := editDependencyEdge(tasks, target, dependency, input.Operation)
	if err != nil {
		return result, err
	}
	previewTasks := withDependencies(tasks, target, after)
	validation := s.validateInMemory(state, previewTasks)
	if !validation.Valid {
		return result, WithMachineErrorCode(MachineCodeValidationFailed,
			fmt.Errorf("dependency candidate failed validation: %s", strings.Join(validation.Violations, "; ")))
	}
	result = DependencyResult{
		TaskID: input.TaskID, DependencyID: input.DependencyID, Operation: input.Operation,
		DependenciesBefore: nonNilStrings(before), DependenciesAfter: nonNilStrings(after),
		Validation: &validation,
	}
	if input.DryRun {
		return result, nil
	}

	taskBytes, err := os.ReadFile(target.Filename)
	if err != nil {
		return DependencyResult{}, fmt.Errorf("read task %s: %w", target.Path, fsCause(err))
	}
	taskCandidate, err := replaceDependenciesField(taskBytes, before, after)
	if err != nil {
		return DependencyResult{}, WithMachineErrorCode(MachineCodeRepositoryInvalid, err)
	}
	stateCandidate := *state
	stateCandidate.Frontmatter.UpdatedAt = timestamp(s.now())
	stateCandidate.Body = renderStateBody(stateCandidate.Frontmatter, previewTasks)

	validation, err = s.commitTaskWriter(own, writer, taskWriterLedger{
		state:     &stateCandidate,
		preview:   previewTasks,
		published: []repotx.Candidate{managedCandidate(target.Path, target.Filename, taskCandidate)},
		selected:  input.TaskID,
		tasks:     tasks,
		written:   []*Task{target},
		corpus:    corpus,
		baseline:  baseline,
		strict:    true,
	})
	if err != nil {
		return DependencyResult{}, err
	}
	result.Validation = &validation
	result.Applied = true
	return result, nil
}

func editDependencyEdge(tasks []*Task, target, dependency *Task, operation DependencyOperation) ([]string, error) {
	dependencies := slices.Clone(target.Frontmatter.Dependencies)
	index := slices.Index(dependencies, dependency.Frontmatter.ID)
	switch operation {
	case DependencyAdd:
		if target == dependency {
			return nil, WithMachineErrorCode(MachineCodeDependencyCycle, fmt.Errorf("task %s cannot depend on itself", target.Frontmatter.ID))
		}
		if index >= 0 {
			return nil, WithMachineErrorCode(MachineCodeDependencyExists, fmt.Errorf("task %s already depends on %s", target.Frontmatter.ID, dependency.Frontmatter.ID))
		}
		if dependency.Frontmatter.Status == "cancelled" {
			return nil, WithMachineErrorCode(MachineCodeCancelledDependency, fmt.Errorf("dependency %s is cancelled", dependency.Frontmatter.ID))
		}
		if dependencyReaches(tasks, dependency.Frontmatter.ID, target.Frontmatter.ID) {
			return nil, WithMachineErrorCode(MachineCodeDependencyCycle, fmt.Errorf("adding %s -> %s would create a dependency cycle", target.Frontmatter.ID, dependency.Frontmatter.ID))
		}
		return append(dependencies, dependency.Frontmatter.ID), nil
	case DependencyRemove:
		if index < 0 {
			return nil, WithMachineErrorCode(MachineCodeDependencyAbsent, fmt.Errorf("task %s does not depend on %s", target.Frontmatter.ID, dependency.Frontmatter.ID))
		}
		return append(dependencies[:index:index], dependencies[index+1:]...), nil
	default:
		return nil, invalidArgumentsf("dependency operation must be add or remove")
	}
}

func dependencyReaches(tasks []*Task, from, target string) bool {
	seen := map[string]bool{}
	var visit func(string) bool
	visit = func(id string) bool {
		if id == target {
			return true
		}
		if seen[id] {
			return false
		}
		seen[id] = true
		task, ok := taskByID(tasks, id)
		if !ok {
			return false
		}
		for _, dependency := range task.Frontmatter.Dependencies {
			if visit(dependency) {
				return true
			}
		}
		return false
	}
	return visit(from)
}

func withDependencies(tasks []*Task, target *Task, dependencies []string) []*Task {
	preview := slices.Clone(tasks)
	edited := *target
	edited.Frontmatter.Dependencies = slices.Clone(dependencies)
	for i, task := range preview {
		if task == target {
			preview[i] = &edited
			break
		}
	}
	return preview
}

func nonNilStrings(values []string) []string { return append([]string{}, values...) }

func delegatedInvocation() bool {
	for _, name := range []string{"TASKRAIL_DELEGATION_ID", "TASKRAIL_DELEGATION_TOKEN", "TASKRAIL_EXECUTABLE_SHA256"} {
		if _, present := os.LookupEnv(name); present {
			return true
		}
	}
	return false
}

func replaceDependenciesField(data []byte, before, after []string) ([]byte, error) {
	newline := "\n"
	if strings.Contains(string(data), "\r\n") {
		newline = "\r\n"
	}
	text := string(data)
	lines := strings.SplitAfter(text, newline)
	inFrontmatter := false
	start, end := -1, -1
	for i, line := range lines {
		plain := strings.TrimSuffix(line, newline)
		if plain == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			if start >= 0 {
				end = i
			}
			break
		}
		if !inFrontmatter {
			continue
		}
		if start < 0 && (plain == "dependencies:" || strings.HasPrefix(plain, "dependencies: ") || strings.HasPrefix(plain, "dependencies:\t")) {
			start = i
			continue
		}
		if start >= 0 && plain != "" && plain[0] != ' ' && plain[0] != '\t' {
			end = i
			break
		}
	}
	if start < 0 || end < 0 {
		return nil, errors.New("task frontmatter has no bounded dependencies field")
	}
	headerValue := strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(lines[start], newline), "dependencies:"))
	blockStyle := headerValue == "" || strings.HasPrefix(headerValue, "#")
	if blockStyle && len(after) == len(before)+1 && slices.Equal(after[:len(before)], before) {
		indent := "    "
		insert := start + 1
		for i := start + 1; i < end; i++ {
			plain := strings.TrimSuffix(lines[i], newline)
			trimmed := strings.TrimSpace(plain)
			if strings.HasPrefix(trimmed, "- ") {
				indent = plain[:len(plain)-len(strings.TrimLeft(plain, " \t"))]
				insert = i + 1
			}
		}
		if len(before) > 0 {
			lines = append(lines[:insert], append([]string{indent + "- " + after[len(after)-1] + newline}, lines[insert:]...)...)
			return []byte(strings.Join(lines, "")), nil
		}
	}
	if blockStyle && len(before) == len(after)+1 {
		removed := ""
		for _, dependency := range before {
			if !slices.Contains(after, dependency) {
				removed = dependency
				break
			}
		}
		if removed != "" && len(after) > 0 {
			for i := start + 1; i < end; i++ {
				if strings.TrimSpace(strings.TrimSuffix(lines[i], newline)) == "- "+removed {
					lines = append(lines[:i], lines[i+1:]...)
					return []byte(strings.Join(lines, "")), nil
				}
			}
		}
	}
	var replacement strings.Builder
	if len(after) == 0 {
		replacement.WriteString("dependencies: []" + newline)
	} else {
		replacement.WriteString("dependencies:" + newline)
		for _, dependency := range after {
			replacement.WriteString("    - " + dependency + newline)
		}
	}
	lines = append(lines[:start], append([]string{replacement.String()}, lines[end:]...)...)
	return []byte(strings.Join(lines, "")), nil
}

// writerConsumedPaths derives the consumed set a task writer's transaction
// snapshots: every loaded task it does not itself publish, the spec files the
// corpus anchors to, the specs index, and the repository config — the reads a
// stale candidate could have been built from. written names the tasks the
// writer's published set replaces; their paths become publication candidates
// instead, which a transaction cannot also consume.
func writerConsumedPaths(paths Paths, tasks []*Task, written ...*Task) ([]repotx.Path, error) {
	published := make(map[*Task]struct{}, len(written))
	for _, task := range written {
		published[task] = struct{}{}
	}
	consumed := make([]repotx.Path, 0, len(tasks)+2)
	for _, task := range tasks {
		if _, ok := published[task]; ok {
			continue
		}
		consumed = append(consumed, repotx.Path{Kind: repotx.Managed, Reported: task.Path, Physical: task.Filename})
	}
	seenSpecs := map[string]bool{}
	for _, task := range tasks {
		specPath, _, _ := strings.Cut(task.Frontmatter.SpecRef, "#")
		if seenSpecs[specPath] {
			continue
		}
		physical, err := paths.physicalSpecPath(specPath)
		if err != nil {
			return nil, err
		}
		seenSpecs[specPath] = true
		consumed = append(consumed, repotx.Path{Kind: repotx.Managed, Reported: filepath.ToSlash(specPath), Physical: physical})
	}
	readmeReported := filepath.ToSlash(filepath.Join(paths.LogicalSpecsDir, "README.md"))
	if !seenSpecs[readmeReported] {
		consumed = append(consumed, repotx.Path{Kind: repotx.Managed, Reported: readmeReported, Physical: filepath.Join(paths.SpecsDir, "README.md")})
	}
	if paths.ConfigFile != "" {
		consumed = append(consumed, repotx.Path{Kind: repotx.Worktree, Reported: relPath(paths.RepoRoot, paths.ConfigFile), Physical: paths.ConfigFile})
	}
	return consumed, nil
}

func managedCandidate(reported, physical string, content []byte) repotx.Candidate {
	return repotx.Candidate{Path: repotx.Path{Kind: repotx.Managed, Reported: filepath.ToSlash(reported), Physical: physical}, Content: content}
}

func writerLockError(err error) error {
	if errors.Is(err, repolock.ErrHeld) || errors.Is(err, repolock.ErrSameProcess) {
		return WithMachineErrorCode(MachineCodeLockHeld, err)
	}
	if errors.Is(err, repolock.ErrRefused) {
		return WithMachineErrorCode(MachineCodeDelegatedRefused, err)
	}
	return WithMachineErrorCode(MachineCodeRepositoryInvalid, err)
}

func writerTransactionError(err error) error {
	var txErr *repotx.Error
	if errors.As(err, &txErr) {
		if code, ok := txErr.MachineCode(); ok {
			failure := MachineFailure{Code: code, Paths: append([]string{}, txErr.Preserved...)}
			for _, snapshot := range txErr.Snapshots() {
				failure.Snapshots = append(failure.Snapshots, MachineSnapshot{
					PathKind: string(snapshot.Kind), Path: snapshot.Path,
					OriginalSHA256: snapshot.OriginalSHA256, CandidateSHA256: snapshot.CandidateSHA256,
					CurrentSHA256: snapshot.CurrentSHA256,
				})
			}
			return WithMachineFailure(failure, err)
		}
	}
	return WithMachineErrorCode(MachineCodePartialWrite, err)
}

// refreshRecoveryAfterLock accepts only the mutation lock's creation of a
// previously absent shared runtime parent. Retained transaction state still
// refuses, while the refreshed ancestor identity prevents the command boundary
// from mistaking its own lock directory for recovery activity.
func (s *Service) refreshRecoveryAfterLock() error {
	current, err := observeRecovery(s.paths)
	if err != nil || recoveryRetained(current) {
		return recoveryPending(s.paths, current)
	}
	s.recovery.snapshot = current
	return nil
}
