# Invalidazione di gruppo (tag)

Quando più chiavi rappresentano la stessa entità logica (es. tutte le chiavi che dipendono dalla tabella `users`), invalidarle una a una è scomodo e fragile. XCache supporta l'invalidazione di gruppo tramite **tag**: si associa uno o più tag alla chiave al momento della scrittura, poi una sola chiamata `DeleteByTag` rimuove tutte le chiavi corrispondenti.

## Scrivere con tag

```go title="main.go"
cache.Set(ctx, "user:1",   u1, xcache.WithTags("users"))
cache.Set(ctx, "user:2",   u2, xcache.WithTags("users", "admin"))
cache.Set(ctx, "user:bio:1", bio, xcache.WithTags("users", "bio"))
cache.Set(ctx, "product:1", p1, xcache.WithTags("products"))
```

Ogni chiave può avere un numero arbitrario di tag.

## Invalidare per tag

```go title="main.go"
// Drop di tutti gli utenti, bio inclusa, ma non dei prodotti
cache.DeleteByTag(ctx, "users")
```

Equivale a un `Delete` di tutte le chiavi che hanno `users` tra i loro tag — ma con un solo round trip e senza che il chiamante debba conoscerle in anticipo.

!!! note "Tag inesistenti sono no-op"
    `DeleteByTag` su un tag che non esiste (mai usato, oppure già svuotato) ritorna `nil` senza errore. Idempotenza garantita.

## Sovrascrittura coerente

Un `Set` successivo con tag diversi ri-aggancia la chiave ai nuovi tag e la stacca dai vecchi:

```go title="main.go"
cache.Set(ctx, "u1", v, xcache.WithTags("v1"))
cache.Set(ctx, "u1", v, xcache.WithTags("v2")) // detach da "v1", attach a "v2"

cache.DeleteByTag(ctx, "v1") // u1 sopravvive
cache.DeleteByTag(ctx, "v2") // u1 viene rimosso
```

L'indice tag è sempre coerente con il valore corrente — non vengono lasciati riferimenti stale.

## Backend supportati

| Backend | Supporto |
|---|---|
| `memory.MemoryStore` | Nativo: indice tag basato su set per shard, O(1) per chiave |
| `redis.RedisStore` | In progress — ritornerà `xcache.ErrNotSupported` finché non implementato |

Per i backend custom che non possono mantenere un indice tag, restituire `xcache.ErrNotSupported`. Vedere la [guida sull'aggiunta di un backend](adding-a-sid.md).

## Comportamento in chain cache

`ChainStore.DeleteByTag` propaga la chiamata a tutti i tier in ordine. Anche il backfill di `Get` propaga i tag al tier ripopolato, quindi `DeleteByTag` continua a funzionare anche su chiavi entrate in L1 tramite write-back.

```go title="main.go"
l1 := memory.NewStore()
l2 := memory.NewStore()
cache := xcache.New[User](xcache.NewChain(l1, l2))

_ = l2.Set(ctx, "u1", u, xcache.WithTags("users"))
_, _ = cache.Get(ctx, "u1")        // L1 ripopolato con tag "users"
_ = cache.DeleteByTag(ctx, "users") // rimosso da entrambi i tier
```

!!! warning "Non atomica tra tier"
    Come `Clear` e `Set`, `DeleteByTag` su `ChainStore` non è atomica: se il primo tier riesce e il secondo fallisce, il primo è già mutato. Per invalidazioni critiche prevedere retry o reconciliation lato applicativo.

## Quando preferire `DeleteByTag`

- Invalidazione dopo un cambio di schema o un'eliminazione massiva nel DB
- Cache busting per evento di dominio (es. "rilasciato un nuovo prodotto" → `DeleteByTag(ctx, "products")`)
- Test di integrazione: pulizia mirata di una porzione della cache senza azzerare lo store

Quando invece serve rimuovere chiavi note in anticipo, preferire `Delete` o `DeleteMany` — non hanno bisogno di un indice tag e sono più veloci.

---

*[TTL]: Time To Live
*[L1]: Layer 1 — cache veloce in memoria
*[L2]: Layer 2 — cache distribuita
