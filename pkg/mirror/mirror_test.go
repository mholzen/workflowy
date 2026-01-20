package mirror

import (
	"testing"

	"github.com/mholzen/workflowy/pkg/workflowy"
)

func TestCollectMirrorInfos_NilItems(t *testing.T) {
	infos := CollectMirrorInfos(nil)
	if len(infos) != 0 {
		t.Errorf("expected empty result for nil items, got %d", len(infos))
	}
}

func TestCollectMirrorInfos_NoMirrorData(t *testing.T) {
	items := []*workflowy.Item{
		{ID: "test-id", Name: "Test"},
	}
	infos := CollectMirrorInfos(items)
	if len(infos) != 0 {
		t.Errorf("expected empty result for items without mirror data, got %d", len(infos))
	}
}

func TestCollectMirrorInfos_MirrorCopyOnly(t *testing.T) {
	// Mirror copies (with originalId but no mirrorRootIds) should not be collected
	items := []*workflowy.Item{
		{
			ID:   "mirror-copy-id",
			Name: "Mirror Copy",
			Data: map[string]any{
				"mirror": map[string]any{
					"originalId":   "original-id",
					"isMirrorRoot": true,
				},
			},
		},
	}

	infos := CollectMirrorInfos(items)
	if len(infos) != 0 {
		t.Errorf("expected empty result for mirror copy only, got %d", len(infos))
	}
}

func TestCollectMirrorInfos_OriginalWithMirrors(t *testing.T) {
	items := []*workflowy.Item{
		{
			ID:   "original-id",
			Name: "Original Node",
			Data: map[string]any{
				"mirror": map[string]any{
					"mirrorRootIds": map[string]any{
						"mirror-loc-1": true,
						"mirror-loc-2": true,
						"mirror-loc-3": true,
					},
				},
			},
		},
	}

	infos := CollectMirrorInfos(items)
	if len(infos) != 1 {
		t.Fatalf("expected 1 info, got %d", len(infos))
	}

	info := infos[0]
	if info.NodeID != "original-id" {
		t.Errorf("expected NodeID 'original-id', got '%s'", info.NodeID)
	}
	if info.MirrorCount() != 3 {
		t.Errorf("expected MirrorCount 3, got %d", info.MirrorCount())
	}
}

func TestCollectMirrorInfos_WithParentInfo(t *testing.T) {
	// Create a tree where the mirror location has a known parent
	items := []*workflowy.Item{
		{
			ID:   "root",
			Name: "Root",
			Children: []*workflowy.Item{
				{
					ID:   "original-id",
					Name: "Original Node",
					Data: map[string]any{
						"mirror": map[string]any{
							"mirrorRootIds": map[string]any{
								"mirror-loc-1": true,
							},
						},
					},
				},
				{
					ID:   "parent-of-mirror",
					Name: "Parent Of Mirror",
					Children: []*workflowy.Item{
						{
							ID:   "mirror-loc-1",
							Name: "Mirror Location",
						},
					},
				},
			},
		},
	}

	infos := CollectMirrorInfos(items)
	if len(infos) != 1 {
		t.Fatalf("expected 1 info, got %d", len(infos))
	}

	info := infos[0]
	if len(info.MirrorLocations) != 1 {
		t.Fatalf("expected 1 mirror location, got %d", len(info.MirrorLocations))
	}

	loc := info.MirrorLocations[0]
	if loc.ID != "mirror-loc-1" {
		t.Errorf("expected mirror ID 'mirror-loc-1', got '%s'", loc.ID)
	}
	if loc.ParentID != "parent-of-mirror" {
		t.Errorf("expected parent ID 'parent-of-mirror', got '%s'", loc.ParentID)
	}
	if loc.ParentName != "Parent Of Mirror" {
		t.Errorf("expected parent name 'Parent Of Mirror', got '%s'", loc.ParentName)
	}
}

func TestCollectMirrorInfos_MultipleOriginals(t *testing.T) {
	items := []*workflowy.Item{
		{
			ID:   "root",
			Name: "Root",
			Children: []*workflowy.Item{
				{
					ID:   "original-1",
					Name: "Original 1",
					Data: map[string]any{
						"mirror": map[string]any{
							"mirrorRootIds": map[string]any{
								"loc-1": true,
								"loc-2": true,
							},
						},
					},
				},
				{
					ID:   "mirror-copy",
					Name: "Mirror Copy",
					Data: map[string]any{
						"mirror": map[string]any{
							"originalId": "original-1",
						},
					},
				},
				{
					ID:   "original-2",
					Name: "Original 2",
					Data: map[string]any{
						"mirror": map[string]any{
							"mirrorRootIds": map[string]any{
								"loc-3": true,
							},
						},
					},
					Children: []*workflowy.Item{
						{
							ID:   "nested-original",
							Name: "Nested Original",
							Data: map[string]any{
								"mirror": map[string]any{
									"mirrorRootIds": map[string]any{
										"loc-4": true,
										"loc-5": true,
										"loc-6": true,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	infos := CollectMirrorInfos(items)

	// Should only collect originals with mirrorRootIds, not mirror copies
	if len(infos) != 3 {
		t.Errorf("expected 3 originals with mirrors, got %d", len(infos))
	}
}

func TestRankByMirrorCount(t *testing.T) {
	infos := []*MirrorInfo{
		{NodeID: "a", MirrorLocations: []MirrorLocation{{ID: "1"}}},
		{NodeID: "b", MirrorLocations: []MirrorLocation{{ID: "1"}, {ID: "2"}, {ID: "3"}}},
		{NodeID: "c", MirrorLocations: []MirrorLocation{{ID: "1"}, {ID: "2"}}},
	}

	ranked := RankByMirrorCount(infos, 0)
	if len(ranked) != 3 {
		t.Fatalf("expected 3 results, got %d", len(ranked))
	}
	if ranked[0].NodeID != "b" {
		t.Errorf("expected first node to be 'b' (3 mirrors), got '%s'", ranked[0].NodeID)
	}
	if ranked[1].NodeID != "c" {
		t.Errorf("expected second node to be 'c' (2 mirrors), got '%s'", ranked[1].NodeID)
	}
	if ranked[2].NodeID != "a" {
		t.Errorf("expected third node to be 'a' (1 mirror), got '%s'", ranked[2].NodeID)
	}
}

func TestRankByMirrorCount_TopN(t *testing.T) {
	infos := []*MirrorInfo{
		{NodeID: "a", MirrorLocations: []MirrorLocation{{ID: "1"}}},
		{NodeID: "b", MirrorLocations: []MirrorLocation{{ID: "1"}, {ID: "2"}, {ID: "3"}}},
		{NodeID: "c", MirrorLocations: []MirrorLocation{{ID: "1"}, {ID: "2"}}},
	}

	ranked := RankByMirrorCount(infos, 2)
	if len(ranked) != 2 {
		t.Fatalf("expected 2 results with topN=2, got %d", len(ranked))
	}
	if ranked[0].NodeID != "b" {
		t.Errorf("expected first node to be 'b', got '%s'", ranked[0].NodeID)
	}
	if ranked[1].NodeID != "c" {
		t.Errorf("expected second node to be 'c', got '%s'", ranked[1].NodeID)
	}
}

func TestRankByMirrorCount_TopNLargerThanList(t *testing.T) {
	infos := []*MirrorInfo{
		{NodeID: "a", MirrorLocations: []MirrorLocation{{ID: "1"}}},
	}

	ranked := RankByMirrorCount(infos, 10)
	if len(ranked) != 1 {
		t.Fatalf("expected 1 result when topN > list size, got %d", len(ranked))
	}
}
