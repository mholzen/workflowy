package workflowy

import (
	"fmt"
	"log/slog"
)

type ResolveOptions struct {
	ResolveMirrors bool
	Depth          int
	Operation      string
}

type MirrorResolutionSummary struct {
	Resolved          int
	MissingOrigin     int
	MalformedMetadata int
	Cycles            int
}

type ResolvedFetch struct {
	Item       *Item
	Items      []*Item
	Occurrence NodeOccurrence
	Summary    MirrorResolutionSummary
}

type NodeOccurrence struct {
	Item      *Item
	Ancestors []*Item
	ViaMirror bool
}

type OccurrenceVisitResult struct {
	RetainedOccurrences int
}

type OccurrenceVisitor func(NodeOccurrence) (OccurrenceVisitResult, error)

type sourceNode struct {
	item   *Item
	parent *sourceNode
}

type ResolvedTree struct {
	sourceRoots       []*Item
	sourceLabel       string
	index             map[string]*sourceNode
	logger            *slog.Logger
	memoryStatsReader func() traversalMemoryStats
	sourceNodeCount   int
	mirrorCount       int
}

func NewResolvedTree(sourceRoots []*Item, sourceLabel string) *ResolvedTree {
	tree := &ResolvedTree{
		sourceRoots:       sourceRoots,
		sourceLabel:       sourceLabel,
		index:             make(map[string]*sourceNode),
		logger:            slog.Default(),
		memoryStatsReader: readTraversalMemoryStats,
	}
	for _, root := range sourceRoots {
		tree.indexSource(root, nil)
	}
	tree.logger.Debug(
		"indexed Workflowy resolved tree source",
		"source", sourceLabel,
		"source_roots", len(sourceRoots),
		"source_nodes", tree.sourceNodeCount,
		"mirrors", tree.mirrorCount,
	)
	return tree
}

func (tree *ResolvedTree) indexSource(item *Item, parent *sourceNode) {
	if item == nil {
		return
	}
	tree.sourceNodeCount++
	if MirrorReferenceFromItem(item).IsMirror() {
		tree.mirrorCount++
	}

	node := &sourceNode{item: item, parent: parent}
	if _, exists := tree.index[item.ID]; !exists {
		tree.index[item.ID] = node
	}
	for _, child := range item.Children {
		tree.indexSource(child, node)
	}
}

func (tree *ResolvedTree) Fetch(readScopeID, targetID string, options ResolveOptions) (*ResolvedFetch, error) {
	tracker := newResolutionTracker(tree, readScopeID, targetID, options)
	defer tracker.finish()
	scope, err := tree.readScope(readScopeID, options, tracker)
	if err != nil {
		return nil, err
	}

	if targetID == "None" || targetID == "" {
		items := make([]*Item, 0, len(scope))
		for _, selection := range scope {
			items = append(items, tree.materialize(selection, options.Depth, options, tracker))
		}
		return &ResolvedFetch{Items: items, Summary: tracker.summary}, nil
	}

	selection := tree.preferredSelection(scope, readScopeID, targetID, options, tracker)
	if selection == nil {
		return nil, fmt.Errorf(
			"Cannot find Workflowy node %q within resolved read scope %q from %s",
			targetID,
			readScopeID,
			tree.sourceLabel,
		)
	}

	item := tree.materialize(*selection, options.Depth, options, tracker)
	return &ResolvedFetch{
		Item:       item,
		Occurrence: selection.occurrence.Snapshot(),
		Summary:    tracker.summary,
	}, nil
}

func (tree *ResolvedTree) readScope(readScopeID string, options ResolveOptions, tracker *resolutionTracker) ([]nodeSelection, error) {
	if readScopeID == "" || readScopeID == "None" {
		selections := make([]nodeSelection, 0, len(tree.sourceRoots))
		for _, root := range tree.sourceRoots {
			selections = append(selections, selectionForRoot(root))
		}
		return selections, nil
	}

	rootSelections := make([]nodeSelection, 0, len(tree.sourceRoots))
	for _, root := range tree.sourceRoots {
		rootSelections = append(rootSelections, selectionForRoot(root))
	}
	selection := tree.preferredSelection(rootSelections, "None", readScopeID, options, tracker)
	if selection == nil {
		return nil, fmt.Errorf(
			"Cannot find Workflowy read scope %q in %s",
			readScopeID,
			tree.sourceLabel,
		)
	}
	return []nodeSelection{*selection}, nil
}

func selectionForRoot(root *Item) nodeSelection {
	active := []string{root.ID}
	return nodeSelection{
		occurrence: NodeOccurrence{Item: root, ViaMirror: MirrorReferenceFromItem(root).IsMirror()},
		activeIDs:  active,
	}
}

func cloneItemWithoutChildren(item *Item) *Item {
	if item == nil {
		return nil
	}
	clone := *item
	clone.Note = cloneStringPointer(item.Note)
	clone.CompletedAt = cloneInt64Pointer(item.CompletedAt)
	clone.Data = cloneStringMap(item.Data)
	clone.Children = nil
	return &clone
}

func cloneStringPointer(value *string) *string {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneInt64Pointer(value *int64) *int64 {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func cloneStringMap(source map[string]interface{}) map[string]interface{} {
	if source == nil {
		return nil
	}
	clone := make(map[string]interface{}, len(source))
	for key, value := range source {
		clone[key] = cloneDataValue(value)
	}
	return clone
}

func cloneDataValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case map[string]interface{}:
		return cloneStringMap(typed)
	case []interface{}:
		clone := make([]interface{}, len(typed))
		for index, element := range typed {
			clone[index] = cloneDataValue(element)
		}
		return clone
	default:
		return value
	}
}
