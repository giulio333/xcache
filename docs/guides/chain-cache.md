# Chain cache (L1 → L2)

`ChainStore` mette in cascata più backend: il primo che risponde vince, gli store precedenti vengono ripopolati automaticamente preservando TTL e tag originali.

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
       └─ L2.Get → hit → backfill L1 con TTL e tag → ritorna valore
```

Alla richiesta successiva L1 ha già la chiave — L2 non viene toccato.

**Scrittura, cancellazione e `DeleteByTag`** propagano a tutti gli store in ordine. La prima operazione che fallisce ferma la propagazione.

## TTL e tag preservati nel backfill

`ChainStore.Get` legge l'`Entry` dal tier che ha risposto e ricostruisce le `Option` (`WithTTL` con il TTL residuo, `WithTags` con i tag originali) prima di scrivere sui tier precedenti.

Conseguenze:

- La chiave in L1 scade insieme a quella in L2 — non rimane "viva per sempre" oltre il TTL originale.
- `DeleteByTag` continua a funzionare anche sui tier ripopolati: i tag sono presenti su entrambi i livelli.

## `GetOrLoad` con chain

```go title="main.go"
user, err := cache.GetOrLoad(ctx, "user:1", func(ctx context.Context) (User, error) {
    return db.FindUserByID(1) // (1)
}, xcache.WithTTL(5*time.Minute), xcache.WithTags("users"))
```

1. Chiamato solo se la chiave manca in tutti gli store della chain. Il risultato viene scritto in L1 e L2 con TTL di 5 minuti e tag `users`.

## Atomicità

!!! warning "Non atomica tra tier"
    `Set`, `Delete`, `DeleteMany`, `DeleteByTag` e `Clear` propagano in ordine e si fermano al primo errore. Se la propagazione fallisce su L2, L1 risulta già mutato. Per invalidazioni critiche, considerare retry o reconciliation lato applicativo.

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
