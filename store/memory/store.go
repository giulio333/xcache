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
	tags  map[string][]string // tag → []key
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
			tags:  make(map[string][]string),
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
	sh.items[key] = item{value: value, expiresAt: expiresAt, tags: o.Tags}
	for _, tag := range o.Tags {
		sh.tags[tag] = append(sh.tags[tag], key)
	}
	sh.mu.Unlock()
	return nil
}

func (s *MemoryStore) Delete(_ context.Context, key string) error {
	sh := s.getShard(key)
	sh.mu.Lock()
	delete(sh.items, key)
	sh.mu.Unlock()
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
		sh.tags = make(map[string][]string)
		sh.mu.Unlock()
	}
	return nil
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
