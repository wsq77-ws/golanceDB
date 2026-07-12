package encode

import (
	"reflect"
	"testing"
)

func TestMiniBlockInt64(t *testing.T) {
	data := make([]int64, 1000)
	for i := range data {
		data[i] = int64(i) * 3
	}

	for _, comp := range []CompressionType{CompressionNone, CompressionZstd} {
		enc := NewMiniBlockEncoder(comp)
		encoded, err := enc.Encode(data, TypeInt64)
		if err != nil {
			t.Fatalf("Encode failed (comp=%d): %v", comp, err)
		}
		decoded, err := enc.Decode(encoded, TypeInt64, int64(len(data)))
		if err != nil {
			t.Fatalf("Decode failed (comp=%d): %v", comp, err)
		}
		result, ok := decoded.([]int64)
		if !ok {
			t.Fatalf("expected []int64, got %T (comp=%d)", decoded, comp)
		}
		if !reflect.DeepEqual(data, result) {
			t.Fatalf("roundtrip mismatch (comp=%d)", comp)
		}
	}
}

func TestMiniBlockInt32(t *testing.T) {
	data := make([]int32, 500)
	for i := range data {
		data[i] = int32(i) * 7
	}

	enc := NewMiniBlockEncoder(CompressionZstd)
	encoded, err := enc.Encode(data, TypeInt32)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decoded, err := enc.Decode(encoded, TypeInt32, int64(len(data)))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	result, ok := decoded.([]int32)
	if !ok {
		t.Fatalf("expected []int32, got %T", decoded)
	}
	if !reflect.DeepEqual(data, result) {
		t.Fatal("roundtrip mismatch")
	}
}

func TestMiniBlockFloat32Vector(t *testing.T) {
	numRows := 100
	dim := 128
	data := make([]float32, numRows*dim)
	for i := range data {
		data[i] = float32(i) * 0.5
	}

	enc := NewMiniBlockEncoder(CompressionZstd)
	encoded, err := enc.Encode(data, TypeFixedSizeList)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decoded, err := enc.Decode(encoded, TypeFixedSizeList, int64(numRows))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	result, ok := decoded.([]float32)
	if !ok {
		t.Fatalf("expected []float32, got %T", decoded)
	}
	if len(result) != len(data) {
		t.Fatalf("length mismatch: expected %d, got %d", len(data), len(result))
	}
	if !reflect.DeepEqual(data, result) {
		t.Fatal("roundtrip mismatch for FixedSizeList vector data")
	}
}

func TestMiniBlockFloat64(t *testing.T) {
	data := make([]float64, 300)
	for i := range data {
		data[i] = float64(i) * 1.5
	}

	enc := NewMiniBlockEncoder(CompressionNone)
	encoded, err := enc.Encode(data, TypeFloat64)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decoded, err := enc.Decode(encoded, TypeFloat64, int64(len(data)))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	result, ok := decoded.([]float64)
	if !ok {
		t.Fatalf("expected []float64, got %T", decoded)
	}
	if !reflect.DeepEqual(data, result) {
		t.Fatal("roundtrip mismatch")
	}
}

func TestMiniBlockString(t *testing.T) {
	data := make([]string, 200)
	for i := range data {
		data[i] = "string_value_" + string(rune('A'+i%26)) + string(rune('a'+i%26))
	}

	for _, comp := range []CompressionType{CompressionNone, CompressionZstd} {
		enc := NewMiniBlockEncoder(comp)
		encoded, err := enc.Encode(data, TypeString)
		if err != nil {
			t.Fatalf("Encode failed (comp=%d): %v", comp, err)
		}
		decoded, err := enc.Decode(encoded, TypeString, int64(len(data)))
		if err != nil {
			t.Fatalf("Decode failed (comp=%d): %v", comp, err)
		}
		result, ok := decoded.([]string)
		if !ok {
			t.Fatalf("expected []string, got %T (comp=%d)", decoded, comp)
		}
		if !reflect.DeepEqual(data, result) {
			t.Fatalf("roundtrip mismatch (comp=%d)", comp)
		}
	}
}

func TestMiniBlockBinary(t *testing.T) {
	data := make([][]byte, 100)
	for i := range data {
		data[i] = []byte("binary_data_" + string(rune('A'+i%26)))
	}

	enc := NewMiniBlockEncoder(CompressionZstd)
	encoded, err := enc.Encode(data, TypeBinary)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}
	decoded, err := enc.Decode(encoded, TypeBinary, int64(len(data)))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}
	result, ok := decoded.([][]byte)
	if !ok {
		t.Fatalf("expected [][]byte, got %T", decoded)
	}
	if !reflect.DeepEqual(data, result) {
		t.Fatal("roundtrip mismatch")
	}
}

func TestMiniBlockEmpty(t *testing.T) {
	for _, tc := range []struct {
		name     string
		dataType DataType
	}{
		{"int64", TypeInt64},
		{"int32", TypeInt32},
		{"float32", TypeFloat32},
		{"float64", TypeFloat64},
		{"string", TypeString},
		{"binary", TypeBinary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enc := NewMiniBlockEncoder(CompressionZstd)
			encoded, err := enc.Encode(emptyInput(tc.dataType), tc.dataType)
			if err != nil {
				t.Fatalf("Encode failed: %v", err)
			}
			decoded, err := enc.Decode(encoded, tc.dataType, 0)
			if err != nil {
				t.Fatalf("Decode failed: %v", err)
			}
			if err := assertEmpty(decoded, tc.dataType); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMiniBlockSingleElement(t *testing.T) {
	t.Run("int64", func(t *testing.T) {
		data := []int64{42}
		enc := NewMiniBlockEncoder(CompressionZstd)
		encoded, err := enc.Encode(data, TypeInt64)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
		decoded, err := enc.Decode(encoded, TypeInt64, 1)
		if err != nil {
			t.Fatalf("Decode failed: %v", err)
		}
		result := decoded.([]int64)
		if !reflect.DeepEqual(data, result) {
			t.Fatal("roundtrip mismatch")
		}
	})

	t.Run("string", func(t *testing.T) {
		data := []string{"hello"}
		enc := NewMiniBlockEncoder(CompressionNone)
		encoded, err := enc.Encode(data, TypeString)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
		decoded, err := enc.Decode(encoded, TypeString, 1)
		if err != nil {
			t.Fatalf("Decode failed: %v", err)
		}
		result := decoded.([]string)
		if !reflect.DeepEqual(data, result) {
			t.Fatal("roundtrip mismatch")
		}
	})

	t.Run("float32", func(t *testing.T) {
		data := []float32{3.14}
		enc := NewMiniBlockEncoder(CompressionZstd)
		encoded, err := enc.Encode(data, TypeFloat32)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}
		decoded, err := enc.Decode(encoded, TypeFloat32, 1)
		if err != nil {
			t.Fatalf("Decode failed: %v", err)
		}
		result := decoded.([]float32)
		if !reflect.DeepEqual(data, result) {
			t.Fatal("roundtrip mismatch")
		}
	})
}

func emptyInput(dataType DataType) interface{} {
	switch dataType {
	case TypeInt64:
		return []int64{}
	case TypeInt32:
		return []int32{}
	case TypeFloat32:
		return []float32{}
	case TypeFloat64:
		return []float64{}
	case TypeString:
		return []string{}
	case TypeBinary:
		return [][]byte{}
	default:
		return nil
	}
}

func assertEmpty(result interface{}, dataType DataType) error {
	switch dataType {
	case TypeInt64:
		if r, ok := result.([]int64); !ok || len(r) != 0 {
			return errAssertion("expected empty []int64")
		}
	case TypeInt32:
		if r, ok := result.([]int32); !ok || len(r) != 0 {
			return errAssertion("expected empty []int32")
		}
	case TypeFloat32:
		if r, ok := result.([]float32); !ok || len(r) != 0 {
			return errAssertion("expected empty []float32")
		}
	case TypeFloat64:
		if r, ok := result.([]float64); !ok || len(r) != 0 {
			return errAssertion("expected empty []float64")
		}
	case TypeString:
		if r, ok := result.([]string); !ok || len(r) != 0 {
			return errAssertion("expected empty []string")
		}
	case TypeBinary:
		if r, ok := result.([][]byte); !ok || len(r) != 0 {
			return errAssertion("expected empty [][]byte")
		}
	}
	return nil
}

type errAssertion string

func (e errAssertion) Error() string { return string(e) }
