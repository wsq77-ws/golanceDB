package query

import "sort"

// Reranker merges and re-sorts results from multiple sources.
type Reranker struct{}

// NewReranker creates a Reranker.
func NewReranker() *Reranker {
	return &Reranker{}
}

// MergeTopK merges multiple sorted result lists and returns the global top-k.
func (r *Reranker) MergeTopK(sources [][]SearchResult, k int) []SearchResult {
	var all []SearchResult
	for _, s := range sources {
		all = append(all, s...)
	}
	sort.Slice(all, func(i, j int) bool {
		return all[i].Score < all[j].Score
	})
	if k > 0 && k < len(all) {
		return all[:k]
	}
	return all
}
