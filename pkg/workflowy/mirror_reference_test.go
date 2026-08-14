package workflowy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMirrorReferenceFromItem(t *testing.T) {
	tests := []struct {
		name     string
		data     map[string]interface{}
		expected MirrorReference
	}{
		{
			name:     "beta origin ID",
			data:     map[string]interface{}{"mirror": map[string]interface{}{"origin_id": "origin-beta"}},
			expected: MirrorReference{Kind: MirrorReferenceWithOrigin, OriginID: "origin-beta", Field: "origin_id"},
		},
		{
			name:     "backup original ID",
			data:     map[string]interface{}{"mirror": map[string]interface{}{"originalId": "origin-backup"}},
			expected: MirrorReference{Kind: MirrorReferenceWithOrigin, OriginID: "origin-backup", Field: "originalId"},
		},
		{
			name:     "null beta origin",
			data:     map[string]interface{}{"mirror": map[string]interface{}{"origin_id": nil}},
			expected: MirrorReference{Kind: MirrorReferenceNullOrigin, Field: "origin_id"},
		},
		{
			name:     "null backup origin",
			data:     map[string]interface{}{"mirror": map[string]interface{}{"originalId": nil}},
			expected: MirrorReference{Kind: MirrorReferenceNullOrigin, Field: "originalId"},
		},
		{
			name:     "malformed beta origin",
			data:     map[string]interface{}{"mirror": map[string]interface{}{"origin_id": 42}},
			expected: MirrorReference{Kind: MirrorReferenceMalformed, Field: "origin_id", ValueType: "int"},
		},
		{
			name:     "malformed backup origin",
			data:     map[string]interface{}{"mirror": map[string]interface{}{"originalId": true}},
			expected: MirrorReference{Kind: MirrorReferenceMalformed, Field: "originalId", ValueType: "bool"},
		},
		{
			name:     "origin mirror IDs only",
			data:     map[string]interface{}{"mirror": map[string]interface{}{"mirror_ids": []interface{}{"mirror-a"}}},
			expected: MirrorReference{Kind: MirrorReferenceOrdinary},
		},
		{
			name:     "backup mirror root IDs only",
			data:     map[string]interface{}{"mirror": map[string]interface{}{"mirrorRootIds": map[string]interface{}{"mirror-a": true}}},
			expected: MirrorReference{Kind: MirrorReferenceOrdinary},
		},
		{
			name:     "non-object mirror",
			data:     map[string]interface{}{"mirror": "mirror-a"},
			expected: MirrorReference{Kind: MirrorReferenceOrdinary},
		},
		{
			name:     "missing data",
			data:     nil,
			expected: MirrorReference{Kind: MirrorReferenceOrdinary},
		},
		{
			name:     "ordinary data",
			data:     map[string]interface{}{"layoutMode": "bullets"},
			expected: MirrorReference{Kind: MirrorReferenceOrdinary},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := MirrorReferenceFromItem(&Item{Data: test.data})

			assert.Equal(t, test.expected, actual)
			assert.Equal(t, test.expected.Kind != MirrorReferenceOrdinary, actual.IsMirror())
		})
	}
}
