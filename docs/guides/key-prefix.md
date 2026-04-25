# Prefisso delle chiavi

`WithPrefix` è un'opzione di costruzione per `Cache[T]` che antepone automaticamente una stringa fissa a ogni chiave prima che raggiunga lo `Store`. Il prefisso è completamente trasparente al chiamante: il codice applicativo lavora sempre con chiavi corte e leggibili.

## Setup

```go title="main.go"
import (
    "github.com/giulio333/xcache"
    "github.com/giulio333/xcache/store/memory"
)

store := memory.NewStore()
defer store.Close()

userCache  := xcache.New[User](store, xcache.WithPrefix("users:"))
orderCache := xcache.New[Order](store, xcache.WithPrefix("orders:"))
```

Le due cache condividono lo stesso `Store` senza collisioni: `"1"` viene salvato come `"users:1"` o `"orders:1"` a seconda di quale cache scrive.

## Trasparente su tutte le operazioni

Il prefisso viene aggiunto (e rimosso) automaticamente su ogni metodo.

| Chiamata | Chiave inviata allo Store |
|---|---|
| `Get(ctx, "1")` | `"users:1"` |
| `Set(ctx, "1", u)` | `"users:1"` |
| `Delete(ctx, "1")` | `"users:1"` |
| `GetMany(ctx, ["1","2"])` | `["users:1","users:2"]` |
| `DeleteMany(ctx, ["1"])` | `["users:1"]` |
| `GetOrLoad(ctx, "1", ...)` | chiave singleflight: `"users:1"` |

`GetMany` rimuove il prefisso dalle chiavi della mappa restituita, quindi il chiamante riceve sempre `{"1": ..., "2": ...}`, mai `{"users:1": ...}`.

## Store condiviso, namespace isolati

`WithPrefix` è il modo standard per condividere un singolo `Store` tra più tipi di dominio senza collisioni di chiavi:

```go title="main.go"
store := memory.NewStore()

utenti   := xcache.New[User](store,    xcache.WithPrefix("u:"))
prodotti := xcache.New[Product](store, xcache.WithPrefix("p:"))
sessioni := xcache.New[Session](store, xcache.WithPrefix("s:"))
```

## Con ChainStore e middleware

`WithPrefix` vive su `Cache[T]`, non su `Store`, quindi si compone liberamente con `ChainStore` e qualsiasi middleware:

```go title="main.go"
chain := xcache.NewChain(l1, l2)
cache := xcache.New[User](
    logging.Wrap(chain, logger),
    xcache.WithPrefix("users:"),
)
```

## Prefisso vuoto

Un prefisso vuoto (il default) è un no-op — nessun overhead, nessuna trasformazione delle chiavi.

---

*[TTL]: Time To Live
*[L1]: Layer 1 — cache veloce in memoria
*[L2]: Layer 2 — cache distribuita
