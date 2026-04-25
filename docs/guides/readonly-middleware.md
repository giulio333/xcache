# Readonly middleware

`readonly.Wrap` decora qualsiasi `Store` bloccando ogni operazione di scrittura. Le letture sono delegate al backend sottostante; i tentativi di scrittura restituiscono immediatamente `ErrReadOnly` senza toccare lo store.

## Setup

```go title="main.go"
import (
    "github.com/giulio333/xcache"
    "github.com/giulio333/xcache/middleware/readonly"
    "github.com/giulio333/xcache/store/memory"
)

base := memory.NewStore()
ro := readonly.Wrap(base)
defer ro.Close()

cache := xcache.New[Product](ro)
```

## Comportamento per operazione

| Operazione | Comportamento |
|---|---|
| `Get`, `GetMany` | Delegate al next store |
| `Set` | Restituisce `ErrReadOnly` |
| `Delete`, `DeleteMany` | Restituisce `ErrReadOnly` |
| `DeleteByTag` | Restituisce `ErrReadOnly` |
| `Clear` | Restituisce `ErrReadOnly` |
| `Close` | Delegato al next store |

## Rilevare ErrReadOnly

`ErrReadOnly` è un sentinel esportato: usare `errors.Is` per distinguerlo da altri errori.

```go title="main.go"
err := cache.Set(ctx, "key", value)
if errors.Is(err, readonly.ErrReadOnly) {
    // scrittura rifiutata — il chiamante decide cosa fare
}
```

## Caso d'uso: cache condivisa in staging

Un pattern comune è seminare uno store con dati da produzione, avvolgerlo con `readonly.Wrap` e condividerlo tra più servizi di staging. Nessun servizio può alterare accidentalmente i dati condivisi.

```go title="main.go"
// Seeding una volta sola all'avvio
base := memory.NewStore()
seedFromProduction(ctx, base)

// Tutti i servizi ricevono una vista in sola lettura
sharedCache := readonly.Wrap(base)

serviceA := xcache.New[Order](sharedCache)
serviceB := xcache.New[Product](sharedCache)
```

## Composizione con altri middleware

Il middleware si aggancia a livello `Store` e si compone per annidamento con gli altri decorator.

```go title="main.go"
// Logging + readonly: le operazioni vengono loggale anche se rifiutate
store := logging.Wrap(
    readonly.Wrap(memory.NewStore()),
    logger,
)
```

!!! note
    Applicando `logging.Wrap` sopra `readonly.Wrap`, ogni tentativo di scrittura viene loggato come errore (`ErrReadOnly`). Invertire l'ordine se si vuole silenziare questi log.

---

*[TTL]: Time To Live
