package encode

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

const defaultMiniBlockSize int32 = 4096

// MiniBlockEncoder implements columnar encoding using Mini-Block pages.
type MiniBlockEncoder struct {
	Compression CompressionType
	BlockSize   int32
}

// NewMiniBlockEncoder creates a MiniBlockEncoder with the given compression and default block size.
func NewMiniBlockEncoder(compression CompressionType) *MiniBlockEncoder {
	return &MiniBlockEncoder{
		Compression: compression,
		BlockSize:   defaultMiniBlockSize,
	}
}

// Encode serializes data into a Mini-Block encoded page.
func (e *MiniBlockEncoder) Encode(data interface{}, dataType DataType) ([]byte, error) {
	buf, err := e.serialize(data, dataType)
	if err != nil {
		return nil, err
	}

	blocks := e.splitBlocks(buf)

	var page bytes.Buffer
	binary.Write(&page, binary.LittleEndian, int32(EncodingMiniBlock))
	binary.Write(&page, binary.LittleEndian, int32(len(blocks)))
	binary.Write(&page, binary.LittleEndian, e.BlockSize)

	for _, block := range blocks {
		compressed, err := Compress(block, e.Compression)
		if err != nil {
			return nil, fmt.Errorf("encode: %w", err)
		}
		binary.Write(&page, binary.LittleEndian, int32(len(compressed)))
		page.Write(compressed)
	}

	return page.Bytes(), nil
}

// Decode reconstructs the original Go slice from a Mini-Block encoded page.
func (e *MiniBlockEncoder) Decode(encoded []byte, dataType DataType, numRows int64) (interface{}, error) {
	if len(encoded) == 0 {
		return e.emptyResult(dataType)
	}

	reader := bytes.NewReader(encoded)
	var encoding EncodingType
	var numBlocks int32
	var blockSize int32

	if err := binary.Read(reader, binary.LittleEndian, (*int32)(&encoding)); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	if err := binary.Read(reader, binary.LittleEndian, &numBlocks); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}
	if err := binary.Read(reader, binary.LittleEndian, &blockSize); err != nil {
		return nil, fmt.Errorf("encode: %w", err)
	}

	var dataBuf bytes.Buffer
	for i := int32(0); i < numBlocks; i++ {
		var blockLen int32
		if err := binary.Read(reader, binary.LittleEndian, &blockLen); err != nil {
			return nil, fmt.Errorf("encode: %w", err)
		}
		compressed := make([]byte, blockLen)
		if _, err := io.ReadFull(reader, compressed); err != nil {
			return nil, fmt.Errorf("encode: %w", err)
		}
		decompressed, err := Decompress(compressed, e.Compression)
		if err != nil {
			return nil, fmt.Errorf("encode: %w", err)
		}
		dataBuf.Write(decompressed)
	}

	return e.deserialize(dataBuf.Bytes(), dataType, numRows)
}

func (e *MiniBlockEncoder) splitBlocks(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}
	blockSize := int(e.BlockSize)
	if blockSize <= 0 {
		blockSize = int(defaultMiniBlockSize)
	}
	var blocks [][]byte
	for i := 0; i < len(data); i += blockSize {
		end := i + blockSize
		if end > len(data) {
			end = len(data)
		}
		blocks = append(blocks, data[i:end])
	}
	return blocks
}

func (e *MiniBlockEncoder) serialize(data interface{}, dataType DataType) ([]byte, error) {
	switch dataType {
	case TypeInt64:
		d, ok := data.([]int64)
		if !ok {
			return nil, fmt.Errorf("encode: expected []int64 for TypeInt64, got %T", data)
		}
		return serializeInt64(d), nil
	case TypeInt32:
		d, ok := data.([]int32)
		if !ok {
			return nil, fmt.Errorf("encode: expected []int32 for TypeInt32, got %T", data)
		}
		return serializeInt32(d), nil
	case TypeFloat32, TypeFixedSizeList:
		d, ok := data.([]float32)
		if !ok {
			return nil, fmt.Errorf("encode: expected []float32 for type %d, got %T", dataType, data)
		}
		return serializeFloat32(d), nil
	case TypeFloat64:
		d, ok := data.([]float64)
		if !ok {
			return nil, fmt.Errorf("encode: expected []float64 for TypeFloat64, got %T", data)
		}
		return serializeFloat64(d), nil
	case TypeString:
		d, ok := data.([]string)
		if !ok {
			return nil, fmt.Errorf("encode: expected []string for TypeString, got %T", data)
		}
		return serializeStrings(d), nil
	case TypeBinary:
		d, ok := data.([][]byte)
		if !ok {
			return nil, fmt.Errorf("encode: expected [][]byte for TypeBinary, got %T", data)
		}
		return serializeBytes(d), nil
	default:
		return nil, fmt.Errorf("encode: unsupported data type %d", dataType)
	}
}

func (e *MiniBlockEncoder) deserialize(data []byte, dataType DataType, numRows int64) (interface{}, error) {
	switch dataType {
	case TypeInt64:
		return deserializeInt64(data), nil
	case TypeInt32:
		return deserializeInt32(data), nil
	case TypeFloat32, TypeFixedSizeList:
		return deserializeFloat32(data), nil
	case TypeFloat64:
		return deserializeFloat64(data), nil
	case TypeString:
		return deserializeStrings(data), nil
	case TypeBinary:
		return deserializeBytes(data), nil
	default:
		return nil, fmt.Errorf("encode: unsupported data type %d", dataType)
	}
}

func (e *MiniBlockEncoder) emptyResult(dataType DataType) (interface{}, error) {
	switch dataType {
	case TypeInt64:
		return []int64{}, nil
	case TypeInt32:
		return []int32{}, nil
	case TypeFloat32, TypeFixedSizeList:
		return []float32{}, nil
	case TypeFloat64:
		return []float64{}, nil
	case TypeString:
		return []string{}, nil
	case TypeBinary:
		return [][]byte{}, nil
	default:
		return nil, fmt.Errorf("encode: unsupported data type %d", dataType)
	}
}

func serializeInt64(data []int64) []byte {
	buf := make([]byte, 8*len(data))
	for i, v := range data {
		binary.LittleEndian.PutUint64(buf[i*8:], uint64(v))
	}
	return buf
}

func serializeInt32(data []int32) []byte {
	buf := make([]byte, 4*len(data))
	for i, v := range data {
		binary.LittleEndian.PutUint32(buf[i*4:], uint32(v))
	}
	return buf
}

func serializeFloat32(data []float32) []byte {
	buf := make([]byte, 4*len(data))
	for i, v := range data {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

func serializeFloat64(data []float64) []byte {
	buf := make([]byte, 8*len(data))
	for i, v := range data {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(v))
	}
	return buf
}

func serializeStrings(data []string) []byte {
	var buf bytes.Buffer
	for _, s := range data {
		b := []byte(s)
		binary.Write(&buf, binary.LittleEndian, int32(len(b)))
		buf.Write(b)
	}
	return buf.Bytes()
}

func serializeBytes(data [][]byte) []byte {
	var buf bytes.Buffer
	for _, b := range data {
		binary.Write(&buf, binary.LittleEndian, int32(len(b)))
		buf.Write(b)
	}
	return buf.Bytes()
}

func deserializeInt64(data []byte) []int64 {
	n := len(data) / 8
	result := make([]int64, n)
	for i := 0; i < n; i++ {
		result[i] = int64(binary.LittleEndian.Uint64(data[i*8:]))
	}
	return result
}

func deserializeInt32(data []byte) []int32 {
	n := len(data) / 4
	result := make([]int32, n)
	for i := 0; i < n; i++ {
		result[i] = int32(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return result
}

func deserializeFloat32(data []byte) []float32 {
	n := len(data) / 4
	result := make([]float32, n)
	for i := 0; i < n; i++ {
		result[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return result
}

func deserializeFloat64(data []byte) []float64 {
	n := len(data) / 8
	result := make([]float64, n)
	for i := 0; i < n; i++ {
		result[i] = math.Float64frombits(binary.LittleEndian.Uint64(data[i*8:]))
	}
	return result
}

func deserializeStrings(data []byte) []string {
	if len(data) == 0 {
		return []string{}
	}
	reader := bytes.NewReader(data)
	var result []string
	for reader.Len() > 0 {
		var length int32
		if err := binary.Read(reader, binary.LittleEndian, &length); err != nil {
			return result
		}
		s := make([]byte, length)
		if _, err := io.ReadFull(reader, s); err != nil {
			return result
		}
		result = append(result, string(s))
	}
	return result
}

func deserializeBytes(data []byte) [][]byte {
	if len(data) == 0 {
		return [][]byte{}
	}
	reader := bytes.NewReader(data)
	var result [][]byte
	for reader.Len() > 0 {
		var length int32
		if err := binary.Read(reader, binary.LittleEndian, &length); err != nil {
			return result
		}
		b := make([]byte, length)
		if _, err := io.ReadFull(reader, b); err != nil {
			return result
		}
		result = append(result, b)
	}
	return result
}
