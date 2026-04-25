# Cache[T]

Interfaccia generica esposta all'applicazione. Nessun type assertion richiesto.

---

## Costruzione

```go
func New[T any](store Store, opts ...CacheOption) Cache[T]
```

Crea un'istanza `Cache[T]` che usa `store` come backend. Lo stesso store può essere condiviso tra cache di tipi diversi usando `WithPrefix`.

```go
store := memory.NewStore()
defer store.Close()

userCache    := xcache.New[User](store, xcache.WithPrefix("u:"))
productCache := xcache.New[Product](store, xcache.WithPrefix("p:"))
```

---

## Interfaccia

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

---

## Metodi

### Get

```go
Get(ctx context.Context, key string) (T, error)
```

Ritorna il valore associato alla chiave. Ritorna `ErrNotFound` se la chiave è assente o scaduta.

```go title="esempio"
user, err := cache.Get(ctx, "user:1")
if errors.Is(err, xcache.ErrNotFound) {
    // miss — ricaricare dal DB
}
```

### GetMany

```go
GetMany(ctx context.Context, keys []string) (map[string]T, error)
```

Ritorna una mappa con i valori trovati. Le chiavi assenti o scadute vengono omesse senza errore. Se il prefisso è configurato, le chiavi nella mappa restituita hanno il prefisso rimosso.

```go title="esempio"
users, err := cache.GetMany(ctx, []string{"user:1", "user:2", "user:3"})
// users = {"user:1": ..., "user:3": ...}  → "user:2" era assente
```

### Set

```go
Set(ctx context.Context, key string, value T, opts ...Option) error
```

Scrive un valore. Accetta `WithTTL` e `WithTags`. Su sovrascrittura di una chiave esistente, l'indice tag viene aggiornato: la chiave viene staccata dai tag vecchi e agganciata ai nuovi.

```go title="esempio"
cache.Set(ctx, "user:1", user,
    xcache.WithTTL(10*time.Minute),
    xcache.WithTags("users"),
)
```

### Delete

```go
Delete(ctx context.Context, key string) error
```

Rimuove la chiave. No-op se la chiave non esiste, senza errore.

### DeleteMany

```go
DeleteMany(ctx context.Context, keys []string) error
```

Rimuove più chiavi in una sola chiamata. I backend possono ottimizzare l'operazione internamente (es. pipeline Redis).

```go title="esempio"
cache.DeleteMany(ctx, []string{"user:1", "user:2", "user:3"})
```

### DeleteByTag

```go
DeleteByTag(ctx context.Context, tag string) error
```

Rimuove tutte le chiavi associate al tag. No-op se il tag non esiste. Ritorna `ErrNotSupported` se il backend non mantiene un indice tag.

```go title="esempio"
cache.DeleteByTag(ctx, "users")
```

### Clear

```go
Clear(ctx context.Context) error
```

Rimuove tutte le chiavi dalla cache, indice tag incluso.

### GetOrLoad

```go
GetOrLoad(ctx context.Context, key string, loader func(context.Context) (T, error), opts ...Option) (T, error)
```

Ritorna il valore dalla cache se presente. Se la chiave è assente, esegue `loader`, salva il risultato con le `opts` indicate e lo ritorna. Garantisce che `loader` venga chiamato al più una volta per chiave, anche sotto carico concorrente (singleflight).

`loader` riceve lo stesso `ctx` passato a `GetOrLoad`.

```go title="esempio"
user, err := cache.GetOrLoad(ctx, "user:1", func(ctx context.Context) (User, error) {
    return db.FindUser(1)
}, xcache.WithTTL(10*time.Minute), xcache.WithTags("users"))
```

---

## Opzioni per Set e GetOrLoad

### WithTTL

```go
func WithTTL(d time.Duration) Option
```

Imposta la scadenza dell'entry. `WithTTL(0)` equivale a nessuna scadenza (default).

### WithTags

```go
func WithTags(tags ...string) Option
```

Associa uno o più tag all'entry per l'invalidazione di gruppo. Un `Set` successivo con tag diversi aggiorna l'indice automaticamente.

---

## Opzioni di costruzione

### WithPrefix

```go
func WithPrefix(prefix string) CacheOption
```

Antepone una stringa fissa a ogni chiave prima che raggiunga lo store. Trasparente per il chiamante: `Get("1")` con prefisso `"users:"` legge la chiave `"users:1"`. Le chiavi restituite da `GetMany` hanno il prefisso rimosso. Stringa vuota è un no-op.

---

## Errori sentinella

```go
var ErrNotFound     error  // chiave assente o scaduta
var ErrNotSupported error  // operazione non supportata dal backend
```

Usare sempre `errors.Is`:

```go title="esempio"
_, err := cache.Get(ctx, "key")
if errors.Is(err, xcache.ErrNotFound) { ... }

err = cache.DeleteByTag(ctx, "users")
if errors.Is(err, xcache.ErrNotSupported) { ... }
```

---

*[TTL]: Time To Live
