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
    if err != nil {
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

## Operazioni batch

```go title="main.go"
// Lettura multipla
users, err := cache.GetMany(ctx, []string{"user:1", "user:2", "user:3"})
// users è map[string]User — le chiavi mancanti vengono omesse

// Cancellazione multipla
cache.DeleteMany(ctx, []string{"user:1", "user:2"})
```

## Svuotare la cache

```go title="main.go"
cache.Clear(ctx)
```

---

*[TTL]: Time To Live
