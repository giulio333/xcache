# Getting started

## Prerequisiti

Go
:   Versione 1.21 o superiore. Scaricabile da [go.dev/dl](https://go.dev/dl).

## Installazione

```bash
go get github.com/giulio333/xcache
```

## Uso base — memoria locale

```go title="main.go"
package main

import (
    "context"
    "errors"
    "fmt"
    "time"

    "github.com/giulio333/xcache"
    "github.com/giulio333/xcache/store/memory"
)

type User struct {
    ID   int
    Name string
}

func main() {
    store := memory.NewStore()
    defer store.Close()

    cache := xcache.New[User](store)
    ctx   := context.Background()

    // Scrittura
    cache.Set(ctx, "user:1", User{ID: 1, Name: "Alice"}, xcache.WithTTL(5*time.Minute))

    // Lettura
    user, err := cache.Get(ctx, "user:1")
    if errors.Is(err, xcache.ErrNotFound) {
        fmt.Println("non trovato")
        return
    }
    fmt.Println(user.Name) // Alice
}
```

## Uso con `GetOrLoad`

Preferire `GetOrLoad` a Get+Set manuale: gestisce la cache stampede automaticamente.

```go title="main.go"
user, err := cache.GetOrLoad(ctx, "user:1", func(ctx context.Context) (User, error) {
    return db.FindUserByID(1) // chiamato una sola volta anche sotto carico concorrente
}, xcache.WithTTL(5*time.Minute))
```

Il `loader` riceve lo stesso `ctx` passato a `GetOrLoad` e partecipa quindi a cancellation e deadline.

## Operazioni batch

```go title="main.go"
// Lettura multipla
users, err := cache.GetMany(ctx, []string{"user:1", "user:2", "user:3"})
// users è map[string]User — le chiavi mancanti vengono omesse

// Cancellazione multipla
cache.DeleteMany(ctx, []string{"user:1", "user:2"})
```

## Invalidazione di gruppo (tag)

Associando un tag alla chiave al momento della scrittura è possibile invalidare insiemi correlati di chiavi in un'unica chiamata.

```go title="main.go"
cache.Set(ctx, "user:1", u1, xcache.WithTags("users"))
cache.Set(ctx, "user:2", u2, xcache.WithTags("users", "admin"))
cache.Set(ctx, "product:1", p1, xcache.WithTags("products"))

// Drop di tutti gli utenti, ma non dei prodotti
cache.DeleteByTag(ctx, "users")
```

!!! note "Sovrascrittura coerente"
    Un `Set` successivo con tag diversi ri-aggancia la chiave ai nuovi tag e la stacca dai vecchi. L'indice tag rimane sempre coerente con il valore corrente.

## Svuotare la cache

```go title="main.go"
cache.Clear(ctx)
```

---

*[TTL]: Time To Live
