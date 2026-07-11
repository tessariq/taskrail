package main

import (
	"strings"

	"github.com/spf13/cobra"
)

// completeSpecVersion completes the spec-version positional argument (`spec show`,
// `spec activate`) to the versioned specs under specs/. It takes only the first
// positional and never offers file completion, so a version slot cannot fall back
// to arbitrary paths.
func completeSpecVersion(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	svc, err := serviceFromCmd(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	versions, err := svc.SpecVersionCompletions()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return versions, cobra.ShellCompDirectiveNoFileComp
}

// completeSpecRef completes `task new --spec-ref` to real `<path>#<anchor>`
// values. In the path phase (no '#' typed yet) it suppresses the trailing space
// so the shell stays on the word for the anchor phase; once an anchor is being
// typed the completed `<path>#<anchor>` is a whole value and the space returns.
func completeSpecRef(cmd *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	svc, err := serviceFromCmd(cmd)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	refs, err := svc.SpecRefCompletions(toComplete)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	directive := cobra.ShellCompDirectiveNoFileComp
	if !strings.Contains(toComplete, "#") {
		directive |= cobra.ShellCompDirectiveNoSpace
	}
	return refs, directive
}
