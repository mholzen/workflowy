package formatter

import (
	"testing"

	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
)

func TestExample1_BasicHeadersAndParagraphs(t *testing.T) {
	items := []*workflowy.Item{
		{
			Name: "Item A",
			Children: []*workflowy.Item{
				{Name: "Item A1"},
				{Name: "Item A2"},
			},
		},
		{
			Name: "Item B",
			Children: []*workflowy.Item{
				{Name: "Item B1"},
				{Name: ""},
				{Name: "Item B2"},
			},
		},
		{
			Name: "Item C",
			Children: []*workflowy.Item{
				{
					Name: "Item C1",
					Children: []*workflowy.Item{
						{Name: "Item C11"},
						{Name: "Item C12"},
					},
				},
				{
					Name: "Item C2",
					Children: []*workflowy.Item{
						{Name: "Item C21"},
						{Name: "Item C22"},
					},
				},
			},
		},
	}

	expected := `# Item A
Item A1. Item A2.

# Item B
Item B1.

Item B2.

# Item C

## Item C1
Item C11. Item C12.

## Item C2
Item C21. Item C22.
`

	formatter := NewMarkdownFormatter()
	result, err := formatter.FormatTree(items)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestExample2_MixedChildrenWithSubheaders(t *testing.T) {
	items := []*workflowy.Item{
		{
			Name: "Item C",
			Children: []*workflowy.Item{
				{Name: "Item C1"},
				{
					Name: "Item C2",
					Children: []*workflowy.Item{
						{Name: "Item C21, a paragraph with some text"},
						{Name: "Item C22, another paragraph with more text"},
					},
				},
				{
					Name: "Item C2",
					Children: []*workflowy.Item{
						{Name: "Item C21, a third paragraph with so much text"},
						{Name: "Item C22, all these paragraphs"},
					},
				},
			},
		},
	}

	expected := `# Item C

Item C1.

## Item C2
Item C21, a paragraph with some text. Item C22, another paragraph with more text.

## Item C2
Item C21, a third paragraph with so much text. Item C22, all these paragraphs.
`

	formatter := NewMarkdownFormatter()
	result, err := formatter.FormatTree(items)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestExample3_ListDetection(t *testing.T) {
	items := []*workflowy.Item{
		{
			Name: "Item A",
			Children: []*workflowy.Item{
				{Name: "This is a rather lengthy paragraph"},
				{
					Name: "This looks like the beginning of a list:",
					Children: []*workflowy.Item{
						{Name: "item 1"},
						{Name: "item 2"},
						{Name: "item 3"},
					},
				},
				{Name: "This looks like another lengthy paragraph."},
			},
		},
	}

	expected := `# Item A
This is a rather lengthy paragraph. This looks like the beginning of a list:
- item 1
- item 2
- item 3

This looks like another lengthy paragraph.
`

	formatter := NewMarkdownFormatter()
	result, err := formatter.FormatTree(items)

	assert.NoError(t, err)
	assert.Equal(t, expected, result)
}

func TestIsListIntroduction(t *testing.T) {
	assert.True(t, IsListIntroduction("This is a list:"))
	assert.True(t, IsListIntroduction("Items:"))
	assert.False(t, IsListIntroduction("This is not a list"))
	assert.False(t, IsListIntroduction(""))
	assert.False(t, IsListIntroduction("   "))
}

func TestAreChildrenSimilarLength(t *testing.T) {
	similar := []*workflowy.Item{
		{Name: "item one"},
		{Name: "item two"},
		{Name: "item three"},
	}
	assert.True(t, AreChildrenSimilarLength(similar))

	dissimilar := []*workflowy.Item{
		{Name: "short"},
		{Name: "this is a much longer item with many more words"},
	}
	assert.False(t, AreChildrenSimilarLength(dissimilar))

	single := []*workflowy.Item{
		{Name: "only one"},
	}
	assert.True(t, AreChildrenSimilarLength(single))

	empty := []*workflowy.Item{}
	assert.True(t, AreChildrenSimilarLength(empty))
}

func TestIsListPattern(t *testing.T) {
	listItem := &workflowy.Item{
		Name: "Here are some items:",
		Children: []*workflowy.Item{
			{Name: "item 1"},
			{Name: "item 2"},
			{Name: "item 3"},
		},
	}
	assert.True(t, IsListPattern(listItem))

	noColon := &workflowy.Item{
		Name: "Here are some items",
		Children: []*workflowy.Item{
			{Name: "item 1"},
			{Name: "item 2"},
		},
	}
	assert.False(t, IsListPattern(noColon))

	hasGrandchildren := &workflowy.Item{
		Name: "Here are some items:",
		Children: []*workflowy.Item{
			{Name: "item 1", Children: []*workflowy.Item{{Name: "subitem"}}},
			{Name: "item 2"},
		},
	}
	assert.False(t, IsListPattern(hasGrandchildren))

	noChildren := &workflowy.Item{
		Name: "Here are some items:",
	}
	assert.False(t, IsListPattern(noChildren))
}

func TestFormatAsSentence(t *testing.T) {
	assert.Equal(t, "Hello world.", FormatAsSentence("hello world"))
	assert.Equal(t, "Already punctuated!", FormatAsSentence("already punctuated!"))
	assert.Equal(t, "Has question?", FormatAsSentence("has question?"))
	assert.Equal(t, "Has colon:", FormatAsSentence("has colon:"))
	assert.Equal(t, "Capitalized.", FormatAsSentence("Capitalized"))
	assert.Equal(t, "", FormatAsSentence(""))
}

func TestExcludeTag(t *testing.T) {
	items := []*workflowy.Item{
		{
			Name: "Visible Item",
			Children: []*workflowy.Item{
				{Name: "Child 1"},
				{Name: "Child 2 #exclude"},
				{Name: "Child 3"},
			},
		},
	}

	formatter := NewMarkdownFormatter()
	result, err := formatter.FormatTree(items)

	assert.NoError(t, err)
	assert.Contains(t, result, "Child 1")
	assert.NotContains(t, result, "Child 2")
	assert.Contains(t, result, "Child 3")
}

func TestTagStripping(t *testing.T) {
	items := []*workflowy.Item{
		{
			Name: "Header #h1",
			Children: []*workflowy.Item{
				{Name: "Paragraph content"},
			},
		},
	}

	formatter := NewMarkdownFormatter()
	result, err := formatter.FormatTree(items)

	assert.NoError(t, err)
	assert.Contains(t, result, "# Header")
	assert.NotContains(t, result, "#h1")
}

func TestEmptyBulletsCollapse(t *testing.T) {
	items := []*workflowy.Item{
		{
			Name: "Header",
			Children: []*workflowy.Item{
				{Name: "First paragraph"},
				{Name: ""},
				{Name: ""},
				{Name: ""},
				{Name: "Second paragraph"},
			},
		},
	}

	formatter := NewMarkdownFormatter()
	result, err := formatter.FormatTree(items)

	assert.NoError(t, err)
	assert.Contains(t, result, "First paragraph.")
	assert.Contains(t, result, "Second paragraph.")
}

func TestLayoutModeFromData(t *testing.T) {
	items := []*workflowy.Item{
		{
			Name: "Quote content",
			Data: map[string]interface{}{"layoutMode": "quote"},
			Children: []*workflowy.Item{
				{Name: "Line 1"},
				{Name: "Line 2"},
			},
		},
	}

	formatter := NewMarkdownFormatter()
	result, err := formatter.FormatTree(items)

	assert.NoError(t, err)
	assert.Contains(t, result, "> Quote content")
	assert.Contains(t, result, "> Line 1")
}

func TestDividerLayoutMode(t *testing.T) {
	items := []*workflowy.Item{
		{
			Name: "Header",
			Children: []*workflowy.Item{
				{Name: "Content"},
			},
		},
		{
			Name: "",
			Data: map[string]interface{}{"layoutMode": "divider"},
		},
		{
			Name: "Another Header",
			Children: []*workflowy.Item{
				{Name: "More content"},
			},
		},
	}

	formatter := NewMarkdownFormatter()
	result, err := formatter.FormatTree(items)

	assert.NoError(t, err)
	assert.Contains(t, result, "---")
}

func TestCodeLayoutMode(t *testing.T) {
	items := []*workflowy.Item{
		{
			Name: "function example()",
			Data: map[string]interface{}{"layoutMode": "code"},
			Children: []*workflowy.Item{
				{Name: "  return true"},
			},
		},
	}

	formatter := NewMarkdownFormatter()
	result, err := formatter.FormatTree(items)

	assert.NoError(t, err)
	assert.Contains(t, result, "```")
	assert.Contains(t, result, "function example()")
	assert.Contains(t, result, "return true")
}

func TestWordCount(t *testing.T) {
	assert.Equal(t, 0, WordCount(""))
	assert.Equal(t, 0, WordCount("   "))
	assert.Equal(t, 1, WordCount("hello"))
	assert.Equal(t, 3, WordCount("hello world there"))
	assert.Equal(t, 3, WordCount("  hello   world   there  "))
}

func TestChildrenHaveGrandchildren(t *testing.T) {
	withGrandchildren := []*workflowy.Item{
		{Name: "child1"},
		{Name: "child2", Children: []*workflowy.Item{{Name: "grandchild"}}},
	}
	assert.True(t, ChildrenHaveGrandchildren(withGrandchildren))

	withoutGrandchildren := []*workflowy.Item{
		{Name: "child1"},
		{Name: "child2"},
	}
	assert.False(t, ChildrenHaveGrandchildren(withoutGrandchildren))

	empty := []*workflowy.Item{}
	assert.False(t, ChildrenHaveGrandchildren(empty))
}

