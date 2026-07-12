package storage

import (
	"bytes"
	"strings"
	"testing"
)

func TestFileFooter_SerializeDeserialize(t *testing.T) {
	f := NewFileFooter(7)
	f.ColumnMetadataOffset = 1024
	f.ColumnMetadataOffsetTableOffset = 2048
	f.GlobalBuffersOffsetTableOffset = 4096
	f.NumGlobalBuffers = 3

	data := f.Serialize()
	if int64(len(data)) != FooterSize() {
		t.Fatalf("Serialize length = %d, want %d", len(data), FooterSize())
	}

	got, err := DeserializeFooter(data)
	if err != nil {
		t.Fatalf("DeserializeFooter failed: %v", err)
	}
	if got.ColumnMetadataOffset != 1024 {
		t.Fatalf("ColumnMetadataOffset = %d, want 1024", got.ColumnMetadataOffset)
	}
	if got.ColumnMetadataOffsetTableOffset != 2048 {
		t.Fatalf("ColumnMetadataOffsetTableOffset = %d, want 2048", got.ColumnMetadataOffsetTableOffset)
	}
	if got.GlobalBuffersOffsetTableOffset != 4096 {
		t.Fatalf("GlobalBuffersOffsetTableOffset = %d, want 4096", got.GlobalBuffersOffsetTableOffset)
	}
	if got.NumGlobalBuffers != 3 {
		t.Fatalf("NumGlobalBuffers = %d, want 3", got.NumGlobalBuffers)
	}
	if got.NumColumns != 7 {
		t.Fatalf("NumColumns = %d, want 7", got.NumColumns)
	}
	if got.MajorVersion != 2 {
		t.Fatalf("MajorVersion = %d, want 2", got.MajorVersion)
	}
	if got.MinorVersion != 1 {
		t.Fatalf("MinorVersion = %d, want 1", got.MinorVersion)
	}
	if got.Magic != MagicBytes {
		t.Fatalf("Magic = %v, want %v", got.Magic, MagicBytes)
	}
}

func TestNewFileFooter_Defaults(t *testing.T) {
	f := NewFileFooter(5)
	if f.NumColumns != 5 {
		t.Fatalf("NumColumns = %d, want 5", f.NumColumns)
	}
	if f.MajorVersion != 2 || f.MinorVersion != 1 {
		t.Fatalf("version = %d.%d, want 2.1", f.MajorVersion, f.MinorVersion)
	}
	if f.Magic != MagicBytes {
		t.Fatalf("Magic = %v, want %v", f.Magic, MagicBytes)
	}
	if string(f.Magic[:]) != "LANC" {
		t.Fatalf("Magic string = %q, want LANC", f.Magic[:])
	}
}

func TestFileFooter_MagicValidation(t *testing.T) {
	f := NewFileFooter(1)
	data := f.Serialize()
	// Corrupt the magic bytes.
	data[36] = 'X'
	data[37] = 'X'
	data[38] = 'X'
	data[39] = 'X'

	_, err := DeserializeFooter(data)
	if err == nil {
		t.Fatal("expected error for invalid magic, got nil")
	}
	if !strings.Contains(err.Error(), "magic") {
		t.Fatalf("error should mention magic, got: %v", err)
	}
}

func TestFileFooter_DeserializeTooShort(t *testing.T) {
	short := make([]byte, FooterSize()-1)
	_, err := DeserializeFooter(short)
	if err == nil {
		t.Fatal("expected error for short data, got nil")
	}
	if !strings.Contains(err.Error(), "short") {
		t.Fatalf("error should mention short, got: %v", err)
	}
}

func TestFileFooter_WriteReadFooter(t *testing.T) {
	f := NewFileFooter(12)
	f.ColumnMetadataOffset = 500
	f.ColumnMetadataOffsetTableOffset = 1000
	f.GlobalBuffersOffsetTableOffset = 2000
	f.NumGlobalBuffers = 2

	// Build a file = payload + footer.
	payload := bytes.Repeat([]byte{0xAB}, 100)
	var buf bytes.Buffer
	buf.Write(payload)
	if err := f.WriteFooter(&buf); err != nil {
		t.Fatalf("WriteFooter failed: %v", err)
	}

	fileSize := int64(buf.Len())
	if fileSize != int64(len(payload))+FooterSize() {
		t.Fatalf("fileSize = %d, want %d", fileSize, int64(len(payload))+FooterSize())
	}

	got, err := ReadFooter(bytes.NewReader(buf.Bytes()), fileSize)
	if err != nil {
		t.Fatalf("ReadFooter failed: %v", err)
	}
	if got.NumColumns != 12 {
		t.Fatalf("NumColumns = %d, want 12", got.NumColumns)
	}
	if got.ColumnMetadataOffset != 500 {
		t.Fatalf("ColumnMetadataOffset = %d, want 500", got.ColumnMetadataOffset)
	}
	if got.ColumnMetadataOffsetTableOffset != 1000 {
		t.Fatalf("ColumnMetadataOffsetTableOffset = %d, want 1000", got.ColumnMetadataOffsetTableOffset)
	}
	if got.GlobalBuffersOffsetTableOffset != 2000 {
		t.Fatalf("GlobalBuffersOffsetTableOffset = %d, want 2000", got.GlobalBuffersOffsetTableOffset)
	}
	if got.NumGlobalBuffers != 2 {
		t.Fatalf("NumGlobalBuffers = %d, want 2", got.NumGlobalBuffers)
	}
	if got.MajorVersion != 2 || got.MinorVersion != 1 {
		t.Fatalf("version = %d.%d, want 2.1", got.MajorVersion, got.MinorVersion)
	}
}

func TestFileFooter_ReadFooterTooSmall(t *testing.T) {
	small := make([]byte, 10)
	_, err := ReadFooter(bytes.NewReader(small), int64(len(small)))
	if err == nil {
		t.Fatal("expected error for file smaller than footer, got nil")
	}
}

func TestFooterSize_Constant(t *testing.T) {
	// 8 + 8 + 8 + 4 + 4 + 2 + 2 + 4 = 40
	if FooterSize() != 40 {
		t.Fatalf("FooterSize = %d, want 40", FooterSize())
	}
}
