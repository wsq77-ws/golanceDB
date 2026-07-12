package table

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/glancedb/glancedb/storage"
)

// manifestExt is the file extension for manifest files.
const manifestExt = ".manifest"

// ManifestStore manages manifest files with version tracking.
type ManifestStore struct {
	store    storage.ObjectStore
	basePath string
}

// NewManifestStore creates a ManifestStore rooted at basePath.
func NewManifestStore(store storage.ObjectStore, basePath string) *ManifestStore {
	return &ManifestStore{store: store, basePath: basePath}
}

// manifestPath returns the full path for a version's manifest file.
func (ms *ManifestStore) manifestPath(version int64) string {
	return filepath.Join(ms.basePath, fmt.Sprintf("%d%s", version, manifestExt))
}

// Write writes the manifest to its versioned path.
func (ms *ManifestStore) Write(ctx context.Context, manifest *Manifest) error {
	data, err := manifest.Serialize()
	if err != nil {
		return fmt.Errorf("table: %w", err)
	}
	if err := ms.store.Write(ctx, ms.manifestPath(manifest.Version), data); err != nil {
		return fmt.Errorf("table: %w", err)
	}
	return nil
}

// Read reads the manifest for the given version.
func (ms *ManifestStore) Read(ctx context.Context, version int64) (*Manifest, error) {
	path := ms.manifestPath(version)
	size, err := ms.store.Size(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	data, err := ms.store.Read(ctx, path, 0, size)
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	manifest, err := DeserializeManifest(data)
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	return manifest, nil
}

// LatestVersion returns the highest version number, or an error if none exist.
func (ms *ManifestStore) LatestVersion(ctx context.Context) (int64, error) {
	versions, err := ms.ListVersions(ctx)
	if err != nil {
		return 0, fmt.Errorf("table: %w", err)
	}
	if len(versions) == 0 {
		return 0, fmt.Errorf("table: no manifests found in %s", ms.basePath)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] > versions[j] })
	return versions[0], nil
}

// ListVersions returns all version numbers sorted ascending.
func (ms *ManifestStore) ListVersions(ctx context.Context) ([]int64, error) {
	exists, err := ms.store.Exists(ctx, ms.basePath)
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	if !exists {
		return nil, nil
	}
	paths, err := ms.store.List(ctx, ms.basePath)
	if err != nil {
		return nil, fmt.Errorf("table: %w", err)
	}
	var versions []int64
	for _, p := range paths {
		name := filepath.Base(p)
		if !strings.HasSuffix(name, manifestExt) {
			continue
		}
		numStr := strings.TrimSuffix(name, manifestExt)
		v, err := strconv.ParseInt(numStr, 10, 64)
		if err != nil {
			continue
		}
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	return versions, nil
}

// DeleteVersion removes the manifest file for the given version.
func (ms *ManifestStore) DeleteVersion(ctx context.Context, version int64) error {
	if err := ms.store.Delete(ctx, ms.manifestPath(version)); err != nil {
		return fmt.Errorf("table: %w", err)
	}
	return nil
}
