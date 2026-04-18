# Chain cache (L1 → L2)

`ChainStore` mette in cascata più backend: il primo che risponde vince, gli store precedenti vengono ripopolati automaticamente.

## Setup

```go title="main.go"
import (
    "github.com/giulio333/xcache"
    "github.com/giulio333/xcache/store/memory"
)

l1 := memory.NewStore()                          // in-process, velocissimo
l2 := memory.NewStore(memory.WithShards(128))    // secondo livello (es. Redis in produzione)
defer l1.Close()
defer l2.Close()

cache := xcache.New[User](xcache.NewChain(l1, l2))
```

## Come funziona

**Lettura:**

```
Get("user:1")
  └─ L1.Get → miss
       └─ L2.Get → hit → backfill L1 → ritorna valore
```

Alla richiesta successiva L1 ha già la chiave — L2 non viene toccato.

**Scrittura e cancellazione** propagano a tutti gli store in ordine.

## Backfill senza TTL

!!! warning "TTL non propagato nel backfill"
    Quando `Get` ripopola L1 da L2, usa le `Option` di default (nessun TTL). La chiave in L1 non scade mai finché non viene cancellata o lo store viene svuotato.

    Se serve propagare il TTL originale, usa `GetOrLoad` con un TTL esplicito — la chiave verrà scritta in entrambi gli store con il TTL corretto.

## `GetOrLoad` con chain

```go title="main.go"
user, err := cache.GetOrLoad(ctx, "user:1", func(ctx context.Context) (User, error) {
    return db.FindUserByID(1) // (1)
}, xcache.WithTTL(5*time.Minute))
```

1. Chiamato solo se la chiave manca in tutti gli store della chain. Il risultato viene scritto in L1 e L2 con TTL di 5 minuti.

## Tre o più livelli

`NewChain` accetta un numero arbitrario di store:

```go title="main.go"
xcache.NewChain(l1, l2, l3)
```

La ricerca scorre da sinistra a destra. Il backfill popola tutti gli store a sinistra dello store che ha risposto.

---

*[TTL]: Time To Live
*[L1]: Layer 1 — cache veloce in memoria
*[L2]: Layer 2 — cache distribuita
