package formatter

import "github.com/mholzen/workflowy/pkg/workflowy"

// LayoutMode represents the type of content layout
type LayoutMode string

const (
	LayoutH1      LayoutMode = "h1"
	LayoutH2      LayoutMode = "h2"
	LayoutH3      LayoutMode = "h3"
	LayoutH4      LayoutMode = "h4"
	LayoutH5      LayoutMode = "h5"
	LayoutH6      LayoutMode = "h6"
	LayoutP       LayoutMode = "p"
	LayoutBullets LayoutMode = "bullets"
	LayoutTodo    LayoutMode = "todo"
)

// Formatter defines the interface for converting WorkFlowy items to markdown
type Formatter interface {
	// FormatTree converts entire tree to markdown
	FormatTree(items []*workflowy.Item) (string, error)

	// FormatNode converts a single node to markdown
	FormatNode(item *workflowy.Item, depth int) (string, error)

	// ShouldExclude checks if node should be excluded from output
	ShouldExclude(item *workflowy.Item) bool

	// GetLayoutMode determines effective layoutMode considering tags, depth, config
	GetLayoutMode(item *workflowy.Item, depth int) LayoutMode
}

// Config holds formatter configuration
type Config struct {
	Name string

	// Header rules
	H1Uppercase bool
	H2Uppercase bool
	H3Uppercase bool
	H4Uppercase bool
	H5Uppercase bool
	H6Uppercase bool

	// Paragraph rules
	ParagraphCapitalize      bool
	ParagraphPunctuate       bool
	JoinBulletsAsParagraphs  bool // Join consecutive bullets as paragraphs until empty bullet

	// Punctuation rules
	AddColonBeforeLists bool // Add colon at end of paragraph node with bullet children

	// Tag recognition
	ExcludeTag string // default: "#exclude"
	H1Tag      string // default: "#h1"
	H2Tag      string // default: "#h2"
	H3Tag      string // default: "#h3"
	H4Tag      string // default: "#h4"
	H5Tag      string // default: "#h5"
	H6Tag      string // default: "#h6"

	// Fallback behavior when no layoutMode
	UseDepthForHeaders bool // depth 1=h1, 2=h2, 3=h3, etc.
}

// DefaultConfig returns the default formatter configuration
func DefaultConfig() *Config {
	return &Config{
		Name: "default",

		// Headers
		H1Uppercase: true,
		H2Uppercase: false,
		H3Uppercase: false,
		H4Uppercase: false,
		H5Uppercase: false,
		H6Uppercase: false,

		// Paragraphs
		ParagraphCapitalize:     true,
		ParagraphPunctuate:      true,
		JoinBulletsAsParagraphs: true,

		// Punctuation
		AddColonBeforeLists: true,

		// Tags
		ExcludeTag: "#exclude",
		H1Tag:      "#h1",
		H2Tag:      "#h2",
		H3Tag:      "#h3",
		H4Tag:      "#h4",
		H5Tag:      "#h5",
		H6Tag:      "#h6",

		// Fallback
		UseDepthForHeaders: true,
	}
}
