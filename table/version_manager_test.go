package table

import (
	"sync"
	"testing"

	"github.com/glancedb/glancedb/encode"
)

func newTestSchema() *Schema {
	return NewSchema([]*Field{{Name: "id", Type: encode.TypeInt64}})
}

func TestNewVersionManagerUnlimited(t *testing.T) {
	vm := NewVersionManager(0)
	if vm == nil {
		t.Fatal("expected non-nil VersionManager")
	}
	if got := vm.LatestVersion(); got != -1 {
		t.Errorf("expected latest -1 for empty manager, got %d", got)
	}
	if _, ok := vm.Latest(); ok {
		t.Error("expected (nil, false) from Latest on empty manager")
	}
	if vm.HasVersion(1) {
		t.Error("expected HasVersion(1) to be false on empty manager")
	}
	if versions := vm.Versions(); len(versions) != 0 {
		t.Errorf("expected empty Versions, got %v", versions)
	}
}

func TestNewVersionManagerWithLimit(t *testing.T) {
	vm := NewVersionManager(3)
	schema := newTestSchema()

	for v := int64(1); v <= 3; v++ {
		vm.Publish(NewManifest(v, schema))
	}
	if got := vm.LatestVersion(); got != 3 {
		t.Errorf("expected latest 3, got %d", got)
	}
	if versions := vm.Versions(); len(versions) != 3 {
		t.Errorf("expected 3 versions, got %d (%v)", len(versions), versions)
	}
}

func TestPublishAndGet(t *testing.T) {
	vm := NewVersionManager(0)
	schema := newTestSchema()

	vm.Publish(NewManifest(7, schema))

	m, ok := vm.Get(7)
	if !ok {
		t.Fatal("expected Get(7) to find the manifest")
	}
	if m.Version != 7 {
		t.Errorf("expected version 7, got %d", m.Version)
	}

	if _, ok := vm.Get(6); ok {
		t.Error("expected Get(6) to return false for unpublished version")
	}
}

func TestLatestReturnsMostRecent(t *testing.T) {
	vm := NewVersionManager(0)
	schema := newTestSchema()

	for _, v := range []int64{1, 2, 3, 10, 4} {
		vm.Publish(NewManifest(v, schema))
	}

	m, ok := vm.Latest()
	if !ok {
		t.Fatal("expected Latest to return a manifest")
	}
	if m.Version != 10 {
		t.Errorf("expected latest version 10, got %d", m.Version)
	}
}

func TestLatestEmpty(t *testing.T) {
	vm := NewVersionManager(0)

	m, ok := vm.Latest()
	if ok || m != nil {
		t.Errorf("expected (nil, false) from Latest on empty manager, got (%v, %v)", m, ok)
	}
}

func TestLatestVersionEmptyReturnsMinusOne(t *testing.T) {
	vm := NewVersionManager(5)
	if got := vm.LatestVersion(); got != -1 {
		t.Errorf("expected -1, got %d", got)
	}
}

func TestRemove(t *testing.T) {
	vm := NewVersionManager(0)
	schema := newTestSchema()

	vm.Publish(NewManifest(1, schema))
	vm.Publish(NewManifest(2, schema))
	vm.Publish(NewManifest(3, schema))

	vm.Remove(2)
	if vm.HasVersion(2) {
		t.Error("expected HasVersion(2) to be false after Remove")
	}
	if !vm.HasVersion(1) || !vm.HasVersion(3) {
		t.Error("removing version 2 should not affect 1 and 3")
	}

	// Removing the latest should recompute the latest pointer.
	vm.Remove(3)
	if got := vm.LatestVersion(); got != 1 {
		t.Errorf("expected latest 1 after removing 3, got %d", got)
	}
}

func TestRemoveNonExistent(t *testing.T) {
	vm := NewVersionManager(0)
	schema := newTestSchema()
	vm.Publish(NewManifest(1, schema))

	vm.Remove(999)
	if got := vm.LatestVersion(); got != 1 {
		t.Errorf("removing non-existent version should not change latest, got %d", got)
	}
	if !vm.HasVersion(1) {
		t.Error("existing version should still be present")
	}
}

func TestRemoveLastVersionResetsLatest(t *testing.T) {
	vm := NewVersionManager(0)
	schema := newTestSchema()
	vm.Publish(NewManifest(5, schema))

	vm.Remove(5)
	if got := vm.LatestVersion(); got != -1 {
		t.Errorf("expected latest -1 after removing last version, got %d", got)
	}
	if _, ok := vm.Latest(); ok {
		t.Error("expected Latest to return false after removing last version")
	}
}

func TestClear(t *testing.T) {
	vm := NewVersionManager(0)
	schema := newTestSchema()

	for v := int64(1); v <= 5; v++ {
		vm.Publish(NewManifest(v, schema))
	}

	vm.Clear()
	if got := vm.LatestVersion(); got != -1 {
		t.Errorf("expected latest -1 after Clear, got %d", got)
	}
	if versions := vm.Versions(); len(versions) != 0 {
		t.Errorf("expected empty Versions after Clear, got %v", versions)
	}
	if vm.HasVersion(1) {
		t.Error("expected HasVersion to be false after Clear")
	}
}

func TestVersionsSortedAscending(t *testing.T) {
	vm := NewVersionManager(0)
	schema := newTestSchema()

	// Publish out of order.
	for _, v := range []int64{5, 1, 9, 3, 7} {
		vm.Publish(NewManifest(v, schema))
	}

	versions := vm.Versions()
	want := []int64{1, 3, 5, 7, 9}
	if len(versions) != len(want) {
		t.Fatalf("expected %d versions, got %d (%v)", len(want), len(versions), versions)
	}
	for i, v := range want {
		if versions[i] != v {
			t.Errorf("position %d: expected %d, got %d", i, v, versions[i])
		}
	}
}

func TestHasVersion(t *testing.T) {
	vm := NewVersionManager(0)
	schema := newTestSchema()

	vm.Publish(NewManifest(2, schema))

	if !vm.HasVersion(2) {
		t.Error("expected HasVersion(2) to be true")
	}
	if vm.HasVersion(3) {
		t.Error("expected HasVersion(3) to be false")
	}
}

func TestEvictionKeepsLatestVersions(t *testing.T) {
	vm := NewVersionManager(3)
	schema := newTestSchema()

	// Publish 5 versions with maxVersions=3; oldest two (1, 2) should be evicted.
	for v := int64(1); v <= 5; v++ {
		vm.Publish(NewManifest(v, schema))
	}

	want := []int64{3, 4, 5}
	versions := vm.Versions()
	if len(versions) != len(want) {
		t.Fatalf("expected %d versions, got %d (%v)", len(want), len(versions), versions)
	}
	for i, v := range want {
		if versions[i] != v {
			t.Errorf("position %d: expected %d, got %d", i, v, versions[i])
		}
	}

	if vm.HasVersion(1) || vm.HasVersion(2) {
		t.Error("expected versions 1 and 2 to be evicted")
	}
	if !vm.HasVersion(5) {
		t.Error("expected latest version 5 to be retained")
	}
	if got := vm.LatestVersion(); got != 5 {
		t.Errorf("expected latest 5, got %d", got)
	}
}

func TestEvictionNeverEvictsLatest(t *testing.T) {
	vm := NewVersionManager(1)
	schema := newTestSchema()

	vm.Publish(NewManifest(1, schema))
	vm.Publish(NewManifest(2, schema))

	versions := vm.Versions()
	if len(versions) != 1 {
		t.Fatalf("expected 1 version after eviction, got %d (%v)", len(versions), versions)
	}
	if versions[0] != 2 {
		t.Errorf("expected only latest version 2 retained, got %d", versions[0])
	}
}

func TestEvictionUnlimitedDoesNotEvict(t *testing.T) {
	vm := NewVersionManager(0)
	schema := newTestSchema()

	for v := int64(1); v <= 20; v++ {
		vm.Publish(NewManifest(v, schema))
	}

	versions := vm.Versions()
	if len(versions) != 20 {
		t.Errorf("expected 20 versions (unlimited), got %d", len(versions))
	}
	if !vm.HasVersion(1) {
		t.Error("expected version 1 to be retained with unlimited cache")
	}
}

func TestPublishOutOfOrderKeepsHighestLatest(t *testing.T) {
	vm := NewVersionManager(0)
	schema := newTestSchema()

	vm.Publish(NewManifest(5, schema))
	if got := vm.LatestVersion(); got != 5 {
		t.Errorf("expected latest 5, got %d", got)
	}

	// Publishing a lower version must not lower the latest pointer.
	vm.Publish(NewManifest(3, schema))
	if got := vm.LatestVersion(); got != 5 {
		t.Errorf("expected latest to remain 5 after publishing 3, got %d", got)
	}
	if !vm.HasVersion(3) {
		t.Error("expected version 3 to be cached")
	}
	if !vm.HasVersion(5) {
		t.Error("expected version 5 to be cached")
	}

	m, ok := vm.Latest()
	if !ok || m.Version != 5 {
		t.Errorf("expected Latest to return version 5, got (%v, %v)", m, ok)
	}
}

func TestPublishOverwritesSameVersion(t *testing.T) {
	vm := NewVersionManager(0)
	schema := newTestSchema()

	vm.Publish(NewManifest(1, schema))
	updated := NewManifest(1, schema)
	updated.Tags["note"] = "updated"
	vm.Publish(updated)

	m, ok := vm.Get(1)
	if !ok {
		t.Fatal("expected Get(1) to find the manifest")
	}
	if m.Tags["note"] != "updated" {
		t.Errorf("expected updated tag, got %v", m.Tags)
	}
	if got := vm.LatestVersion(); got != 1 {
		t.Errorf("expected latest 1, got %d", got)
	}
}

func TestConcurrentPublish(t *testing.T) {
	vm := NewVersionManager(0)
	schema := newTestSchema()

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(v int64) {
			defer wg.Done()
			vm.Publish(NewManifest(v, schema))
		}(int64(i + 1))
	}
	wg.Wait()

	versions := vm.Versions()
	if len(versions) != goroutines {
		t.Errorf("expected %d versions, got %d", goroutines, len(versions))
	}
	if got := vm.LatestVersion(); got != goroutines {
		t.Errorf("expected latest %d, got %d", goroutines, got)
	}
}

func TestConcurrentGetAndPublish(t *testing.T) {
	vm := NewVersionManager(0)
	schema := newTestSchema()

	// Seed some versions for readers to query.
	for v := int64(1); v <= 10; v++ {
		vm.Publish(NewManifest(v, schema))
	}

	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		go func(v int64) {
			defer wg.Done()
			vm.Publish(NewManifest(v, schema))
		}(int64(100 + i))
	}
	for i := 0; i < goroutines; i++ {
		go func(v int64) {
			defer wg.Done()
			_, _ = vm.Get(v)
			_, _ = vm.Latest()
			_ = vm.LatestVersion()
			_ = vm.HasVersion(v)
			_ = vm.Versions()
		}(int64(i + 1))
	}
	wg.Wait()

	// After all goroutines complete, the cache should hold all published versions.
	if got := vm.LatestVersion(); got != int64(100+goroutines-1) {
		t.Errorf("expected latest %d, got %d", 100+goroutines-1, got)
	}
}

func TestConcurrentRemoveAndPublish(t *testing.T) {
	vm := NewVersionManager(0)
	schema := newTestSchema()

	for v := int64(1); v <= 20; v++ {
		vm.Publish(NewManifest(v, schema))
	}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for i := 0; i < goroutines; i++ {
		v := int64(i + 1)
		go func() {
			defer wg.Done()
			vm.Remove(v)
		}()
		go func(v int64) {
			defer wg.Done()
			vm.Publish(NewManifest(v, schema))
		}(int64(100 + i))
	}
	wg.Wait()

	// Just ensure no panic / race occurred; state is consistent.
	_ = vm.Versions()
	_ = vm.LatestVersion()
}
