package storage

import (
	"fmt"
	"sync"
	"testing"
)

func TestBufferPool_PutGet(t *testing.T) {
	p := NewBufferPool(1024)
	p.Put("a", []byte("alpha"))
	p.Put("b", []byte("beta"))

	got, ok := p.Get("a")
	if !ok {
		t.Fatal("expected to find key a")
	}
	if string(got) != "alpha" {
		t.Fatalf("got %q, want alpha", got)
	}

	if _, ok := p.Get("missing"); ok {
		t.Fatal("expected miss for missing key")
	}

	used, entries := p.Stats()
	if entries != 2 {
		t.Fatalf("entries = %d, want 2", entries)
	}
	if used != int64(len("alpha")+len("beta")) {
		t.Fatalf("used = %d, want %d", used, len("alpha")+len("beta"))
	}

	// Overwrite existing key adjusts used size.
	p.Put("a", []byte("longer-alpha-value"))
	used, entries = p.Stats()
	if entries != 2 {
		t.Fatalf("after overwrite entries = %d, want 2", entries)
	}
	want := int64(len("longer-alpha-value") + len("beta"))
	if used != want {
		t.Fatalf("after overwrite used = %d, want %d", used, want)
	}
}

func TestBufferPool_LRUEviction(t *testing.T) {
	// Each entry is 4 bytes; capacity 12 holds at most 3 entries.
	p := NewBufferPool(12)
	p.Put("k1", []byte("1111"))
	p.Put("k2", []byte("2222"))
	p.Put("k3", []byte("3333"))

	// Touch k1 so k2 becomes LRU.
	if _, ok := p.Get("k1"); !ok {
		t.Fatal("expected k1 present")
	}

	// Adding k4 should evict k2 (the least recently used).
	p.Put("k4", []byte("4444"))

	if _, ok := p.Get("k2"); ok {
		t.Fatal("expected k2 to be evicted")
	}
	for _, k := range []string{"k1", "k3", "k4"} {
		if _, ok := p.Get(k); !ok {
			t.Fatalf("expected %s to be present", k)
		}
	}

	_, entries := p.Stats()
	if entries != 3 {
		t.Fatalf("entries = %d, want 3", entries)
	}
}

func TestBufferPool_LRUEvictionOrder(t *testing.T) {
	p := NewBufferPool(8)
	p.Put("a", []byte("AAAA"))
	p.Put("b", []byte("BBBB"))
	// Order (front->back): b, a. Adding c evicts a first.
	p.Put("c", []byte("CCCC"))
	if _, ok := p.Get("a"); ok {
		t.Fatal("expected a evicted")
	}
	if _, ok := p.Get("b"); !ok {
		t.Fatal("expected b present")
	}
	if _, ok := p.Get("c"); !ok {
		t.Fatal("expected c present")
	}
}

func TestBufferPool_Remove(t *testing.T) {
	p := NewBufferPool(1024)
	p.Put("x", []byte("data"))
	p.Put("y", []byte("more"))

	usedBefore, _ := p.Stats()
	p.Remove("x")
	usedAfter, entries := p.Stats()
	if entries != 1 {
		t.Fatalf("entries = %d, want 1", entries)
	}
	if usedAfter != usedBefore-int64(len("data")) {
		t.Fatalf("used = %d, want %d", usedAfter, usedBefore-int64(len("data")))
	}
	if _, ok := p.Get("x"); ok {
		t.Fatal("expected x to be removed")
	}

	// Removing a missing key is a no-op.
	p.Remove("nope")
	_, entries = p.Stats()
	if entries != 1 {
		t.Fatalf("entries = %d, want 1 after removing missing key", entries)
	}
}

func TestBufferPool_ZeroCapacity(t *testing.T) {
	p := NewBufferPool(0)
	p.Put("a", []byte("anything"))
	if _, ok := p.Get("a"); ok {
		t.Fatal("zero-capacity pool should not return cached data")
	}
	used, entries := p.Stats()
	if used != 0 || entries != 0 {
		t.Fatalf("zero-capacity stats: used=%d entries=%d, want 0/0", used, entries)
	}
}

func TestBufferPool_Concurrent(t *testing.T) {
	p := NewBufferPool(2048)
	const goroutines = 16
	const ops = 100

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < ops; i++ {
				key := fmt.Sprintf("g%d-k%d", id, i)
				p.Put(key, []byte(key))
				p.Get(key)
				if i%2 == 0 {
					p.Remove(key)
				}
			}
		}(g)
	}
	wg.Wait()

	used, entries := p.Stats()
	if used < 0 {
		t.Fatalf("used = %d, must be non-negative", used)
	}
	if entries < 0 {
		t.Fatalf("entries = %d, must be non-negative", entries)
	}
}
