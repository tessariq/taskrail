package taskrail

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// Init makes a repository Taskrail-managed in a version-aware, non-destructive
// way: it writes the `.taskrail/config.yml` marker and, when the marker records
// an older layout_version, migrates to the current layout. Migration and
// retrofit default to a dry run reporting the plan; callers must pass Apply to
// write it. Content created for the layout uses writeFileIfMissing/saveState-if-
// missing semantics, so human-authored content under specs/ and planning/ is
// never rewritten.
//
// The reported plan is computed before the first write, which is what lets a
// preview and the apply that follows it name the same candidate paths.
func (s *Service) Init(in InitInput) (InitResult, error) {
	if in.Local {
		return s.initLocal(in)
	}
	if s.paths.Storage.Mode == StorageLocal && in.WithSkills {
		if !in.ForceSkills {
			return InitResult{}, invalidArgumentsf("initialized local storage refreshes packaged skills only with init --with-skills --force")
		}
		return s.refreshLocalSkills(in)
	}
	previewSnapshot, err := s.snapshotInitPreview("")
	if err != nil {
		return InitResult{}, err
	}
	cfg, hasMarker, err := readMarker(s.paths.RepoRoot)
	if err != nil {
		return InitResult{}, err
	}
	// An upgradable current layout routes to the layout-2 upgrade flow, with one
	// interim bridge: an explicit skill-install request cannot be a write-free
	// preview, so it keeps the ordinary current-layout flow adopters already
	// rely on until the durable migration publisher owns the combined refresh.
	if hasMarker && upgradableCurrentLayout(cfg) && (in.Apply || !in.WithSkills) {
		result, err := s.initLayout2Upgrade(in)
		if err != nil || result.Applied {
			return result, err
		}
		if testHookInitPreviewBuilt != nil {
			testHookInitPreviewBuilt()
		}
		if err := s.recheckInitPreview(previewSnapshot, ""); err != nil {
			return InitResult{}, err
		}
		return result, nil
	}
	if err := rejectUpgradeOnlyInputs(in); err != nil {
		return InitResult{}, err
	}
	plan := s.planInit(cfg, hasMarker, in.Apply)
	result, err := s.reportInit(plan)
	if err != nil {
		return InitResult{}, err
	}
	if !plan.applied {
		if testHookInitPreviewBuilt != nil {
			testHookInitPreviewBuilt()
		}
		if err := s.recheckInitPreview(previewSnapshot, ""); err != nil {
			return InitResult{}, err
		}
		return result, nil
	}
	return s.applyInitTransaction(in)
}

// initPlan is everything init decided from the pre-write repository: which
// outcome it reached, which marker that outcome publishes, and which of the
// three write steps it performs. Deciding once keeps the reported inventory and
// the performed writes from disagreeing.
type initPlan struct {
	outcome      InitOutcome
	fromVersion  int
	marker       LayoutConfig
	configAction string
	// createsLayout is set when this outcome — or the apply a preview describes —
	// creates missing layout content. It drives the reported inventory, which is
	// why a preview sets it while writing nothing.
	createsLayout bool
	// scaffolds, writesMarker, and validates are the three write steps this
	// invocation actually performs.
	scaffolds    bool
	writesMarker bool
	validates    bool
	applied      bool
	mapping      []RetrofitMapping
}

// planInit classifies the repository. A marker pinning an older layout migrates
// and a current one is an idempotent no-op; without a marker, an existing
// v0.1.0 layout is adopted (marker only, nothing else touched), a non-standard
// repository with candidate directories proposes a retrofit, and an empty one
// gets a fresh layout. A newer marker version never reaches here — readMarker
// already refused it through the shared layout-version guard.
//
// Fresh creation and adoption apply unconditionally, as they always have: there
// is nothing pre-existing for a dry run to protect.
func (s *Service) planInit(cfg LayoutConfig, hasMarker bool, apply bool) initPlan {
	if hasMarker {
		if cfg.LayoutVersion != currentLayoutVersion && cfg.LayoutVersion != layout2Version {
			return initPlan{
				outcome:       pickOutcome(apply, InitMigrated, InitMigrationPreview),
				fromVersion:   cfg.LayoutVersion,
				marker:        migratedMarker(cfg),
				configAction:  configActionMigrate,
				createsLayout: true,
				scaffolds:     apply,
				writesMarker:  apply,
				validates:     apply,
				applied:       apply,
			}
		}
		return initPlan{
			outcome:       InitCurrent,
			fromVersion:   cfg.LayoutVersion,
			marker:        cfg,
			configAction:  configActionPreserve,
			createsLayout: true,
			scaffolds:     true,
			applied:       true,
		}
	}

	// An absent marker records no prior layout version, which the contract
	// reports as version 0 rather than as the version init is moving to.
	if s.layoutExists() {
		return initPlan{
			outcome:      InitAdopted,
			marker:       defaultLayoutConfig(),
			configAction: configActionCreate,
			writesMarker: true,
			applied:      true,
		}
	}
	if mapping := s.detectRetrofit(); len(mapping) > 0 {
		return initPlan{
			outcome:       pickOutcome(apply, InitRetrofitApplied, InitRetrofitPreview),
			marker:        defaultLayoutConfig(),
			configAction:  configActionCreate,
			createsLayout: true,
			scaffolds:     apply,
			writesMarker:  apply,
			validates:     apply,
			applied:       apply,
			mapping:       mapping,
		}
	}
	return initPlan{
		outcome:       InitCreated,
		marker:        defaultLayoutConfig(),
		configAction:  configActionCreate,
		createsLayout: true,
		scaffolds:     true,
		writesMarker:  true,
		applied:       true,
	}
}

func pickOutcome(apply bool, applied, preview InitOutcome) InitOutcome {
	if apply {
		return applied
	}
	return preview
}

// migratedMarker is the marker a migration publishes: the recorded layout with
// the current version and any location the older marker omitted defaulted.
func migratedMarker(cfg LayoutConfig) LayoutConfig {
	migrated := cfg
	migrated.LayoutVersion = currentLayoutVersion
	if migrated.SpecsDir == "" {
		migrated.SpecsDir = defaultSpecsDir
	}
	if migrated.PlanningDir == "" {
		migrated.PlanningDir = defaultPlanningDir
	}
	return migrated
}

// reportInit builds the machine result from the pre-write repository. Every
// action it records describes what this outcome does to that path, so a preview
// and the apply that follows it report the same inventory.
func (s *Service) reportInit(plan initPlan) (InitResult, error) {
	digest, err := markerDigest(plan.marker)
	if err != nil {
		return InitResult{}, err
	}
	// One classification and one state read feed both inventories: two
	// independent looks at the same pre-write repository could disagree, which is
	// exactly what deciding the plan once exists to prevent.
	notesPresent, err := classifyNotesDestination(s.paths.RepoRoot, s.paths.PlanningDir)
	if err != nil {
		return InitResult{}, err
	}
	continuation := s.continuationNotes()
	return InitResult{
		Outcome:     plan.outcome,
		FromVersion: plan.fromVersion,
		ToVersion:   currentLayoutVersion,
		Applied:     plan.applied,
		StorageMode: string(s.paths.Storage.Mode),
		Config: InitConfig{
			Path:            markerRelPath(),
			Action:          plan.configAction,
			CandidateSHA256: digest,
		},
		Writes:            s.initWrites(plan, notesPresent),
		Notes:             s.initNotes(plan.createsLayout, notesPresent, continuation),
		Skills:            []InitSkill{},
		SkillExclusions:   []InitSkillExclusion{},
		ContinuationNotes: continuation,
		Mapping:           plan.mapping,
	}, nil
}

// markerDigest is the candidate digest over the exact marker bytes writeMarker
// would persist, so the reported digest and the written file cannot diverge.
func markerDigest(marker LayoutConfig) (string, error) {
	data, err := yaml.Marshal(marker)
	if err != nil {
		return "", fmt.Errorf("marshal layout marker: %w", err)
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// layoutFile is one file the current layout contains: its managed logical path,
// the physical location the active storage context puts it at, the kind of
// content it holds, and — for the two files ensureLayout writes directly — its
// starter body. One table keeps the writer and the two reporters from drifting
// apart as the layout grows.
type layoutFile struct {
	logical  string
	physical string
	kind     string
	starter  func() string
}

// layoutFiles lists the layout's content in the order ensureLayout creates it,
// which is also the order the retrofit dry run reports.
func (s *Service) layoutFiles() []layoutFile {
	return []layoutFile{
		{
			logical:  path.Join(s.paths.LogicalSpecsDir, "README.md"),
			physical: filepath.Join(s.paths.SpecsDir, "README.md"),
			kind:     writeKindSpec,
			starter:  starterSpecsReadme,
		},
		{
			logical:  path.Join(s.paths.LogicalSpecsDir, "v0.1.0.md"),
			physical: filepath.Join(s.paths.SpecsDir, "v0.1.0.md"),
			kind:     writeKindSpec,
			starter:  starterSpecV010,
		},
		// The state file and the notes sidecar have writers of their own
		// (saveState and the no-clobber ensureNotesTemplate), so they carry no
		// starter here.
		{
			logical:  path.Join(s.paths.LogicalPlanningDir, "STATE.md"),
			physical: s.paths.StateFile,
			kind:     writeKindState,
		},
		{
			logical:  s.logicalNotesPath(),
			physical: notesPath(s.paths.PlanningDir),
			kind:     writeKindNote,
		},
	}
}

// initWrites inventories every path this outcome touches or deliberately keeps,
// in path order. A path that is absent and that this outcome does not create is
// left out: init reports what it does, not what some other outcome would do.
// The sidecar's presence is passed in because classifying it can fail, and that
// refusal belongs at the top of the report rather than inside one entry.
func (s *Service) initWrites(plan initPlan, notesPresent bool) []WriteEntry {
	writes := []WriteEntry{{
		Path:   markerRelPath(),
		Kind:   writeKindConfig,
		Action: markerWriteAction(plan.configAction),
	}}
	for _, file := range s.layoutFiles() {
		present := fileExists(file.physical)
		if file.kind == writeKindNote {
			present = notesPresent
		}
		action, reported := layoutWriteAction(present, plan.createsLayout)
		if !reported {
			continue
		}
		writes = append(writes, WriteEntry{Path: file.logical, Kind: file.kind, Action: action})
	}
	slices.SortFunc(writes, func(a, b WriteEntry) int { return strings.Compare(a.Path, b.Path) })
	return writes
}

// logicalNotesPath is the sidecar's managed logical path, which is where every
// reported note entry names it regardless of the physical storage root.
func (s *Service) logicalNotesPath() string {
	return path.Join(s.paths.LogicalPlanningDir, notesFileName)
}

// layoutWriteAction maps one layout path's pre-state onto the action this
// outcome takes on it. An existing file is preserved by every outcome, since
// init only ever adds; an absent one is created only where the outcome creates
// layout content — including the apply a preview describes — and is otherwise
// not part of this outcome at all.
func layoutWriteAction(present, createsLayout bool) (action string, reported bool) {
	switch {
	case present:
		return writeActionPreserve, true
	case createsLayout:
		return writeActionCreate, true
	default:
		return "", false
	}
}

// markerWriteAction expresses the marker's own action in the write vocabulary: a
// migration rewrites the machine-owned marker in place.
func markerWriteAction(configAction string) string {
	if configAction == configActionMigrate {
		return writeActionRefresh
	}
	return configAction
}

// initNotes reports the sidecar's disposition plus the continuation-note choices
// the repository makes available. Extraction is offered only when it could
// actually run: there must be notes to move, and an existing human-owned sidecar
// is never appended to or replaced.
func (s *Service) initNotes(createsLayout, present bool, continuation []string) []InitNote {
	fileAction := noteActionNone
	switch {
	case present:
		fileAction = noteActionPreserve
	case createsLayout:
		fileAction = noteActionCreateTemplate
	}

	choices := []string{}
	if len(continuation) > 0 {
		if !present {
			choices = append(choices, continuationChoiceExtract)
		}
		choices = append(choices, continuationChoiceDrop)
	}
	return []InitNote{{
		Path:       s.logicalNotesPath(),
		FileAction: fileAction,
		// Null until an operator selects a disposition. Init has no surface for
		// that selection yet, so it reports the choices without inventing one.
		ContinuationAction:  nil,
		ContinuationChoices: choices,
	}}
}

// continuationNotes decodes the notes currently recorded in state. A repository
// without readable state simply has none to report: init exists to make such a
// repository usable, so an unreadable snapshot must not make it refuse.
func (s *Service) continuationNotes() []string {
	state, err := s.loadState()
	if err != nil {
		return []string{}
	}
	return append([]string{}, state.Frontmatter.ContinuationNotes...)
}

// retrofitCandidates lists the source directory names a non-standard repository
// might already use, in priority order, and the Taskrail directory (target) and
// role each would fill. Detection is deliberately conservative: it only
// recognizes this small, well-known set rather than guessing from arbitrary
// directory names.
var retrofitCandidates = []struct {
	dir    string
	role   string
	target string
}{
	{defaultSpecsDir, "specs", defaultSpecsDir},
	{defaultPlanningDir, "planning", defaultPlanningDir},
	{"notes", "planning", defaultPlanningDir},
}

// detectRetrofit scans an unmarked, non-standard repository for candidate
// directories that suggest an existing layout to adopt, returning the proposed
// mapping onto the Taskrail layout. It returns nil for an empty repository (which
// should be fresh-initialized) and never proposes the same role twice, so a
// human confirms one clear mapping rather than a redundant one.
func (s *Service) detectRetrofit() []RetrofitMapping {
	var mapping []RetrofitMapping
	claimed := map[string]bool{}
	for _, c := range retrofitCandidates {
		if claimed[c.role] {
			continue
		}
		if !dirExists(filepath.Join(s.paths.RepoRoot, c.dir)) {
			continue
		}
		mapping = append(mapping, RetrofitMapping{Source: c.dir, Target: c.target, Role: c.role})
		claimed[c.role] = true
	}
	return mapping
}

// layoutExists reports whether the repository already carries a v0.1.0 layout,
// used to tell legacy adoption apart from a fresh empty-repo init.
func (s *Service) layoutExists() bool {
	return fileExists(s.paths.StateFile) || dirExists(s.paths.TasksDir)
}

// ensureLayout creates the current layout idempotently: directories via ensureDir
// and content via writeFileIfMissing, with the state file written only when
// absent. Re-running it never overwrites existing files.
func (s *Service) ensureLayout() error {
	// Only tracked, committed directories are provisioned. Gitignored artifact
	// output (verify/, runs/, manual-test/) is created on demand by verify and
	// manual testing; a clean checkout drops it, so pre-creating it here would
	// leave init and validate inconsistent (T-024/T-025).
	for _, dir := range []string{s.paths.SpecsDir, s.paths.TasksDir} {
		if err := ensureDir(s.paths.RepoRoot, dir); err != nil {
			return err
		}
	}
	for _, file := range s.layoutFiles() {
		if file.starter == nil {
			continue
		}
		if err := writeFileIfMissing(s.paths.RepoRoot, file.physical, []byte(file.starter())); err != nil {
			return err
		}
	}
	if err := ensureNotesTemplate(s.paths.RepoRoot, s.paths.PlanningDir); err != nil {
		return err
	}
	if _, err := os.Stat(s.paths.StateFile); errors.Is(err, os.ErrNotExist) {
		if err := s.saveState(starterState(s.now())); err != nil {
			return err
		}
	} else if err != nil {
		return fmt.Errorf("stat state file: %w", err)
	}
	return nil
}

func markerRelPath() string {
	return path.Join(taskrailConfigDir, taskrailConfigFile)
}
