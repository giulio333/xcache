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
    Clear(ctx context.Context) error
    GetOrLoad(ctx context.Context, key string, loader func(context.Context) (T, error), opts ...Option) (T, error)
}
```

### `Get`

Ritorna il valore tipizzato associato alla chiave. Se la chiave non esiste o è scaduta, ritorna `ErrNotFound`.

```go
user, err := cache.Get(ctx, "user:1")
if errors.Is(err, xcache.ErrNotFound) {
    // chiave assente o scaduta
}
```

### `GetMany`

Ritorna una mappa `map[string]T` con i valori trovati. Le chiavi mancanti o scadute vengono omesse.

```go
users, err := cache.GetMany(ctx, []string{"user:1", "user:2", "user:3"})
// users contiene solo le chiavi presenti in cache
```

### `Set`

Scrive un valore tipizzato. Accetta `Option` per TTL e tag.

```go
cache.Set(ctx, "user:1", user, xcache.WithTTL(5*time.Minute))
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

---

## `ErrNotFound`

```go
var ErrNotFound = errors.New("xcache: key not found")
```

Ritornato da `Get` quando la chiave non esiste o il TTL è scaduto. Usare sempre `errors.Is`:

```go
_, err := cache.Get(ctx, "key")
if errors.Is(err, xcache.ErrNotFound) {
    // chiave assente
}
```

---

## `New[T]`

```go
func New[T any](store Store) Cache[T]
```

Crea un'istanza di `Cache[T]` che usa `store` come backend. Lo stesso store può essere condiviso tra cache di tipi diversi:

```go
store := memory.NewStore()
users    := xcache.New[User](store)
products := xcache.New[Product](store)
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

Associa uno o più tag alla chiave per invalidazione di gruppo.

```go
xcache.WithTags("users", "admin")
```

---

*[TTL]: Time To Live
