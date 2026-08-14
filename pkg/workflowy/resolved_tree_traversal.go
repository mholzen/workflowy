package workflowy

import (
	"fmt"
	"strings"
)

type nodeSelection struct {
	occurrence NodeOccurrence
	activeIDs  []string
}

func (occurrence NodeOccurrence) Identity() string {
	ids := make([]string, 0, len(occurrence.Ancestors)+1)
	for _, ancestor := range occurrence.Ancestors {
		ids = append(ids, ancestor.ID)
	}
	if occurrence.Item != nil {
		ids = append(ids, occurrence.Item.ID)
	}
	return strings.Join(ids, "/")
}

func (occurrence NodeOccurrence) EqualityID() string {
	if occurrence.Item == nil {
		return ""
	}
	reference := MirrorReferenceFromItem(occurrence.Item)
	if reference.Kind == MirrorReferenceWithOrigin && reference.OriginID != "" {
		return reference.OriginID
	}
	return occurrence.Item.ID
}

func (occurrence NodeOccurrence) Snapshot() NodeOccurrence {
	snapshot := occurrence
	snapshot.Ancestors = append([]*Item(nil), occurrence.Ancestors...)
	return snapshot
}

func (tree *ResolvedTree) preferredSelection(
	scope []nodeSelection,
	readScopeID string,
	targetID string,
	options ResolveOptions,
	tracker *resolutionTracker,
) *nodeSelection {
	if original := tree.reachableOriginalSelection(scope, readScopeID, targetID, options, tracker); original != nil {
		return original
	}

	for _, root := range scope {
		if found := tree.findFirstSelection(root, targetID, options, tracker); found != nil {
			return found
		}
	}
	return nil
}

func (tree *ResolvedTree) reachableOriginalSelection(
	scope []nodeSelection,
	readScopeID string,
	targetID string,
	options ResolveOptions,
	tracker *resolutionTracker,
) *nodeSelection {
	targetNode := tree.index[targetID]
	if targetNode == nil {
		return nil
	}

	path := sourcePath(targetNode)
	if readScopeID != "" && readScopeID != "None" {
		scopeIndex := -1
		for index, item := range path {
			if item.ID == readScopeID {
				scopeIndex = index
				break
			}
		}
		if scopeIndex < 0 || len(scope) != 1 || scope[0].occurrence.ViaMirror {
			return nil
		}
		path = path[scopeIndex:]
	}

	if len(path) == 0 {
		return nil
	}
	selection := nodeSelection{
		occurrence: NodeOccurrence{
			Item:      path[0],
			Ancestors: append([]*Item(nil), scopeAncestors(scope, path[0])...),
			ViaMirror: scopeViaMirror(scope, path[0]),
		},
		activeIDs: append([]string(nil), scopeActiveIDs(scope, path[0])...),
	}
	if len(selection.activeIDs) == 0 {
		selection.activeIDs = []string{path[0].ID}
	}
	tracker.observe(selection)

	for index := 0; index < len(path)-1; index++ {
		parent := selection.occurrence.Item
		children, hiddenActive, childViaMirror := tree.resolvedChildren(selection, options, tracker)
		next := path[index+1]
		if !containsItemPointer(children, next) {
			return nil
		}
		selection.occurrence.Ancestors = append(selection.occurrence.Ancestors, parent)
		selection.occurrence.Item = next
		selection.occurrence.ViaMirror = childViaMirror || MirrorReferenceFromItem(next).IsMirror()
		selection.activeIDs = append(append([]string(nil), hiddenActive...), next.ID)
		tracker.observe(selection)
	}

	if selection.occurrence.ViaMirror {
		return nil
	}
	selection.occurrence.Ancestors = append([]*Item(nil), selection.occurrence.Ancestors...)
	selection.activeIDs = append([]string(nil), selection.activeIDs...)
	return &selection
}

func scopeAncestors(scope []nodeSelection, first *Item) []*Item {
	if len(scope) == 1 && scope[0].occurrence.Item == first {
		return scope[0].occurrence.Ancestors
	}
	return nil
}

func scopeViaMirror(scope []nodeSelection, first *Item) bool {
	return len(scope) == 1 && scope[0].occurrence.Item == first && scope[0].occurrence.ViaMirror
}

func scopeActiveIDs(scope []nodeSelection, first *Item) []string {
	if len(scope) == 1 && scope[0].occurrence.Item == first {
		return scope[0].activeIDs
	}
	return nil
}

func sourcePath(node *sourceNode) []*Item {
	reversed := make([]*Item, 0)
	for current := node; current != nil; current = current.parent {
		reversed = append(reversed, current.item)
	}
	path := make([]*Item, len(reversed))
	for index := range reversed {
		path[len(reversed)-1-index] = reversed[index]
	}
	return path
}

func containsItemPointer(items []*Item, target *Item) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func (tree *ResolvedTree) findFirstSelection(
	selection nodeSelection,
	targetID string,
	options ResolveOptions,
	tracker *resolutionTracker,
) *nodeSelection {
	tracker.observe(selection)
	if selection.occurrence.Item.ID == targetID {
		found := selection
		found.occurrence = selection.occurrence.Snapshot()
		found.activeIDs = append([]string(nil), selection.activeIDs...)
		return &found
	}

	children, activeIDs, childViaMirror := tree.resolvedChildren(selection, options, tracker)
	for _, child := range children {
		childSelection := nodeSelection{
			occurrence: NodeOccurrence{
				Item:      child,
				Ancestors: append(selection.occurrence.Ancestors, selection.occurrence.Item),
				ViaMirror: childViaMirror || MirrorReferenceFromItem(child).IsMirror(),
			},
			activeIDs: append(activeIDs, child.ID),
		}
		if found := tree.findFirstSelection(childSelection, targetID, options, tracker); found != nil {
			return found
		}
	}
	return nil
}

func (tree *ResolvedTree) materialize(
	selection nodeSelection,
	depth int,
	options ResolveOptions,
	tracker *resolutionTracker,
) *Item {
	tracker.observe(selection)
	item := cloneItemWithoutChildren(selection.occurrence.Item)
	tracker.retain(selection)
	if depth == 0 {
		return item
	}

	children, activeIDs, childViaMirror := tree.resolvedChildren(selection, options, tracker)
	childDepth := depth
	if childDepth > 0 {
		childDepth--
	}
	item.Children = make([]*Item, 0, len(children))
	for _, child := range children {
		childSelection := nodeSelection{
			occurrence: NodeOccurrence{
				Item:      child,
				Ancestors: append(selection.occurrence.Ancestors, selection.occurrence.Item),
				ViaMirror: childViaMirror || MirrorReferenceFromItem(child).IsMirror(),
			},
			activeIDs: append(activeIDs, child.ID),
		}
		item.Children = append(item.Children, tree.materialize(childSelection, childDepth, options, tracker))
	}
	return item
}

func (tree *ResolvedTree) Visit(
	readScopeID, targetID string,
	options ResolveOptions,
	visitor OccurrenceVisitor,
) (MirrorResolutionSummary, error) {
	tracker := newResolutionTracker(tree, readScopeID, targetID, options)
	defer tracker.finish()
	scope, err := tree.readScope(readScopeID, options, tracker)
	if err != nil {
		return tracker.summary, err
	}

	if targetID != "" && targetID != "None" {
		selection := tree.preferredSelection(scope, readScopeID, targetID, options, tracker)
		if selection == nil {
			return tracker.summary, fmt.Errorf(
				"Cannot find Workflowy node %q within resolved read scope %q from %s",
				targetID,
				readScopeID,
				tree.sourceLabel,
			)
		}
		scope = []nodeSelection{*selection}
	}

	for _, selection := range scope {
		if err := tree.visitSelection(selection, options.Depth, options, tracker, visitor); err != nil {
			return tracker.summary, err
		}
	}
	return tracker.summary, nil
}

func (tree *ResolvedTree) visitSelection(
	selection nodeSelection,
	depth int,
	options ResolveOptions,
	tracker *resolutionTracker,
	visitor OccurrenceVisitor,
) error {
	tracker.observe(selection)
	visitResult, err := visitor(selection.occurrence)
	if err != nil {
		return fmt.Errorf(
			"Cannot visit Workflowy node %q during %s from %s: %w",
			selection.occurrence.Item.ID,
			options.Operation,
			tree.sourceLabel,
			err,
		)
	}
	tracker.retainedOccurrences = visitResult.RetainedOccurrences
	if depth == 0 {
		return nil
	}

	children, activeIDs, childViaMirror := tree.resolvedChildren(selection, options, tracker)
	childDepth := depth
	if childDepth > 0 {
		childDepth--
	}
	for _, child := range children {
		ancestorCount := len(selection.occurrence.Ancestors)
		selection.occurrence.Ancestors = append(selection.occurrence.Ancestors, selection.occurrence.Item)
		childSelection := nodeSelection{
			occurrence: NodeOccurrence{
				Item:      child,
				Ancestors: selection.occurrence.Ancestors,
				ViaMirror: childViaMirror || MirrorReferenceFromItem(child).IsMirror(),
			},
			activeIDs: append(activeIDs, child.ID),
		}
		if err := tree.visitSelection(childSelection, childDepth, options, tracker, visitor); err != nil {
			return err
		}
		selection.occurrence.Ancestors = selection.occurrence.Ancestors[:ancestorCount]
	}
	return nil
}

func (tree *ResolvedTree) resolvedChildren(
	selection nodeSelection,
	options ResolveOptions,
	tracker *resolutionTracker,
) ([]*Item, []string, bool) {
	item := selection.occurrence.Item
	reference := MirrorReferenceFromItem(item)
	if !reference.IsMirror() {
		return item.Children, selection.activeIDs, selection.occurrence.ViaMirror
	}
	if !options.ResolveMirrors {
		return nil, selection.activeIDs, true
	}

	switch reference.Kind {
	case MirrorReferenceNullOrigin:
		return item.Children, selection.activeIDs, true
	case MirrorReferenceMalformed:
		tracker.record("malformed", selection, reference)
		return item.Children, selection.activeIDs, true
	case MirrorReferenceWithOrigin:
		origin := tree.index[reference.OriginID]
		if origin == nil {
			tracker.record("missing", selection, reference)
			return item.Children, selection.activeIDs, true
		}
		if containsString(selection.activeIDs, reference.OriginID) {
			tracker.record("cycle", selection, reference)
			return nil, selection.activeIDs, true
		}
		tracker.record("resolved", selection, reference)
		activeIDs := append(selection.activeIDs, reference.OriginID)
		return origin.item.Children, activeIDs, true
	default:
		return item.Children, selection.activeIDs, selection.occurrence.ViaMirror
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
