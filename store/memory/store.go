package memory

import (
	"context"
	"hash/fnv"
	"sync"
	"time"

	xcache "github.com/giulio333/xcache"
)

type MemoryStore struct {
	shards    []*shard
	numShards uint64
	stopSweep chan struct{}
}

type shard struct {
	mu    sync.RWMutex
	items map[string]item
	tags  map[string]map[string]struct{} // tag → set of keys
}

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

func (s *MemoryStore) getShard(key string) *shard {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return s.shards[h.Sum64()%s.numShards]
}

func (s *MemoryStore) Get(_ context.Context, key string) (xcache.Entry, error) {
	sh := s.getShard(key)
	sh.mu.RLock()
	it, ok := sh.items[key]
	sh.mu.RUnlock()

	if !ok {
		return xcache.Entry{}, xcache.ErrNotFound
	}
	if it.isExpired() {
		sh.mu.Lock()
		removeFromTagIndex(sh, key, it.tags)
		delete(sh.items, key)
		sh.mu.Unlock()
		return xcache.Entry{}, xcache.ErrNotFound
	}
	return xcache.Entry{Value: it.value, ExpiresAt: it.expiresAt, Tags: it.tags}, nil
}

func (s *MemoryStore) GetMany(ctx context.Context, keys []string) (map[string]xcache.Entry, error) {
	result := make(map[string]xcache.Entry, len(keys))
	for _, k := range keys {
		entry, err := s.Get(ctx, k)
		if err != nil {
			continue // chiavi mancanti/scadute vengono saltate
		}
		result[k] = entry
	}
	return result, nil
}

func (s *MemoryStore) Set(_ context.Context, key string, value any, opts ...xcache.Option) error {
	o := xcache.ApplyOptions(opts)

	var expiresAt time.Time
	if o.TTL > 0 {
		expiresAt = time.Now().Add(o.TTL)
	}

	sh := s.getShard(key)
	sh.mu.Lock()
	// Rimuovi la chiave dall'indice precedente prima di sovrascrivere
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

func (s *MemoryStore) DeleteMany(ctx context.Context, keys []string) error {
	for _, k := range keys {
		_ = s.Delete(ctx, k)
	}
	return nil
}

func (s *MemoryStore) Clear(_ context.Context) error {
	for _, sh := range s.shards {
		sh.mu.Lock()
		sh.items = make(map[string]item)
		sh.tags = make(map[string]map[string]struct{})
		sh.mu.Unlock()
	}
	return nil
}

// removeFromTagIndex rimuove key dall'indice per ciascuno dei suoi tag.
// Deve essere chiamato con sh.mu già acquisito in scrittura.
func removeFromTagIndex(sh *shard, key string, tags []string) {
	for _, tag := range tags {
		delete(sh.tags[tag], key)
		if len(sh.tags[tag]) == 0 {
			delete(sh.tags, tag)
		}
	}
}

func (s *MemoryStore) Close() error {
	close(s.stopSweep)
	return nil
}

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
