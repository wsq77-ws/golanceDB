package api

import (
	"github.com/glancedb/glancedb/distance"
	"github.com/glancedb/glancedb/query"
)

// QueryBuilder provides a convenient way to construct queries.
// Usage:
//
//	results, err := tbl.Search(ctx, api.NewQuery(api.Vector(vec)).
//	    TopK(10).
//	    Filter(api.EQ("category", "science")).
//	    Build())
type QueryBuilder struct {
	q query.Query
}

// NewQuery creates a new QueryBuilder with an optional vector query.
func NewQuery(vq *VectorQueryBuilder) *QueryBuilder {
	qb := &QueryBuilder{}
	if vq != nil {
		qb.q.Vector = vq.vq
	}
	return qb
}

// TopK sets the maximum number of results to return.
// If n <= 0, all matching results are returned.
func (qb *QueryBuilder) TopK(n int) *QueryBuilder {
	qb.q.Limit = n
	return qb
}

// Filter adds a scalar filter to the query.
func (qb *QueryBuilder) Filter(f *FilterBuilder) *QueryBuilder {
	qb.q.Filter = f.Build()
	return qb
}

// Select limits the columns returned in results.
func (qb *QueryBuilder) Select(columns ...string) *QueryBuilder {
	qb.q.Columns = append(qb.q.Columns, columns...)
	return qb
}

// Build returns the constructed query.
func (qb *QueryBuilder) Build() *query.Query {
	return &qb.q
}

// VectorQueryBuilder builds a vector search query.
type VectorQueryBuilder struct {
	vq *query.VectorQuery
}

// Vector specifies a vector search query against the given column
// using the given distance metric.
func Vector(vec []float32) *VectorQueryBuilder {
	return &VectorQueryBuilder{
		vq: &query.VectorQuery{
			Vector: vec,
			Metric: distance.DistanceCosine,
		},
	}
}

// Column sets the vector column name (default: "embedding").
func (vqb *VectorQueryBuilder) Column(name string) *VectorQueryBuilder {
	vqb.vq.Column = name
	return vqb
}

// Metric sets the distance metric for the search.
func (vqb *VectorQueryBuilder) Metric(m distance.DistanceMetric) *VectorQueryBuilder {
	vqb.vq.Metric = m
	return vqb
}

// NProbes sets the number of partitions to probe (for IVF index).
func (vqb *VectorQueryBuilder) NProbes(n int) *VectorQueryBuilder {
	vqb.vq.NProbes = n
	return vqb
}

// Build returns the underlying VectorQuery.
func (vqb *VectorQueryBuilder) Build() *query.VectorQuery {
	return vqb.vq
}

// FilterBuilder builds a scalar filter.
type FilterBuilder struct {
	conditions []*query.ScalarCondition
}

// EQ adds an equals condition.
func EQ(column string, value interface{}) *FilterBuilder {
	return new(FilterBuilder).EQ(column, value)
}

// NE adds a not-equals condition.
func NE(column string, value interface{}) *FilterBuilder {
	return new(FilterBuilder).NE(column, value)
}

// LT adds a less-than condition.
func LT(column string, value interface{}) *FilterBuilder {
	return new(FilterBuilder).LT(column, value)
}

// GT adds a greater-than condition.
func GT(column string, value interface{}) *FilterBuilder {
	return new(FilterBuilder).GT(column, value)
}

// LE adds a less-than-or-equal condition.
func LE(column string, value interface{}) *FilterBuilder {
	return new(FilterBuilder).LE(column, value)
}

// GE adds a greater-than-or-equal condition.
func GE(column string, value interface{}) *FilterBuilder {
	return new(FilterBuilder).GE(column, value)
}

// In adds an "in" condition.
func In(column string, values interface{}) *FilterBuilder {
	return new(FilterBuilder).In(column, values)
}

func (fb *FilterBuilder) add(op query.OpType, column string, value interface{}) *FilterBuilder {
	fb.conditions = append(fb.conditions, &query.ScalarCondition{
		Column: column, Operator: op, Value: value,
	})
	return fb
}

// EQ adds an equals condition (AND).
func (fb *FilterBuilder) EQ(column string, value interface{}) *FilterBuilder {
	return fb.add(query.OpEQ, column, value)
}

// NE adds a not-equals condition (AND).
func (fb *FilterBuilder) NE(column string, value interface{}) *FilterBuilder {
	return fb.add(query.OpNE, column, value)
}

// LT adds a less-than condition (AND).
func (fb *FilterBuilder) LT(column string, value interface{}) *FilterBuilder {
	return fb.add(query.OpLT, column, value)
}

// GT adds a greater-than condition (AND).
func (fb *FilterBuilder) GT(column string, value interface{}) *FilterBuilder {
	return fb.add(query.OpGT, column, value)
}

// LE adds a less-than-or-equal condition (AND).
func (fb *FilterBuilder) LE(column string, value interface{}) *FilterBuilder {
	return fb.add(query.OpLE, column, value)
}

// GE adds a greater-than-or-equal condition (AND).
func (fb *FilterBuilder) GE(column string, value interface{}) *FilterBuilder {
	return fb.add(query.OpGE, column, value)
}

// In adds an "in" condition (AND).
func (fb *FilterBuilder) In(column string, values interface{}) *FilterBuilder {
	return fb.add(query.OpIn, column, values)
}

// Build returns the underlying ScalarFilter.
func (fb *FilterBuilder) Build() *query.ScalarFilter {
	return &query.ScalarFilter{Conditions: fb.conditions}
}
