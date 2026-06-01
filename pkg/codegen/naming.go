package codegen

import (
	"strings"
	"unicode"
)

// kebabCase converts a camelCase, PascalCase, snake_case, or already-kebab
// identifier to kebab-case (lower-case words joined by '-'). Drives the
// action name on disk and in the manifest `name` field.
func kebabCase(s string) string { return joinWords(splitWords(s), "-") }

// snakeCase converts an identifier to snake_case. Drives the connector op
// name in the [[execute]] block.
func snakeCase(s string) string { return joinWords(splitWords(s), "_") }

// firstWord returns the lower-cased first word of an identifier — used to
// derive the [[execute]] id (e.g. "send" from "sendMessage").
func firstWord(s string) string {
	words := splitWords(s)
	if len(words) == 0 {
		return ""
	}
	return strings.ToLower(words[0])
}

// splitWords cuts an identifier on camelCase boundaries and on '-' / '_'.
func splitWords(s string) []string {
	var out []string
	var cur []rune
	flush := func() {
		if len(cur) > 0 {
			out = append(out, string(cur))
			cur = cur[:0]
		}
	}
	for _, r := range s {
		switch {
		case r == '-' || r == '_':
			flush()
		case unicode.IsUpper(r):
			flush()
			cur = append(cur, r)
		default:
			cur = append(cur, r)
		}
	}
	flush()
	return out
}

func joinWords(parts []string, sep string) string {
	lower := make([]string, len(parts))
	for i, p := range parts {
		lower[i] = strings.ToLower(p)
	}
	return strings.Join(lower, sep)
}
