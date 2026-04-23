package xcache

import (
	"context"
	"errors"
)

// ChainStore esegue le operazioni in cascata su una lista di Store (es. L1→L2).
// Get cerca nell'ordine: primo store che risponde vince.
// Set e Delete propagano a tutti gli store.
type ChainStore struct {
	stores []Store
}

func NewChain(stores ...Store) *ChainStore {
	return &ChainStore{stores: stores}
}

func (c *ChainStore) Get(ctx context.Context, key string) (Entry, error) {
	for i, s := range c.stores {
		entry, err := s.Get(ctx, key)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return Entry{}, err
		}
		// Popola gli store precedenti (più veloci) propagando TTL e tag originali
		opts := entryOpts(entry)
		for _, prev := range c.stores[:i] {
			_ = prev.Set(ctx, key, entry.Value, opts...)
		}
		return entry, nil
	}
	return Entry{}, ErrNotFound
}

func (c *ChainStore) GetMany(ctx context.Context, keys []string) (map[string]Entry, error) {
	result := make(map[string]Entry, len(keys))
	missing := keys

	for i, s := range c.stores {
		if len(missing) == 0 {
			break
		}
		found, err := s.GetMany(ctx, missing)
		if err != nil {
			return nil, err
		}
		for k, entry := range found {
			result[k] = entry
			opts := entryOpts(entry)
			for _, prev := range c.stores[:i] {
				_ = prev.Set(ctx, k, entry.Value, opts...)
			}
		}
		next := missing[:0]
		for _, k := range missing {
			if _, ok := found[k]; !ok {
				next = append(next, k)
			}
		}
		missing = next
	}
	return result, nil
}

func (c *ChainStore) Set(ctx context.Context, key string, value any, opts ...Option) error {
	for _, s := range c.stores {
		if err := s.Set(ctx, key, value, opts...); err != nil {
			return err
		}
	}
	return nil
}

func (c *ChainStore) Delete(ctx context.Context, key string) error {
	for _, s := range c.stores {
		if err := s.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

func (c *ChainStore) DeleteMany(ctx context.Context, keys []string) error {
	for _, s := range c.stores {
		if err := s.DeleteMany(ctx, keys); err != nil {
			return err
		}
	}
	return nil
}

func (c *ChainStore) Clear(ctx context.Context) error {
	for _, s := range c.stores {
		if err := s.Clear(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (c *ChainStore) Close() error {
	for _, s := range c.stores {
		if err := s.Close(); err != nil {
			return err
		}
	}
	return nil
}

// entryOpts converte i metadati di un Entry nelle Option da passare a Set.
func entryOpts(e Entry) []Option {
	var opts []Option
	if ttl := e.RemainingTTL(); ttl > 0 {
		opts = append(opts, WithTTL(ttl))
	}
	if len(e.Tags) > 0 {
		opts = append(opts, WithTags(e.Tags...))
	}
	return opts
}
