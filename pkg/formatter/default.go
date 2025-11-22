package formatter

import (
	"strings"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

// DefaultFormatter implements the Formatter interface with standard rules
type DefaultFormatter struct {
	config *Config
}

// NewDefaultFormatter creates a new formatter with default configuration
func NewDefaultFormatter() *DefaultFormatter {
	return &DefaultFormatter{
		config: DefaultConfig(),
	}
}

// NewFormatter creates a new formatter with custom configuration
func NewFormatter(config *Config) *DefaultFormatter {
	return &DefaultFormatter{
		config: config,
	}
}

// FormatTree converts entire tree to markdown
func (f *DefaultFormatter) FormatTree(items []*workflowy.Item) (string, error) {
	var result strings.Builder

	for _, item := range items {
		markdown, err := f.FormatNode(item, 0)
		if err != nil {
			return "", err
		}
		if markdown != "" {
			result.WriteString(markdown)
		}
	}

	return result.String(), nil
}

// FormatNode converts a single node and its children to markdown
func (f *DefaultFormatter) FormatNode(item *workflowy.Item, depth int) (string, error) {
	// Check if node should be excluded
	if f.ShouldExclude(item) {
		return "", nil
	}

	// Get effective layout mode
	layoutMode := f.GetLayoutMode(item, depth)

	// Strip tags from name
	name := f.stripTags(item.Name)

	var result strings.Builder

	switch layoutMode {
	case LayoutH1, LayoutH2, LayoutH3, LayoutH4, LayoutH5, LayoutH6:
		result.WriteString(f.formatHeader(name, layoutMode, item))

	case LayoutP:
		// Check if this paragraph has bullet children (list introduction)
		hasBulletChildren := f.hasBulletChildren(item)
		if hasBulletChildren && f.config.AddColonBeforeLists {
			name = AddColon(name)
		}

		formatted := f.formatParagraph(name)
		result.WriteString(formatted)
		result.WriteString("\n\n")

		// Format children (which should be bullets)
		for _, child := range item.Children {
			childMd, err := f.FormatNode(child, depth+1)
			if err != nil {
				return "", err
			}
			result.WriteString(childMd)
		}

	case LayoutBullets:
		// Format as bullet
		result.WriteString(f.formatBullet(name, depth))

		// Format children
		for _, child := range item.Children {
			childMd, err := f.FormatNode(child, depth+1)
			if err != nil {
				return "", err
			}
			result.WriteString(childMd)
		}

	case LayoutTodo:
		result.WriteString(f.formatTodo(name, depth))

		// Format children
		for _, child := range item.Children {
			childMd, err := f.FormatNode(child, depth+1)
			if err != nil {
				return "", err
			}
			result.WriteString(childMd)
		}
	}

	return result.String(), nil
}

// ShouldExclude checks if node should be excluded from output
func (f *DefaultFormatter) ShouldExclude(item *workflowy.Item) bool {
	return HasTag(item.Name, f.config.ExcludeTag)
}

// GetLayoutMode determines effective layoutMode considering tags, depth, config
func (f *DefaultFormatter) GetLayoutMode(item *workflowy.Item, depth int) LayoutMode {
	// Check for tag overrides first
	if HasTag(item.Name, f.config.H1Tag) {
		return LayoutH1
	}
	if HasTag(item.Name, f.config.H2Tag) {
		return LayoutH2
	}
	if HasTag(item.Name, f.config.H3Tag) {
		return LayoutH3
	}
	if HasTag(item.Name, f.config.H4Tag) {
		return LayoutH4
	}
	if HasTag(item.Name, f.config.H5Tag) {
		return LayoutH5
	}
	if HasTag(item.Name, f.config.H6Tag) {
		return LayoutH6
	}

	// Check if item has layoutMode in Data
	if item.Data != nil {
		if mode, ok := item.Data["layoutMode"].(string); ok && mode != "" {
			return LayoutMode(mode)
		}
	}

	// Fallback: use depth for headers if configured
	if f.config.UseDepthForHeaders {
		switch depth {
		case 0:
			return LayoutH1
		case 1:
			return LayoutH2
		case 2:
			return LayoutH3
		case 3:
			return LayoutH4
		case 4:
			return LayoutH5
		case 5:
			return LayoutH6
		default:
			// Beyond h6, treat as paragraphs
			return LayoutP
		}
	}

	// Default to bullets
	return LayoutBullets
}

// formatHeader formats a header with appropriate casing
func (f *DefaultFormatter) formatHeader(name string, mode LayoutMode, item *workflowy.Item) string {
	var level int
	var uppercase bool

	switch mode {
	case LayoutH1:
		level = 1
		uppercase = f.config.H1Uppercase
	case LayoutH2:
		level = 2
		uppercase = f.config.H2Uppercase
	case LayoutH3:
		level = 3
		uppercase = f.config.H3Uppercase
	case LayoutH4:
		level = 4
		uppercase = f.config.H4Uppercase
	case LayoutH5:
		level = 5
		uppercase = f.config.H5Uppercase
	case LayoutH6:
		level = 6
		uppercase = f.config.H6Uppercase
	}

	if uppercase {
		name = Uppercase(name)
	}

	var result strings.Builder
	result.WriteString(HeaderPrefix(level))
	result.WriteString(name)
	result.WriteString("\n\n")

	// Format children (which should be paragraphs or nested structure)
	for _, child := range item.Children {
		childMd, _ := f.FormatNode(child, level) // Use level as depth for children
		result.WriteString(childMd)
	}

	return result.String()
}

// formatParagraph formats text as a paragraph with capitalization and punctuation
func (f *DefaultFormatter) formatParagraph(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	if f.config.ParagraphCapitalize {
		text = Capitalize(text)
	}

	if f.config.ParagraphPunctuate {
		text = Punctuate(text)
	}

	return text
}

// formatBullet formats text as a bullet point
func (f *DefaultFormatter) formatBullet(text string, depth int) string {
	return IndentBullet(depth) + text + "\n"
}

// formatTodo formats text as a todo item
func (f *DefaultFormatter) formatTodo(text string, depth int) string {
	return IndentBullet(depth) + "[ ] " + text + "\n"
}

// stripTags removes all configured tags from the text
func (f *DefaultFormatter) stripTags(text string) string {
	text = StripTag(text, f.config.ExcludeTag)
	text = StripTag(text, f.config.H1Tag)
	text = StripTag(text, f.config.H2Tag)
	text = StripTag(text, f.config.H3Tag)
	text = StripTag(text, f.config.H4Tag)
	text = StripTag(text, f.config.H5Tag)
	text = StripTag(text, f.config.H6Tag)
	return text
}

// hasBulletChildren checks if any immediate children are bullets
func (f *DefaultFormatter) hasBulletChildren(item *workflowy.Item) bool {
	for _, child := range item.Children {
		mode := f.GetLayoutMode(child, 0) // Depth doesn't matter for checking
		if mode == LayoutBullets {
			return true
		}
	}
	return false
}

// collectParagraphBullets collects consecutive bullets to join as paragraph
// Stops at empty bullet or non-paragraph-like content
func (f *DefaultFormatter) collectParagraphBullets(item *workflowy.Item) []string {
	// This is for a single bullet item - check if it should be treated as paragraph
	// For now, just return the item's name if it looks like paragraph text
	// More sophisticated logic could check siblings, etc.

	// If item has children, it's not a simple paragraph bullet
	if len(item.Children) > 0 {
		return nil
	}

	// If item name is empty, stop collection
	if IsEmpty(item.Name) {
		return nil
	}

	// Return this item as a paragraph component
	return []string{item.Name}
}
