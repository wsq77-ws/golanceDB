package query

import (
	"time"

	"github.com/glancedb/glancedb/distance"
)

// DistanceMetric is re-exported from the distance package for convenience.
type DistanceMetric = distance.DistanceMetric

const (
	DistanceCosine     = distance.DistanceCosine
	DistanceEuclidean  = distance.DistanceEuclidean
	DistanceDotProduct = distance.DistanceDotProduct
)

// OpType represents scalar filter operators.
type OpType int32

const (
	OpEQ      OpType = 1
	OpNE      OpType = 2
	OpLT      OpType = 3
	OpGT      OpType = 4
	OpLE      OpType = 5
	OpGE      OpType = 6
	OpIn      OpType = 7
	OpBetween OpType = 8
)

// VectorQuery specifies the vector search parameters.
type VectorQuery struct {
	Vector        []float32
	Column        string
	Metric        DistanceMetric
	NumPartitions int
	NProbes       int
}

// ScalarCondition is a single filter condition.
type ScalarCondition struct {
	Column   string
	Operator OpType
	Value    interface{}
}

// ScalarFilter is a collection of AND conditions.
type ScalarFilter struct {
	Conditions []*ScalarCondition
}

// Query describes a search request.
type Query struct {
	Vector  *VectorQuery
	Filter  *ScalarFilter
	Limit   int
	Columns []string
	Timeout time.Duration
}

// SearchResult is a single search result.
type SearchResult = distance.SearchResult
