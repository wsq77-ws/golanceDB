package encode

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// ConstantEncoder encodes columns where all values are identical.
type ConstantEncoder struct{}

// NewConstantEncoder creates a ConstantEncoder.
func NewConstantEncoder() *ConstantEncoder {
	return &ConstantEncoder{}
}

// Encode stores a single value and its count.
func (e *ConstantEncoder) Encode(data interface{}, dataType DataType) ([]byte, error) {
	valueBytes, count, err := e.extractValue(data, dataType)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, int32(count))
	binary.Write(&buf, binary.LittleEndian, int32(len(valueBytes)))
	buf.Write(valueBytes)

	return buf.Bytes(), nil
}

// Decode replicates the stored value numRows times.
func (e *ConstantEncoder) Decode(encoded []byte, dataType DataType, numRows int64) (interface{}, error) {
	if len(encoded) == 0 {
		return e.emptyResult(dataType)
	}

	reader := bytes.NewReader(encoded)
	var count int32
	var valueLen int32

	if err := binary.Read(reader, binary.LittleEndian, &count); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	if err := binary.Read(reader, binary.LittleEndian, &valueLen); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}

	valueBytes := make([]byte, valueLen)
	if _, err := io.ReadFull(reader, valueBytes); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}

	return e.replicateValue(valueBytes, dataType, numRows)
}

func (e *ConstantEncoder) extractValue(data interface{}, dataType DataType) ([]byte, int, error) {
	switch dataType {
	case TypeInt64:
		d, ok := data.([]int64)
		if !ok {
			return nil, 0, fmt.Errorf("encode: expected []int64 for TypeInt64, got %T", data)
		}
		if len(d) == 0 {
			return nil, 0, fmt.Errorf("encode: cannot extract constant from empty slice")
		}
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, uint64(d[0]))
		return buf, len(d), nil
	case TypeInt32:
		d, ok := data.([]int32)
		if !ok {
			return nil, 0, fmt.Errorf("encode: expected []int32 for TypeInt32, got %T", data)
		}
		if len(d) == 0 {
			return nil, 0, fmt.Errorf("encode: cannot extract constant from empty slice")
		}
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, uint32(d[0]))
		return buf, len(d), nil
	case TypeFloat32:
		d, ok := data.([]float32)
		if !ok {
			return nil, 0, fmt.Errorf("encode: expected []float32 for TypeFloat32, got %T", data)
		}
		if len(d) == 0 {
			return nil, 0, fmt.Errorf("encode: cannot extract constant from empty slice")
		}
		buf := make([]byte, 4)
		binary.LittleEndian.PutUint32(buf, math.Float32bits(d[0]))
		return buf, len(d), nil
	case TypeFloat64:
		d, ok := data.([]float64)
		if !ok {
			return nil, 0, fmt.Errorf("encode: expected []float64 for TypeFloat64, got %T", data)
		}
		if len(d) == 0 {
			return nil, 0, fmt.Errorf("encode: cannot extract constant from empty slice")
		}
		buf := make([]byte, 8)
		binary.LittleEndian.PutUint64(buf, math.Float64bits(d[0]))
		return buf, len(d), nil
	case TypeString:
		d, ok := data.([]string)
		if !ok {
			return nil, 0, fmt.Errorf("encode: expected []string for TypeString, got %T", data)
		}
		if len(d) == 0 {
			return nil, 0, fmt.Errorf("encode: cannot extract constant from empty slice")
		}
		return []byte(d[0]), len(d), nil
	default:
		return nil, 0, fmt.Errorf("encode: unsupported data type %d for ConstantEncoder", dataType)
	}
}

func (e *ConstantEncoder) replicateValue(valueBytes []byte, dataType DataType, numRows int64) (interface{}, error) {
	switch dataType {
	case TypeInt64:
		if len(valueBytes) < 8 {
			return nil, fmt.Errorf("encode: value bytes too short for int64")
		}
		v := int64(binary.LittleEndian.Uint64(valueBytes[:8]))
		result := make([]int64, numRows)
		for i := range result {
			result[i] = v
		}
		return result, nil
	case TypeInt32:
		if len(valueBytes) < 4 {
			return nil, fmt.Errorf("encode: value bytes too short for int32")
		}
		v := int32(binary.LittleEndian.Uint32(valueBytes[:4]))
		result := make([]int32, numRows)
		for i := range result {
			result[i] = v
		}
		return result, nil
	case TypeFloat32:
		if len(valueBytes) < 4 {
			return nil, fmt.Errorf("encode: value bytes too short for float32")
		}
		v := math.Float32frombits(binary.LittleEndian.Uint32(valueBytes[:4]))
		result := make([]float32, numRows)
		for i := range result {
			result[i] = v
		}
		return result, nil
	case TypeFloat64:
		if len(valueBytes) < 8 {
			return nil, fmt.Errorf("encode: value bytes too short for float64")
		}
		v := math.Float64frombits(binary.LittleEndian.Uint64(valueBytes[:8]))
		result := make([]float64, numRows)
		for i := range result {
			result[i] = v
		}
		return result, nil
	case TypeString:
		s := string(valueBytes)
		result := make([]string, numRows)
		for i := range result {
			result[i] = s
		}
		return result, nil
	default:
		return nil, fmt.Errorf("encode: unsupported data type %d for ConstantEncoder", dataType)
	}
}

func (e *ConstantEncoder) emptyResult(dataType DataType) (interface{}, error) {
	switch dataType {
	case TypeInt64:
		return []int64{}, nil
	case TypeInt32:
		return []int32{}, nil
	case TypeFloat32:
		return []float32{}, nil
	case TypeFloat64:
		return []float64{}, nil
	case TypeString:
		return []string{}, nil
	default:
		return nil, fmt.Errorf("encode: unsupported data type %d for ConstantEncoder", dataType)
	}
}
