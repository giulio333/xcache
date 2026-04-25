// Package timeout provides a Store middleware that enforces a per-operation
// deadline on every context-bearing call.
//
// Usage:
//
//	base := memory.NewStore()
//	ts := timeout.Wrap(base, 50*time.Millisecond)
//	defer ts.Close()
//
// Each operation receives a child context created with context.WithTimeout
// before the call is delegated to the underlying Store. When the deadline
// fires, the store returns context.DeadlineExceeded — the standard Go error.
//
// Passing d <= 0 is a no-op: contexts are forwarded unchanged.
package timeout

import (
	"context"
	"time"

	xcache "github.com/giulio333/xcache"
)

// timeoutStore wraps a Store and applies a fixed timeout to every operation.
type timeoutStore struct {
	next xcache.Store
	d    time.Duration
}

// Wrap returns a Store that enforces d as a per-call deadline on every
// context-bearing operation. If d <= 0, the store is returned unchanged.
func Wrap(next xcache.Store, d time.Duration) xcache.Store {
	if d <= 0 {
		return next
	}
	return &timeoutStore{next: next, d: d}
}

// withTimeout derives a child context limited to s.d and returns the cancel
// function. Callers must always invoke cancel to avoid context leaks.
func (s *timeoutStore) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, s.d)
}

func (s *timeoutStore) Get(ctx context.Context, key string) (xcache.Entry, error) {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	return s.next.Get(ctx, key)
}

func (s *timeoutStore) Set(ctx context.Context, key string, value any, opts ...xcache.Option) error {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	return s.next.Set(ctx, key, value, opts...)
}

func (s *timeoutStore) Delete(ctx context.Context, key string) error {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	return s.next.Delete(ctx, key)
}

func (s *timeoutStore) Clear(ctx context.Context) error {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	return s.next.Clear(ctx)
}

func (s *timeoutStore) GetMany(ctx context.Context, keys []string) (map[string]xcache.Entry, error) {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	return s.next.GetMany(ctx, keys)
}

func (s *timeoutStore) DeleteMany(ctx context.Context, keys []string) error {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	return s.next.DeleteMany(ctx, keys)
}

func (s *timeoutStore) DeleteByTag(ctx context.Context, tag string) error {
	ctx, cancel := s.withTimeout(ctx)
	defer cancel()
	return s.next.DeleteByTag(ctx, tag)
}

// Close does not carry a context — it is delegated directly without a timeout.
func (s *timeoutStore) Close() error {
	return s.next.Close()
}
