package query

import (
	"context"
	"fmt"
	"sort"

	"github.com/glancedb/glancedb/distance"
	"github.com/glancedb/glancedb/table"
)

// SearchStrategy selects how vector search and scalar filtering are combined.
type SearchStrategy int

const (
	StrategyFilterFirst SearchStrategy = 1
	StrategySearchFirst SearchStrategy = 2
)

// HybridSearch combines vector similarity search with scalar filtering.
type HybridSearch struct {
	bruteForce *BruteForceSearch
	scanFilter *ScanFilter
}

// NewHybridSearch creates a HybridSearch.
func NewHybridSearch(bruteForce *BruteForceSearch, scanFilter *ScanFilter) *HybridSearch {
	return &HybridSearch{bruteForce: bruteForce, scanFilter: scanFilter}
}

// Search executes a hybrid query using the default FilterFirst strategy.
func (h *HybridSearch) Search(ctx context.Context, manifest *table.Manifest, q *Query) ([]SearchResult, error) {
	return h.SearchWithStrategy(ctx, manifest, q, StrategyFilterFirst)
}

// SearchWithStrategy executes a hybrid query with an explicit strategy.
func (h *HybridSearch) SearchWithStrategy(ctx context.Context, manifest *table.Manifest, q *Query, strategy SearchStrategy) ([]SearchResult, error) {
	if q.Vector != nil && q.Filter == nil {
		results, err := h.bruteForce.Search(ctx, manifest, q.Vector)
		if err != nil {
			return nil, fmt.Errorf("query: %w", err)
		}
		return applyLimit(results, q.Limit), nil
	}

	if q.Vector == nil && q.Filter != nil {
		rowIDs, err := h.scanFilter.Filter(ctx, manifest, q.Filter)
		if err != nil {
			return nil, fmt.Errorf("query: %w", err)
		}
		results := make([]SearchResult, len(rowIDs))
		for i, id := range rowIDs {
			results[i] = SearchResult{RowID: id, Score: 0}
		}
		return applyLimit(results, q.Limit), nil
	}

	if q.Vector == nil && q.Filter == nil {
		return nil, nil
	}

	if strategy == StrategySearchFirst {
		return h.searchFirst(ctx, manifest, q)
	}
	return h.filterFirst(ctx, manifest, q)
}

// filterFirst applies the scalar filter, then computes vector distances only on matching rows.
func (h *HybridSearch) filterFirst(ctx context.Context, manifest *table.Manifest, q *Query) ([]SearchResult, error) {
	rowIDs, err := h.scanFilter.Filter(ctx, manifest, q.Filter)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	if len(rowIDs) == 0 {
		return nil, nil
	}

	matching := make(map[int64]bool, len(rowIDs))
	for _, id := range rowIDs {
		matching[id] = true
	}

	field, err := h.bruteForce.vectorField(q.Vector.Column)
	if err != nil {
		return nil, err
	}
	dim := int(field.Dimension)
	if len(q.Vector.Vector) != dim {
		return nil, fmt.Errorf("query: vector length %d does not match column dimension %d", len(q.Vector.Vector), dim)
	}

	var candidates []SearchResult
	var rowOffset int64

	for _, frag := range manifest.Fragments {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("query: %w", err)
		}
		numRows := frag.NumRows
		if frag.PhysicalRows > 0 {
			numRows = frag.PhysicalRows
		}

		data, err := h.bruteForce.reader.ReadColumn(ctx, frag, field.ID)
		if err != nil {
			return nil, fmt.Errorf("query: %w", err)
		}
		vecData, ok := data.([]float32)
		if !ok {
			return nil, fmt.Errorf("query: expected []float32 for vector column, got %T", data)
		}

		for i := int64(0); i < numRows; i++ {
			rowID := rowOffset + i
			if !matching[rowID] {
				continue
			}
			start := i * int64(dim)
			end := start + int64(dim)
			if end > int64(len(vecData)) {
				break
			}
			vec := vecData[start:end]
			dist, err := distance.Distance(q.Vector.Vector, vec, q.Vector.Metric)
			if err != nil {
				return nil, fmt.Errorf("query: %w", err)
			}
			candidates = append(candidates, SearchResult{RowID: rowID, Score: dist})
		}
		rowOffset += numRows
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score < candidates[j].Score
	})
	return applyLimit(candidates, q.Limit), nil
}

// searchFirst performs vector search, then filters results by scalar conditions.
func (h *HybridSearch) searchFirst(ctx context.Context, manifest *table.Manifest, q *Query) ([]SearchResult, error) {
	results, err := h.bruteForce.Search(ctx, manifest, q.Vector)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	rowIDs, err := h.scanFilter.Filter(ctx, manifest, q.Filter)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	matching := make(map[int64]bool, len(rowIDs))
	for _, id := range rowIDs {
		matching[id] = true
	}

	filtered := results[:0]
	for _, r := range results {
		if matching[r.RowID] {
			filtered = append(filtered, r)
		}
	}
	return applyLimit(filtered, q.Limit), nil
}

// applyLimit truncates results to at most limit entries. A non-positive limit returns all.
func applyLimit(results []SearchResult, limit int) []SearchResult {
	if limit > 0 && limit < len(results) {
		return results[:limit]
	}
	return results
}
