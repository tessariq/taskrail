package main

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/tessariq/taskrail/internal/taskrail"
)

// inheritedLocalCommandForms is the release-facing inventory for commands that
// predate local storage. Keep forms separate when their preview and apply
// behavior differs, so adding a command cannot silently skip local-mode proof.
var inheritedLocalCommandForms = []struct {
	command   string
	form      string
	access    string
	bootstrap bool
	owner     string
}{
	{"init", "default", "writer", false, "TestInitCreatesStructure"},
	{"retrofit", "preview", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"retrofit", "apply", "writer", false, "TestWriterCommandsPublishTheCommonEnvelope"},
	{"retrofit", "emit-prompt", "reader", false, "TestWriterCommandsPublishTheCommonEnvelope"},
	{"validate", "default", "reader", false, "TestSharedReadersAreStorageNeutral"},
	{"repair", "preview", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"repair", "apply", "writer", true, "TestImplicitLocalBootstrapCommandMatrix"},
	{"coverage", "report", "reader", false, "TestSharedReadersAreStorageNeutral"},
	{"coverage", "gaps", "reader", false, "TestSharedReadersAreStorageNeutral"},
	{"coverage", "min", "reader", false, "TestReadOnlyMachineInvocationsAreSideEffectFree"},
	{"status", "default", "reader", false, "TestSharedReadersAreStorageNeutral"},
	{"stats", "default", "reader", false, "TestSharedReadersAreStorageNeutral"},
	{"next", "default", "writer", true, "TestImplicitLocalBootstrapCommandMatrix"},
	{"start", "default", "writer", true, "TestLifecycleAndTaskWritersUseOnlyLocalStorage"},
	{"complete", "default", "writer", true, "TestLifecycleAndTaskWritersUseOnlyLocalStorage"},
	{"block", "default", "writer", true, "TestLifecycleAndTaskWritersUseOnlyLocalStorage"},
	{"unblock", "default", "writer", true, "TestLifecycleAndTaskWritersUseOnlyLocalStorage"},
	{"verify", "default", "writer", true, "TestLifecycleAndTaskWritersUseOnlyLocalStorage"},
	{"task new", "default", "writer", true, "TestLifecycleAndTaskWritersUseOnlyLocalStorage"},
	{"task rename", "preview", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"task rename", "apply", "writer", true, "TestLifecycleAndTaskWritersUseOnlyLocalStorage"},
	{"task repoint", "preview", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"task repoint", "apply", "writer", true, "TestLifecycleAndTaskWritersUseOnlyLocalStorage"},
	{"task release", "preview", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"task release", "apply", "writer", true, "TestLifecycleAndTaskWritersUseOnlyLocalStorage"},
	{"task show", "default", "reader", false, "TestTaskShowReadsExactTaskBytesThroughActiveStorage"},
	{"task author", "preview", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"task author", "apply", "writer", true, "TestImplicitLocalBootstrapCommandMatrix"},
	{"task dependency add", "preview", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"task dependency add", "apply", "writer", false, "TestLifecycleAndTaskWritersUseOnlyLocalStorage"},
	{"task dependency remove", "preview", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"task dependency remove", "apply", "writer", false, "TestLifecycleAndTaskWritersUseOnlyLocalStorage"},
	{"task loop list", "default", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"task loop allow", "default", "writer", true, "TestImplicitLocalBootstrapCommandMatrix"},
	{"task loop hold", "default", "writer", true, "TestImplicitLocalBootstrapCommandMatrix"},
	{"task loop clear", "default", "writer", true, "TestImplicitLocalBootstrapCommandMatrix"},
	{"spec list", "default", "reader", false, "TestSharedReadersAreStorageNeutral"},
	{"spec show", "default", "reader", false, "TestSharedReadersAreStorageNeutral"},
	{"spec add", "default", "writer", true, "TestStructuralWritersUseLocalStorageAndLogicalPaths"},
	{"spec diff", "default", "reader", false, "TestSharedReadersAreStorageNeutral"},
	{"spec activate", "default", "writer", true, "TestStructuralWritersUseLocalStorageAndLogicalPaths"},
	{"import", "preview", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"import", "emit-prompt", "reader", false, "TestWriterCommandsPublishTheCommonEnvelope"},
	{"import", "apply", "writer", true, "TestStructuralWritersUseLocalStorageAndLogicalPaths"},
	{"prompt list", "default", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"prompt show", "default", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"prompt render", "default", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"review publish", "preview", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"review publish", "apply", "writer", false, "TestReviewPublishTaskPreviewAndApplyBindExactBytes"},
	{"review show", "default", "reader", false, "TestSharedReadersAreStorageNeutral"},
	{"lock status", "default", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"lock clear", "default", "writer", false, "TestLockClearRemovesOnlyTheUnchangedStaleLock"},
	{"recover", "preview", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"recover", "apply", "writer", false, "TestRecoverTransactionRecoversMixedPathKindsInLocalStorage"},
	{"loop", "dry-run", "reader", false, "TestImplicitLocalBootstrapExclusions"},
	{"loop", "execute", "writer", true, "TestImplicitLocalBootstrapCommandMatrix"},
}

// inheritedLocalFormKeys is deliberately form-granular: Cobra exposes command
// paths but does not expose semantic preview/apply forms for inventory checks.
var inheritedLocalFormKeys = []string{
	"init default", "retrofit preview", "retrofit apply", "retrofit emit-prompt",
	"validate default", "repair preview", "repair apply", "coverage report",
	"coverage gaps", "coverage min", "status default", "stats default", "next default",
	"start default", "complete default", "block default", "unblock default", "verify default",
	"task new default", "task rename preview", "task rename apply", "task repoint preview",
	"task repoint apply", "task release preview", "task release apply", "task show default",
	"task author preview", "task author apply", "task dependency add preview",
	"task dependency add apply", "task dependency remove preview",
	"task dependency remove apply", "task loop list default", "task loop allow default",
	"task loop hold default", "task loop clear default", "spec list default",
	"spec show default", "spec add default", "spec diff default", "spec activate default",
	"import preview", "import emit-prompt", "import apply", "prompt list default",
	"prompt show default", "prompt render default", "review publish preview",
	"review publish apply", "review show default", "lock status default", "lock clear default",
	"recover preview", "recover apply", "loop dry-run", "loop execute",
}

func TestInheritedLocalCommandInventoryIsComplete(t *testing.T) {
	covered := map[string]bool{}
	forms := map[string]bool{}
	for _, form := range inheritedLocalCommandForms {
		t.Run(form.command+" "+form.form, func(t *testing.T) {
			if form.access != "reader" && form.access != "writer" {
				t.Fatalf("access = %q, want reader or writer", form.access)
			}
			if !strings.HasPrefix(form.owner, "Test") {
				t.Fatalf("owner = %q, want focused test name", form.owner)
			}
			key := form.command + " " + form.form
			if forms[key] {
				t.Fatalf("duplicate inherited form %q", key)
			}
			forms[key] = true
			covered[form.command] = true
		})
	}
	wantForms := slices.Clone(inheritedLocalFormKeys)
	slices.Sort(wantForms)
	if got := sortedFormKeys(forms); !slices.Equal(got, wantForms) {
		t.Fatalf("inherited local forms = %v, want %v", got, wantForms)
	}

	for _, entry := range taskrail.MachineCommandInventory() {
		if entry.Origin != taskrail.MachineOriginConstructed || entry.Surface != taskrail.MachineSurfaceStdout {
			continue
		}
		// Local storage commands are introduced with local mode; they are not
		// inherited commands required to preserve committed-mode parity.
		if entry.Command == "local status" || entry.Command == "local path" || entry.Command == "local promote" {
			continue
		}
		if !covered[entry.Command] {
			t.Errorf("inherited command %q has no local parity form", entry.Command)
		}
	}
}

func TestInheritedLocalCommandEvidenceOwnersExist(t *testing.T) {
	files := []string{
		"smoke_test.go", "local_bootstrap_test.go", "machine_smoke_test.go",
		"machine_writer_smoke_test.go", "task_show_smoke_test.go",
		"../../internal/taskrail/storage_neutral_test.go", "../../internal/taskrail/task_show_test.go",
		"../../internal/taskrail/init_local_test.go", "../../internal/taskrail/review_publish_test.go",
		"../../internal/taskrail/lock_test.go", "../../internal/taskrail/recover_test.go",
	}
	var sources strings.Builder
	for _, name := range files {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read focused evidence file %q: %v", name, err)
		}
		sources.Write(data)
	}
	for _, form := range inheritedLocalCommandForms {
		if !strings.Contains(sources.String(), "func "+form.owner+"(") {
			t.Errorf("%s %s names missing evidence owner %s", form.command, form.form, form.owner)
		}
	}
}

func TestInheritedLocalCommandBootstrapPolicy(t *testing.T) {
	bootstrapForms := map[string]bool{}
	for _, form := range inheritedLocalCommandForms {
		key := form.command + " " + form.form
		if form.bootstrap {
			bootstrapForms[key] = true
		}
	}
	want := []string{
		"complete default", "block default", "import apply", "loop execute", "next default",
		"repair apply", "spec activate default", "spec add default", "start default", "task author apply",
		"task loop allow default", "task loop clear default", "task loop hold default", "task new default",
		"task rename apply", "task repoint apply", "task release apply", "unblock default", "verify default",
	}
	slices.Sort(want)
	if got := sortedFormKeys(bootstrapForms); !slices.Equal(got, want) {
		t.Fatalf("bootstrap forms = %v, want %v", got, want)
	}
	for _, form := range inheritedLocalCommandForms {
		if !form.bootstrap {
			continue
		}
		surface := taskrail.MachineSurfaceStdout
		if form.command == "loop" && form.form == "execute" {
			// Loop execution sends its final envelope to the required result file.
			surface = taskrail.MachineSurfaceResultFile
		}
		entry, ok := taskrail.MachineCommandEntryFor(form.command, surface)
		if !ok {
			t.Fatalf("bootstrap form %q has no machine command entry", form.command)
		}
		if !containsLocalInitializedWarning(entry.Warnings) {
			t.Errorf("bootstrap form %q is missing local_initialized from its command contract", form.command)
		}
	}
}

func formsKeys(forms map[string]bool) []string {
	keys := make([]string, 0, len(forms))
	for key := range forms {
		keys = append(keys, key)
	}
	return keys
}

func sortedFormKeys(forms map[string]bool) []string {
	keys := formsKeys(forms)
	slices.Sort(keys)
	return keys
}

func containsLocalInitializedWarning(warnings []string) bool {
	for _, warning := range warnings {
		if warning == "local_initialized" {
			return true
		}
	}
	return false
}
