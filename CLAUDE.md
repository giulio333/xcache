# XCache — Claude Code Instructions

## Progetto

Libreria Go generics-first per caching type-safe con supporto multi-backend (memory, Redis, Memcached).
Pubblicata su GitHub come progetto open-source.

## Architettura

```
xcache/
├── cache.go          # Interfacce pubbliche (Cache[T], Store, Options)
├── chain.go          # ChainStore: decorator L1→L2
├── store/
│   ├── memory/       # MemoryStore: in-memory con sharding, TTL attivo/passivo
│   └── redis/        # RedisStore: backend Redis con object pooling
├── middleware/
│   └── prometheus/   # Decorator metriche (Wrap)
└── internal/
    └── singleflight/ # Wrapper golang.org/x/sync/singleflight per GetOrLoad
```

## Pattern adottati

- **Strategy**: backend intercambiabili tramite interfaccia `Store`
- **Functional Options**: configurazioni opzionali via `Option func(*Options)`
- **Decorator/Middleware**: Chain cache, Loadable cache, Observability come livelli
- **Singleflight**: obbligatorio in `GetOrLoad` — una sola query al DB per chiavi concorrenti

## Regole di sviluppo

- Go 1.21+ — usare generics, evitare `interface{}` nell'API pubblica
- `Cache[T any]` è type-safe: nessun type assertion esposto all'utente
- `Store` usa `any` internamente (gestisce bytes/interfacce); `Cache[T]` wrappa con generics
- Object pooling (`sync.Pool`) per i buffer di serializzazione verso Redis
- Eviction memoria: **passiva** (al Get) + **attiva** (goroutine background sweep)
- Integration test preferiti ai mock — testare con Redis reale via `testcontainers-go`

## Dipendenze ammesse

- `golang.org/x/sync` — singleflight
- `github.com/redis/go-redis/v9` — client Redis
- `github.com/prometheus/client_golang` — metriche (solo nel package middleware/prometheus)
- `github.com/testcontainers/testcontainers-go` — test di integrazione

## Attenzione GitHub pubblico

- Nessun segreto, credenziale o endpoint interno nei test o commenti
- I test di integrazione che richiedono Redis usano `testcontainers-go` (nessun hardcoded host)
- Esempi in `examples/` devono compilare e girare senza configurazione esterna
- Licenza: MIT (da aggiungere)
