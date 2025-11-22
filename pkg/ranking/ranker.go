package ranking

import (
	"fmt"
	"sort"
)

// Rankable represents anything that can be ranked
type Rankable[T any] interface {
	GetValue() T
	GetRankingValue() int
}

// RankItem wraps a rankable item for display
type RankItem[T any] struct {
	Item Rankable[T]
}

func (r RankItem[T]) String() string {
	return fmt.Sprintf("%d: %v", r.Item.GetRankingValue(), r.Item.GetValue())
}

// RankByValue ranks items by their ranking value in descending order
func RankByValue[T any](items []Rankable[T], topN int) []RankItem[T] {
	sort.Slice(items, func(i, j int) bool {
		return items[i].GetRankingValue() > items[j].GetRankingValue()
	})

	limit := len(items)
	if topN > 0 && topN < limit {
		limit = topN
	}

	result := make([]RankItem[T], limit)
	for i := 0; i < limit; i++ {
		result[i] = RankItem[T]{Item: items[i]}
	}

	return result
}
