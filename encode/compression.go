package encode

import (
	"bytes"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
	"github.com/pierrec/lz4/v4"
)

// Compress applies the specified compression to data.
func Compress(data []byte, algo CompressionType) ([]byte, error) {
	switch algo {
	case CompressionNone:
		return data, nil
	case CompressionZstd:
		enc, err := zstd.NewWriter(nil)
		if err != nil {
			return nil, fmt.Errorf("encode: %w", err)
		}
		defer enc.Close()
		return enc.EncodeAll(data, make([]byte, 0, len(data))), nil
	case CompressionLz4:
		var buf bytes.Buffer
		w := lz4.NewWriter(&buf)
		if _, err := w.Write(data); err != nil {
			w.Close()
			return nil, fmt.Errorf("encode: %w", err)
		}
		if err := w.Close(); err != nil {
			return nil, fmt.Errorf("encode: %w", err)
		}
		return buf.Bytes(), nil
	default:
		return nil, fmt.Errorf("encode: unknown compression type %d", algo)
	}
}

// Decompress reverses compression.
func Decompress(data []byte, algo CompressionType) ([]byte, error) {
	switch algo {
	case CompressionNone:
		return data, nil
	case CompressionZstd:
		dec, err := zstd.NewReader(nil)
		if err != nil {
			return nil, fmt.Errorf("encode: %w", err)
		}
		defer dec.Close()
		out, err := dec.DecodeAll(data, make([]byte, 0, len(data)))
		if err != nil {
			return nil, fmt.Errorf("encode: %w", err)
		}
		return out, nil
	case CompressionLz4:
		r := lz4.NewReader(bytes.NewReader(data))
		out, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("encode: %w", err)
		}
		return out, nil
	default:
		return nil, fmt.Errorf("encode: unknown compression type %d", algo)
	}
}
