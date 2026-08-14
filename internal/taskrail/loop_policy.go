package taskrail

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const DefaultLoopReason = "implicit hold: loop policy is not set"

// LoopPolicyMetadata uses pointers because absent fields and present empty
// scalars have different validity under the paired-field contract.
type LoopPolicyMetadata struct {
	Policy        *string `yaml:"loop_policy,omitempty" json:"loop_policy,omitempty"`
	Reason        *string `yaml:"loop_reason,omitempty" json:"loop_reason,omitempty"`
	policyPresent bool
	reasonPresent bool
}

type EffectiveLoopPolicy struct {
	Source string
	Policy string
	Reason string
}

func ResolveLoopPolicy(meta LoopPolicyMetadata) EffectiveLoopPolicy {
	policyPresent, reasonPresent := loopPolicyPresence(meta)
	if !policyPresent && !reasonPresent {
		return EffectiveLoopPolicy{Source: "default", Policy: "hold", Reason: DefaultLoopReason}
	}
	policy, reason := "", ""
	if meta.Policy != nil {
		policy = *meta.Policy
	}
	if meta.Reason != nil {
		reason = *meta.Reason
	}
	return EffectiveLoopPolicy{Source: "explicit", Policy: policy, Reason: reason}
}

func ValidateLoopPolicyMetadata(meta LoopPolicyMetadata) []string {
	policyPresent, reasonPresent := loopPolicyPresence(meta)
	if policyPresent != reasonPresent {
		return []string{"loop_policy and loop_reason must both be present or both absent"}
	}
	if !policyPresent {
		return nil
	}

	var violations []string
	if meta.Policy == nil {
		violations = append(violations, "loop_policy must be a string")
	}
	if meta.Reason == nil {
		violations = append(violations, "loop_reason must be a string")
	}
	if meta.Policy == nil || meta.Reason == nil {
		return violations
	}
	if *meta.Policy != "allow" && *meta.Policy != "hold" {
		violations = append(violations, `loop_policy must be "allow" or "hold"`)
	}
	reason := *meta.Reason
	if !utf8.ValidString(reason) {
		violations = append(violations, "loop_reason must be valid UTF-8")
	}
	if reason == "" || len(reason) > 512 {
		violations = append(violations, "loop_reason must be between 1 and 512 bytes")
	}
	if strings.TrimSpace(reason) != reason {
		violations = append(violations, "loop_reason must be trimmed")
	}
	if strings.IndexFunc(reason, unicode.IsControl) >= 0 {
		violations = append(violations, "loop_reason must not contain a newline or control character")
	}
	if err := ensurePortableNote("loop_reason", reason); err != nil {
		violations = append(violations, err.Error())
	}
	return violations
}

func loopPolicyPresence(meta LoopPolicyMetadata) (bool, bool) {
	return meta.policyPresent || meta.Policy != nil, meta.reasonPresent || meta.Reason != nil
}

// UnmarshalYAML records mapping-key presence separately from pointer values so
// an explicit YAML null cannot masquerade as an omitted policy field.
func (f *TaskFrontmatter) UnmarshalYAML(node *yaml.Node) error {
	type plain TaskFrontmatter
	var decoded plain
	if err := node.Decode(&decoded); err != nil {
		return err
	}
	*f = TaskFrontmatter(decoded)
	for i := 0; i+1 < len(node.Content); i += 2 {
		switch node.Content[i].Value {
		case "loop_policy":
			f.policyPresent = true
		case "loop_reason":
			f.reasonPresent = true
		}
	}
	return nil
}

// MarshalYAML keeps explicit null policy keys present. They remain invalid, but
// an unrelated writer must not silently turn malformed explicit metadata into a
// valid implicit hold before post-write validation reports it.
func (f TaskFrontmatter) MarshalYAML() (any, error) {
	type plain TaskFrontmatter
	var node yaml.Node
	if err := node.Encode(plain(f)); err != nil {
		return nil, err
	}
	appendNull := func(key string) {
		node.Content = append(node.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"},
		)
	}
	if f.policyPresent && f.Policy == nil {
		appendNull("loop_policy")
	}
	if f.reasonPresent && f.Reason == nil {
		appendNull("loop_reason")
	}
	return &node, nil
}

func loopPolicyFieldsInBody(body string) []string {
	fields := make([]string, 0, 2)
	seen := make(map[string]bool, 2)
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimLeft(line, " \t")
		for _, field := range []string{"loop_policy", "loop_reason"} {
			if strings.HasPrefix(line, field+":") && !seen[field] {
				fields = append(fields, field)
				seen[field] = true
			}
		}
	}
	return fields
}
