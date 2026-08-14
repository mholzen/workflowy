package workflowy

import "fmt"

type MirrorReferenceKind uint8

const (
	MirrorReferenceOrdinary MirrorReferenceKind = iota
	MirrorReferenceWithOrigin
	MirrorReferenceNullOrigin
	MirrorReferenceMalformed
)

type MirrorReference struct {
	Kind      MirrorReferenceKind
	OriginID  string
	Field     string
	ValueType string
}

func MirrorReferenceFromItem(item *Item) MirrorReference {
	if item == nil {
		return MirrorReference{Kind: MirrorReferenceOrdinary}
	}

	mirror, ok := item.Data["mirror"].(map[string]interface{})
	if !ok {
		return MirrorReference{Kind: MirrorReferenceOrdinary}
	}

	if value, exists := mirror["origin_id"]; exists {
		return mirrorReferenceFromValue("origin_id", value)
	}
	if value, exists := mirror["originalId"]; exists {
		return mirrorReferenceFromValue("originalId", value)
	}

	return MirrorReference{Kind: MirrorReferenceOrdinary}
}

func mirrorReferenceFromValue(field string, value interface{}) MirrorReference {
	if value == nil {
		return MirrorReference{Kind: MirrorReferenceNullOrigin, Field: field}
	}

	originID, ok := value.(string)
	if !ok {
		return MirrorReference{
			Kind:      MirrorReferenceMalformed,
			Field:     field,
			ValueType: fmt.Sprintf("%T", value),
		}
	}

	return MirrorReference{Kind: MirrorReferenceWithOrigin, OriginID: originID, Field: field}
}

func (reference MirrorReference) IsMirror() bool {
	return reference.Kind != MirrorReferenceOrdinary
}
