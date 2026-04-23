package xcache

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a key is not found or has expired.
var ErrNotFound = errors.New("xcache: key not found")

// Entry holds a cached value together with its storage metadata.
// ExpiresAt is zero when the entry has no expiration.
type Entry struct {
	Value     any
	ExpiresAt time.Time
	Tags      []string
}

// RemainingTTL returns the time left before expiry, or 0 if the entry never expires.
func (e Entry) RemainingTTL() time.Duration {
	if e.ExpiresAt.IsZero() {
		return 0
	}
	if d := time.Until(e.ExpiresAt); d > 0 {
		return d
	}
	return 0
}

type Store interface {
	Get(ctx context.Context, key string) (Entry, error)
	Set(ctx context.Context, key string, value any, opts ...Option) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	Close() error
	GetMany(ctx context.Context, keys []string) (map[string]Entry, error)
	DeleteMany(ctx context.Context, keys []string) error
}

type Cache[T any] interface {
	Get(ctx context.Context, key string) (T, error)
	Set(ctx context.Context, key string, value T, opts ...Option) error
	Delete(ctx context.Context, key string) error
	Clear(ctx context.Context) error
	GetOrLoad(ctx context.Context, key string, loader func(ctx context.Context) (T, error), opts ...Option) (T, error)
	GetMany(ctx context.Context, keys []string) (map[string]T, error)
	DeleteMany(ctx context.Context, keys []string) error
}
