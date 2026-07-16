package table

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/glancedb/glancedb/storage"
)

// manifestExt is the file extension for manifest files.
const manifestExt = ".manifest"

// ManifestStore manages manifest files with version tracking.
// When store is nil, it uses direct filesystem operations (local mode),
// which avoids ObjectStore interface dispatch overhead.
type ManifestStore struct {
	store    storage.ObjectStore // nil = direct filesystem mode
	basePath string              // absolute path to _versions directory
	mu       sync.Mutex
}

// NewManifestStore creates a ManifestStore backed by the given ObjectStore.
func NewManifestStore(store storage.ObjectStore, basePath string) *ManifestStore {
	return &ManifestStore{store: store, basePath: basePath}
}

// NewLocalManifestStore creates a ManifestStore using direct filesystem
// access without going through the ObjectStore interface. This is the
// default for local deployments and reduces interface dispatch overhead.
func NewLocalManifestStore(basePath string) *ManifestStore {
	return &ManifestStore{basePath: basePath}
}

// manifestPath returns the full path for a version's manifest file.
func (ms *ManifestStore) manifestPath(version int64) string {
	return filepath.Join(ms.basePath, fmt.Sprintf("%d%s", version, manifestExt))
}

// writeFile writes data to path, using ObjectStore or direct FS.
func (ms *ManifestStore) writeFile(ctx context.Context, path string, data []byte) error {
	if ms.store != nil {
		return ms.store.Write(ctx, path, data)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	return os.WriteFile(path, data, 0o644)
}

// readFile reads the full contents of path, using ObjectStore or direct FS.
func (ms *ManifestStore) readFile(ctx context.Context, path string) ([]byte, error) {
	if ms.store != nil {
		size, err := ms.store.Size(ctx, path)
		if err != nil {
			return nil, err
		}
		return ms.store.Read(ctx, path, 0, size)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	return os.ReadFile(path)
}

// existsFile checks if path exists, using ObjectStore or direct FS.
func (ms *ManifestStore) existsFile(ctx context.Context, path string) (bool, error) {
	if ms.store != nil {
		return ms.store.Exists(ctx, path)
	}
	if err := ctx.Err(); err != nil {
		return false, fmt.Errorf("storage: %w", err)
	}
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// listDir lists files in a directory (non-recursive), using ObjectStore or direct FS.
func (ms *ManifestStore) listDir(ctx context.Context, dir string) ([]string, error) {
	if ms.store != nil {
		return ms.store.List(ctx, dir)
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("storage: %w", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var result []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		result = append(result, filepath.Join(dir, e.Name()))
	}
	return result, nil
}

// deleteFile removes a file, using ObjectStore or direct FS.
func (ms *ManifestStore) deleteFile(ctx context.Context, path string) error {
	if ms.store != nil {
		return ms.store.Delete(ctx, path)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	return os.Remove(path)
}

// Write writes the manifest to its versioned path.
// On storage failure, the error includes the file path for diagnostics.
func (ms *ManifestStore) Write(ctx context.Context, manifest *Manifest) error {
	data, err := manifest.Serialize()
	if err != nil {
		return fmt.Errorf("table: serialize manifest v%d: %w", manifest.Version, err)
	}
	path := ms.manifestPath(manifest.Version)
	if err := ms.writeFile(ctx, path, data); err != nil {
		return fmt.Errorf("table: write manifest v%d to %q: %w", manifest.Version, path, err)
	}
	return nil
}

// Read reads the manifest for the given version.
// On storage failure, the error includes the file path for diagnostics.
func (ms *ManifestStore) Read(ctx context.Context, version int64) (*Manifest, error) {
	path := ms.manifestPath(version)
	data, err := ms.readFile(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("table: read manifest v%d from %q: %w", version, path, err)
	}
	manifest, err := DeserializeManifest(data)
	if err != nil {
		return nil, fmt.Errorf("table: deserialize manifest v%d: %w", version, err)
	}
	return manifest, nil
}

// LatestVersion returns the highest version number, or an error if none exist.
func (ms *ManifestStore) LatestVersion(ctx context.Context) (int64, error) {
	versions, err := ms.ListVersions(ctx)
	if err != nil {
		return 0, fmt.Errorf("table: list versions in %q: %w", ms.basePath, err)
	}
	if len(versions) == 0 {
		return 0, fmt.Errorf("table: no manifests found in %s", ms.basePath)
	}
	return versions[len(versions)-1], nil
}

// ListVersions returns all version numbers sorted ascending.
func (ms *ManifestStore) ListVersions(ctx context.Context) ([]int64, error) {
	exists, err := ms.existsFile(ctx, ms.basePath)
	if err != nil {
		return nil, fmt.Errorf("table: check versions dir %q: %w", ms.basePath, err)
	}
	if !exists {
		return nil, nil
	}
	paths, err := ms.listDir(ctx, ms.basePath)
	if err != nil {
		return nil, fmt.Errorf("table: list versions in %q: %w", ms.basePath, err)
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
	path := ms.manifestPath(version)
	if err := ms.deleteFile(ctx, path); err != nil {
		return fmt.Errorf("table: delete manifest v%d at %q: %w", version, path, err)
	}
	return nil
}

// ErrConflict is returned when a CAS commit detects a version mismatch.
var ErrConflict = errors.New("table: version conflict")

// latestPath returns the path to the _latest pointer file.
func (ms *ManifestStore) latestPath() string {
	return filepath.Join(ms.basePath, "_latest")
}

// ReadLatest reads the _latest file and returns the current latest version.
// Returns 0 if the _latest file doesn't exist (table not yet created).
func (ms *ManifestStore) ReadLatest(ctx context.Context) (int64, error) {
	path := ms.latestPath()
	exists, err := ms.existsFile(ctx, path)
	if err != nil {
		return 0, fmt.Errorf("table: check _latest at %q: %w", path, err)
	}
	if !exists {
		return 0, nil
	}
	data, err := ms.readFile(ctx, path)
	if err != nil {
		return 0, fmt.Errorf("table: read _latest from %q: %w", path, err)
	}
	version, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("table: parse _latest content %q: %w", string(data), err)
	}
	return version, nil
}

// Commit atomically writes a new manifest version using optimistic locking.
// It verifies that the current latest version on disk matches expectedPrevVersion
// before writing. If the check fails, returns ErrConflict.
// The caller should retry the operation (re-read latest, rebuild manifest, re-commit).
func (ms *ManifestStore) Commit(ctx context.Context, newManifest *Manifest, expectedPrevVersion int64) error {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	current, err := ms.ReadLatest(ctx)
	if err != nil {
		return fmt.Errorf("table: commit read latest: %w", err)
	}
	if current != expectedPrevVersion {
		return fmt.Errorf("table: commit expected prev v%d, got v%d: %w", expectedPrevVersion, current, ErrConflict)
	}
	if err := ms.Write(ctx, newManifest); err != nil {
		return fmt.Errorf("table: commit write: %w", err)
	}
	content := []byte(strconv.FormatInt(newManifest.Version, 10))
	latestPath := ms.latestPath()
	if err := ms.writeFile(ctx, latestPath, content); err != nil {
		return fmt.Errorf("table: commit write _latest %q: %w", latestPath, err)
	}
	return nil
}
