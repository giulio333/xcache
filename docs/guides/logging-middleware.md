# Logging middleware

`logging.Wrap` decora qualsiasi `Store` con log strutturati via `log/slog`. Ogni operazione emette un'entry con i campi `op`, `key` (o `keys` per le operazioni batch) e `duration`.

## Setup

```go title="main.go"
import (
    "log/slog"
    "os"

    "github.com/giulio333/xcache"
    "github.com/giulio333/xcache/middleware/logging"
    "github.com/giulio333/xcache/store/memory"
)

logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
}))

store := logging.Wrap(memory.NewStore(), logger)
defer store.Close()

cache := xcache.New[User](store)
```

Passare `nil` come logger usa `slog.Default()`.

## Livelli di log

| Caso | Livello | Messaggio |
|---|---|---|
| Operazione riuscita | `DEBUG` | `xcache op` |
| Chiave assente (`ErrNotFound`) | `DEBUG` | `xcache miss` |
| `DeleteByTag` non supportato (`ErrNotSupported`) | `DEBUG` | `xcache not supported` |
| Errore di sistema | `ERROR` | `xcache error` |

`ErrNotFound` non è un errore — viene loggato a `DEBUG` per non inquinare i log in produzione.

## Composizione con ChainStore

Il middleware si aggancia a livello `Store`, quindi si può applicare a ogni tier individualmente o alla chain nel suo insieme.

```go title="main.go"
// metriche per-tier: ogni livello ha i propri log
chain := xcache.NewChain(
    logging.Wrap(l1, logger.With("tier", "L1")),
    logging.Wrap(l2, logger.With("tier", "L2")),
)

// oppure: vista unica a livello applicativo
chain := logging.Wrap(xcache.NewChain(l1, l2), logger)
```

## Composizione con altri middleware

```go title="main.go"
store := logging.Wrap(
    memory.NewStore(),
    logger,
)
// In futuro: prometheus.Wrap(logging.Wrap(...), metrics)
```

Ogni middleware è una funzione `func(Store) Store` — la composizione è per annidamento, nessun tipo aggiuntivo nel core.

---

*[TTL]: Time To Live
*[L1]: Layer 1 — cache veloce in memoria
*[L2]: Layer 2 — cache distribuita
