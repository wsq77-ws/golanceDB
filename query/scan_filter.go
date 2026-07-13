package query

import (
	"context"
	"fmt"
	"reflect"

	"github.com/glancedb/glancedb/table"
)

// ScanFilter applies scalar predicates to filter rows.
type ScanFilter struct {
	reader *table.FragmentReader
	schema *table.Schema
}

// NewScanFilter creates a ScanFilter.
func NewScanFilter(reader *table.FragmentReader, schema *table.Schema) *ScanFilter {
	return &ScanFilter{reader: reader, schema: schema}
}

// Filter returns the row IDs that match all conditions (AND semantics).
// Row IDs are assigned sequentially across fragments starting from 0.
func (sf *ScanFilter) Filter(ctx context.Context, manifest *table.Manifest, filter *ScalarFilter) ([]int64, error) {
	if filter == nil || len(filter.Conditions) == 0 {
		return nil, nil
	}

	var matchingRowIDs []int64
	var rowOffset int64

	for _, frag := range manifest.Fragments {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("query: %w", err)
		}
		numRows := frag.NumRows
		if frag.PhysicalRows > 0 {
			numRows = frag.PhysicalRows
		}
		matched := make([]bool, numRows)
		for i := range matched {
			matched[i] = true
		}

		for _, cond := range filter.Conditions {
			field := sf.schema.FieldByName(cond.Column)
			if field == nil {
				return nil, fmt.Errorf("query: column %s not found in schema", cond.Column)
			}
			data, err := sf.reader.ReadColumn(ctx, frag, field.ID)
			if err != nil {
				return nil, fmt.Errorf("query: %w", err)
			}
			rowMatched, err := evaluateColumn(data, cond, int(numRows))
			if err != nil {
				return nil, fmt.Errorf("query: %w", err)
			}
			for i := range matched {
				matched[i] = matched[i] && rowMatched[i]
			}
		}

		for i, m := range matched {
			if m {
				matchingRowIDs = append(matchingRowIDs, rowOffset+int64(i))
			}
		}
		rowOffset += numRows
	}

	return matchingRowIDs, nil
}

// evaluateColumn evaluates a condition against each row in the column data.
func evaluateColumn(data interface{}, cond *ScalarCondition, numRows int) ([]bool, error) {
	result := make([]bool, numRows)
	switch d := data.(type) {
	case []int64:
		n := len(d)
		if n > numRows {
			n = numRows
		}
		for i := 0; i < n; i++ {
			result[i] = matchesCondition(d[i], cond)
		}
	case []int32:
		n := len(d)
		if n > numRows {
			n = numRows
		}
		for i := 0; i < n; i++ {
			result[i] = matchesCondition(d[i], cond)
		}
	case []float32:
		n := len(d)
		if n > numRows {
			n = numRows
		}
		for i := 0; i < n; i++ {
			result[i] = matchesCondition(d[i], cond)
		}
	case []float64:
		n := len(d)
		if n > numRows {
			n = numRows
		}
		for i := 0; i < n; i++ {
			result[i] = matchesCondition(d[i], cond)
		}
	case []string:
		n := len(d)
		if n > numRows {
			n = numRows
		}
		for i := 0; i < n; i++ {
			result[i] = matchesCondition(d[i], cond)
		}
	default:
		return nil, fmt.Errorf("query: unsupported column type %T for filter", data)
	}
	return result, nil
}

// matchesCondition checks if a single value matches the condition.
func matchesCondition(value interface{}, cond *ScalarCondition) bool {
	switch cond.Operator {
	case OpEQ:
		return valuesEqual(value, cond.Value)
	case OpNE:
		return !valuesEqual(value, cond.Value)
	case OpLT:
		c, ok := compareNumeric(value, cond.Value)
		return ok && c < 0
	case OpGT:
		c, ok := compareNumeric(value, cond.Value)
		return ok && c > 0
	case OpLE:
		c, ok := compareNumeric(value, cond.Value)
		return ok && c <= 0
	case OpGE:
		c, ok := compareNumeric(value, cond.Value)
		return ok && c >= 0
	case OpIn:
		return inSet(value, cond.Value)
	case OpBetween:
		return betweenRange(value, cond.Value)
	default:
		return false
	}
}

func toFloat64(v interface{}) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}

func valuesEqual(a, b interface{}) bool {
	if af, ok := toFloat64(a); ok {
		if bf, ok := toFloat64(b); ok {
			return af == bf
		}
		return false
	}
	as, aok := a.(string)
	bs, bok := b.(string)
	if aok && bok {
		return as == bs
	}
	return false
}

func compareNumeric(a, b interface{}) (int, bool) {
	af, ok := toFloat64(a)
	if !ok {
		return 0, false
	}
	bf, ok := toFloat64(b)
	if !ok {
		return 0, false
	}
	if af < bf {
		return -1, true
	}
	if af > bf {
		return 1, true
	}
	return 0, true
}

func inSet(value, set interface{}) bool {
	rv := reflect.ValueOf(set)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return false
	}
	for i := 0; i < rv.Len(); i++ {
		if valuesEqual(value, rv.Index(i).Interface()) {
			return true
		}
	}
	return false
}

func betweenRange(value, rangeVal interface{}) bool {
	rv := reflect.ValueOf(rangeVal)
	if rv.Kind() != reflect.Slice && rv.Kind() != reflect.Array {
		return false
	}
	if rv.Len() != 2 {
		return false
	}
	lo := rv.Index(0).Interface()
	hi := rv.Index(1).Interface()
	cLo, okLo := compareNumeric(value, lo)
	cHi, okHi := compareNumeric(value, hi)
	if !okLo || !okHi {
		return false
	}
	return cLo >= 0 && cHi <= 0
}
