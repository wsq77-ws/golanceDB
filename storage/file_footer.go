package storage

import (
	"encoding/binary"
	"fmt"
	"io"
)

// MagicBytes is the Lance file magic identifier ("LANC").
var MagicBytes = [4]byte{'L', 'A', 'N', 'C'}

// footerSize is the fixed on-disk size of a serialized FileFooter.
const footerSize = 8 + 8 + 8 + 4 + 4 + 2 + 2 + 4

// FileFooter is written at the end of each .lance data file to locate metadata.
type FileFooter struct {
	ColumnMetadataOffset            int64
	ColumnMetadataOffsetTableOffset int64
	GlobalBuffersOffsetTableOffset  int64
	NumGlobalBuffers                int32
	NumColumns                      int32
	MajorVersion                    uint16
	MinorVersion                    uint16
	Magic                           [4]byte
}

// NewFileFooter creates a FileFooter with default version 2.1.
func NewFileFooter(numColumns int32) *FileFooter {
	return &FileFooter{
		NumColumns:   numColumns,
		MajorVersion: 2,
		MinorVersion: 1,
		Magic:        MagicBytes,
	}
}

// FooterSize returns the fixed size of a serialized footer in bytes.
func FooterSize() int64 {
	return footerSize
}

// Serialize encodes the footer as little-endian bytes.
func (f *FileFooter) Serialize() []byte {
	buf := make([]byte, footerSize)
	binary.LittleEndian.PutUint64(buf[0:8], uint64(f.ColumnMetadataOffset))
	binary.LittleEndian.PutUint64(buf[8:16], uint64(f.ColumnMetadataOffsetTableOffset))
	binary.LittleEndian.PutUint64(buf[16:24], uint64(f.GlobalBuffersOffsetTableOffset))
	binary.LittleEndian.PutUint32(buf[24:28], uint32(f.NumGlobalBuffers))
	binary.LittleEndian.PutUint32(buf[28:32], uint32(f.NumColumns))
	binary.LittleEndian.PutUint16(buf[32:34], f.MajorVersion)
	binary.LittleEndian.PutUint16(buf[34:36], f.MinorVersion)
	copy(buf[36:40], f.Magic[:])
	return buf
}

// DeserializeFooter parses a serialized footer and validates its magic number.
func DeserializeFooter(data []byte) (*FileFooter, error) {
	if int64(len(data)) < footerSize {
		return nil, fmt.Errorf("storage: footer data too short: got %d, want %d", len(data), footerSize)
	}
	f := &FileFooter{}
	f.ColumnMetadataOffset = int64(binary.LittleEndian.Uint64(data[0:8]))
	f.ColumnMetadataOffsetTableOffset = int64(binary.LittleEndian.Uint64(data[8:16]))
	f.GlobalBuffersOffsetTableOffset = int64(binary.LittleEndian.Uint64(data[16:24]))
	f.NumGlobalBuffers = int32(binary.LittleEndian.Uint32(data[24:28]))
	f.NumColumns = int32(binary.LittleEndian.Uint32(data[28:32]))
	f.MajorVersion = binary.LittleEndian.Uint16(data[32:34])
	f.MinorVersion = binary.LittleEndian.Uint16(data[34:36])
	copy(f.Magic[:], data[36:40])
	if f.Magic != MagicBytes {
		return nil, fmt.Errorf("storage: invalid magic number: %v", f.Magic)
	}
	return f, nil
}

// WriteFooter writes the footer to w.
func (f *FileFooter) WriteFooter(w io.Writer) error {
	if _, err := w.Write(f.Serialize()); err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	return nil
}

// ReadFooter reads the footer from the end of a file of the given size.
func ReadFooter(r io.ReaderAt, fileSize int64) (*FileFooter, error) {
	if fileSize < footerSize {
		return nil, fmt.Errorf("storage: file size %d is smaller than footer size %d", fileSize, footerSize)
	}
	buf := make([]byte, footerSize)
	if _, err := r.ReadAt(buf, fileSize-footerSize); err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	return DeserializeFooter(buf)
}
