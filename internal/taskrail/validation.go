package taskrail

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
)

func (s *Service) Validate() (ValidationResult, error) {
	violations := make([]string, 0)
	// Artifacts (ArtifactsDir/VerifyDir) are gitignored output and empty on a
	// clean checkout, so they are not required; verify creates them on demand.
	for _, requiredDir := range []string{s.paths.SpecsDir, s.paths.PlanningDir, s.paths.TasksDir} {
		if !dirExists(requiredDir) {
			violations = append(violations, fmt.Sprintf("missing required directory %s", relPath(s.paths.RepoRoot, requiredDir)))
		}
	}
	for _, requiredFile := range []string{filepath.Join(s.paths.SpecsDir, "README.md"), s.paths.StateFile} {
		if !fileExists(requiredFile) {
			violations = append(violations, fmt.Sprintf("missing required file %s", relPath(s.paths.RepoRoot, requiredFile)))
		}
	}

	state, stateErr := s.loadState()
	if stateErr != nil {
		violations = append(violations, stateErr.Error())
	}
	tasks, taskErr := s.loadTasks()
	if taskErr != nil {
		violations = append(violations, taskErr.Error())
	}
	if stateErr != nil || taskErr != nil {
		return ValidationResult{Valid: len(violations) == 0, Violations: violations}, nil
	}

	violations = append(violations, s.validateState(state)...)
	violations = append(violations, s.validateTasks(state, tasks)...)

	return ValidationResult{Valid: len(violations) == 0, Violations: violations}, nil
}

func (s *Service) validateState(state *State) []string {
	violations := make([]string, 0)
	if state.Frontmatter.SchemaVersion != stateSchemaVersion {
		violations = append(violations, fmt.Sprintf("state schema_version must be %d", stateSchemaVersion))
	}
	if strings.TrimSpace(state.Frontmatter.ActiveSpecPath) == "" {
		violations = append(violations, "state active_spec_path must not be empty")
	} else if !fileExists(filepath.Join(s.paths.RepoRoot, filepath.Clean(state.Frontmatter.ActiveSpecPath))) {
		violations = append(violations, fmt.Sprintf("state active_spec_path does not exist: %s", state.Frontmatter.ActiveSpecPath))
	}
	if strings.TrimSpace(state.Frontmatter.StatusSummary) == "" {
		violations = append(violations, "state status_summary must not be empty")
	}
	violations = append(violations, stateArtifactRefs(state.Frontmatter)...)
	return violations
}

// gitignoredArtifactPrefix is the repo-relative prefix of the gitignored
// planning/artifacts/ tree. A clean checkout drops that tree, so a committed
// reference to a concrete file under it always dangles. We pattern-match the
// prefix rather than shelling out to git (T-026).
const gitignoredArtifactPrefix = "planning/artifacts/"

// danglingArtifactPaths returns concrete file paths under the gitignored
// artifacts tree referenced in s. It targets producer-only pointers like
// planning/artifacts/verify/T-011/<ts>/report.json (the rot scrubbed in
// T-021/T-022/T-023), not the contract prose the scrub deliberately kept:
// bare directory prefixes (planning/artifacts/verify/) and placeholder paths
// (planning/artifacts/manual-test/T-019/<timestamp>/) are legitimate and pass.
// It scans for the prefix as a plain substring anywhere in s; committed
// planning prose does not embed the prefix inside unrelated tokens (e.g. URLs),
// so keeping the scan simple is preferred over anchoring to word boundaries.
func danglingArtifactPaths(s string) []string {
	paths := make([]string, 0)
	for rest := s; ; {
		idx := strings.Index(rest, gitignoredArtifactPrefix)
		if idx < 0 {
			break
		}
		end := idx
		for end < len(rest) && isArtifactPathByte(rest[end]) {
			end++
		}
		token := rest[idx:end]
		rest = rest[end:]
		if isConcreteArtifactFile(token) {
			paths = append(paths, token)
		}
	}
	return paths
}

func isArtifactPathByte(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z', b >= 'A' && b <= 'Z', b >= '0' && b <= '9':
		return true
	case b == '/' || b == '.' || b == '-' || b == '_' || b == '<' || b == '>':
		return true
	default:
		return false
	}
}

// isConcreteArtifactFile reports whether token names a specific file (not a
// directory or placeholder) inside the artifacts tree. Placeholder markers and
// bare directory references are treated as contract prose and ignored.
func isConcreteArtifactFile(token string) bool {
	if strings.ContainsAny(token, "<>") || strings.Contains(token, "...") {
		return false
	}
	base := path.Base(token)
	dot := strings.LastIndex(base, ".")
	if dot <= 0 || dot == len(base)-1 {
		return false
	}
	ext := base[dot+1:]
	// Artifact files use short extensions (report.json, report.md, .txt, .log).
	// Cap the extension length so a directory segment that merely contains a dot
	// (e.g. a versioned dir) is not mistaken for a file. Longer extensions are
	// out of scope: no producer writes them, and prose stays unflagged.
	if len(ext) > 6 {
		return false
	}
	for i := 0; i < len(ext); i++ {
		b := ext[i]
		if !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9') {
			return false
		}
	}
	return true
}

// stateArtifactRefs flags committed STATE.md frontmatter fields that point at a
// concrete gitignored artifact file, naming the offending field.
func stateArtifactRefs(fm StateFrontmatter) []string {
	violations := make([]string, 0)
	scan := func(field, value string) {
		for _, p := range danglingArtifactPaths(value) {
			violations = append(violations, fmt.Sprintf("state %s references gitignored artifact path %s", field, p))
		}
	}
	scan("last_verification_result", fm.LastVerificationResult)
	scan("next_action", fm.NextAction)
	scan("current_task_title", fm.CurrentTaskTitle)
	for _, v := range fm.RelevantArtifacts {
		scan("relevant_artifacts", v)
	}
	for _, v := range fm.ContinuationNotes {
		scan("continuation_notes", v)
	}
	for _, v := range fm.Blockers {
		scan("blockers", v)
	}
	return violations
}

// taskArtifactRefs flags a committed task file (frontmatter title and body
// lines) that points at a concrete gitignored artifact file.
func taskArtifactRefs(task *Task) []string {
	violations := make([]string, 0)
	id := task.Frontmatter.ID
	for _, p := range danglingArtifactPaths(task.Frontmatter.Title) {
		violations = append(violations, fmt.Sprintf("task %s title references gitignored artifact path %s", id, p))
	}
	// An artifact path never spans a newline (\n is not a path byte), so a
	// single whole-body scan yields the same tokens as a per-line scan.
	for _, p := range danglingArtifactPaths(task.Body) {
		violations = append(violations, fmt.Sprintf("task %s body references gitignored artifact path %s", id, p))
	}
	return violations
}

// detectDependencyCycles reports directed cycles among existing task
// dependencies. Missing deps and self-deps are reported by validateTasks, so
// only edges between distinct existing tasks are followed here. Each distinct
// cycle is reported once regardless of traversal entry point.
func detectDependencyCycles(tasks []*Task) []string {
	exists := make(map[string]struct{}, len(tasks))
	for _, task := range tasks {
		exists[task.Frontmatter.ID] = struct{}{}
	}
	adj := make(map[string][]string, len(tasks))
	for _, task := range tasks {
		id := task.Frontmatter.ID
		for _, dep := range task.Frontmatter.Dependencies {
			if dep == id {
				continue // self-dependency handled by validateTasks
			}
			if _, ok := exists[dep]; ok {
				adj[id] = append(adj[id], dep)
			}
		}
	}

	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(tasks))
	reported := make(map[string]struct{})
	violations := make([]string, 0)
	stack := make([]string, 0, len(tasks))

	// Recursion depth is bounded by the longest dependency chain; planning
	// backlogs are small, so an explicit work stack is not warranted.
	var visit func(id string)
	visit = func(id string) {
		color[id] = gray
		stack = append(stack, id)
		for _, dep := range adj[id] {
			switch color[dep] {
			case gray:
				cycle := append(append([]string{}, stack[slices.Index(stack, dep):]...), dep)
				sig := cycleSignature(cycle)
				if _, seen := reported[sig]; !seen {
					reported[sig] = struct{}{}
					violations = append(violations, "dependency cycle detected: "+strings.Join(cycle, " -> "))
				}
			case white:
				visit(dep)
			}
		}
		stack = stack[:len(stack)-1]
		color[id] = black
	}

	for _, task := range tasks {
		if color[task.Frontmatter.ID] == white {
			visit(task.Frontmatter.ID)
		}
	}
	return violations
}

// cycleSignature normalizes a directed cycle (given as [a, b, ..., a]) to a
// rotation starting at its smallest node so the same cycle discovered from
// different entry points yields one signature.
func cycleSignature(cycle []string) string {
	nodes := cycle[:len(cycle)-1] // drop the repeated closing node
	minIdx := 0
	for i, n := range nodes {
		if n < nodes[minIdx] {
			minIdx = i
		}
	}
	rotated := append(append([]string{}, nodes[minIdx:]...), nodes[:minIdx]...)
	return strings.Join(rotated, ">")
}

func (s *Service) validateTasks(state *State, tasks []*Task) []string {
	violations := make([]string, 0)
	seen := make(map[string]struct{}, len(tasks))
	// seenPrefix maps a numeric prefix to the first id that claimed it, so two
	// distinct ids sharing a `^T-(\d+)` prefix (e.g. T-001 and
	// T-001-milestone-v0.1.0) are reported as a collision even though exact-string
	// dedup treats them as different tasks.
	seenPrefix := make(map[int]string, len(tasks))
	// canonical drops duplicate-id files (each id's first occurrence wins), so the
	// current_task/in_progress reconciliation below counts a duplicated id once,
	// matching the shared inProgressTasks contract repair also relies on.
	canonical := make([]*Task, 0, len(tasks))
	for _, task := range tasks {
		if _, ok := seen[task.Frontmatter.ID]; ok {
			violations = append(violations, fmt.Sprintf("duplicate task id %s", task.Frontmatter.ID))
			continue
		}
		seen[task.Frontmatter.ID] = struct{}{}
		canonical = append(canonical, task)

		if prefix, ok := taskNumericPrefix(task.Frontmatter.ID); ok {
			if firstID, collides := seenPrefix[prefix]; collides {
				violations = append(violations, fmt.Sprintf("tasks %s and %s share numeric id prefix T-%03d", firstID, task.Frontmatter.ID, prefix))
			} else {
				seenPrefix[prefix] = task.Frontmatter.ID
			}
		}

		if task.Frontmatter.ID == "" {
			violations = append(violations, fmt.Sprintf("task file %s missing id", relPath(s.paths.RepoRoot, task.Filename)))
		}
		if base := filepath.Base(task.Filename); base != task.Frontmatter.ID+".md" {
			violations = append(violations, fmt.Sprintf("task %s filename must be %s.md", task.Frontmatter.ID, task.Frontmatter.ID))
		}
		if _, ok := validStatuses[task.Frontmatter.Status]; !ok {
			violations = append(violations, fmt.Sprintf("task %s has invalid status %q", task.Frontmatter.ID, task.Frontmatter.Status))
		}
		if _, ok := validPriorites[task.Frontmatter.Priority]; !ok {
			violations = append(violations, fmt.Sprintf("task %s has invalid priority %q", task.Frontmatter.ID, task.Frontmatter.Priority))
		}
		for _, dep := range task.Frontmatter.Dependencies {
			if dep == task.Frontmatter.ID {
				violations = append(violations, fmt.Sprintf("task %s cannot depend on itself", task.Frontmatter.ID))
			}
		}
		if strings.TrimSpace(task.Frontmatter.SpecRef) == "" {
			violations = append(violations, fmt.Sprintf("task %s missing spec_ref", task.Frontmatter.ID))
		} else if err := s.validateSpecRef(task.Frontmatter.SpecRef); err != nil {
			violations = append(violations, fmt.Sprintf("task %s invalid spec_ref: %v", task.Frontmatter.ID, err))
		}
		violations = append(violations, taskArtifactRefs(task)...)
	}

	for _, task := range tasks {
		for _, dep := range task.Frontmatter.Dependencies {
			if _, ok := seen[dep]; !ok {
				violations = append(violations, fmt.Sprintf("task %s depends on missing task %s", task.Frontmatter.ID, dep))
			}
		}
	}

	violations = append(violations, detectDependencyCycles(tasks)...)

	inProgress := inProgressTasks(canonical)
	if len(inProgress) > 1 {
		ids := make([]string, 0, len(inProgress))
		for _, task := range inProgress {
			ids = append(ids, task.Frontmatter.ID)
		}
		violations = append(violations, fmt.Sprintf("multiple in_progress tasks: %s", strings.Join(ids, ", ")))
	}
	if len(inProgress) == 1 {
		if state.Frontmatter.CurrentTask != inProgress[0].Frontmatter.ID {
			violations = append(violations, fmt.Sprintf("state current_task %q does not match in_progress task %q", state.Frontmatter.CurrentTask, inProgress[0].Frontmatter.ID))
		}
	} else if state.Frontmatter.CurrentTask != "" {
		violations = append(violations, fmt.Sprintf("state current_task %q set but no task is in_progress", state.Frontmatter.CurrentTask))
	}

	return violations
}

// parseSpecRef splits a `path#anchor` spec reference into its structural parts
// without touching the filesystem. It is the shared shape check for both a real
// task's spec_ref (validateSpecRef adds the file/heading existence check) and an
// import draft's spec_ref (validated structurally only, before the spec exists).
func parseSpecRef(specRef string) (string, string, error) {
	parts := strings.SplitN(specRef, "#", 2)
	if len(parts) != 2 {
		return "", "", errors.New("spec_ref must include a path and heading anchor")
	}
	if strings.TrimSpace(parts[0]) == "" {
		return "", "", errors.New("spec_ref path must not be empty")
	}
	pathPart := filepath.Clean(parts[0])
	if pathPart == ".." || strings.HasPrefix(pathPart, ".."+string(filepath.Separator)) {
		return "", "", errors.New("spec_ref path must stay within the repository")
	}
	anchor := strings.TrimSpace(parts[1])
	if anchor == "" {
		return "", "", errors.New("spec_ref anchor must not be empty")
	}
	return pathPart, anchor, nil
}

func (s *Service) validateSpecRef(specRef string) error {
	pathPart, anchor, err := parseSpecRef(specRef)
	if err != nil {
		return err
	}

	fullPath := filepath.Join(s.paths.RepoRoot, pathPart)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return fmt.Errorf("read spec file %s: %w", pathPart, fsCause(err))
	}
	anchors := collectHeadingAnchors(string(data))
	if _, ok := anchors[anchor]; !ok {
		return fmt.Errorf("heading #%s not found in %s", anchor, pathPart)
	}
	return nil
}

// collectHeadingAnchors is the set view of a spec's spec_ref anchors, used by
// validateSpecRef to accept or reject a task's heading anchor. It is derived from
// collectHeadingAnchorList so the anchors `spec show --anchors` lists are exactly
// the anchors validation accepts — one slug rule, no re-implementation.
func collectHeadingAnchors(markdown string) map[string]struct{} {
	list := collectHeadingAnchorList(markdown)
	anchors := make(map[string]struct{}, len(list))
	for _, a := range list {
		anchors[a.Anchor] = struct{}{}
	}
	return anchors
}

// collectHeadingAnchorList returns a spec's spec_ref anchors in document order,
// deduped by slug (first occurrence wins). Empty slugs (headings that are pure
// punctuation) are skipped: parseSpecRef already rejects an empty anchor, so they
// are never a valid spec_ref and would only add noise to the listing.
func collectHeadingAnchorList(markdown string) []SpecAnchor {
	var list []SpecAnchor
	seen := make(map[string]struct{})
	for _, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "#") {
			continue
		}
		afterHashes := strings.TrimLeft(trimmed, "#")
		heading := strings.TrimSpace(afterHashes)
		if heading == "" {
			continue
		}
		slug := slugHeading(heading)
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		list = append(list, SpecAnchor{
			Anchor:  slug,
			Heading: heading,
			Level:   len(trimmed) - len(afterHashes),
		})
	}
	return list
}

func slugHeading(heading string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(heading) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-':
			if !lastDash && builder.Len() > 0 {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}
