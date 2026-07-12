package encode

import (
	"reflect"
	"testing"
)

func TestConstantInt64(t *testing.T) {
	data := []int64{42, 42, 42, 42, 42}
	enc := NewConstantEncoder()

	encoded, err := enc.Encode(data, TypeInt64)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := enc.Decode(encoded, TypeInt64, int64(len(data)))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	result, ok := decoded.([]int64)
	if !ok {
		t.Fatalf("expected []int64, got %T", decoded)
	}
	if !reflect.DeepEqual(data, result) {
		t.Fatal("roundtrip mismatch")
	}
}

func TestConstantFloat32(t *testing.T) {
	data := []float32{3.14, 3.14, 3.14, 3.14}
	enc := NewConstantEncoder()

	encoded, err := enc.Encode(data, TypeFloat32)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := enc.Decode(encoded, TypeFloat32, int64(len(data)))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	result, ok := decoded.([]float32)
	if !ok {
		t.Fatalf("expected []float32, got %T", decoded)
	}
	if !reflect.DeepEqual(data, result) {
		t.Fatal("roundtrip mismatch")
	}
}

func TestConstantString(t *testing.T) {
	data := []string{"hello", "hello", "hello", "hello", "hello", "hello"}
	enc := NewConstantEncoder()

	encoded, err := enc.Encode(data, TypeString)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := enc.Decode(encoded, TypeString, int64(len(data)))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	result, ok := decoded.([]string)
	if !ok {
		t.Fatalf("expected []string, got %T", decoded)
	}
	if !reflect.DeepEqual(data, result) {
		t.Fatal("roundtrip mismatch")
	}
}

func TestConstantSingleElement(t *testing.T) {
	data := []int64{99}
	enc := NewConstantEncoder()

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
}

func TestConstantInt32(t *testing.T) {
	data := []int32{-7, -7, -7, -7, -7, -7, -7}
	enc := NewConstantEncoder()

	encoded, err := enc.Encode(data, TypeInt32)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := enc.Decode(encoded, TypeInt32, int64(len(data)))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	result := decoded.([]int32)
	if !reflect.DeepEqual(data, result) {
		t.Fatal("roundtrip mismatch")
	}
}

func TestConstantFloat64(t *testing.T) {
	data := []float64{2.718281828, 2.718281828, 2.718281828}
	enc := NewConstantEncoder()

	encoded, err := enc.Encode(data, TypeFloat64)
	if err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	decoded, err := enc.Decode(encoded, TypeFloat64, int64(len(data)))
	if err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	result := decoded.([]float64)
	if !reflect.DeepEqual(data, result) {
		t.Fatal("roundtrip mismatch")
	}
}
