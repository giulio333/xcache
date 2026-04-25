// Package memory provides an in-memory implementation of xcache.Store.
//
// The store partitions keys across a configurable number of shards (default
// 64) hashed with FNV-1a, so that concurrent operations on different keys do
// not contend on a single mutex. Each shard owns its own map of items and
// its own tag index.
//
// Expiration is handled by two cooperating mechanisms:
//
//   - Passive: Get checks the deadline and removes the entry on read.
//   - Active: a background goroutine sweeps every shard at a configurable
//     interval (default one minute) and drops every entry whose deadline
//     has passed, even if it is never read again.
//
// MemoryStore must always be released with Close, which stops the sweeper
// goroutine. Without Close the goroutine remains alive for the lifetime of
// the process.
package memory

import (
	"context"
	"hash/fnv"
	"sync"
	"time"

	xcache "github.com/giulio333/xcache"
)

// MemoryStore is an in-memory xcache.Store. It is safe for concurrent use.
type MemoryStore struct {
	shards    []*shard
	numShards uint64
	stopSweep chan struct{}
}

// shard is a single, mutex-protected partition of the store. Each shard
// owns its own items map and tag index. The tag index maps a tag to the set
// of keys carrying it; using a set (rather than a slice) eliminates
// duplicates on repeated Set calls and gives O(1) per-key lookups during
// DeleteByTag.
type shard struct {
	mu    sync.RWMutex
	items map[string]item
	tags  map[string]map[string]struct{} // tag → set of keys
}

// NewStore returns a ready-to-use MemoryStore. The optional StoreOptions
// control the number of shards and the sweep interval. The returned store
// owns a background goroutine; callers must invoke Close to stop it.
func NewStore(opts ...StoreOption) *MemoryStore {
	cfg := applyStoreOptions(opts)

	s := &MemoryStore{
		shards:    make([]*shard, cfg.shards),
		numShards: cfg.shards,
		stopSweep: make(chan struct{}),
	}
	for i := range s.shards {
		s.shards[i] = &shard{
			items: make(map[string]item),
			tags:  make(map[string]map[string]struct{}),
		}
	}

	go s.sweep(cfg.sweepInterval)
	return s
}

// getShard returns the shard responsible for the given key, hashed with
// FNV-1a.
func (s *MemoryStore) getShard(key string) *shard {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return s.shards[h.Sum64()%s.numShards]
}

// Get returns the Entry stored under key. If the entry is expired it is
// removed from both the items map and the tag index, and ErrNotFound is
// returned.
func (s *MemoryStore) Get(_ context.Context, key string) (xcache.Entry, error) {
	sh := s.getShard(key)
	sh.mu.RLock()
	it, ok := sh.items[key]
	sh.mu.RUnlock()

	// key not found
	if !ok {
		return xcache.Entry{}, xcache.ErrNotFound
	}

	if it.isExpired() {
		sh.mu.Lock()
		// Re-read under write lock: a concurrent Set may have replaced the
		// expired entry. Only evict if expiresAt is unchanged.
		if current, ok := sh.items[key]; ok && current.expiresAt.Equal(it.expiresAt) {
			removeFromTagIndex(sh, key, it.tags)
			delete(sh.items, key)
		}
		sh.mu.Unlock()
		return xcache.Entry{}, xcache.ErrNotFound
	}
	return xcache.Entry{Value: it.value, ExpiresAt: it.expiresAt, Tags: it.tags}, nil
}

// GetMany returns the Entries that exist for the given keys. Missing or
// expired keys are silently omitted, mirroring the xcache.Store contract.
func (s *MemoryStore) GetMany(ctx context.Context, keys []string) (map[string]xcache.Entry, error) {
	result := make(map[string]xcache.Entry, len(keys))
	for _, k := range keys {
		entry, err := s.Get(ctx, k)
		if err != nil {
			continue
		}
		result[k] = entry
	}
	return result, nil
}

// Set stores value under key with the given options. If the key already
// exists with a different set of tags, the previous tag index entries are
// cleaned up before the new ones are recorded, so the index never contains
// stale references to overwritten values.
func (s *MemoryStore) Set(_ context.Context, key string, value any, opts ...xcache.Option) error {
	o := xcache.ApplyOptions(opts)

	var expiresAt time.Time
	if o.TTL > 0 {
		expiresAt = time.Now().Add(o.TTL)
	}

	sh := s.getShard(key)
	sh.mu.Lock()
	if old, exists := sh.items[key]; exists {
		removeFromTagIndex(sh, key, old.tags)
	}
	sh.items[key] = item{value: value, expiresAt: expiresAt, tags: o.Tags}
	for _, tag := range o.Tags {
		if sh.tags[tag] == nil {
			sh.tags[tag] = make(map[string]struct{})
		}
		sh.tags[tag][key] = struct{}{}
	}
	sh.mu.Unlock()
	return nil
}

// Delete removes a single key together with its tag index entries. It is a
// no-op if the key does not exist.
func (s *MemoryStore) Delete(_ context.Context, key string) error {
	sh := s.getShard(key)
	sh.mu.Lock()
	if it, ok := sh.items[key]; ok {
		removeFromTagIndex(sh, key, it.tags)
		delete(sh.items, key)
	}
	sh.mu.Unlock()
	return nil
}

// DeleteByTag removes every entry that was stored with the given tag.
//
// The implementation iterates every shard, collects the keys present in the
// tag index for that shard, and deletes them while holding the shard's
// write lock. Both the items map and the tag index are kept consistent.
func (s *MemoryStore) DeleteByTag(_ context.Context, tag string) error {
	for _, sh := range s.shards {
		sh.mu.Lock()
		keys := make([]string, 0, len(sh.tags[tag]))
		for key := range sh.tags[tag] {
			keys = append(keys, key)
		}
		for _, key := range keys {
			if it, ok := sh.items[key]; ok {
				removeFromTagIndex(sh, key, it.tags)
				delete(sh.items, key)
			}
		}
		sh.mu.Unlock()
	}
	return nil
}

// DeleteMany removes a batch of keys.
func (s *MemoryStore) DeleteMany(ctx context.Context, keys []string) error {
	for _, k := range keys {
		_ = s.Delete(ctx, k)
	}
	return nil
}

// Clear empties every shard, resetting both the items map and the tag
// index. Outstanding goroutine sweeps are unaffected.
func (s *MemoryStore) Clear(_ context.Context) error {
	for _, sh := range s.shards {
		sh.mu.Lock()
		sh.items = make(map[string]item)
		sh.tags = make(map[string]map[string]struct{})
		sh.mu.Unlock()
	}
	return nil
}

// removeFromTagIndex removes key from the tag index entries listed in tags.
// The shard's write lock must be held by the caller. When a tag's set
// becomes empty its bucket is removed entirely so the map does not grow
// unbounded with rarely-used tags.
func removeFromTagIndex(sh *shard, key string, tags []string) {
	for _, tag := range tags {
		delete(sh.tags[tag], key)
		if len(sh.tags[tag]) == 0 {
			delete(sh.tags, tag)
		}
	}
}

// Close stops the background sweeper goroutine. It must be called exactly
// once; calling it a second time will panic on the close of an already
// closed channel.
func (s *MemoryStore) Close() error {
	close(s.stopSweep)
	return nil
}

// sweep is the background goroutine that drops expired entries on a fixed
// schedule. It exits when stopSweep is closed by Close.
func (s *MemoryStore) sweep(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			for _, sh := range s.shards {
				sh.mu.Lock()
				for k, it := range sh.items {
					if it.isExpired() {
						removeFromTagIndex(sh, k, it.tags)
						delete(sh.items, k)
					}
				}
				sh.mu.Unlock()
			}
		case <-s.stopSweep:
			return
		}
	}
}
