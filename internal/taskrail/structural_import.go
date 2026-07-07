package taskrail

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Structural import is the deterministic, no-LLM baseline for `taskrail import`
// (T-033). It mechanically parses markdown structure into the T-032 ImportDraft:
// headings become spec sections, and subheadings plus top-level list items become
// task drafts. The output is crude but real and doubles as the reviewable
// `--apply` ingest target the agent-driven path (T-034) refines.

// atxHeadingPattern matches a level-1..6 ATX heading, capturing the marker (for
// its level) and the trimmed text with any trailing closing hashes removed.
var atxHeadingPattern = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*#*$`)

// topBulletPattern matches an unindented list item, unordered (-, *, +) or
// ordered (`1.`). Indented continuation lines do not match by construction, so
// they fold into the item's body rather than becoming their own units.
var topBulletPattern = regexp.MustCompile(`^(?:[-*+]|\d+\.)\s+(.+?)\s*$`)

// ImportInput drives a single structural import preview. The preview is the only
// mode: it parses the source into a draft and returns it for review. Persisting a
// draft is the caller's job (redirect stdout), and writing real files is the
// separate agent-driven apply path (ApplyImportDraft, T-034).
type ImportInput struct {
	SourcePath string
	Target     string
}

// ImportResult reports the parsed draft. StateSeed is populated only for the
// planning bootstrap; it is a non-authoritative seed for review, never a
// substitute for the CLI-managed planning/STATE.md.
type ImportResult struct {
	Source    string      `json:"source"`
	Target    string      `json:"target"`
	Draft     ImportDraft `json:"draft"`
	StateSeed string      `json:"state_seed,omitempty"`
}

// Import performs one deterministic structural import preview. It never calls an
// LLM and never modifies the source file; the semantic lift stays the agent's
// job (T-034). It writes nothing: the returned draft is the reviewable artifact.
func (s *Service) Import(input ImportInput) (ImportResult, error) {
	target, err := parseTarget(input.Target)
	if err != nil {
		return ImportResult{}, err
	}
	markdown, sourceLabel, err := s.readImportSource(input.SourcePath)
	if err != nil {
		return ImportResult{}, err
	}

	draft := ImportDraft{SchemaVersion: importDraftSchemaVersion, Target: string(target), Source: sourceLabel}
	var stateSeed string
	switch target {
	case TargetTasks:
		draft.Tasks = parseTaskDrafts(markdown)
	case TargetSpec:
		draft.SpecSections = parseSpecSections(markdown)
	case TargetPlanning:
		draft.SpecSections = parseSpecSections(markdown)
		draft.Tasks = parseTaskDrafts(markdown)
		stateSeed = renderImportStateSeed(sourceLabel, len(draft.Tasks), len(draft.SpecSections))
	default:
		return ImportResult{}, fmt.Errorf("unhandled import target %q", string(target))
	}

	if violations := ValidateImportDraft(draft); len(violations) > 0 {
		return ImportResult{}, fmt.Errorf("structural import produced no valid draft: %s", strings.Join(violations, "; "))
	}

	return ImportResult{Source: sourceLabel, Target: string(target), Draft: draft, StateSeed: stateSeed}, nil
}

// readImportSource reads a source file (repo-relative or absolute) and returns
// its content plus a portable repo-relative label. The source is never modified.
func (s *Service) readImportSource(sourcePath string) (string, string, error) {
	source := strings.TrimSpace(sourcePath)
	if source == "" {
		return "", "", errors.New("import source path must not be empty")
	}
	absSource := s.resolveRepoPath(source)
	data, err := os.ReadFile(absSource)
	if err != nil {
		return "", "", fmt.Errorf("read import source: %w", err)
	}
	label := relPath(s.paths.RepoRoot, absSource)
	if strings.HasPrefix(label, "..") {
		label = filepath.Base(absSource)
	}
	return string(data), label, nil
}

// resolveRepoPath turns a repo-relative path into one rooted at the repository,
// leaving an already-absolute path untouched.
func (s *Service) resolveRepoPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(s.paths.RepoRoot, path)
}

// parseSpecSections turns every heading into a flat spec section whose body is the
// content up to the next heading of any level. Content before the first heading is
// preamble and is dropped; the semantic pass (T-034) refines the crude split.
func parseSpecSections(markdown string) []SpecSectionDraft {
	sections := make([]SpecSectionDraft, 0)
	var heading string
	var body []string
	flush := func() {
		if heading == "" {
			return
		}
		sections = append(sections, SpecSectionDraft{
			Heading: heading,
			Body:    strings.TrimSpace(strings.Join(body, "\n")),
		})
	}
	for _, line := range strings.Split(markdown, "\n") {
		if m := atxHeadingPattern.FindStringSubmatch(line); m != nil {
			flush()
			heading = strings.TrimSpace(m[2])
			body = nil
			continue
		}
		if heading != "" {
			body = append(body, line)
		}
	}
	flush()
	return sections
}

// parseTaskDrafts turns subheadings (H2+) and top-level list items into task
// drafts in document order. A subheading's body is the paragraph text up to the
// next heading or top-level bullet; a bullet's body is its indented continuation
// (including nested sub-bullets). The H1 document title is not a task.
func parseTaskDrafts(markdown string) []TaskDraft {
	lines := strings.Split(markdown, "\n")
	drafts := make([]TaskDraft, 0)
	for i := 0; i < len(lines); {
		line := lines[i]
		if m := atxHeadingPattern.FindStringSubmatch(line); m != nil {
			level := len(m[1])
			title := strings.TrimSpace(m[2])
			i++
			if level < 2 || title == "" {
				continue
			}
			body, next := gatherParagraph(lines, i)
			i = next
			drafts = append(drafts, TaskDraft{Title: title, Body: body})
			continue
		}
		if m := topBulletPattern.FindStringSubmatch(line); m != nil {
			title := strings.TrimSpace(m[1])
			i++
			body, next := gatherIndented(lines, i)
			i = next
			if title != "" {
				drafts = append(drafts, TaskDraft{Title: title, Body: body})
			}
			continue
		}
		i++
	}
	assignDraftKeys(drafts)
	return drafts
}

// gatherParagraph collects the lines that belong to a heading-derived task: the
// paragraph text following the heading, stopping at the next heading or the first
// top-level bullet (which becomes its own task).
func gatherParagraph(lines []string, start int) (string, int) {
	var body []string
	i := start
	for ; i < len(lines); i++ {
		if atxHeadingPattern.MatchString(lines[i]) || topBulletPattern.MatchString(lines[i]) {
			break
		}
		body = append(body, lines[i])
	}
	return strings.TrimSpace(strings.Join(body, "\n")), i
}

// gatherIndented collects the indented continuation of a top-level bullet,
// stopping at the next unindented non-blank line (a heading, another bullet, or a
// new paragraph). Collected lines are de-indented for a clean draft body.
func gatherIndented(lines []string, start int) (string, int) {
	var body []string
	i := start
	for ; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) == "" {
			body = append(body, "")
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			break
		}
		body = append(body, strings.TrimSpace(line))
	}
	return strings.TrimSpace(strings.Join(body, "\n")), i
}

// assignDraftKeys stamps a unique, deterministic draft-local key on each task so
// intra-draft dependencies stay resolvable and the T-032 uniqueness rule holds.
func assignDraftKeys(drafts []TaskDraft) {
	used := make(map[string]struct{}, len(drafts))
	for i := range drafts {
		base := slugHeading(drafts[i].Title)
		if base == "" {
			base = fmt.Sprintf("task-%d", i+1)
		}
		key := base
		for n := 2; ; n++ {
			if _, taken := used[key]; !taken {
				break
			}
			key = fmt.Sprintf("%s-%d", base, n)
		}
		used[key] = struct{}{}
		drafts[i].Key = key
	}
}

// renderImportStateSeed produces a non-authoritative STATE seed for a planning
// bootstrap. It summarizes what was imported and must be reviewed before any
// tracked work adopts it; it is never written to planning/STATE.md by import.
func renderImportStateSeed(source string, taskCount, sectionCount int) string {
	var b strings.Builder
	b.WriteString("# STATE (import seed)\n\n")
	b.WriteString(fmt.Sprintf("Generated by `taskrail import --to planning` from `%s`.\n", source))
	b.WriteString("Non-authoritative: review and create tracked work through the CLI; do not copy this over `planning/STATE.md`.\n\n")
	b.WriteString("## Active Spec\n\n")
	b.WriteString(fmt.Sprintf("- TODO: assemble a spec from the %d imported section(s) and point STATE at it.\n\n", sectionCount))
	b.WriteString("## Task Counts\n\n")
	b.WriteString(fmt.Sprintf("- todo: %d\n", taskCount))
	b.WriteString("- in_progress: 0\n")
	b.WriteString("- completed: 0\n")
	b.WriteString("- blocked: 0\n")
	b.WriteString("- cancelled: 0\n\n")
	b.WriteString("## Next Action\n\n")
	b.WriteString("- Review the imported task and spec drafts, then create tracked work via taskrail.\n")
	return b.String()
}
