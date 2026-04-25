# Timeout middleware

`timeout.Wrap` decora qualsiasi `Store` applicando un deadline fisso a ogni operazione che riceve un `context.Context`. Quando il backend impiega troppo, il context scade e la chiamata restituisce `context.DeadlineExceeded` — il comportamento standard di Go.

## Setup

```go title="main.go"
import (
    "time"

    "github.com/giulio333/xcache"
    "github.com/giulio333/xcache/middleware/timeout"
    "github.com/giulio333/xcache/store/memory"
)

base := memory.NewStore()
ts := timeout.Wrap(base, 50*time.Millisecond)
defer ts.Close()

cache := xcache.New[Product](ts)
```

## Comportamento per operazione

| Operazione | Comportamento |
|---|---|
| `Get`, `GetMany` | Context wrappato con `WithTimeout(ctx, d)` |
| `Set` | Context wrappato con `WithTimeout(ctx, d)` |
| `Delete`, `DeleteMany` | Context wrappato con `WithTimeout(ctx, d)` |
| `DeleteByTag` | Context wrappato con `WithTimeout(ctx, d)` |
| `Clear` | Context wrappato con `WithTimeout(ctx, d)` |
| `Close` | Delegato direttamente — nessun timeout |

## No-op con `d <= 0`

Passare una durata zero o negativa disabilita il middleware: `Wrap` restituisce lo store originale invariato.

```go title="main.go"
// Utile per disabilitare il timeout in sviluppo senza cambiare il codice.
store := timeout.Wrap(base, 0) // store == base
```

## Interazione con il deadline del chiamante

`context.WithTimeout` crea un context figlio il cui deadline è il **minimo** tra quello del padre e `d`. Se il chiamante ha già un deadline più stretto, è quello a scattare.

```go title="main.go"
// Il chiamante ha 1 ms, il middleware 50 ms: vince il chiamante.
ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
defer cancel()

_, err := cache.Get(ctx, "key")
if errors.Is(err, context.DeadlineExceeded) {
    // deadline del chiamante — non del middleware
}
```

!!! note
    Il middleware non introduce un sentinel di errore proprio. L'errore restituito in caso di scadenza è sempre `context.DeadlineExceeded`, propagato direttamente dal package `context` della libreria standard.

## Rilevare il timeout

```go title="main.go"
_, err := cache.Get(ctx, "product:42")
if errors.Is(err, context.DeadlineExceeded) {
    // l'operazione ha superato il deadline — decidere come fare fallback
}
```

## Caso d'uso: protezione da backend lento

Un pattern comune è avvolgere un RedisStore con il timeout middleware per garantire che un Redis degradato non blocchi i goroutine di serving oltre una soglia accettabile.

```go title="main.go"
redisStore, _ := redis.NewStore(redis.Config{Addr: "redis:6379"})

// Nessuna operazione Redis può bloccare più di 20 ms.
protected := timeout.Wrap(redisStore, 20*time.Millisecond)

// Aggiungere logging sopra il timeout per osservare i DeadlineExceeded.
observed := logging.Wrap(protected, slog.Default())

cache := xcache.New[Order](observed)
```

## Composizione con altri middleware

```go title="main.go"
store := logging.Wrap(
    timeout.Wrap(memory.NewStore(), 50*time.Millisecond),
    logger,
)
```

!!! warning
    Applicare `logging.Wrap` **sopra** `timeout.Wrap` fa sì che i `DeadlineExceeded` vengano loggati come errori. Se si vuole silenziare questi log in sviluppo, impostare `d <= 0` oppure invertire l'ordine dei decorator.

---

*[TTL]: Time To Live
