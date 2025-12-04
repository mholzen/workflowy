package formatter

import (
	"strings"
	"unicode"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

func IsListIntroduction(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	return strings.HasSuffix(name, ":")
}

func WordCount(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	return len(strings.Fields(s))
}

func AreChildrenSimilarLength(children []*workflowy.Item) bool {
	if len(children) < 2 {
		return true
	}

	var nonEmptyChildren []*workflowy.Item
	for _, child := range children {
		if !IsEmpty(child.Name) {
			nonEmptyChildren = append(nonEmptyChildren, child)
		}
	}

	if len(nonEmptyChildren) < 2 {
		return true
	}

	wordCounts := make([]int, len(nonEmptyChildren))
	sum := 0
	for i, child := range nonEmptyChildren {
		wordCounts[i] = WordCount(child.Name)
		sum += wordCounts[i]
	}

	avg := float64(sum) / float64(len(wordCounts))

	const tolerance = 0.5
	for _, count := range wordCounts {
		diff := float64(count) - avg
		if diff < 0 {
			diff = -diff
		}
		if avg > 0 && diff/avg > tolerance {
			return false
		}
	}

	return true
}

func IsListPattern(item *workflowy.Item) bool {
	if len(item.Children) == 0 {
		return false
	}

	if !IsListIntroduction(item.Name) {
		return false
	}

	for _, child := range item.Children {
		if len(child.Children) > 0 {
			return false
		}
	}

	return AreChildrenSimilarLength(item.Children)
}

func ChildrenHaveGrandchildren(children []*workflowy.Item) bool {
	for _, child := range children {
		if len(child.Children) > 0 {
			return true
		}
	}
	return false
}

func IsEmptyBullet(item *workflowy.Item) bool {
	return IsEmpty(item.Name) && len(item.Children) == 0
}

func EndsWithPunctuation(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	lastRune := rune(s[len(s)-1])
	return lastRune == '.' || lastRune == '!' || lastRune == '?' || lastRune == ':' || lastRune == ';'
}

func EnsureCapitalized(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func EnsurePunctuated(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if EndsWithPunctuation(s) {
		return s
	}
	return s + "."
}

func FormatAsSentence(s string) string {
	return EnsurePunctuated(EnsureCapitalized(s))
}



