package table

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/glancedb/glancedb/encode"
	"github.com/glancedb/glancedb/storage"
)

func newTestManifestStore(t *testing.T) (*ManifestStore, storage.ObjectStore) {
	t.Helper()
	dir := t.TempDir()
	store := storage.NewLocalFS(dir)
	return NewManifestStore(store, "tbl/_versions"), store
}

func TestManifestStoreWriteReadRoundtrip(t *testing.T) {
	ctx := context.Background()
	ms, _ := newTestManifestStore(t)

	schema := NewSchema([]*Field{{Name: "id", Type: encode.TypeInt64}})
	m := NewManifest(3, schema)
	m.Timestamp = 12345
	m.Tags["k"] = "v"

	if err := ms.Write(ctx, m); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	got, err := ms.Read(ctx, 3)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got.Version != 3 {
		t.Errorf("expected version 3, got %d", got.Version)
	}
	if got.Timestamp != 12345 {
		t.Errorf("expected timestamp 12345, got %d", got.Timestamp)
	}
	if got.Tags["k"] != "v" {
		t.Errorf("expected tag k=v, got %v", got.Tags)
	}
}

func TestManifestStoreLatestVersion(t *testing.T) {
	ctx := context.Background()
	ms, _ := newTestManifestStore(t)
	schema := NewSchema([]*Field{{Name: "id", Type: encode.TypeInt64}})

	for _, v := range []int64{1, 5, 2, 10, 3} {
		if err := ms.Write(ctx, NewManifest(v, schema)); err != nil {
			t.Fatalf("Write(%d) failed: %v", v, err)
		}
	}

	latest, err := ms.LatestVersion(ctx)
	if err != nil {
		t.Fatalf("LatestVersion failed: %v", err)
	}
	if latest != 10 {
		t.Errorf("expected latest 10, got %d", latest)
	}
}

func TestManifestStoreListVersions(t *testing.T) {
	ctx := context.Background()
	ms, _ := newTestManifestStore(t)
	schema := NewSchema([]*Field{{Name: "id", Type: encode.TypeInt64}})

	for _, v := range []int64{7, 1, 4} {
		if err := ms.Write(ctx, NewManifest(v, schema)); err != nil {
			t.Fatalf("Write(%d) failed: %v", v, err)
		}
	}

	versions, err := ms.ListVersions(ctx)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}
	want := []int64{1, 4, 7}
	for i, v := range want {
		if versions[i] != v {
			t.Errorf("position %d: expected %d, got %d", i, v, versions[i])
		}
	}
}

func TestManifestStoreDeleteVersion(t *testing.T) {
	ctx := context.Background()
	ms, _ := newTestManifestStore(t)
	schema := NewSchema([]*Field{{Name: "id", Type: encode.TypeInt64}})

	if err := ms.Write(ctx, NewManifest(1, schema)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if err := ms.Write(ctx, NewManifest(2, schema)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := ms.DeleteVersion(ctx, 1); err != nil {
		t.Fatalf("DeleteVersion failed: %v", err)
	}

	versions, err := ms.ListVersions(ctx)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 1 || versions[0] != 2 {
		t.Errorf("expected [2], got %v", versions)
	}
}

func TestManifestStoreReadNonExistent(t *testing.T) {
	ctx := context.Background()
	ms, _ := newTestManifestStore(t)

	if _, err := ms.Read(ctx, 999); err == nil {
		t.Error("expected error reading non-existent version")
	}
}

func TestManifestStoreLatestVersionEmpty(t *testing.T) {
	ctx := context.Background()
	ms, _ := newTestManifestStore(t)

	if _, err := ms.LatestVersion(ctx); err == nil {
		t.Error("expected error on empty store")
	}
}

func TestManifestStoreListVersionsEmpty(t *testing.T) {
	ctx := context.Background()
	ms, _ := newTestManifestStore(t)

	versions, err := ms.ListVersions(ctx)
	if err != nil {
		t.Fatalf("ListVersions failed on empty store: %v", err)
	}
	if len(versions) != 0 {
		t.Errorf("expected empty list, got %v", versions)
	}
}

func TestManifestStoreIgnoresNonManifestFiles(t *testing.T) {
	ctx := context.Background()
	ms, store := newTestManifestStore(t)
	schema := NewSchema([]*Field{{Name: "id", Type: encode.TypeInt64}})

	if err := ms.Write(ctx, NewManifest(1, schema)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	// Write a stray non-manifest file into the same directory.
	if err := store.Write(ctx, filepath.Join("tbl/_versions", "README.txt"), []byte("hi")); err != nil {
		t.Fatalf("Write stray failed: %v", err)
	}

	versions, err := ms.ListVersions(ctx)
	if err != nil {
		t.Fatalf("ListVersions failed: %v", err)
	}
	if len(versions) != 1 || versions[0] != 1 {
		t.Errorf("expected [1], got %v", versions)
	}
}

func TestManifestStoreOverwrite(t *testing.T) {
	ctx := context.Background()
	ms, _ := newTestManifestStore(t)
	schema := NewSchema([]*Field{{Name: "id", Type: encode.TypeInt64}})

	m1 := NewManifest(1, schema)
	m1.Tags["v"] = "1"
	if err := ms.Write(ctx, m1); err != nil {
		t.Fatalf("Write 1 failed: %v", err)
	}

	m2 := NewManifest(1, schema)
	m2.Tags["v"] = "2"
	if err := ms.Write(ctx, m2); err != nil {
		t.Fatalf("Write 2 failed: %v", err)
	}

	got, err := ms.Read(ctx, 1)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got.Tags["v"] != "2" {
		t.Errorf("expected overwritten tag v=2, got %v", got.Tags)
	}
}
