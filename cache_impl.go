package xcache

import (
	"context"
	"fmt"

	"golang.org/x/sync/singleflight"
)

// cache is the concrete generic implementation of Cache[T]. It is a thin
// adapter on top of a Store: it converts between the typed world (T) and the
// untyped world (any) used by Store, and it owns the singleflight group that
// powers GetOrLoad.
//
// The struct is unexported on purpose: callers obtain a Cache[T] through
// New, never instantiate cache directly.
type cache[T any] struct {
	store Store
	group singleflight.Group
}

// New returns a Cache[T] backed by the given Store. The same Store may back
// caches of different T types as long as their key namespaces do not
// collide; a type mismatch on Get is reported as an error rather than a
// panic.
func New[T any](store Store) Cache[T] {
	return &cache[T]{store: store}
}

// Get returns the value associated with key.
//
// It returns ErrNotFound when the key is missing or expired, and a typed
// "type mismatch" error when the value found in the store is not assignable
// to T (which usually indicates that the same Store is being used by caches
// of different T with overlapping keys).
func (c *cache[T]) Get(ctx context.Context, key string) (T, error) {
	entry, err := c.store.Get(ctx, key)
	if err != nil {
		var zero T
		return zero, err
	}
	val, ok := entry.Value.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("xcache: type mismatch for key %q", key)
	}
	return val, nil
}

// Set stores value under key with the given Options.
func (c *cache[T]) Set(ctx context.Context, key string, value T, opts ...Option) error {
	return c.store.Set(ctx, key, value, opts...)
}

// Delete removes key from the underlying Store. It is a no-op if the key
// does not exist.
func (c *cache[T]) Delete(ctx context.Context, key string) error {
	return c.store.Delete(ctx, key)
}

// Clear removes every key from the underlying Store.
func (c *cache[T]) Clear(ctx context.Context) error {
	return c.store.Clear(ctx)
}

// GetMany returns the values found for the given keys. Missing or expired
// keys are silently omitted. A type mismatch on any returned entry causes
// the whole call to fail with a typed error.
func (c *cache[T]) GetMany(ctx context.Context, keys []string) (map[string]T, error) {
	raw, err := c.store.GetMany(ctx, keys)
	if err != nil {
		return nil, err
	}
	result := make(map[string]T, len(raw))
	for k, entry := range raw {
		val, ok := entry.Value.(T)
		if !ok {
			return nil, fmt.Errorf("xcache: type mismatch for key %q", k)
		}
		result[k] = val
	}
	return result, nil
}

// DeleteMany removes a batch of keys in a single call.
func (c *cache[T]) DeleteMany(ctx context.Context, keys []string) error {
	return c.store.DeleteMany(ctx, keys)
}

// DeleteByTag removes every entry that was stored with the given tag. It
// delegates to the underlying Store; backends that do not maintain a tag
// index return ErrNotSupported.
func (c *cache[T]) DeleteByTag(ctx context.Context, tag string) error {
	return c.store.DeleteByTag(ctx, tag)
}

// GetOrLoad returns the cached value for key or invokes loader on miss.
//
// Behaviour:
//
//  1. The cache is queried first. On hit the value is returned immediately.
//  2. On miss, loader is invoked exactly once per key under concurrent
//     traffic (singleflight). All concurrent callers for the same key share
//     the same loader result, so an upstream system (typically a database)
//     is shielded from cache stampedes.
//  3. The loader's result is stored back into the cache with the given
//     Options. A failure of that write is intentionally ignored: the
//     caller still receives the loaded value, and the next request will
//     simply repeat the load.
//
// loader receives the same ctx the caller passed to GetOrLoad, so it
// participates in cancellation and deadlines.
func (c *cache[T]) GetOrLoad(ctx context.Context, key string, loader func(context.Context) (T, error), opts ...Option) (T, error) {
	if val, err := c.Get(ctx, key); err == nil {
		return val, nil
	}

	v, err, _ := c.group.Do(key, func() (any, error) {
		return loader(ctx)
	})
	if err != nil {
		var zero T
		return zero, err
	}

	result, ok := v.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("xcache: loader returned unexpected type for key %q", key)
	}
	_ = c.store.Set(ctx, key, result, opts...)
	return result, nil
}
