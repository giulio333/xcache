package xcache

import (
	"context"
	"errors"
)

// ErrNotFound is returned when a key is not found or has expired.
var ErrNotFound = errors.New("xcache: key not found")

type Store interface {
	Get(ctx context.Context, key string) (any, error)
	Set(ctx context.Context, key string, value any, opts ...Option) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	Close() error
	GetMany(ctx context.Context, keys []string) (map[string]any, error)
	DeleteMany(ctx context.Context, keys []string) error
}

type Cache[T any] interface {
	Get(ctx context.Context, key string) (T, error)
	Set(ctx context.Context, key string, value T, opts ...Option) error
	Delete(ctx context.Context, key string) error
	GetOrLoad(ctx context.Context, key string, loader func(ctx context.Context) (T, error), opts ...Option) (T, error)
	GetMany(ctx context.Context, keys []string) (map[string]T, error)
	DeleteMany(ctx context.Context, keys []string) error
}
