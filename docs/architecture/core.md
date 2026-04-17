# API pubblica

Documentazione delle interfacce e funzioni esposte al codice utente.

---

## Interfaccia `Store`

Contratto che ogni backend deve implementare. Lavora con `any` — la type-safety è responsabilità di `Cache[T]`.

```go
type Store interface {
    Get(ctx context.Context, key string) (any, error)
    GetMany(ctx context.Context, keys []string) (map[string]any, error)
    Set(ctx context.Context, key string, value any, opts ...Option) error
    Delete(ctx context.Context, key string) error
    DeleteMany(ctx context.Context, keys []string) error
    Clear(ctx context.Context) error
    Close() error
}
```

| Metodo | Note |
|---|---|
| `Get` | Ritorna `ErrNotFound` se la chiave manca o è scaduta |
| `GetMany` | Le chiavi mancanti vengono omesse dalla mappa risultante — nessun errore |
| `Set` | Accetta `Option` (TTL, tags) |
| `Close` | Obbligatorio: ferma goroutine background, chiude connessioni |

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
    GetOrLoad(ctx context.Context, key string, loader func(context.Context) (T, error), opts ...Option) (T, error)
}
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

| Option | Comportamento |
|---|---|
| `WithTTL(0)` | Nessuna scadenza (default) |
| `WithTTL(5*time.Minute)` | Chiave scade dopo 5 minuti |
| `WithTags("users", "admin")` | Associa tag per invalidazione di gruppo |

---

*[TTL]: Time To Live
