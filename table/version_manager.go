package table

import (
	"sort"
	"sync"
)

// VersionManager maintains an in-memory cache of manifests for MVCC snapshot isolation.
// It tracks the latest version and keeps recent versions accessible for time-travel queries.
type VersionManager struct {
	mu            sync.RWMutex
	manifests     map[int64]*Manifest
	latestVersion int64
	maxVersions   int
}

// NewVersionManager creates a VersionManager that keeps at most maxVersions
// recent versions in memory. A maxVersions of 0 means unlimited.
func NewVersionManager(maxVersions int) *VersionManager {
	return &VersionManager{
		manifests:     make(map[int64]*Manifest),
		latestVersion: -1,
		maxVersions:   maxVersions,
	}
}

// Publish registers a new manifest version and updates the latest pointer.
// When maxVersions is positive and the cache exceeds the limit, the oldest
// versions (lowest version numbers) are evicted; the latest version is never evicted.
func (vm *VersionManager) Publish(manifest *Manifest) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	vm.manifests[manifest.Version] = manifest
	if manifest.Version > vm.latestVersion {
		vm.latestVersion = manifest.Version
	}
	vm.evict()
}

// evict removes the lowest version numbers until the cache fits maxVersions.
// The latest version is never evicted. Caller must hold vm.mu.
func (vm *VersionManager) evict() {
	if vm.maxVersions <= 0 {
		return
	}
	for len(vm.manifests) > vm.maxVersions {
		oldest := vm.oldestVersion()
		if oldest == vm.latestVersion {
			return
		}
		delete(vm.manifests, oldest)
	}
}

// oldestVersion returns the smallest cached version number, or -1 if empty.
// Caller must hold vm.mu.
func (vm *VersionManager) oldestVersion() int64 {
	if len(vm.manifests) == 0 {
		return -1
	}
	versions := vm.versionsLocked()
	return versions[0]
}

// versionsLocked returns cached version numbers sorted ascending.
// Caller must hold vm.mu.
func (vm *VersionManager) versionsLocked() []int64 {
	versions := make([]int64, 0, len(vm.manifests))
	for v := range vm.manifests {
		versions = append(versions, v)
	}
	sort.Slice(versions, func(i, j int) bool { return versions[i] < versions[j] })
	return versions
}

// Get returns the manifest for a specific version from cache.
// Returns (nil, false) if the version is not cached.
func (vm *VersionManager) Get(version int64) (*Manifest, bool) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	m, ok := vm.manifests[version]
	return m, ok
}

// Latest returns the latest cached manifest.
// Returns (nil, false) if the cache is empty.
func (vm *VersionManager) Latest() (*Manifest, bool) {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	if vm.latestVersion < 0 {
		return nil, false
	}
	m, ok := vm.manifests[vm.latestVersion]
	return m, ok
}

// LatestVersion returns the latest version number, or -1 if the cache is empty.
func (vm *VersionManager) LatestVersion() int64 {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	return vm.latestVersion
}

// Remove removes a version from the cache. If the removed version was the latest,
// the latest pointer is recomputed from the remaining cached versions.
func (vm *VersionManager) Remove(version int64) {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	delete(vm.manifests, version)
	if version == vm.latestVersion {
		vm.latestVersion = vm.recomputeLatestLocked()
	}
}

// recomputeLatestLocked returns the highest cached version, or -1 if empty.
// Caller must hold vm.mu.
func (vm *VersionManager) recomputeLatestLocked() int64 {
	if len(vm.manifests) == 0 {
		return -1
	}
	versions := vm.versionsLocked()
	return versions[len(versions)-1]
}

// Clear removes all cached versions and resets the latest pointer.
func (vm *VersionManager) Clear() {
	vm.mu.Lock()
	defer vm.mu.Unlock()

	vm.manifests = make(map[int64]*Manifest)
	vm.latestVersion = -1
}

// Versions returns all cached version numbers sorted ascending.
func (vm *VersionManager) Versions() []int64 {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	return vm.versionsLocked()
}

// HasVersion reports whether a version is present in the cache.
func (vm *VersionManager) HasVersion(version int64) bool {
	vm.mu.RLock()
	defer vm.mu.RUnlock()

	_, ok := vm.manifests[version]
	return ok
}
