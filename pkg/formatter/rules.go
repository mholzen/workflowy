package formatter

import (
	"strings"
	"unicode"
)

// Capitalize returns the string with the first letter capitalized
func Capitalize(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// Uppercase returns the string in all uppercase
func Uppercase(s string) string {
	return strings.ToUpper(s)
}

// Punctuate adds a period at the end if the string doesn't already end with punctuation
func Punctuate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	lastChar := s[len(s)-1]
	// Check if already ends with punctuation
	if lastChar == '.' || lastChar == '!' || lastChar == '?' || lastChar == ':' {
		return s
	}

	return s + "."
}

// AddColon adds a colon at the end if not already present
func AddColon(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	if s[len(s)-1] == ':' {
		return s
	}

	// Remove other punctuation first
	if s[len(s)-1] == '.' || s[len(s)-1] == '!' || s[len(s)-1] == '?' {
		s = s[:len(s)-1]
	}

	return s + ":"
}

// StripTag removes a tag from the string (e.g., "#exclude", "#h1")
func StripTag(s string, tag string) string {
	return strings.TrimSpace(strings.Replace(s, tag, "", 1))
}

// HasTag checks if the string contains a tag
func HasTag(s string, tag string) bool {
	return strings.Contains(s, tag)
}

// IsEmpty checks if a node's name is effectively empty (whitespace only)
func IsEmpty(s string) bool {
	return strings.TrimSpace(s) == ""
}

// JoinParagraphs joins multiple paragraph strings with a space
func JoinParagraphs(paragraphs []string) string {
	return strings.Join(paragraphs, " ")
}

// IndentBullet returns the appropriate markdown bullet indentation
func IndentBullet(depth int) string {
	if depth == 0 {
		return "- "
	}
	return strings.Repeat("  ", depth) + "- "
}

// HeaderPrefix returns the markdown header prefix (e.g., "# ", "## ")
func HeaderPrefix(level int) string {
	if level < 1 {
		level = 1
	}
	if level > 6 {
		level = 6
	}
	return strings.Repeat("#", level) + " "
}
