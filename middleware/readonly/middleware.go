// Package readonly provides a Store middleware that blocks every write
// operation and returns ErrReadOnly to the caller.
//
// Usage:
//
//	base := memory.NewStore()
//	ro := readonly.Wrap(base)
//	defer ro.Close()
//
// Read operations (Get, GetMany) are delegated to the underlying store
// unchanged. Write operations (Set, Delete, DeleteMany, DeleteByTag, Clear)
// return ErrReadOnly immediately without touching the store. Close is
// delegated so that the underlying store is released properly.
package readonly

import (
	"context"
	"errors"

	xcache "github.com/giulio333/xcache"
)

// ErrReadOnly is returned by any mutating operation on a read-only store.
// Callers can detect it with errors.Is.
var ErrReadOnly = errors.New("xcache: store is read-only")

// readonlyStore wraps a Store and rejects all write operations.
type readonlyStore struct {
	next xcache.Store
}

// Wrap returns a Store that delegates Get and GetMany to next and rejects
// every write operation with ErrReadOnly.
func Wrap(next xcache.Store) xcache.Store {
	return &readonlyStore{next: next}
}

func (s *readonlyStore) Get(ctx context.Context, key string) (xcache.Entry, error) {
	return s.next.Get(ctx, key)
}

func (s *readonlyStore) GetMany(ctx context.Context, keys []string) (map[string]xcache.Entry, error) {
	return s.next.GetMany(ctx, keys)
}

func (s *readonlyStore) Set(_ context.Context, _ string, _ any, _ ...xcache.Option) error {
	return ErrReadOnly
}

func (s *readonlyStore) Delete(_ context.Context, _ string) error {
	return ErrReadOnly
}

func (s *readonlyStore) DeleteMany(_ context.Context, _ []string) error {
	return ErrReadOnly
}

func (s *readonlyStore) DeleteByTag(_ context.Context, _ string) error {
	return ErrReadOnly
}

func (s *readonlyStore) Clear(_ context.Context) error {
	return ErrReadOnly
}

func (s *readonlyStore) Close() error {
	return s.next.Close()
}
