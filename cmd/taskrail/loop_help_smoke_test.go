package main

import (
	"strings"
	"testing"
)

// TestLoopHelpDescribesExecution guards against the help surface drifting back
// to describing the default invocation as a preview. `loop` launches real
// external child processes and, under `--parallel >1`, clones the repository and
// fast-forwards the attached branch; only `--dry-run` previews.
func TestLoopHelpDescribesExecution(t *testing.T) {
	out, err := runRoot(t, "loop", "--help")
	if err != nil {
		t.Fatalf("loop --help: %v (output %q)", err, out)
	}
	summary, _, found := strings.Cut(out, "\nUsage:")
	if !found {
		t.Fatalf("loop help has no usage section: %q", out)
	}
	// A regression is not "the word preview disappeared" but "a sentence claims
	// the default invocation previews". Bind the word to --dry-run per sentence,
	// so text like "the default invocation previews the run; see --dry-run"
	// still fails.
	for _, sentence := range strings.Split(summary, ". ") {
		if !strings.Contains(strings.ToLower(sentence), "preview") {
			continue
		}
		if !strings.Contains(sentence, "--dry-run") {
			t.Fatalf("help sentence claims previewing without naming --dry-run: %q", sentence)
		}
	}
	for _, want := range []string{
		"--dry-run",
		"external",
	} {
		if !strings.Contains(summary, want) {
			t.Fatalf("loop help description missing %q: %q", want, summary)
		}
	}
	flags := out[strings.Index(out, "Flags:"):]
	for _, line := range strings.Split(flags, "\n") {
		if !strings.Contains(line, "--parallel") && !strings.Contains(line, "--max-iterations") {
			continue
		}
		if strings.Contains(strings.ToLower(line), "preview") {
			t.Fatalf("execution flag help describes previewing: %q", line)
		}
	}
	if !strings.Contains(flags, "--dry-run") {
		t.Fatalf("loop help lost the --dry-run flag: %q", flags)
	}
}
