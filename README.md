# XCache

> **WIP** — work in progress

A type-safe, zero-config Go caching library with pluggable backends.

```go
// One line to start
cache := xcache.New[User](memory.NewStore())

// Scale to Redis when needed
l1 := memory.NewStore(memory.WithShards(64))
l2 := redis.NewStore(redisClient)
userCache := xcache.New[User](xcache.NewChain(l1, l2))

// Cache stampede protection built-in
user, err := userCache.GetOrLoad(ctx, "user:123", func(ctx context.Context) (User, error) {
    return db.FindUserByID(123)
}, xcache.WithTTL(10*time.Minute))

// Group invalidation by tag
_ = userCache.Set(ctx, "user:123", u, xcache.WithTags("users"))
_ = userCache.DeleteByTag(ctx, "users")
```

## Features

- **Type-safe generics API** — no type assertions, ever
- **Zero-config** — works in-memory with one line
- **Pluggable backends** — Memory (built-in), Redis, Memcached
- **Chain cache** — L1 (memory) → L2 (Redis) fallback with automatic
  back-fill that preserves the original TTL and tags
- **Singleflight** — prevents cache stampede on `GetOrLoad`
- **Tag-based invalidation** — group keys with `WithTags`, drop them
  together with `DeleteByTag`
- **Observability** — Prometheus decorator (planned)

## API at a glance

```go
type Cache[T any] interface {
    Get(ctx, key) (T, error)
    GetMany(ctx, keys) (map[string]T, error)
    Set(ctx, key, value, opts...) error
    Delete(ctx, key) error
    DeleteMany(ctx, keys) error
    DeleteByTag(ctx, tag) error
    Clear(ctx) error
    GetOrLoad(ctx, key, loader, opts...) (T, error)
}
```

Errors:

- `ErrNotFound` — returned by `Get` for missing or expired keys
- `ErrNotSupported` — returned by backends that do not implement an
  optional operation (for example, a Redis store without a tag index)

Options for `Set` and `GetOrLoad`:

- `WithTTL(d time.Duration)` — entry deadline (`0` = no expiration)
- `WithTags(tags ...string)` — labels for group invalidation

## Testing

```bash
# Unit tests (verbose)
go test -v ./...

# With race detector
go test -v -race ./...

# Benchmarks (all cores: 1, 2, 4, 8)
# Linux / macOS
go test -bench=. -benchmem -cpu=1,2,4,8 ./...
# Windows (PowerShell)
go test --% -bench=. -benchmem -cpu=1,2,4,8 ./...

# Specific package
go test -v ./store/memory/...
```

## Requirements

Go 1.21+

## License

MIT
