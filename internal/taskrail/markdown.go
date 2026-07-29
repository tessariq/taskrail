package taskrail

import "strings"

// markdownLinesWithoutFencedContent preserves line indexes and opening fence
// delimiters while blanking fenced content, so scanners share one fence policy
// without allowing code blocks to bridge structural boundaries.
func markdownLinesWithoutFencedContent(markdown string) []string {
	lines := strings.Split(markdown, "\n")
	var openFence string
	for i, line := range lines {
		fence, rest := markdownFence(line)
		if openFence != "" {
			lines[i] = ""
			if fence != "" && fence[0] == openFence[0] && len(fence) >= len(openFence) && strings.TrimSpace(rest) == "" {
				openFence = ""
			}
			continue
		}
		if fence != "" {
			openFence = fence
		}
	}
	return lines
}

func markdownFence(line string) (fence, rest string) {
	line = strings.TrimSuffix(line, "\r")
	indent := len(line) - len(strings.TrimLeft(line, " "))
	if indent > 3 || indent == len(line) {
		return "", ""
	}
	line = line[indent:]
	marker := line[0]
	if marker != '`' && marker != '~' {
		return "", ""
	}
	length := 0
	for length < len(line) && line[length] == marker {
		length++
	}
	if length < 3 {
		return "", ""
	}
	rest = line[length:]
	if marker == '`' && strings.Contains(rest, "`") {
		return "", ""
	}
	return line[:length], rest
}
