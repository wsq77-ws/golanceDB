package storage

import (
	"container/list"
	"sync"
)

// CacheEntry is a single buffer pool entry.
type CacheEntry struct {
	key     string
	data    []byte
	size    int64
	element *list.Element
}

// BufferPool is an LRU cache of byte buffers keyed by "path:offset:length".
type BufferPool struct {
	mu       sync.RWMutex
	capacity int64
	used     int64
	entries  map[string]*CacheEntry
	lru      *list.List
}

// NewBufferPool creates a BufferPool with the given byte capacity.
func NewBufferPool(capacity int64) *BufferPool {
	return &BufferPool{
		capacity: capacity,
		entries:  make(map[string]*CacheEntry),
		lru:      list.New(),
	}
}

// Get returns cached data and updates the LRU order.
func (p *BufferPool) Get(key string) ([]byte, bool) {
	if p.capacity == 0 {
		return nil, false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.entries[key]
	if !ok {
		return nil, false
	}
	p.lru.MoveToFront(entry.element)
	return entry.data, true
}

// Put stores data under key, evicting LRU entries when over capacity.
func (p *BufferPool) Put(key string, data []byte) {
	if p.capacity == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	size := int64(len(data))
	if entry, ok := p.entries[key]; ok {
		p.used -= entry.size
		entry.data = data
		entry.size = size
		p.used += size
		p.lru.MoveToFront(entry.element)
	} else {
		entry := &CacheEntry{
			key:  key,
			data: data,
			size: size,
		}
		entry.element = p.lru.PushFront(entry)
		p.entries[key] = entry
		p.used += size
	}
	p.evict()
}

// Remove removes a specific entry from the pool.
func (p *BufferPool) Remove(key string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	entry, ok := p.entries[key]
	if !ok {
		return
	}
	p.lru.Remove(entry.element)
	delete(p.entries, key)
	p.used -= entry.size
}

// Stats returns the used bytes and number of cached entries.
func (p *BufferPool) Stats() (used int64, entries int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.used, len(p.entries)
}

// evict removes least recently used entries until used <= capacity.
func (p *BufferPool) evict() {
	for p.used > p.capacity {
		elem := p.lru.Back()
		if elem == nil {
			return
		}
		entry := elem.Value.(*CacheEntry)
		p.lru.Remove(elem)
		delete(p.entries, entry.key)
		p.used -= entry.size
	}
}
