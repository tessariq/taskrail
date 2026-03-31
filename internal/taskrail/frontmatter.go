package taskrail

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

func parseFrontmatter[T any](data []byte) (T, string, error) {
	var zero T
	text := string(data)
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
