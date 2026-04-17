# XCache

**XCache** è una libreria Go per il caching type-safe con supporto multi-backend.

---

## Obiettivi

- **Type-safety assoluta** — inserisci una `User`, recuperi una `User`. Nessun type assertion esposto.
- **Zero-config** — funziona in memoria locale con una riga. Scala su Redis quando necessario.
- **Estendibilità trasparente** — metriche, chain cache, singleflight sono layer componibili, non hardcoded nel core.

## Avvio rapido

```go
store := memory.NewStore()
cache := xcache.New[User](store)
defer store.Close()

// Scrittura
cache.Set(ctx, "user:1", User{ID: 1, Name: "Alice"}, xcache.WithTTL(10*time.Minute))

// Lettura
user, err := cache.Get(ctx, "user:1")
if errors.Is(err, xcache.ErrNotFound) { ... }

// Lettura con fallback al DB — cache stampede safe
user, err = cache.GetOrLoad(ctx, "user:1", func(ctx context.Context) (User, error) {
    return db.FindUserByID(1)
}, xcache.WithTTL(10*time.Minute))
```

## Setup avanzato: chain L1 → L2

```go
l1 := memory.NewStore(memory.WithShards(64))
l2 := redis.NewStore(redisClient)

cache := xcache.New[User](xcache.NewChain(l1, l2))
```

---

## Struttura del progetto

```
xcache/
├── cache.go          # Interfacce pubbliche: Store, Cache[T]
├── cache_impl.go     # Implementazione concreta di Cache[T]
├── chain.go          # ChainStore: decorator L1→L2
├── options.go        # Option, WithTTL, WithTags
└── store/
    ├── memory/       # MemoryStore: in-memory con sharding e TTL
    └── redis/        # RedisStore (coming soon)
```

---

*[L1]: Layer 1 — cache veloce in memoria
*[L2]: Layer 2 — cache distribuita (Redis)
