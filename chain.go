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

func (c *ChainStore) Get(ctx context.Context, key string) (any, error) {
	for i, s := range c.stores {
		val, err := s.Get(ctx, key)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return nil, err
		}
		// Popola gli store precedenti (più veloci) che non avevano la chiave
		for _, prev := range c.stores[:i] {
			_ = prev.Set(ctx, key, val)
		}
		return val, nil
	}
	return nil, ErrNotFound
}

func (c *ChainStore) GetMany(ctx context.Context, keys []string) (map[string]any, error) {
	result := make(map[string]any, len(keys))
	missing := keys

	for i, s := range c.stores {
		if len(missing) == 0 {
			break
		}
		found, err := s.GetMany(ctx, missing)
		if err != nil {
			return nil, err
		}
		for k, v := range found {
			result[k] = v
			// Popola gli store precedenti
			for _, prev := range c.stores[:i] {
				_ = prev.Set(ctx, k, v)
			}
		}
		// Calcola le chiavi ancora mancanti
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
