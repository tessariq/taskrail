package taskrail

import (
	"encoding/json"
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

// ImportInput drives a single structural import. Preview is the default; Apply
// opts into writing the draft (and, for planning, a STATE seed). OutPath, when
// set, overrides the default reviewable location and must stay within the repo.
type ImportInput struct {
	SourcePath string
	Target     string
	Apply      bool
	OutPath    string
}

// ImportResult reports the parsed draft and, when applied, where it landed.
// StateSeed is populated only for the planning bootstrap; it is a non-authoritative
// seed for review, never a substitute for the CLI-managed planning/STATE.md.
type ImportResult struct {
	Source    string      `json:"source"`
	Target    string      `json:"target"`
	Draft     ImportDraft `json:"draft"`
	StateSeed string      `json:"state_seed,omitempty"`
	Applied   bool        `json:"applied"`
	DraftPath string      `json:"draft_path,omitempty"`
	SeedPath  string      `json:"seed_path,omitempty"`
}

// Import performs one deterministic structural import. It never calls an LLM and
// never modifies the source file; the semantic lift stays the agent's job (T-034).
func (s *Service) Import(input ImportInput) (ImportResult, error) {
	target := strings.TrimSpace(input.Target)
	if _, ok := validImportTargets[target]; !ok {
		return ImportResult{}, fmt.Errorf("import target must be one of tasks, spec, planning; got %q", target)
	}
	source := strings.TrimSpace(input.SourcePath)
	if source == "" {
		return ImportResult{}, errors.New("import source path must not be empty")
	}

	absSource := source
	if !filepath.IsAbs(absSource) {
		absSource = filepath.Join(s.paths.RepoRoot, absSource)
	}
	data, err := os.ReadFile(absSource)
	if err != nil {
		return ImportResult{}, fmt.Errorf("read import source: %w", err)
	}
	sourceLabel := relPath(s.paths.RepoRoot, absSource)
	if strings.HasPrefix(sourceLabel, "..") {
		sourceLabel = filepath.Base(absSource)
	}

	markdown := string(data)
	draft := ImportDraft{SchemaVersion: importDraftSchemaVersion, Target: target, Source: sourceLabel}
	var stateSeed string
	switch target {
	case "tasks":
		draft.Tasks = parseTaskDrafts(markdown)
	case "spec":
		draft.SpecSections = parseSpecSections(markdown)
	case "planning":
		draft.SpecSections = parseSpecSections(markdown)
		draft.Tasks = parseTaskDrafts(markdown)
		stateSeed = renderImportStateSeed(sourceLabel, len(draft.Tasks), len(draft.SpecSections))
	}

	if violations := ValidateImportDraft(draft); len(violations) > 0 {
		return ImportResult{}, fmt.Errorf("structural import produced no valid draft: %s", strings.Join(violations, "; "))
	}

	result := ImportResult{Source: sourceLabel, Target: target, Draft: draft, StateSeed: stateSeed}
	if !input.Apply {
		return result, nil
	}

	draftPath, seedPath, err := s.writeImportDraft(input, draft, stateSeed, absSource, target)
	if err != nil {
		return ImportResult{}, err
	}
	result.Applied = true
	result.DraftPath = draftPath
	result.SeedPath = seedPath
	return result, nil
}

// writeImportDraft persists the draft (and optional STATE seed) for review. It is
// the only write path and is constrained to the repository so an import can never
// escape the repo root.
func (s *Service) writeImportDraft(input ImportInput, draft ImportDraft, stateSeed, absSource, target string) (string, string, error) {
	draftFile := strings.TrimSpace(input.OutPath)
	if draftFile == "" {
		stem := slugHeading(strings.TrimSuffix(filepath.Base(absSource), filepath.Ext(absSource)))
		if stem == "" {
			stem = "import"
		}
		draftFile = filepath.Join(s.paths.PlanningDir, "imports", fmt.Sprintf("%s.%s.import.json", stem, target))
	} else if !filepath.IsAbs(draftFile) {
		draftFile = filepath.Join(s.paths.RepoRoot, draftFile)
	}
	if rel := relPath(s.paths.RepoRoot, draftFile); rel == ".." || strings.HasPrefix(rel, "../") {
		return "", "", fmt.Errorf("import out path %q escapes the repository root", input.OutPath)
	}

	if err := ensureDir(filepath.Dir(draftFile)); err != nil {
		return "", "", err
	}
	payload, err := json.MarshalIndent(draft, "", "  ")
	if err != nil {
		return "", "", fmt.Errorf("marshal import draft: %w", err)
	}
	if err := os.WriteFile(draftFile, append(payload, '\n'), 0o644); err != nil {
		return "", "", fmt.Errorf("write import draft: %w", err)
	}

	seedPath := ""
	if stateSeed != "" {
		seedFile := strings.TrimSuffix(draftFile, filepath.Ext(draftFile)) + ".STATE.seed.md"
		if err := os.WriteFile(seedFile, []byte(stateSeed), 0o644); err != nil {
			return "", "", fmt.Errorf("write import state seed: %w", err)
		}
		seedPath = relPath(s.paths.RepoRoot, seedFile)
	}
	return relPath(s.paths.RepoRoot, draftFile), seedPath, nil
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
