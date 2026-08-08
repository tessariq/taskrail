package repolock

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// Capability bounds one owner's authority: the canonical commands it may run and
// the task fields it may write. Ownership is checked against it before every
// mutation, and a delegated join can only narrow it — never widen it — so a
// child cannot grant itself work its parent did not select.
type Capability struct {
	Commands   []string
	TaskFields []string
}

// delegatedCommands is the exact command set a delegated child writer may join
// for. Task creation and loop-policy mutation are excluded because either would
// let a delegate enlarge the work its parent selected
// (specs/v0.5.0.md#cross-platform-autonomous-loop). `verify` stays in the set:
// its `--create-followup` output is bound to the selected task's own report.
var delegatedCommands = []string{"block", "complete", "start", "unblock", "verify"}

// delegatedTaskFields is the exact task write set a delegate may touch. It
// mirrors the fields a loop iteration is allowed to change — canonical lifecycle
// status, its timestamp, the blocker reason, and Implementation Notes — so
// identity, ranking, spec anchoring, dependencies, and loop policy are all
// unreachable from a delegated write.
var delegatedTaskFields = []string{"blocker", "implementation_notes", "status", "updated_at"}

// DelegatedCapability is the fixed upper bound on any delegated join. It is a
// protocol constant rather than lock metadata: the normative metadata set is
// closed, and a bound a child could read out of the lock file is a bound an
// attacker could read too.
func DelegatedCapability() Capability {
	return Capability{
		Commands:   slices.Clone(delegatedCommands),
		TaskFields: slices.Clone(delegatedTaskFields),
	}
}

// normalized returns the capability with entries trimmed, sorted, and
// deduplicated, so comparison never depends on how a caller happened to spell a
// set. An entry that trims to empty is dropped.
func (c Capability) normalized() Capability {
	return Capability{Commands: normalizeSet(c.Commands), TaskFields: normalizeSet(c.TaskFields)}
}

func normalizeSet(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// Includes reports whether other is entirely within c.
func (c Capability) Includes(other Capability) bool {
	outer, inner := c.normalized(), other.normalized()
	return subset(inner.Commands, outer.Commands) && subset(inner.TaskFields, outer.TaskFields)
}

func subset(inner, outer []string) bool {
	for _, value := range inner {
		if !slices.Contains(outer, value) {
			return false
		}
	}
	return true
}

// Narrow returns the requested capability once it is proven to be within c. A
// request that adds any command or task field is a widening attempt and is
// refused rather than silently intersected — the caller asked for authority it
// does not have, and quietly granting less would hide that.
func (c Capability) Narrow(requested Capability) (Capability, error) {
	if !c.Includes(requested) {
		return Capability{}, fmt.Errorf("%w: requested capability %s exceeds %s",
			ErrRefused, requested.describe(), c.describe())
	}
	return requested.normalized(), nil
}

// Allows reports whether command and every named task field are within c. It is
// the check a writer runs before mutating anything, so an unsupported or
// unrelated write refuses while the repository is still untouched.
func (c Capability) Allows(command string, fields ...string) error {
	normalized := c.normalized()
	if !slices.Contains(normalized.Commands, strings.TrimSpace(command)) {
		return fmt.Errorf("%w: command %q is outside %s", ErrRefused, command, c.describe())
	}
	for _, field := range fields {
		if !slices.Contains(normalized.TaskFields, strings.TrimSpace(field)) {
			return fmt.Errorf("%w: task field %q is outside %s", ErrRefused, field, c.describe())
		}
	}
	return nil
}

// validate refuses a capability that cannot bound anything.
func (c Capability) validate() error {
	if len(c.normalized().Commands) == 0 {
		return errors.New("capability names no command")
	}
	return nil
}

func (c Capability) describe() string {
	normalized := c.normalized()
	return fmt.Sprintf("capability{commands: [%s], task_fields: [%s]}",
		strings.Join(normalized.Commands, " "), strings.Join(normalized.TaskFields, " "))
}
