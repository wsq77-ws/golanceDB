package encode

import (
	"bytes"
	"testing"
)

func TestCompressNone(t *testing.T) {
	data := []byte("hello world, this is a test string")
	compressed, err := Compress(data, CompressionNone)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
	if !bytes.Equal(data, compressed) {
		t.Fatal("CompressionNone should return data as-is")
	}

	decompressed, err := Decompress(compressed, CompressionNone)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}
	if !bytes.Equal(data, decompressed) {
		t.Fatal("roundtrip failed for CompressionNone")
	}
}

func TestCompressZstdSmall(t *testing.T) {
	data := []byte("hello world, this is a small test string for zstd compression")
	compressed, err := Compress(data, CompressionZstd)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
	if len(compressed) >= len(data) {
		t.Logf("warning: compressed size %d >= original size %d", len(compressed), len(data))
	}

	decompressed, err := Decompress(compressed, CompressionZstd)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}
	if !bytes.Equal(data, decompressed) {
		t.Fatal("roundtrip failed for CompressionZstd small data")
	}
}

func TestCompressZstdLarge(t *testing.T) {
	data := bytes.Repeat([]byte("abcdefghijklmnopqrstuvwxyz0123456789"), 4000)
	compressed, err := Compress(data, CompressionZstd)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
	if len(compressed) >= len(data) {
		t.Fatalf("compressed size %d should be smaller than original %d", len(compressed), len(data))
	}

	decompressed, err := Decompress(compressed, CompressionZstd)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}
	if !bytes.Equal(data, decompressed) {
		t.Fatal("roundtrip failed for CompressionZstd large data")
	}
}

func TestCompressLz4Small(t *testing.T) {
	data := []byte("hello world, this is a small test string for lz4 compression")
	compressed, err := Compress(data, CompressionLz4)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	decompressed, err := Decompress(compressed, CompressionLz4)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}
	if !bytes.Equal(data, decompressed) {
		t.Fatal("roundtrip failed for CompressionLz4 small data")
	}
}

func TestCompressLz4Large(t *testing.T) {
	data := bytes.Repeat([]byte("abcdefghijklmnopqrstuvwxyz0123456789"), 4000)
	compressed, err := Compress(data, CompressionLz4)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
	if len(compressed) >= len(data) {
		t.Fatalf("compressed size %d should be smaller than original %d", len(compressed), len(data))
	}

	decompressed, err := Decompress(compressed, CompressionLz4)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}
	if !bytes.Equal(data, decompressed) {
		t.Fatal("roundtrip failed for CompressionLz4 large data")
	}
}

func TestCompressEmpty(t *testing.T) {
	data := []byte{}
	for _, algo := range []CompressionType{CompressionNone, CompressionZstd, CompressionLz4} {
		compressed, err := Compress(data, algo)
		if err != nil {
			t.Fatalf("Compress failed for algo %d: %v", algo, err)
		}
		decompressed, err := Decompress(compressed, algo)
		if err != nil {
			t.Fatalf("Decompress failed for algo %d: %v", algo, err)
		}
		if !bytes.Equal(data, decompressed) {
			t.Fatalf("roundtrip failed for algo %d", algo)
		}
	}
}

func TestCompressUnknown(t *testing.T) {
	_, err := Compress([]byte("test"), CompressionType(99))
	if err == nil {
		t.Fatal("expected error for unknown compression type")
	}

	_, err = Decompress([]byte("test"), CompressionType(99))
	if err == nil {
		t.Fatal("expected error for unknown compression type")
	}
}
