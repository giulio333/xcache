package xcache

import (
	"context"
	"errors"
)

// ChainStore composes several Store instances into a read-through cascade,
// typically used to layer a fast in-memory L1 in front of a slower
// distributed L2 (Redis, Memcached, ...).
//
// Read semantics:
//
//   - Get queries each store in order. The first hit wins; every store that
//     was queried and missed is back-filled with the value, propagating the
//     remaining TTL and the tags of the original Entry.
//   - GetMany follows the same logic for batches.
//
// Write semantics:
//
//   - Set, Delete, DeleteMany, DeleteByTag and Clear propagate to every
//     store in order. The first error short-circuits the call and is
//     returned to the caller. The operation is therefore not atomic across
//     stores: a failure on the L2 leaves the L1 already mutated.
//
// ChainStore is itself a Store, so it can be wrapped by Cache[T] like any
// other backend.
type ChainStore struct {
	stores []Store
}

// NewChain returns a ChainStore that consults the given stores in the order
// they are provided. The first store is the most-preferred (fastest) tier;
// subsequent stores are progressively slower and more durable.
func NewChain(stores ...Store) *ChainStore {
	return &ChainStore{stores: stores}
}

// Get returns the Entry for key by consulting every store in order until
// one of them produces a value. Stores that missed are back-filled with the
// hit value; the original ExpiresAt and Tags are preserved through
// entryOpts so the back-fill does not extend the lifetime of the entry.
func (c *ChainStore) Get(ctx context.Context, key string) (Entry, error) {
	for i, s := range c.stores {
		entry, err := s.Get(ctx, key)
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				continue
			}
			return Entry{}, err
		}
		// Populate the faster (preceding) tiers with the original TTL and tags.
		opts := entryOpts(entry)
		for _, prev := range c.stores[:i] {
			_ = prev.Set(ctx, key, entry.Value, opts...)
		}
		return entry, nil
	}
	return Entry{}, ErrNotFound
}

// GetMany behaves like Get, but for a batch. Missing keys are forwarded to
// the next store; keys that are still missing after the last store are
// simply omitted from the result map.
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

// Set writes value to every store in order. The first error stops the
// propagation and is returned. The operation is not atomic across stores.
func (c *ChainStore) Set(ctx context.Context, key string, value any, opts ...Option) error {
	for _, s := range c.stores {
		if err := s.Set(ctx, key, value, opts...); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes key from every store in order. The first error stops the
// propagation.
func (c *ChainStore) Delete(ctx context.Context, key string) error {
	for _, s := range c.stores {
		if err := s.Delete(ctx, key); err != nil {
			return err
		}
	}
	return nil
}

// DeleteMany removes the given keys from every store in order.
func (c *ChainStore) DeleteMany(ctx context.Context, keys []string) error {
	for _, s := range c.stores {
		if err := s.DeleteMany(ctx, keys); err != nil {
			return err
		}
	}
	return nil
}

// Clear empties every store in order.
func (c *ChainStore) Clear(ctx context.Context) error {
	for _, s := range c.stores {
		if err := s.Clear(ctx); err != nil {
			return err
		}
	}
	return nil
}

// DeleteByTag invalidates every entry tagged with tag in every store. The
// first error stops the propagation, so the operation is not atomic across
// stores: a failure on a later tier leaves earlier tiers already mutated
// (the same caveat applies to Clear and Set).
func (c *ChainStore) DeleteByTag(ctx context.Context, tag string) error {
	for _, s := range c.stores {
		if err := s.DeleteByTag(ctx, tag); err != nil {
			return err
		}
	}
	return nil
}

// Close releases resources held by every store in order. It stops at the
// first error.
func (c *ChainStore) Close() error {
	for _, s := range c.stores {
		if err := s.Close(); err != nil {
			return err
		}
	}
	return nil
}

// entryOpts converts the metadata stored in an Entry back into the Options
// that a Store.Set call would consume. It is used by ChainStore to preserve
// TTL and tags when back-filling faster tiers from a slower one.
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
