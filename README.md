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
user, err := userCache.GetOrLoad(ctx, "user:123", func() (User, error) {
    return db.FindUserByID(123)
}, xcache.WithTTL(10*time.Minute))
```

## Features

- **Type-safe generics API** — no type assertions, ever
- **Zero-config** — works in-memory with one line
- **Pluggable backends** — Memory, Redis, Memcached
- **Chain cache** — L1 (memory) → L2 (Redis) fallback
- **Singleflight** — prevents cache stampede on `GetOrLoad`
- **Observability** — Prometheus decorator

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
