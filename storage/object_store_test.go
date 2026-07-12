package storage

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"
)

func TestLocalFS_WriteReadRoundtrip(t *testing.T) {
	fs := NewLocalFS(t.TempDir())
	ctx := context.Background()
	data := []byte("hello glancedb storage")

	if err := fs.Write(ctx, "data/file.lance", data); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	size, err := fs.Size(ctx, "data/file.lance")
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	got, err := fs.Read(ctx, "data/file.lance", 0, size)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("roundtrip mismatch: got %q, want %q", got, data)
	}
}

func TestLocalFS_ReadOffsetLength(t *testing.T) {
	fs := NewLocalFS(t.TempDir())
	ctx := context.Background()
	data := []byte("0123456789ABCDEF")
	if err := fs.Write(ctx, "f.bin", data); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := fs.Read(ctx, "f.bin", 4, 6)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if want := data[4:10]; !bytes.Equal(got, want) {
		t.Fatalf("Read offset/length mismatch: got %q, want %q", got, want)
	}

	if _, err := fs.Read(ctx, "f.bin", 0, -1); err == nil {
		t.Fatal("expected error for negative length, got nil")
	}

	if _, err := fs.Read(ctx, "missing", 0, 4); err == nil {
		t.Fatal("expected error reading missing file, got nil")
	}
}

func TestLocalFS_Exists(t *testing.T) {
	fs := NewLocalFS(t.TempDir())
	ctx := context.Background()

	exists, err := fs.Exists(ctx, "nope.bin")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("expected file to not exist before write")
	}

	if err := fs.Write(ctx, "yes.bin", []byte("x")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	exists, err = fs.Exists(ctx, "yes.bin")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected file to exist after write")
	}
}

func TestLocalFS_Delete(t *testing.T) {
	fs := NewLocalFS(t.TempDir())
	ctx := context.Background()

	if err := fs.Write(ctx, "del.bin", []byte("x")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := fs.Delete(ctx, "del.bin"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	exists, err := fs.Exists(ctx, "del.bin")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("expected file to not exist after delete")
	}

	if err := fs.Delete(ctx, "del.bin"); err == nil {
		t.Fatal("expected error deleting missing file, got nil")
	}
}

func TestLocalFS_List(t *testing.T) {
	fs := NewLocalFS(t.TempDir())
	ctx := context.Background()

	if err := fs.Write(ctx, "dir/a.lance", []byte("a")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := fs.Write(ctx, "dir/b.lance", []byte("b")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := fs.Write(ctx, "dir/sub/c.lance", []byte("c")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := fs.List(ctx, "dir")
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	// Non-recursive: only immediate files a.lance and b.lance.
	want := []string{filepath.Join("dir", "a.lance"), filepath.Join("dir", "b.lance")}
	if len(got) != len(want) {
		t.Fatalf("List returned %d entries, want %d: %v", len(got), len(want), got)
	}
	for i := range want {
		// On Windows, normalize separators for comparison.
		if filepath.ToSlash(got[i]) != filepath.ToSlash(want[i]) {
			t.Fatalf("List entry %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestLocalFS_Size(t *testing.T) {
	fs := NewLocalFS(t.TempDir())
	ctx := context.Background()

	data := []byte("meow meow")
	if err := fs.Write(ctx, "s.bin", data); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	size, err := fs.Size(ctx, "s.bin")
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size != int64(len(data)) {
		t.Fatalf("Size = %d, want %d", size, len(data))
	}

	if _, err := fs.Size(ctx, "missing"); err == nil {
		t.Fatal("expected error for Size on missing file, got nil")
	}
}

func TestLocalFS_OpenWrite(t *testing.T) {
	fs := NewLocalFS(t.TempDir())
	ctx := context.Background()

	w, err := fs.OpenWrite(ctx, "stream/out.lance")
	if err != nil {
		t.Fatalf("OpenWrite failed: %v", err)
	}
	data := []byte("streamed payload")
	if _, err := w.Write(data); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	size, err := fs.Size(ctx, "stream/out.lance")
	if err != nil {
		t.Fatalf("Size failed: %v", err)
	}
	if size != int64(len(data)) {
		t.Fatalf("Size = %d, want %d", size, len(data))
	}
	got, err := fs.Read(ctx, "stream/out.lance", 0, size)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if !bytes.Equal(got, data) {
		t.Fatalf("OpenWrite content mismatch: got %q, want %q", got, data)
	}

	var _ io.WriteCloser = w
}
