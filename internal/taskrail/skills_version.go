package taskrail

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	skillVersionKey       = "taskrail_version"
	legacySkillVersionKey = skillVersionKey
	skillFileName         = "SKILL.md"
)

var agentSkillTopLevelFields = map[string]bool{
	"name":          true,
	"description":   true,
	"license":       true,
	"compatibility": true,
	"metadata":      true,
	"allowed-tools": true,
}

// InstalledSkill reports the Taskrail version recorded in one materialized skill
// file. Version is empty when the file carries no marker.
type InstalledSkill struct {
	Path           string `json:"path"`
	Skill          string `json:"skill"`
	Version        string `json:"version"`
	MatchesPackage bool   `json:"matches_package"`
}

// validateAgentSkill enforces the Agent Skills frontmatter surface used by the
// embedded package and its committed mirrors.
func validateAgentSkill(data []byte) error {
	root, _, err := parseSkillDocument(data)
	if err != nil {
		return err
	}
	mapping := root.Content[0]
	seen := map[string]bool{}
	for i := 0; i < len(mapping.Content); i += 2 {
		key, value := mapping.Content[i], mapping.Content[i+1]
		if key.Kind != yaml.ScalarNode || key.Tag != "!!str" {
			return fmt.Errorf("top-level field names must be strings")
		}
		if seen[key.Value] {
			return fmt.Errorf("duplicate top-level field %q", key.Value)
		}
		seen[key.Value] = true
		if !agentSkillTopLevelFields[key.Value] {
			return fmt.Errorf("unsupported top-level field %q", key.Value)
		}
		switch key.Value {
		case "name", "description", "license", "compatibility", "allowed-tools":
			if value.Kind != yaml.ScalarNode || value.Tag != "!!str" {
				return fmt.Errorf("%s must be a string", key.Value)
			}
		case "metadata":
			if value.Kind != yaml.MappingNode {
				return fmt.Errorf("metadata must be a mapping")
			}
			for j := 0; j < len(value.Content); j += 2 {
				if value.Content[j].Kind != yaml.ScalarNode || value.Content[j].Tag != "!!str" || value.Content[j+1].Kind != yaml.ScalarNode || value.Content[j+1].Tag != "!!str" {
					return fmt.Errorf("metadata entries must have string keys and values")
				}
			}
		}
	}
	for _, required := range []string{"name", "description"} {
		value := mappingValue(mapping, required)
		if value == nil || value.Kind != yaml.ScalarNode || value.Tag != "!!str" || strings.TrimSpace(value.Value) == "" {
			return fmt.Errorf("%s must be a non-empty string", required)
		}
	}
	return nil
}

// stampSkillVersion writes only metadata.taskrail_version. Re-marshalling the
// small frontmatter mapping keeps arbitrary valid metadata while leaving the body
// bytes untouched.
func stampSkillVersion(data []byte, version string) ([]byte, error) {
	if strings.TrimSpace(version) == "" {
		return nil, fmt.Errorf("taskrail skill version must be non-empty")
	}
	root, body, err := parseSkillDocument(data)
	if err != nil {
		return nil, err
	}
	if _, err := skillVersionFromRoot(root); err != nil {
		return nil, err
	}

	mapping := root.Content[0]
	removeMappingValue(mapping, legacySkillVersionKey)
	metadata := mappingValue(mapping, "metadata")
	if metadata == nil {
		metadata = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "metadata"}, metadata)
	}
	setMappingValue(metadata, skillVersionKey, version)

	frontmatter, err := yaml.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("marshal skill frontmatter: %w", err)
	}
	return append(append([]byte("---\n"), frontmatter...), append([]byte("---\n"), body...)...), nil
}

func parseSkillDocument(data []byte) (*yaml.Node, []byte, error) {
	first, frontmatterStart, ok := nextSkillLine(data, 0)
	if !ok || string(first) != "---" {
		return nil, nil, fmt.Errorf("missing skill frontmatter start")
	}
	frontmatterEnd, bodyStart := -1, -1
	for offset := frontmatterStart; offset < len(data); {
		line, next, ok := nextSkillLine(data, offset)
		if !ok {
			break
		}
		if string(line) == "---" {
			frontmatterEnd, bodyStart = offset, next
			break
		}
		offset = next
	}
	if frontmatterEnd < 0 {
		return nil, nil, fmt.Errorf("missing skill frontmatter end")
	}
	frontmatter := strings.ReplaceAll(string(data[frontmatterStart:frontmatterEnd]), "\r\n", "\n")
	frontmatter = strings.ReplaceAll(frontmatter, "\r", "\n")
	var root yaml.Node
	if err := yaml.Unmarshal([]byte(frontmatter), &root); err != nil {
		return nil, nil, fmt.Errorf("parse skill frontmatter: %w", err)
	}
	if len(root.Content) != 1 || root.Content[0].Kind != yaml.MappingNode {
		return nil, nil, fmt.Errorf("skill frontmatter must be a mapping")
	}
	return &root, data[bodyStart:], nil
}

func nextSkillLine(data []byte, offset int) ([]byte, int, bool) {
	if offset >= len(data) {
		return nil, offset, false
	}
	for i := offset; i < len(data); i++ {
		switch data[i] {
		case '\n':
			return data[offset:i], i + 1, true
		case '\r':
			next := i + 1
			if next < len(data) && data[next] == '\n' {
				next++
			}
			return data[offset:i], next, true
		}
	}
	return data[offset:], len(data), true
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			return mapping.Content[i+1]
		}
	}
	return nil
}

func uniqueMappingValue(mapping *yaml.Node, key string) (*yaml.Node, error) {
	var found *yaml.Node
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != key {
			continue
		}
		if found != nil {
			return nil, fmt.Errorf("duplicate %s field", key)
		}
		found = mapping.Content[i+1]
	}
	return found, nil
}

func removeMappingValue(mapping *yaml.Node, key string) {
	for i := 0; i < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value == key {
			mapping.Content = append(mapping.Content[:i], mapping.Content[i+2:]...)
			return
		}
	}
}

func setMappingValue(mapping *yaml.Node, key, value string) {
	node := mappingValue(mapping, key)
	if node == nil {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value})
		return
	}
	node.Kind = yaml.ScalarNode
	node.Tag = "!!str"
	node.Value = value
	node.Content = nil
}

func markerString(node *yaml.Node, location string) (string, bool, error) {
	if node == nil {
		return "", false, nil
	}
	if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
		return "", false, fmt.Errorf("%s must be a string", location)
	}
	if strings.TrimSpace(node.Value) == "" {
		return "", false, fmt.Errorf("%s must be non-empty", location)
	}
	return node.Value, true, nil
}

func skillVersionFromRoot(root *yaml.Node) (string, error) {
	mapping := root.Content[0]
	legacyNode, err := uniqueMappingValue(mapping, legacySkillVersionKey)
	if err != nil {
		return "", err
	}
	legacy, hasLegacy, err := markerString(legacyNode, "top-level taskrail_version")
	if err != nil {
		return "", err
	}
	var nestedNode *yaml.Node
	metadata, err := uniqueMappingValue(mapping, "metadata")
	if err != nil {
		return "", err
	}
	if metadata != nil {
		if metadata.Kind != yaml.MappingNode {
			return "", fmt.Errorf("metadata must be a mapping")
		}
		nestedNode, err = uniqueMappingValue(metadata, skillVersionKey)
		if err != nil {
			return "", err
		}
	}
	nested, hasNested, err := markerString(nestedNode, "metadata.taskrail_version")
	if err != nil {
		return "", err
	}
	if hasLegacy && hasNested && legacy != nested {
		return "", fmt.Errorf("conflicting taskrail version markers %q and %q", legacy, nested)
	}
	if hasNested {
		return nested, nil
	}
	return legacy, nil
}

// skillVersionOf accepts the current nested marker and the legacy top-level
// marker. A file without frontmatter is marker-free; malformed marker evidence is
// returned as an error rather than being downgraded to unknown-version.
func skillVersionOf(data []byte) (string, error) {
	root, _, err := parseSkillDocument(data)
	if err != nil {
		first, _, ok := nextSkillLine(data, 0)
		if !ok || string(first) != "---" {
			return "", nil
		}
		return "", err
	}
	return skillVersionFromRoot(root)
}

// InstalledSkillVersions reports which version wrote each materialized skill in
// deterministic path order.
func (s *Service) InstalledSkillVersions() ([]InstalledSkill, error) {
	var installed []InstalledSkill
	for _, target := range shippableSkillTargets {
		err := fs.WalkDir(shippableSkillsFS, shippableSkillsRoot, func(packagePath string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || d.Name() != skillFileName {
				return nil
			}
			packaged, err := shippableSkillsFS.ReadFile(packagePath)
			if err != nil {
				return fmt.Errorf("read embedded skill %s: %w", packagePath, err)
			}
			rel := strings.TrimPrefix(packagePath, shippableSkillsRoot+"/")
			diskPath := filepath.Join(s.paths.RepoRoot, target, filepath.FromSlash(rel))
			data, err := os.ReadFile(diskPath)
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if err != nil {
				return fmt.Errorf("read %s: %w", relPath(s.paths.RepoRoot, diskPath), fsCause(err))
			}
			version, err := skillVersionOf(data)
			if err != nil {
				return fmt.Errorf("read skill marker %s: %w", relPath(s.paths.RepoRoot, diskPath), err)
			}
			installed = append(installed, InstalledSkill{
				Path:           relPath(s.paths.RepoRoot, diskPath),
				Skill:          filepath.Base(filepath.Dir(diskPath)),
				Version:        version,
				MatchesPackage: bytes.Equal(packaged, data),
			})
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return installed, nil
}
