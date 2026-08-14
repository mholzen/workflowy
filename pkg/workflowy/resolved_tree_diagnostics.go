package workflowy

import (
	"context"
	"fmt"
	"log/slog"
	"runtime"
	"time"
)

const traversalProgressStart = 1000

type traversalMemoryStats struct {
	HeapBytes           uint64
	HeapObjects         uint64
	TotalAllocatedBytes uint64
	GarbageCollections  uint32
}

type cycleDiagnostic struct {
	mirrorID    string
	originID    string
	firstPath   string
	occurrences int
}

type resolutionTracker struct {
	tree                *ResolvedTree
	options             ResolveOptions
	readScopeID         string
	targetID            string
	startedAt           time.Time
	debugEnabled        bool
	summary             MirrorResolutionSummary
	seenEvents          map[string]struct{}
	seenOccurrences     map[string]struct{}
	retainedIdentities  map[string]struct{}
	cycles              map[string]*cycleDiagnostic
	visitedOccurrences  int
	retainedOccurrences int
	currentPathDepth    int
	maximumPathDepth    int
	nextProgress        int
}

func (summary MirrorResolutionSummary) Attempts() int {
	return summary.Resolved + summary.Failures()
}

func (summary MirrorResolutionSummary) Failures() int {
	return summary.MissingOrigin + summary.MalformedMetadata + summary.Cycles
}

func (summary MirrorResolutionSummary) ThresholdWarning(operation, source string) string {
	attempts := summary.Attempts()
	if attempts == 0 || summary.Failures()*2 < attempts {
		return ""
	}
	failureRatio := float64(summary.Failures()) / float64(attempts) * 100
	return fmt.Sprintf(
		"Mirror resolution failures reached %.1f%% during %s from %s (attempts=%d failures=%d resolved=%d missing_origin=%d malformed_metadata=%d cycles=%d)",
		failureRatio,
		operation,
		source,
		attempts,
		summary.Failures(),
		summary.Resolved,
		summary.MissingOrigin,
		summary.MalformedMetadata,
		summary.Cycles,
	)
}

func readTraversalMemoryStats() traversalMemoryStats {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return traversalMemoryStats{
		HeapBytes:           stats.HeapAlloc,
		HeapObjects:         stats.HeapObjects,
		TotalAllocatedBytes: stats.TotalAlloc,
		GarbageCollections:  stats.NumGC,
	}
}

func newResolutionTracker(tree *ResolvedTree, readScopeID, targetID string, options ResolveOptions) *resolutionTracker {
	tracker := &resolutionTracker{
		tree:               tree,
		options:            options,
		readScopeID:        readScopeID,
		targetID:           targetID,
		startedAt:          time.Now(),
		debugEnabled:       tree.logger.Enabled(context.Background(), slog.LevelDebug),
		seenEvents:         make(map[string]struct{}),
		seenOccurrences:    make(map[string]struct{}),
		retainedIdentities: make(map[string]struct{}),
		cycles:             make(map[string]*cycleDiagnostic),
		nextProgress:       traversalProgressStart,
	}
	if tracker.debugEnabled {
		tracker.logWithMemory(slog.LevelDebug, "Workflowy mirror traversal started")
	}
	return tracker
}

func (tracker *resolutionTracker) observe(selection nodeSelection) {
	identity := selection.occurrence.Identity()
	if _, exists := tracker.seenOccurrences[identity]; exists {
		return
	}
	tracker.seenOccurrences[identity] = struct{}{}
	tracker.visitedOccurrences++
	tracker.currentPathDepth = len(selection.occurrence.Ancestors) + 1
	if tracker.currentPathDepth > tracker.maximumPathDepth {
		tracker.maximumPathDepth = tracker.currentPathDepth
	}
	if tracker.debugEnabled && tracker.visitedOccurrences >= tracker.nextProgress {
		tracker.logWithMemory(slog.LevelDebug, "Workflowy mirror traversal progress")
		tracker.nextProgress *= 2
	}
}

func (tracker *resolutionTracker) retain(selection nodeSelection) {
	identity := selection.occurrence.Identity()
	if _, exists := tracker.retainedIdentities[identity]; exists {
		return
	}
	tracker.retainedIdentities[identity] = struct{}{}
	tracker.retainedOccurrences++
}

func (tracker *resolutionTracker) record(kind string, selection nodeSelection, reference MirrorReference) {
	identity := selection.occurrence.Identity()
	eventKey := kind + "\x00" + identity
	if _, exists := tracker.seenEvents[eventKey]; exists {
		return
	}
	tracker.seenEvents[eventKey] = struct{}{}

	switch kind {
	case "resolved":
		tracker.summary.Resolved++
	case "missing":
		tracker.summary.MissingOrigin++
		tracker.tree.logger.Warn(
			"Workflowy mirror origin is missing; using source children",
			"mirror_id", selection.occurrence.Item.ID,
			"origin_id", reference.OriginID,
			"path", identity,
			"source", tracker.tree.sourceLabel,
			"operation", tracker.options.Operation,
		)
	case "malformed":
		tracker.summary.MalformedMetadata++
		tracker.tree.logger.Warn(
			"Workflowy mirror metadata is malformed; using source children",
			"mirror_id", selection.occurrence.Item.ID,
			"field", reference.Field,
			"value_type", reference.ValueType,
			"path", identity,
			"source", tracker.tree.sourceLabel,
			"operation", tracker.options.Operation,
		)
	case "cycle":
		tracker.summary.Cycles++
		key := selection.occurrence.Item.ID + "\x00" + reference.OriginID
		diagnostic := tracker.cycles[key]
		if diagnostic == nil {
			diagnostic = &cycleDiagnostic{
				mirrorID:  selection.occurrence.Item.ID,
				originID:  reference.OriginID,
				firstPath: identity,
			}
			tracker.cycles[key] = diagnostic
		}
		diagnostic.occurrences++
	}
}

func (tracker *resolutionTracker) finish() {
	for _, diagnostic := range tracker.cycles {
		tracker.tree.logger.Warn(
			"Workflowy mirror cycle stopped",
			"mirror_id", diagnostic.mirrorID,
			"origin_id", diagnostic.originID,
			"path", diagnostic.firstPath,
			"occurrences", diagnostic.occurrences,
			"source", tracker.tree.sourceLabel,
			"operation", tracker.options.Operation,
		)
	}

	if warning := tracker.summary.ThresholdWarning(tracker.options.Operation, tracker.tree.sourceLabel); warning != "" {
		tracker.tree.logger.Warn(
			warning,
			"attempts", tracker.summary.Attempts(),
			"failures", tracker.summary.Failures(),
			"failure_ratio", float64(tracker.summary.Failures())/float64(tracker.summary.Attempts()),
			"source", tracker.tree.sourceLabel,
			"operation", tracker.options.Operation,
		)
	}

	tracker.logCompletion()
}

func (tracker *resolutionTracker) logCompletion() {
	attributes := tracker.commonAttributes()
	attributes = append(attributes,
		"resolved", tracker.summary.Resolved,
		"missing_origin", tracker.summary.MissingOrigin,
		"malformed_metadata", tracker.summary.MalformedMetadata,
		"cycles", tracker.summary.Cycles,
		"attempts", tracker.summary.Attempts(),
		"failures", tracker.summary.Failures(),
	)
	if tracker.debugEnabled {
		attributes = append(attributes, tracker.memoryAttributes()...)
	}
	tracker.tree.logger.Info("Workflowy mirror traversal completed", attributes...)
}

func (tracker *resolutionTracker) logWithMemory(level slog.Level, message string) {
	attributes := append(tracker.commonAttributes(), tracker.memoryAttributes()...)
	tracker.tree.logger.Log(context.Background(), level, message, attributes...)
}

func (tracker *resolutionTracker) commonAttributes() []interface{} {
	return []interface{}{
		"operation", tracker.options.Operation,
		"source", tracker.tree.sourceLabel,
		"scope", tracker.readScopeID,
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

func (tracker *resolutionTracker) memoryAttributes() []interface{} {
	stats := tracker.tree.memoryStatsReader()
	return []interface{}{
		"heap_bytes", stats.HeapBytes,
		"heap_objects", stats.HeapObjects,
		"total_allocated_bytes", stats.TotalAllocatedBytes,
		"gc_count", stats.GarbageCollections,
	}
}
