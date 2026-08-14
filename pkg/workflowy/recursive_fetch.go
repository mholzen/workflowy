package workflowy

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

type RecursiveFetchOptions struct {
	Depth          int
	ResolveMirrors bool
	Operation      string
	RootItem       *Item
}

type RecursiveFetchResult struct {
	Response *ListChildrenResponse
	Summary  MirrorResolutionSummary
}

// ListChildrenRecursive retrieves children to the default depth. For a non-root
// ID, this compatibility method retrieves the root item before listing children.
func (wc *WorkflowyClient) ListChildrenRecursive(ctx context.Context, itemID string) (*ListChildrenResponse, error) {
	return wc.ListChildrenRecursiveWithDepth(ctx, itemID, 5)
}

// ListChildrenRecursiveWithDepth retrieves children to the requested depth. For
// a non-root ID, this compatibility method retrieves the root item first so its
// mirror metadata and initial cycle path are available.
func (wc *WorkflowyClient) ListChildrenRecursiveWithDepth(ctx context.Context, itemID string, depth int) (*ListChildrenResponse, error) {
	result, err := wc.ListChildrenRecursiveWithOptions(ctx, itemID, RecursiveFetchOptions{
		Depth:          depth,
		ResolveMirrors: true,
		Operation:      "get",
	})
	if err != nil {
		return nil, err
	}
	return result.Response, nil
}

func (wc *WorkflowyClient) ListChildrenRecursiveWithOptions(
	ctx context.Context,
	itemID string,
	options RecursiveFetchOptions,
) (*RecursiveFetchResult, error) {
	if options.RootItem != nil && options.RootItem.ID != itemID {
		return nil, fmt.Errorf(
			"Cannot recursively fetch Workflowy node %q: supplied root item has ID %q",
			itemID,
			options.RootItem.ID,
		)
	}

	tracker := newRecursiveFetchTracker(wc.recursiveSourceLabel(), itemID, options)
	defer tracker.finish()
	if options.Depth <= 0 {
		return &RecursiveFetchResult{Response: &ListChildrenResponse{}, Summary: tracker.summary}, nil
	}

	if itemID == "None" {
		response, err := wc.ListChildren(ctx, itemID)
		if err != nil {
			return nil, recursiveFetchError(itemID, options.Operation, tracker.source, err)
		}
		for _, item := range response.Items {
			occurrencePath := []string{item.ID}
			activeIDs := []string{item.ID}
			tracker.observe(occurrencePath)
			tracker.retain(occurrencePath)
			if options.Depth > 1 {
				if err := wc.populateRecursiveChildren(ctx, item, options.Depth-1, options, occurrencePath, activeIDs, tracker); err != nil {
					return nil, err
				}
			}
		}
		return &RecursiveFetchResult{Response: response, Summary: tracker.summary}, nil
	}

	root := options.RootItem
	if root == nil {
		var err error
		root, err = wc.GetItem(ctx, itemID)
		if err != nil {
			return nil, recursiveFetchError(itemID, options.Operation, tracker.source, err)
		}
		if root.ID != itemID {
			return nil, fmt.Errorf(
				"Cannot recursively fetch Workflowy node %q: API returned root item with ID %q",
				itemID,
				root.ID,
			)
		}
	}
	occurrencePath := []string{root.ID}
	activeIDs := []string{root.ID}
	tracker.observe(occurrencePath)
	response, err := wc.fetchRecursiveChildren(ctx, root, options.Depth, options, occurrencePath, activeIDs, tracker)
	if err != nil {
		return nil, err
	}
	return &RecursiveFetchResult{Response: response, Summary: tracker.summary}, nil
}

func (wc *WorkflowyClient) populateRecursiveChildren(
	ctx context.Context,
	item *Item,
	depth int,
	options RecursiveFetchOptions,
	occurrencePath []string,
	activeIDs []string,
	tracker *recursiveFetchTracker,
) error {
	response, err := wc.fetchRecursiveChildren(ctx, item, depth, options, occurrencePath, activeIDs, tracker)
	if err != nil {
		return err
	}
	item.Children = response.Items
	return nil
}

func (wc *WorkflowyClient) fetchRecursiveChildren(
	ctx context.Context,
	item *Item,
	depth int,
	options RecursiveFetchOptions,
	occurrencePath []string,
	activeIDs []string,
	tracker *recursiveFetchTracker,
) (*ListChildrenResponse, error) {
	if depth <= 0 {
		return &ListChildrenResponse{}, nil
	}

	reference := MirrorReferenceFromItem(item)
	if reference.IsMirror() && !options.ResolveMirrors {
		return &ListChildrenResponse{}, nil
	}
	if options.ResolveMirrors {
		switch reference.Kind {
		case MirrorReferenceMalformed:
			tracker.recordMalformed(item, reference, occurrencePath)
		case MirrorReferenceWithOrigin:
			if containsString(activeIDs, reference.OriginID) {
				tracker.recordCycle(item, reference, occurrencePath)
				return &ListChildrenResponse{}, nil
			}
			tracker.recordResolved(occurrencePath)
			activeIDs = append(activeIDs, reference.OriginID)
		}
	}

	response, err := wc.ListChildren(ctx, item.ID)
	if err != nil {
		return nil, recursiveFetchError(item.ID, options.Operation, tracker.source, err)
	}
	for _, child := range response.Items {
		childOccurrencePath := append(occurrencePath, child.ID)
		childActiveIDs := append(activeIDs, child.ID)
		tracker.observe(childOccurrencePath)
		tracker.retain(childOccurrencePath)
		if depth > 1 {
			if err := wc.populateRecursiveChildren(ctx, child, depth-1, options, childOccurrencePath, childActiveIDs, tracker); err != nil {
				return nil, err
			}
		}
	}
	return response, nil
}

func recursiveFetchError(itemID, operation, source string, err error) error {
	return fmt.Errorf(
		"Cannot recursively fetch Workflowy node %q during %s from %s: %w",
		itemID,
		operation,
		source,
		err,
	)
}

func (wc *WorkflowyClient) recursiveSourceLabel() string {
	if wc.deployment == "" {
		return "Workflowy API"
	}
	return fmt.Sprintf("Workflowy %s API", wc.deployment)
}

type recursiveFetchTracker struct {
	logger              *slog.Logger
	source              string
	targetID            string
	options             RecursiveFetchOptions
	startedAt           time.Time
	debugEnabled        bool
	summary             MirrorResolutionSummary
	cycles              map[string]*cycleDiagnostic
	visited             map[string]struct{}
	retained            map[string]struct{}
	visitedOccurrences  int
	retainedOccurrences int
	currentPathDepth    int
	maximumPathDepth    int
	nextProgress        int
}

func newRecursiveFetchTracker(source, targetID string, options RecursiveFetchOptions) *recursiveFetchTracker {
	logger := slog.Default()
	tracker := &recursiveFetchTracker{
		logger:       logger,
		source:       source,
		targetID:     targetID,
		options:      options,
		startedAt:    time.Now(),
		debugEnabled: logger.Enabled(context.Background(), slog.LevelDebug),
		cycles:       make(map[string]*cycleDiagnostic),
		visited:      make(map[string]struct{}),
		retained:     make(map[string]struct{}),
		nextProgress: traversalProgressStart,
	}
	if tracker.debugEnabled {
		tracker.logDebug("Workflowy recursive mirror traversal started")
	}
	return tracker
}

func (tracker *recursiveFetchTracker) observe(path []string) {
	identity := strings.Join(path, "/")
	if _, exists := tracker.visited[identity]; exists {
		return
	}
	tracker.visited[identity] = struct{}{}
	tracker.visitedOccurrences++
	tracker.currentPathDepth = len(path)
	if len(path) > tracker.maximumPathDepth {
		tracker.maximumPathDepth = len(path)
	}
	if tracker.debugEnabled && tracker.visitedOccurrences >= tracker.nextProgress {
		tracker.logDebug("Workflowy recursive mirror traversal progress")
		tracker.nextProgress *= 2
	}
}

func (tracker *recursiveFetchTracker) retain(path []string) {
	identity := strings.Join(path, "/")
	if _, exists := tracker.retained[identity]; exists {
		return
	}
	tracker.retained[identity] = struct{}{}
	tracker.retainedOccurrences++
}

func (tracker *recursiveFetchTracker) recordResolved(path []string) {
	tracker.summary.Resolved++
	tracker.observe(path)
}

func (tracker *recursiveFetchTracker) recordMalformed(item *Item, reference MirrorReference, path []string) {
	tracker.summary.MalformedMetadata++
	tracker.logger.Warn(
		"Workflowy mirror metadata is malformed; using API children",
		"mirror_id", item.ID,
		"field", reference.Field,
		"value_type", reference.ValueType,
		"path", strings.Join(path, "/"),
		"source", tracker.source,
		"operation", tracker.options.Operation,
	)
}

func (tracker *recursiveFetchTracker) recordCycle(item *Item, reference MirrorReference, path []string) {
	tracker.summary.Cycles++
	key := item.ID + "\x00" + reference.OriginID
	diagnostic := tracker.cycles[key]
	if diagnostic == nil {
		diagnostic = &cycleDiagnostic{mirrorID: item.ID, originID: reference.OriginID, firstPath: strings.Join(path, "/")}
		tracker.cycles[key] = diagnostic
	}
	diagnostic.occurrences++
}

func (tracker *recursiveFetchTracker) finish() {
	for _, diagnostic := range tracker.cycles {
		tracker.logger.Warn(
			"Workflowy mirror cycle stopped",
			"mirror_id", diagnostic.mirrorID,
			"origin_id", diagnostic.originID,
			"path", diagnostic.firstPath,
			"occurrences", diagnostic.occurrences,
			"source", tracker.source,
			"operation", tracker.options.Operation,
		)
	}
	if warning := tracker.summary.ThresholdWarning(tracker.options.Operation, tracker.source); warning != "" {
		tracker.logger.Warn(warning)
	}
	attributes := tracker.attributes()
	attributes = append(attributes,
		"resolved", tracker.summary.Resolved,
		"missing_origin", tracker.summary.MissingOrigin,
		"malformed_metadata", tracker.summary.MalformedMetadata,
		"cycles", tracker.summary.Cycles,
		"attempts", tracker.summary.Attempts(),
		"failures", tracker.summary.Failures(),
	)
	if tracker.debugEnabled {
		attributes = append(attributes, recursiveMemoryAttributes()...)
	}
	tracker.logger.Info("Workflowy recursive mirror traversal completed", attributes...)
}

func (tracker *recursiveFetchTracker) logDebug(message string) {
	attributes := append(tracker.attributes(), recursiveMemoryAttributes()...)
	tracker.logger.Debug(message, attributes...)
}

func (tracker *recursiveFetchTracker) attributes() []interface{} {
	return []interface{}{
		"operation", tracker.options.Operation,
		"source", tracker.source,
		"scope", tracker.targetID,
		"target", tracker.targetID,
		"requested_depth", tracker.options.Depth,
		"effective_depth", tracker.options.Depth,
		"visited_occurrences", tracker.visitedOccurrences,
		"retained_occurrences", tracker.retainedOccurrences,
		"current_path_depth", tracker.currentPathDepth,
		"maximum_path_depth", tracker.maximumPathDepth,
		"elapsed", time.Since(tracker.startedAt),
	}
}

func recursiveMemoryAttributes() []interface{} {
	stats := readTraversalMemoryStats()
	return []interface{}{
		"heap_bytes", stats.HeapBytes,
		"heap_objects", stats.HeapObjects,
		"total_allocated_bytes", stats.TotalAllocatedBytes,
		"gc_count", stats.GarbageCollections,
	}
}
