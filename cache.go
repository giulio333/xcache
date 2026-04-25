// Package xcache provides a type-safe, generics-first caching library with
// pluggable backends.
//
// The package is organised in three layers:
//
//   - Cache[T] is the generic, type-safe API exposed to user code. It is
//     parameterised on the cached value type T so that callers never have to
//     perform a type assertion.
//   - Store is the backend contract. Implementations work with any internally
//     and are responsible for serialisation, eviction and persistence. The same
//     Store instance can be shared by multiple Cache[T] of different T.
//   - ChainStore is a decorator that composes several Store instances into a
//     read-through cascade (typically L1 in-memory → L2 Redis).
//
// A minimal example:
//
//	store := memory.NewStore()
//	defer store.Close()
//
//	cache := xcache.New[User](store)
//	_ = cache.Set(ctx, "user:1", User{ID: 1}, xcache.WithTTL(5*time.Minute))
//	u, err := cache.Get(ctx, "user:1")
//
// GetOrLoad provides cache-stampede protection through an internal
// singleflight group: under concurrent traffic on a missing key the loader is
// invoked exactly once and the resulting value is shared by all waiters.
package xcache

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned by Get-style operations when the requested key does
// not exist or has expired. Callers should match it with errors.Is.
var ErrNotFound = errors.New("xcache: key not found")

// ErrNotSupported is returned when a backend does not implement an optional
// operation (for example, DeleteByTag on a backend that has no tag index).
// It allows decorators such as ChainStore to distinguish a "feature missing"
// case from a transport or data error.
var ErrNotSupported = errors.New("xcache: operation not supported by this backend")

// Entry holds a cached value together with its storage metadata. It is the
// raw record returned by Store.Get; the type-safe Cache[T] layer unwraps
// Entry.Value into T before returning it to the caller.
//
// ExpiresAt is the absolute deadline at which the entry becomes invalid; it
// is the zero value when the entry has no expiration. RemainingTTL derives
// the remaining lifetime from ExpiresAt.
//
// Tags are the labels associated with the entry and are used by DeleteByTag
// for group invalidation.
type Entry struct {
	Value     any
	ExpiresAt time.Time
	Tags      []string
}

// RemainingTTL returns the time left before the entry expires.
//
// It returns 0 if the entry has no expiration (ExpiresAt is zero) or if the
// deadline has already passed. The value is used by ChainStore to propagate
// the original TTL when writing back from a slower tier (for example L2) to
// a faster tier (for example L1).
func (e Entry) RemainingTTL() time.Duration {
	if e.ExpiresAt.IsZero() {
		return 0
	}
	if d := time.Until(e.ExpiresAt); d > 0 {
		return d
	}
	return 0
}

// Store is the contract every backend must satisfy. It is generic with
// respect to the value type: implementations operate on any and never need
// to know about the user's domain types.
//
// All methods accept a context.Context for cancellation and deadlines. Get
// must return ErrNotFound for missing or expired keys (never an empty Entry
// with a nil error). GetMany must omit missing keys from the resulting map
// rather than reporting them as errors.
type Store interface {
	// Get returns the Entry for the given key. It must return ErrNotFound
	// when the key is absent or has expired.
	Get(ctx context.Context, key string) (Entry, error)

	// Set stores value under key. The provided Options are applied through
	// ApplyOptions; supported options include WithTTL and WithTags.
	Set(ctx context.Context, key string, value any, opts ...Option) error

	// Delete removes a single key. It must not return an error when the key
	// is absent.
	Delete(ctx context.Context, key string) error

	// Clear removes all keys stored in the backend.
	Clear(ctx context.Context) error

	// Close releases backend resources (background goroutines, network
	// connections). It must be safe to call exactly once.
	Close() error

	// GetMany returns the Entries that exist for the given keys. Missing or
	// expired keys are silently omitted from the resulting map.
	GetMany(ctx context.Context, keys []string) (map[string]Entry, error)

	// DeleteMany removes a batch of keys in a single call. Backends are free
	// to optimise the operation (e.g. Redis pipeline); a default
	// implementation looping over Delete is acceptable.
	DeleteMany(ctx context.Context, keys []string) error

	// DeleteByTag removes every entry that was stored with the given tag.
	// Backends that do not maintain a tag index should return
	// ErrNotSupported.
	DeleteByTag(ctx context.Context, tag string) error
}

// Cache is the generics-first, type-safe API exposed to user code. T is the
// concrete value type stored in the cache; the implementation guarantees that
// callers never observe any or have to perform a type assertion.
//
// A Cache[T] is constructed via New and wraps a Store. The same Store can be
// shared by multiple Cache[T] of different T (for example, one for users
// and one for products), provided that key namespaces do not collide.
type Cache[T any] interface {
	// Get returns the value associated with key. It returns ErrNotFound when
	// the key is absent or expired, and a typed error when the stored value
	// is incompatible with T (which indicates a misuse of a shared Store).
	Get(ctx context.Context, key string) (T, error)

	// Set stores value under key, with the given Options.
	Set(ctx context.Context, key string, value T, opts ...Option) error

	// Delete removes a single key. It does not return an error when the key
	// is absent.
	Delete(ctx context.Context, key string) error

	// Clear removes every key currently stored in the underlying Store.
	Clear(ctx context.Context) error

	// GetOrLoad returns the cached value for key or, on miss, invokes loader
	// to produce it and stores the result with the given Options. Concurrent
	// callers for the same key share a single loader invocation
	// (singleflight), which protects the upstream from cache stampedes.
	GetOrLoad(ctx context.Context, key string, loader func(ctx context.Context) (T, error), opts ...Option) (T, error)

	// GetMany returns the values found for the given keys. Missing or
	// expired keys are silently omitted from the result map.
	GetMany(ctx context.Context, keys []string) (map[string]T, error)

	// DeleteMany removes a batch of keys.
	DeleteMany(ctx context.Context, keys []string) error

	// DeleteByTag removes every entry that was stored with the given tag.
	// It is the standard way to perform group invalidation
	// (for example, "drop all entries tagged users after a schema change").
	DeleteByTag(ctx context.Context, tag string) error
}
