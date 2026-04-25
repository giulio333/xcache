# Middleware

Tutti i middleware decorano uno `Store` esistente con la firma `Wrap(store Store, ...) Store` e si compongono per annidamento. Il middleware più esterno viene eseguito per primo.

```go
store := logging.Wrap(
    timeout.Wrap(
        metrics.Wrap(memory.NewStore(), recorder),
        50*time.Millisecond,
    ),
    logger,
)
```

---

## logging

```go
import "github.com/giulio333/xcache/middleware/logging"

func Wrap(store Store, logger *slog.Logger) Store
```

Emette un log strutturato `log/slog` per ogni operazione con i campi `op`, `key` (o `keys` per le operazioni batch) e `duration`. Passare `nil` usa `slog.Default()`.

| Caso | Livello |
|---|---|
| Operazione riuscita | `DEBUG` |
| `ErrNotFound` | `DEBUG` |
| `ErrNotSupported` | `DEBUG` |
| Altro errore | `ERROR` |

`ErrNotFound` viene loggato a `DEBUG` per non inquinare i log in produzione: un miss è comportamento atteso, non un errore.

```go title="esempio"
store := logging.Wrap(memory.NewStore(), slog.Default())
```

---

## readonly

```go
import "github.com/giulio333/xcache/middleware/readonly"

func Wrap(store Store) Store

var ErrReadOnly error
```

Blocca tutte le operazioni di scrittura e ritorna `ErrReadOnly`. Le letture vengono delegate al backend sottostante invariate.

| Operazione | Comportamento |
|---|---|
| `Get`, `GetMany` | Delegata al backend |
| `Set` | Ritorna `ErrReadOnly` |
| `Delete`, `DeleteMany` | Ritorna `ErrReadOnly` |
| `DeleteByTag` | Ritorna `ErrReadOnly` |
| `Clear` | Ritorna `ErrReadOnly` |
| `Close` | Delegata al backend |

Usare `errors.Is` per distinguere `ErrReadOnly` da altri errori:

```go title="esempio"
err := cache.Set(ctx, "key", value)
if errors.Is(err, readonly.ErrReadOnly) {
    // scrittura rifiutata
}
```

---

## timeout

```go
import "github.com/giulio333/xcache/middleware/timeout"

func Wrap(store Store, d time.Duration) Store
```

Avvolge ogni operazione con `context.WithTimeout(ctx, d)`. Il deadline effettivo è il minimo tra quello del chiamante e `d` — se il chiamante ha già un deadline più stretto, è quello a scattare.

`d <= 0` è un no-op: `Wrap` ritorna lo store originale invariato. Utile per disabilitare il timeout in sviluppo senza modificare il codice di composizione.

`Close` non riceve un timeout — viene sempre delegato direttamente.

L'errore in caso di scadenza è `context.DeadlineExceeded` dalla libreria standard, senza wrapper aggiuntivi.

```go title="esempio"
store := timeout.Wrap(memory.NewStore(), 50*time.Millisecond)

_, err := cache.Get(ctx, "key")
if errors.Is(err, context.DeadlineExceeded) {
    // l'operazione ha superato il deadline
}
```

---

## metrics

```go
import "github.com/giulio333/xcache/middleware/metrics"

func Wrap(store Store, rec Recorder) Store
```

Registra hit, miss, errori e durata per ogni operazione tramite l'interfaccia `Recorder`. Il package non porta dipendenze esterne: il chiamante fornisce l'implementazione per il sistema di metriche che preferisce (Prometheus, StatsD, DataDog, ecc.).

### Interfaccia Recorder

```go
type Recorder interface {
    RecordHit(op string)
    RecordMiss(op string)
    RecordError(op string)
    RecordDuration(op string, d time.Duration)
}
```

`op` è il nome dell'operazione: `"Get"`, `"GetMany"`, `"Set"`, `"Delete"`, `"DeleteMany"`, `"DeleteByTag"`, `"Clear"`, `"Close"`.

| Evento | Quando |
|---|---|
| `RecordHit` | Operazione completata senza errore |
| `RecordMiss` | Solo per letture che ritornano `ErrNotFound` |
| `RecordError` | Qualsiasi altro errore |
| `RecordDuration` | Sempre, indipendentemente dall'esito |

### Implementazione Prometheus

```go title="esempio"
type PrometheusRecorder struct {
    hits      *prometheus.CounterVec
    misses    *prometheus.CounterVec
    errors    *prometheus.CounterVec
    durations *prometheus.HistogramVec
}

func (r *PrometheusRecorder) RecordHit(op string)      { r.hits.WithLabelValues(op).Inc() }
func (r *PrometheusRecorder) RecordMiss(op string)     { r.misses.WithLabelValues(op).Inc() }
func (r *PrometheusRecorder) RecordError(op string)    { r.errors.WithLabelValues(op).Inc() }
func (r *PrometheusRecorder) RecordDuration(op string, d time.Duration) {
    r.durations.WithLabelValues(op).Observe(d.Seconds())
}
```

---

*[TTL]: Time To Live
