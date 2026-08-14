package workflowy

import (
	"context"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMirrorResolutionSummaryMetricsAndThresholdWarning(t *testing.T) {
	summary := MirrorResolutionSummary{Resolved: 2, MissingOrigin: 1, MalformedMetadata: 1, Cycles: 2}

	assert.Equal(t, 6, summary.Attempts())
	assert.Equal(t, 4, summary.Failures())
	assert.Equal(t,
		"Mirror resolution failures reached 66.7% during search from test export (attempts=6 failures=4 resolved=2 missing_origin=1 malformed_metadata=1 cycles=2)",
		summary.ThresholdWarning("search", "test export"),
	)
	assert.Empty(t, (MirrorResolutionSummary{Resolved: 2, MissingOrigin: 1}).ThresholdWarning("get", "test export"))
	assert.NotEmpty(t, (MirrorResolutionSummary{Resolved: 1, MissingOrigin: 1}).ThresholdWarning("get", "test export"))
	assert.Empty(t, (MirrorResolutionSummary{}).ThresholdWarning("get", "test export"))
}

func TestResolvedTreeLogsContextualFallbacksAndOneSummary(t *testing.T) {
	logs := captureResolvedTreeLogs(t, slog.LevelDebug)
	malformed := &Item{ID: "malformed", Data: map[string]interface{}{
		"mirror": map[string]interface{}{"origin_id": 17},
	}}
	missing := testMirror("missing", "absent")
	tree := NewResolvedTree([]*Item{malformed, missing}, "test export")

	_, err := tree.Fetch("None", "None", resolvedFetchOptions(1))
	require.NoError(t, err)

	malformedRecord := logs.requireOne(t, "Workflowy mirror metadata is malformed; using source children")
	assert.Equal(t, "malformed", malformedRecord.attrs["mirror_id"])
	assert.Equal(t, "origin_id", malformedRecord.attrs["field"])
	assert.Equal(t, "int", malformedRecord.attrs["value_type"])
	assert.Equal(t, "test export", malformedRecord.attrs["source"])
	assert.Equal(t, "get", malformedRecord.attrs["operation"])

	missingRecord := logs.requireOne(t, "Workflowy mirror origin is missing; using source children")
	assert.Equal(t, "missing", missingRecord.attrs["mirror_id"])
	assert.Equal(t, "absent", missingRecord.attrs["origin_id"])
	assert.Equal(t, "missing", missingRecord.attrs["path"])
	assert.Len(t, logs.withMessage("Workflowy mirror traversal completed"), 1)
}

func TestResolvedTreeLogsConsolidatedCycles(t *testing.T) {
	logs := captureResolvedTreeLogs(t, slog.LevelDebug)
	selfMirror := testMirror("self-mirror", "origin")
	origin := testItem("origin", selfMirror)
	tree := NewResolvedTree([]*Item{origin, testMirror("entry-one", origin.ID), testMirror("entry-two", origin.ID)}, "test backup")

	result, err := tree.Fetch("None", "None", resolvedFetchOptions(-1))
	require.NoError(t, err)
	assert.Equal(t, 3, result.Summary.Cycles)

	cycleRecord := logs.requireOne(t, "Workflowy mirror cycle stopped")
	assert.Equal(t, "self-mirror", cycleRecord.attrs["mirror_id"])
	assert.Equal(t, "origin", cycleRecord.attrs["origin_id"])
	assert.Equal(t, int64(3), cycleRecord.attrs["occurrences"])
	assert.NotEmpty(t, cycleRecord.attrs["path"])
}

func TestResolvedTreeLogsIndexTraversalProgressAndCompletion(t *testing.T) {
	logs := captureResolvedTreeLogs(t, slog.LevelDebug)
	roots := make([]*Item, 4096)
	for index := range roots {
		roots[index] = testItem(stringID(index))
	}
	tree := NewResolvedTree(roots, "large export")
	tree.memoryStatsReader = func() traversalMemoryStats {
		return traversalMemoryStats{HeapBytes: 10, HeapObjects: 20, TotalAllocatedBytes: 30, GarbageCollections: 40}
	}

	_, err := tree.Visit("None", "None", resolvedFetchOptions(0), func(NodeOccurrence) (OccurrenceVisitResult, error) {
		return OccurrenceVisitResult{RetainedOccurrences: 7}, nil
	})
	require.NoError(t, err)

	indexRecord := logs.requireOne(t, "indexed Workflowy resolved tree source")
	assert.Equal(t, int64(4096), indexRecord.attrs["source_roots"])
	assert.Equal(t, int64(4096), indexRecord.attrs["source_nodes"])
	assert.Equal(t, int64(0), indexRecord.attrs["mirrors"])

	start := logs.requireOne(t, "Workflowy mirror traversal started")
	assertTraversalContext(t, start, "get", "large export", "None", "None", int64(0))

	progress := logs.withMessage("Workflowy mirror traversal progress")
	require.Len(t, progress, 3)
	assert.Equal(t, int64(1000), progress[0].attrs["visited_occurrences"])
	assert.Equal(t, int64(2000), progress[1].attrs["visited_occurrences"])
	assert.Equal(t, int64(4000), progress[2].attrs["visited_occurrences"])

	completion := logs.requireOne(t, "Workflowy mirror traversal completed")
	assertTraversalContext(t, completion, "get", "large export", "None", "None", int64(0))
	assert.Equal(t, int64(4096), completion.attrs["visited_occurrences"])
	assert.Equal(t, int64(7), completion.attrs["retained_occurrences"])
	assert.Equal(t, int64(1), completion.attrs["maximum_path_depth"])
	assertTraversalMemory(t, completion)
}

func TestResolvedTreeDoesNotReadMemoryStatsWhenDebugLoggingIsDisabled(t *testing.T) {
	captureResolvedTreeLogs(t, slog.LevelInfo)
	tree := NewResolvedTree([]*Item{testItem("root")}, "test export")
	reads := 0
	tree.memoryStatsReader = func() traversalMemoryStats {
		reads++
		return traversalMemoryStats{}
	}

	_, err := tree.Fetch("None", "None", resolvedFetchOptions(0))
	require.NoError(t, err)
	assert.Zero(t, reads)
}

func TestResolvedTreeLogsDuplicateSourceIDs(t *testing.T) {
	logs := captureResolvedTreeLogs(t, slog.LevelDebug)
	NewResolvedTree([]*Item{
		testItem("first-parent", testItem("duplicate")),
		testItem("second-parent", testItem("duplicate")),
	}, "test export")

	record := logs.requireOne(t, "Cannot index duplicate Workflowy node ID")
	assert.Equal(t, "duplicate", record.attrs["node_id"])
	assert.Equal(t, "test export", record.attrs["source"])
	assert.Equal(t, "first-parent", record.attrs["first_parent_id"])
	assert.Equal(t, "second-parent", record.attrs["duplicate_parent_id"])
}

func TestResolvedTreeLogsMirrorTargetWithoutDoubleCounting(t *testing.T) {
	logs := captureResolvedTreeLogs(t, slog.LevelDebug)
	child := testItem("child")
	origin := testItem("origin", child)
	mirror := testMirror("mirror", origin.ID)
	tree := NewResolvedTree([]*Item{mirror, origin}, "test export")

	result, err := tree.Fetch("None", mirror.ID, resolvedFetchOptions(-1))
	require.NoError(t, err)
	assert.Equal(t, 1, result.Summary.Resolved)

	completion := logs.requireOne(t, "Workflowy mirror traversal completed")
	assert.Equal(t, int64(2), completion.attrs["visited_occurrences"])
	assert.Equal(t, int64(2), completion.attrs["retained_occurrences"])
	assert.Equal(t, int64(1), completion.attrs["resolved"])
}

func TestTraversalTrackersRetainOnlyConsolidatedCycleState(t *testing.T) {
	assert.Equal(t, []string{"cycles"}, mapFieldNames(reflect.TypeOf(resolutionTracker{})))
	assert.Equal(t, []string{"cycles"}, mapFieldNames(reflect.TypeOf(recursiveFetchTracker{})))
}

func mapFieldNames(structType reflect.Type) []string {
	fields := make([]string, 0)
	for index := 0; index < structType.NumField(); index++ {
		field := structType.Field(index)
		if field.Type.Kind() == reflect.Map {
			fields = append(fields, field.Name)
		}
	}
	return fields
}

func assertTraversalContext(t *testing.T, record capturedLogRecord, operation, source, scope, target string, depth int64) {
	t.Helper()
	assert.Equal(t, operation, record.attrs["operation"])
	assert.Equal(t, source, record.attrs["source"])
	assert.Equal(t, scope, record.attrs["scope"])
	assert.Equal(t, target, record.attrs["target"])
	assert.Equal(t, depth, record.attrs["requested_depth"])
	assert.Equal(t, depth, record.attrs["effective_depth"])
}

func assertTraversalMemory(t *testing.T, record capturedLogRecord) {
	t.Helper()
	assert.Equal(t, uint64(10), record.attrs["heap_bytes"])
	assert.Equal(t, uint64(20), record.attrs["heap_objects"])
	assert.Equal(t, uint64(30), record.attrs["total_allocated_bytes"])
	assert.Equal(t, uint64(40), record.attrs["gc_count"])
	assert.Contains(t, record.attrs, "elapsed")
	assert.Contains(t, record.attrs, "current_path_depth")
}

func stringID(index int) string {
	return "node-" + strconv.Itoa(index)
}

type capturedLogRecord struct {
	message string
	attrs   map[string]interface{}
}

type capturedLogs struct {
	mu      sync.Mutex
	level   slog.Level
	records []capturedLogRecord
}

func captureResolvedTreeLogs(t *testing.T, level slog.Level) *capturedLogs {
	t.Helper()
	logs := &capturedLogs{level: level}
	previous := slog.Default()
	slog.SetDefault(slog.New(logs))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return logs
}

func (logs *capturedLogs) Enabled(_ context.Context, level slog.Level) bool {
	return level >= logs.level
}

func (logs *capturedLogs) Handle(_ context.Context, record slog.Record) error {
	attrs := make(map[string]interface{})
	record.Attrs(func(attribute slog.Attr) bool {
		attrs[attribute.Key] = attribute.Value.Any()
		return true
	})
	logs.mu.Lock()
	defer logs.mu.Unlock()
	logs.records = append(logs.records, capturedLogRecord{message: record.Message, attrs: attrs})
	return nil
}

func (logs *capturedLogs) WithAttrs(_ []slog.Attr) slog.Handler { return logs }
func (logs *capturedLogs) WithGroup(_ string) slog.Handler      { return logs }

func (logs *capturedLogs) withMessage(message string) []capturedLogRecord {
	logs.mu.Lock()
	defer logs.mu.Unlock()
	matching := make([]capturedLogRecord, 0)
	for _, record := range logs.records {
		if record.message == message {
			matching = append(matching, record)
		}
	}
	return matching
}

func (logs *capturedLogs) withMessagePrefix(prefix string) []capturedLogRecord {
	logs.mu.Lock()
	defer logs.mu.Unlock()
	matching := make([]capturedLogRecord, 0)
	for _, record := range logs.records {
		if strings.HasPrefix(record.message, prefix) {
			matching = append(matching, record)
		}
	}
	return matching
}

func (logs *capturedLogs) requireOne(t *testing.T, message string) capturedLogRecord {
	t.Helper()
	records := logs.withMessage(message)
	require.Len(t, records, 1, "records with message %q", message)
	return records[0]
}
