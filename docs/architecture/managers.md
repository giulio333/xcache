# API pubblica

Documentazione delle interfacce e funzioni esposte all'utente finale della libreria.

---

## Interfaccia `Cache[T]`

API generica esposta all'utente. Nessun type assertion richiesto.

```go
type Cache[T any] interface {
    Get(ctx context.Context, key string) (T, error)
    GetMany(ctx context.Context, keys []string) (map[string]T, error)
    Set(ctx context.Context, key string, value T, opts ...Option) error
    Delete(ctx context.Context, key string) error
    DeleteMany(ctx context.Context, keys []string) error
    DeleteByTag(ctx context.Context, tag string) error
    Clear(ctx context.Context) error
    GetOrLoad(ctx context.Context, key string, loader func(context.Context) (T, error), opts ...Option) (T, error)
}
```

### `Get`

Ritorna il valore tipizzato associato alla chiave. Se la chiave non esiste o è scaduta, ritorna `ErrNotFound`. Se il valore presente nello store non è assegnabile a `T` ritorna un errore tipizzato (sintomo che lo stesso `Store` viene condiviso da `Cache[T]` con namespace di chiave sovrapposti).

```go
user, err := cache.Get(ctx, "user:1")
if errors.Is(err, xcache.ErrNotFound) {
    // chiave assente o scaduta
}
```

### `GetMany`

Ritorna una mappa `map[string]T` con i valori trovati. Le chiavi mancanti o scadute vengono omesse. Un type mismatch su una qualsiasi entry fa fallire l'intera chiamata.

```go
users, err := cache.GetMany(ctx, []string{"user:1", "user:2", "user:3"})
// users contiene solo le chiavi presenti in cache
```

### `Set`

Scrive un valore tipizzato. Accetta `Option` per TTL e tag.

```go
cache.Set(ctx, "user:1", user,
    xcache.WithTTL(5*time.Minute),
    xcache.WithTags("users"),
)
```

### `Delete`

Rimuove una chiave. Non ritorna errore se la chiave non esiste.

```go
cache.Delete(ctx, "user:1")
```

### `DeleteMany`

Rimuove più chiavi in una sola chiamata.

```go
cache.DeleteMany(ctx, []string{"user:1", "user:2"})
```

### `DeleteByTag`

Rimuove tutte le chiavi che erano state scritte con il tag indicato. È il modo standard per fare invalidazione di gruppo (es. "drop tutte le entries `users` dopo un cambio schema").

```go
cache.Set(ctx, "user:1", u, xcache.WithTags("users"))
cache.Set(ctx, "user:2", u, xcache.WithTags("users", "admin"))
cache.DeleteByTag(ctx, "users") // rimuove sia user:1 che user:2
```

!!! note "Atomicità su ChainStore"
    `DeleteByTag` su `ChainStore` propaga a tutti i tier in ordine, fermandosi al primo errore. La stessa limitazione vale per `Clear` e `Set`: se l'operazione fallisce su L2, L1 risulta già mutato.

### `Clear`

Rimuove tutte le chiavi dalla cache.

```go
cache.Clear(ctx)
```

### `GetOrLoad`

Combina Get + Set con protezione dalla **cache stampede**:

1. Se la chiave esiste in cache → la ritorna immediatamente
2. Se manca → esegue `loader` una sola volta, anche sotto carico concorrente (singleflight)
3. Salva il risultato in cache con le `opts` fornite

```go
user, err := cache.GetOrLoad(ctx, "user:1", func(ctx context.Context) (User, error) {
    return db.FindUserByID(1)  // chiamato una sola volta anche con 10.000 richieste concorrenti
}, xcache.WithTTL(5*time.Minute))
```

`loader` riceve lo stesso `ctx` passato a `GetOrLoad`, partecipando a cancellation e deadline.

---

## Errori sentinella

```go
var ErrNotFound      = errors.New("xcache: key not found")
var ErrNotSupported  = errors.New("xcache: operation not supported by this backend")
```

`ErrNotFound` viene ritornato da `Get` per chiavi assenti o scadute. `ErrNotSupported` segnala che il backend corrente non implementa un'operazione opzionale (es. `DeleteByTag` su un Redis store senza indice tag). Usare sempre `errors.Is`:

```go
_, err := cache.Get(ctx, "key")
if errors.Is(err, xcache.ErrNotFound) {
    // chiave assente
}

if err := cache.DeleteByTag(ctx, "users"); errors.Is(err, xcache.ErrNotSupported) {
    // backend senza indice tag — fallback a strategia diversa
}
```

---

## `New[T]`

```go
func New[T any](store Store, opts ...CacheOption) Cache[T]
```

Crea un'istanza di `Cache[T]` che usa `store` come backend. Lo stesso store può essere condiviso tra cache di tipi diversi. Le `CacheOption` (come `WithPrefix`) configurano la cache a tempo di costruzione:

```go
store := memory.NewStore()
users    := xcache.New[User](store, xcache.WithPrefix("u:"))
products := xcache.New[Product](store, xcache.WithPrefix("p:"))
```

---

## `NewChain`

```go
func NewChain(stores ...Store) *ChainStore
```

Crea uno store a cascata. Vedere la [guida sulla chain cache](../guides/chain-cache.md).

---

## Options

```go
func WithTTL(d time.Duration) Option
func WithTags(tags ...string) Option
```

### `WithTTL`

Imposta la scadenza della chiave. `WithTTL(0)` equivale a nessuna scadenza (default).

```go
xcache.WithTTL(5 * time.Minute)
```

### `WithTags`

Associa uno o più tag alla chiave per invalidazione di gruppo. I tag vengono mantenuti nell'`Entry` e sono usati da `DeleteByTag`.

```go
xcache.WithTags("users", "admin")
```

!!! tip "Tag e overwrite"
    Un `Set` successivo con tag diversi ri-aggancia la chiave ai nuovi tag e la stacca da quelli vecchi. L'indice tag rimane sempre coerente con il valore corrente.

---

*[TTL]: Time To Live
