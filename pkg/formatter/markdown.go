package formatter

import (
	"strings"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

type MarkdownFormatter struct {
	config *MarkdownConfig
}

type MarkdownConfig struct {
	ExcludeTag string
	H1Tag      string
	H2Tag      string
	H3Tag      string
	PTag       string
	ListTag    string
}

func DefaultMarkdownConfig() *MarkdownConfig {
	return &MarkdownConfig{
		ExcludeTag: "#exclude",
		H1Tag:      "#h1",
		H2Tag:      "#h2",
		H3Tag:      "#h3",
		PTag:       "#p",
		ListTag:    "#list",
	}
}

func NewMarkdownFormatter() *MarkdownFormatter {
	return &MarkdownFormatter{
		config: DefaultMarkdownConfig(),
	}
}

func NewMarkdownFormatterWithConfig(config *MarkdownConfig) *MarkdownFormatter {
	return &MarkdownFormatter{
		config: config,
	}
}

func (f *MarkdownFormatter) FormatTree(items []*workflowy.Item) (string, error) {
	var result strings.Builder

	for _, item := range items {
		output := f.formatNode(item, 1)
		if output != "" {
			result.WriteString(output)
		}
	}

	return strings.TrimRight(result.String(), "\n") + "\n", nil
}

func (f *MarkdownFormatter) formatNode(item *workflowy.Item, headerLevel int) string {
	if f.shouldExclude(item) {
		return ""
	}

	layoutMode := f.getLayoutMode(item)
	name := f.stripAllTags(item.Name)

	switch layoutMode {
	case "h1":
		return f.formatAsHeader(item, name, 1)
	case "h2":
		return f.formatAsHeader(item, name, 2)
	case "h3":
		return f.formatAsHeader(item, name, 3)
	case "p":
		return f.formatAsParagraph(name) + "\n\n"
	case "ol":
		return f.formatAsOrderedList(item, name)
	case "quote":
		return f.formatAsQuote(item, name)
	case "code":
		return f.formatAsCode(item, name)
	case "divider":
		return "---\n\n"
	default:
		return f.formatBulletsNode(item, name, headerLevel)
	}
}

func (f *MarkdownFormatter) formatBulletsNode(item *workflowy.Item, name string, headerLevel int) string {
	if len(item.Children) == 0 {
		return ""
	}

	if IsListPattern(item) {
		return f.formatWithListChildren(item, name, headerLevel)
	}

	if f.childrenAreAllSubheaders(item.Children) {
		return f.formatAsHeaderWithSubheaders(item, name, headerLevel)
	}

	return f.formatAsHeaderWithParagraph(item, name, headerLevel)
}

func (f *MarkdownFormatter) childrenAreAllSubheaders(children []*workflowy.Item) bool {
	hasAnyWithGrandchildren := false
	for _, child := range children {
		if f.shouldExclude(child) {
			continue
		}
		if IsEmpty(child.Name) {
			continue
		}
		if len(child.Children) > 0 {
			if IsListPattern(child) {
				return false
			}
			hasAnyWithGrandchildren = true
		} else {
			if hasAnyWithGrandchildren {
				return false
			}
		}
	}
	return hasAnyWithGrandchildren
}

func (f *MarkdownFormatter) formatAsHeader(item *workflowy.Item, name string, level int) string {
	var result strings.Builder
	result.WriteString(HeaderPrefix(level))
	result.WriteString(Capitalize(name))
	result.WriteString("\n\n")

	for _, child := range item.Children {
		childOutput := f.formatNode(child, level+1)
		result.WriteString(childOutput)
	}

	return result.String()
}

func (f *MarkdownFormatter) formatAsHeaderWithParagraph(item *workflowy.Item, name string, headerLevel int) string {
	var result strings.Builder

	result.WriteString(HeaderPrefix(headerLevel))
	result.WriteString(Capitalize(name))
	result.WriteString("\n")

	paragraphs := f.collectParagraphs(item.Children)
	for _, para := range paragraphs {
		if para == "" {
			result.WriteString("\n")
			continue
		}
		result.WriteString(para)
		result.WriteString("\n")
	}
	result.WriteString("\n")

	return result.String()
}

func (f *MarkdownFormatter) collectParagraphs(children []*workflowy.Item) []string {
	var paragraphs []string
	var currentSentences []string
	needsBlankBefore := false

	for _, child := range children {
		if f.shouldExclude(child) {
			continue
		}

		childName := f.stripAllTags(child.Name)

		if IsEmptyBullet(child) || IsEmpty(childName) {
			if len(currentSentences) > 0 {
				paragraphs = append(paragraphs, strings.Join(currentSentences, " "))
				currentSentences = nil
			}
			needsBlankBefore = true
			continue
		}

		if needsBlankBefore && len(paragraphs) > 0 {
			paragraphs = append(paragraphs, "")
			needsBlankBefore = false
		}

		if IsListPattern(child) {
			currentSentences = append(currentSentences, FormatAsSentence(childName))
			intro := strings.Join(currentSentences, " ")
			currentSentences = nil
			listOutput := f.formatInlineListWithIntro(child, intro)
			paragraphs = append(paragraphs, listOutput)
			needsBlankBefore = true
			continue
		}

		currentSentences = append(currentSentences, FormatAsSentence(childName))
	}

	if len(currentSentences) > 0 {
		if needsBlankBefore && len(paragraphs) > 0 {
			paragraphs = append(paragraphs, "")
		}
		paragraphs = append(paragraphs, strings.Join(currentSentences, " "))
	}

	return paragraphs
}

func (f *MarkdownFormatter) formatInlineList(item *workflowy.Item, name string) string {
	return f.formatInlineListWithIntro(item, FormatAsSentence(name))
}

func (f *MarkdownFormatter) formatInlineListWithIntro(item *workflowy.Item, intro string) string {
	var result strings.Builder
	result.WriteString(intro)
	result.WriteString("\n")

	for _, child := range item.Children {
		if f.shouldExclude(child) {
			continue
		}
		childName := f.stripAllTags(child.Name)
		if !IsEmpty(childName) {
			result.WriteString("- ")
			result.WriteString(childName)
			result.WriteString("\n")
		}
	}

	return strings.TrimRight(result.String(), "\n")
}

func (f *MarkdownFormatter) formatAsHeaderWithSubheaders(item *workflowy.Item, name string, headerLevel int) string {
	var result strings.Builder

	result.WriteString(HeaderPrefix(headerLevel))
	result.WriteString(Capitalize(name))
	result.WriteString("\n\n")

	for _, child := range item.Children {
		if f.shouldExclude(child) {
			continue
		}

		childName := f.stripAllTags(child.Name)

		if len(child.Children) > 0 {
			childOutput := f.formatBulletsNode(child, childName, headerLevel+1)
			result.WriteString(childOutput)
		} else if !IsEmpty(childName) {
			result.WriteString(FormatAsSentence(childName))
			result.WriteString("\n\n")
		}
	}

	return result.String()
}

func (f *MarkdownFormatter) formatWithListChildren(item *workflowy.Item, name string, headerLevel int) string {
	var result strings.Builder

	result.WriteString(HeaderPrefix(headerLevel))
	result.WriteString(Capitalize(name))
	result.WriteString("\n")

	for _, child := range item.Children {
		if f.shouldExclude(child) {
			continue
		}
		childName := f.stripAllTags(child.Name)
		if !IsEmpty(childName) {
			result.WriteString("- ")
			result.WriteString(childName)
			result.WriteString("\n")
		}
	}
	result.WriteString("\n")

	return result.String()
}

func (f *MarkdownFormatter) formatAsParagraph(text string) string {
	return FormatAsSentence(text)
}

func (f *MarkdownFormatter) formatAsOrderedList(item *workflowy.Item, name string) string {
	var result strings.Builder

	if name != "" {
		result.WriteString(name)
		result.WriteString("\n")
	}

	for i, child := range item.Children {
		if f.shouldExclude(child) {
			continue
		}
		childName := f.stripAllTags(child.Name)
		if !IsEmpty(childName) {
			result.WriteString(strings.Repeat(" ", 0))
			result.WriteString(string(rune('1'+i)) + ". ")
			result.WriteString(childName)
			result.WriteString("\n")
		}
	}
	result.WriteString("\n")

	return result.String()
}

func (f *MarkdownFormatter) formatAsQuote(item *workflowy.Item, name string) string {
	var result strings.Builder

	if name != "" {
		result.WriteString("> ")
		result.WriteString(name)
		result.WriteString("\n")
	}

	for _, child := range item.Children {
		if f.shouldExclude(child) {
			continue
		}
		childName := f.stripAllTags(child.Name)
		if !IsEmpty(childName) {
			result.WriteString("> ")
			result.WriteString(childName)
			result.WriteString("\n")
		}
	}
	result.WriteString("\n")

	return result.String()
}

func (f *MarkdownFormatter) formatAsCode(item *workflowy.Item, name string) string {
	var result strings.Builder

	result.WriteString("```\n")
	if name != "" {
		result.WriteString(name)
		result.WriteString("\n")
	}

	for _, child := range item.Children {
		if f.shouldExclude(child) {
			continue
		}
		childName := f.stripAllTags(child.Name)
		result.WriteString(childName)
		result.WriteString("\n")
	}
	result.WriteString("```\n\n")

	return result.String()
}

func (f *MarkdownFormatter) shouldExclude(item *workflowy.Item) bool {
	return HasTag(item.Name, f.config.ExcludeTag)
}

func (f *MarkdownFormatter) getLayoutMode(item *workflowy.Item) string {
	if HasTag(item.Name, f.config.H1Tag) {
		return "h1"
	}
	if HasTag(item.Name, f.config.H2Tag) {
		return "h2"
	}
	if HasTag(item.Name, f.config.H3Tag) {
		return "h3"
	}
	if HasTag(item.Name, f.config.PTag) {
		return "p"
	}
	if HasTag(item.Name, f.config.ListTag) {
		return "list"
	}

	if item.Data != nil {
		if mode, ok := item.Data["layoutMode"].(string); ok && mode != "" {
			return mode
		}
	}

	return "bullets"
}

func (f *MarkdownFormatter) stripAllTags(text string) string {
	text = StripTag(text, f.config.ExcludeTag)
	text = StripTag(text, f.config.H1Tag)
	text = StripTag(text, f.config.H2Tag)
	text = StripTag(text, f.config.H3Tag)
	text = StripTag(text, f.config.PTag)
	text = StripTag(text, f.config.ListTag)
	return strings.TrimSpace(text)
}

func FormatItemsAsMarkdown(items []*workflowy.Item) (string, error) {
	formatter := NewMarkdownFormatter()
	return formatter.FormatTree(items)
}

