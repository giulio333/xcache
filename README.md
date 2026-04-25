# XCache

Type-safe Go caching library with pluggable backends.

```bash
go get github.com/giulio333/xcache
```

Requires Go 1.21+.

## Quick start

```go
store := memory.NewStore()
defer store.Close()

cache := xcache.New[User](store)

// Write
cache.Set(ctx, "user:1", User{ID: 1, Name: "Alice"}, xcache.WithTTL(5*time.Minute))

// Read
user, err := cache.Get(ctx, "user:1")
if errors.Is(err, xcache.ErrNotFound) { ... }

// Read with DB fallback — stampede-safe
user, err = cache.GetOrLoad(ctx, "user:1", func(ctx context.Context) (User, error) {
    return db.FindUser(1)
}, xcache.WithTTL(5*time.Minute))

// Group invalidation
cache.Set(ctx, "user:2", u2, xcache.WithTags("users"))
cache.DeleteByTag(ctx, "users")

// L1 → L2 chain
l1 := memory.NewStore()
l2 := redis.NewStore(client)
cache = xcache.New[User](xcache.NewChain(l1, l2))

// Middleware
store = logging.Wrap(timeout.Wrap(memory.NewStore(), 50*time.Millisecond), logger)
```

## Features

- **Type-safe generics API** — no type assertions, ever
- **Pluggable backends** — Memory (built-in), Redis, or bring your own
- **Cache stampede protection** — singleflight built into `GetOrLoad`
- **Tag-based invalidation** — group keys with `WithTags`, drop them with `DeleteByTag`
- **Chain cache** — L1/L2 fallback with automatic backfill preserving TTL and tags
- **Middleware** — logging, read-only, per-operation timeout, backend-agnostic metrics

## Documentation

Full documentation at **[giulio333.github.io/xcache](https://giulio333.github.io/xcache)**.

- [Getting started](https://giulio333.github.io/xcache/guides/getting-started/)
- [Concepts](https://giulio333.github.io/xcache/architecture/overview/)
- [API reference](https://giulio333.github.io/xcache/reference/cache/)

## License

MIT
