package taskrail

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func parseFrontmatter[T any](data []byte) (T, string, error) {
	var zero T
	// Normalize CRLF (Windows) and lone CR (classic Mac / legacy tools) to LF so
	// files parse identically regardless of authoring platform; the delimiter and
	// body logic below is LF-only.
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return zero, "", fmt.Errorf("missing frontmatter start")
	}

	parts := strings.SplitN(text, "\n---\n", 2)
	if len(parts) != 2 {
		return zero, "", fmt.Errorf("missing frontmatter end")
	}

	frontmatterText := strings.TrimPrefix(parts[0], "---\n")
	var parsed T
	if err := yaml.Unmarshal([]byte(frontmatterText), &parsed); err != nil {
		return zero, "", fmt.Errorf("parse frontmatter: %w", err)
	}

	return parsed, strings.TrimLeft(parts[1], "\n"), nil
}

// taskIDFromFrontmatter extracts the identity from a syntactically readable
// mapping without decoding the complete typed task. A duplicate or malformed
// non-identity field must not erase a row whose required identity is available.
func taskIDFromFrontmatter(data []byte) (string, bool) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	if !strings.HasPrefix(text, "---\n") {
		return "", false
	}
	parts := strings.SplitN(text, "\n---\n", 2)
	if len(parts) != 2 {
		return "", false
	}
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(strings.TrimPrefix(parts[0], "---\n")), &node); err != nil || len(node.Content) != 1 {
		return "", false
	}
	mapping := node.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return "", false
	}
	var id string
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		if mapping.Content[i].Value != "id" {
			continue
		}
		if id != "" || mapping.Content[i+1].Tag != "!!str" || mapping.Content[i+1].Value == "" {
			return "", false
		}
		id = mapping.Content[i+1].Value
	}
	return id, id != ""
}

func marshalFrontmatter[T any](frontmatter T, body string) ([]byte, error) {
	data, err := yaml.Marshal(frontmatter)
	if err != nil {
		return nil, fmt.Errorf("marshal frontmatter: %w", err)
	}

	var out bytes.Buffer
	out.WriteString("---\n")
	out.Write(data)
	out.WriteString("---\n")
	if body != "" {
		out.WriteString("\n")
		out.WriteString(strings.TrimLeft(body, "\n"))
		if !strings.HasSuffix(body, "\n") {
			out.WriteString("\n")
		}
	}
	return out.Bytes(), nil
}
