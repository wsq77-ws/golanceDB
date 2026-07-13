package query

import (
	"context"
	"fmt"
	"sort"

	"github.com/glancedb/glancedb/distance"
	"github.com/glancedb/glancedb/encode"
	"github.com/glancedb/glancedb/table"
)

// BruteForceSearch performs a full-scan vector similarity search across all fragments.
type BruteForceSearch struct {
	reader *table.FragmentReader
	schema *table.Schema
}

// NewBruteForceSearch creates a BruteForceSearch.
func NewBruteForceSearch(reader *table.FragmentReader, schema *table.Schema) *BruteForceSearch {
	return &BruteForceSearch{reader: reader, schema: schema}
}

// vectorField finds and validates the vector column field.
func (b *BruteForceSearch) vectorField(column string) (*table.Field, error) {
	field := b.schema.FieldByName(column)
	if field == nil {
		return nil, fmt.Errorf("query: vector column %s not found in schema", column)
	}
	if field.Type != encode.TypeFixedSizeList {
		return nil, fmt.Errorf("query: column %s is not a fixed-size list", column)
	}
	if field.Dimension <= 0 {
		return nil, fmt.Errorf("query: column %s has invalid dimension %d", column, field.Dimension)
	}
	return field, nil
}

// Search scans all fragments, computes distances, and returns sorted results.
// Row IDs are assigned sequentially across fragments starting from 0.
func (b *BruteForceSearch) Search(ctx context.Context, manifest *table.Manifest, query *VectorQuery) ([]SearchResult, error) {
	field, err := b.vectorField(query.Column)
	if err != nil {
		return nil, err
	}
	dim := int(field.Dimension)
	if len(query.Vector) != dim {
		return nil, fmt.Errorf("query: vector length %d does not match column dimension %d", len(query.Vector), dim)
	}

	var candidates []SearchResult
	var rowOffset int64

	for _, frag := range manifest.Fragments {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("query: %w", err)
		}
		data, err := b.reader.ReadColumn(ctx, frag, field.ID)
		if err != nil {
			return nil, fmt.Errorf("query: %w", err)
		}
		vecData, ok := data.([]float32)
		if !ok {
			return nil, fmt.Errorf("query: expected []float32 for vector column, got %T", data)
		}

		numRows := frag.NumRows
		if frag.PhysicalRows > 0 {
			numRows = frag.PhysicalRows
		}
		for i := int64(0); i < numRows; i++ {
			start := i * int64(dim)
			end := start + int64(dim)
			if end > int64(len(vecData)) {
				break
			}
			vec := vecData[start:end]
			dist, err := distance.Distance(query.Vector, vec, query.Metric)
			if err != nil {
				return nil, fmt.Errorf("query: %w", err)
			}
			candidates = append(candidates, SearchResult{
				RowID: rowOffset + i,
				Score: dist,
			})
		}
		rowOffset += numRows
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score < candidates[j].Score
	})
	return candidates, nil
}
