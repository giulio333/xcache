package xcache

import (
	"context"
	"fmt"

	"golang.org/x/sync/singleflight"
)

type cache[T any] struct {
	store Store
	group singleflight.Group
}

func New[T any](store Store) Cache[T] {
	return &cache[T]{store: store}
}

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

func (c *cache[T]) Set(ctx context.Context, key string, value T, opts ...Option) error {
	return c.store.Set(ctx, key, value, opts...)
}

func (c *cache[T]) Delete(ctx context.Context, key string) error {
	return c.store.Delete(ctx, key)
}

func (c *cache[T]) Clear(ctx context.Context) error {
	return c.store.Clear(ctx)
}

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

func (c *cache[T]) DeleteMany(ctx context.Context, keys []string) error {
	return c.store.DeleteMany(ctx, keys)
}

func (c *cache[T]) DeleteByTag(ctx context.Context, tag string) error {
	return c.store.DeleteByTag(ctx, tag)
}

func (c *cache[T]) GetOrLoad(ctx context.Context, key string, loader func(context.Context) (T, error), opts ...Option) (T, error) {
	// Prima prova dalla cache
	if val, err := c.Get(ctx, key); err == nil {
		return val, nil
	}

	// Singleflight: una sola chiamata al loader per chiave sotto carico concorrente
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
