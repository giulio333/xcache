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
- **Middleware** — logging, read-only, per-operation timeout, backend-agnostic metrics

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

Construction-time options for `New`:

- `WithPrefix(prefix string)` — prepend a fixed string to every key before it
  reaches the Store. Callers always use short keys; the translation is
  transparent. An empty prefix is a no-op.

  ```go
  userCache := xcache.New[User](store, xcache.WithPrefix("users:"))
  _ = userCache.Set(ctx, "1", u)         // stored as "users:1"
  u, _ := userCache.Get(ctx, "1")        // reads "users:1", returns User
  result, _ := userCache.GetMany(ctx, []string{"1", "2"})
  // result keys are "1" and "2" (prefix stripped in output)
  ```

## Middleware

Middlewares wrap any `Store` and are composed by nesting:

```go
store := logging.Wrap(
    timeout.Wrap(
        metrics.Wrap(memory.NewStore(), rec),
        5*time.Second,
    ),
    logger,
)
cache := xcache.New[User](store)
```

### Logging

Structured `log/slog` entries for every operation. `ErrNotFound` and `ErrNotSupported` are logged at `DEBUG`; real errors at `ERROR`. Pass `nil` to fall back to `slog.Default()`.

```go
store := logging.Wrap(memory.NewStore(), logger)
```

### Read-only

Blocks all write operations (`Set`, `Delete`, `DeleteMany`, `DeleteByTag`, `Clear`) and returns the exported `ErrReadOnly` sentinel. Useful for staging environments or shared read-only caches.

```go
store := readonly.Wrap(memory.NewStore())
// store.Set(...) → readonly.ErrReadOnly
```

### Timeout

Wraps every operation with `context.WithTimeout`, protecting against slow backends. `d <= 0` is a no-op.

```go
store := timeout.Wrap(memory.NewStore(), 200*time.Millisecond)
```

### Metrics

Exposes a `Recorder` interface so you can plug in any metrics backend (Prometheus, StatsD, DataDog) without the middleware carrying external dependencies.

```go
type Recorder interface {
    RecordHit(op string)
    RecordMiss(op string)
    RecordError(op string)
    RecordDuration(op string, d time.Duration)
}

store := metrics.Wrap(memory.NewStore(), myRecorder)
```

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
