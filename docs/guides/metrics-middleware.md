# Metrics middleware

`metrics.Wrap` decora qualsiasi `Store` registrando hit, miss, errori e durata di ogni operazione tramite un'interfaccia `Recorder` fornita dal chiamante. Il package non porta dipendenze esterne: il chiamante decide quale sistema di metriche usare (Prometheus, StatsD, DataDog, ecc.).

## Interfaccia Recorder

```go title="middleware/metrics/middleware.go"
type Recorder interface {
    RecordHit(op string)
    RecordMiss(op string)
    RecordError(op string)
    RecordDuration(op string, d time.Duration)
}
```

`op` è il nome dell'operazione: `"Get"`, `"Set"`, `"Delete"`, `"DeleteMany"`, `"DeleteByTag"`, `"Clear"`, `"Close"`, `"GetMany"`.

Semantica degli eventi:

| Evento | Quando viene emesso |
|---|---|
| `RecordHit` | Operazione completata senza errore |
| `RecordMiss` | Solo per operazioni di lettura che restituiscono `ErrNotFound` |
| `RecordError` | Qualsiasi altro errore |
| `RecordDuration` | Sempre, indipendentemente dall'esito |

## Setup

```go title="main.go"
import (
    "github.com/giulio333/xcache"
    "github.com/giulio333/xcache/middleware/metrics"
    "github.com/giulio333/xcache/store/memory"
)

store := metrics.Wrap(memory.NewStore(), myRecorder)
defer store.Close()

cache := xcache.New[Product](store)
```

## Implementazione con Prometheus

```go title="recorder/prometheus.go"
import (
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

type PrometheusRecorder struct {
    hits      *prometheus.CounterVec
    misses    *prometheus.CounterVec
    errors    *prometheus.CounterVec
    durations *prometheus.HistogramVec
}

func NewPrometheusRecorder(reg prometheus.Registerer) *PrometheusRecorder {
    factory := promauto.With(reg)
    return &PrometheusRecorder{
        hits: factory.NewCounterVec(prometheus.CounterOpts{
            Name: "xcache_hits_total",
            Help: "Number of successful cache operations.",
        }, []string{"op"}),
        misses: factory.NewCounterVec(prometheus.CounterOpts{
            Name: "xcache_misses_total",
            Help: "Number of cache misses (ErrNotFound).",
        }, []string{"op"}),
        errors: factory.NewCounterVec(prometheus.CounterOpts{
            Name: "xcache_errors_total",
            Help: "Number of cache operations that returned an error.",
        }, []string{"op"}),
        durations: factory.NewHistogramVec(prometheus.HistogramOpts{
            Name:    "xcache_duration_seconds",
            Help:    "Latency of cache operations in seconds.",
            Buckets: prometheus.DefBuckets,
        }, []string{"op"}),
    }
}

func (r *PrometheusRecorder) RecordHit(op string) {
    r.hits.WithLabelValues(op).Inc()
}
func (r *PrometheusRecorder) RecordMiss(op string) {
    r.misses.WithLabelValues(op).Inc()
}
func (r *PrometheusRecorder) RecordError(op string) {
    r.errors.WithLabelValues(op).Inc()
}
func (r *PrometheusRecorder) RecordDuration(op string, d time.Duration) {
    r.durations.WithLabelValues(op).Observe(d.Seconds())
}
```

## Composizione con altri middleware

Il middleware si aggancia a livello `Store` e si compone per annidamento.

```go title="main.go"
// Logging + metrics: ogni operazione viene loggata e misurata
store := logging.Wrap(
    metrics.Wrap(memory.NewStore(), rec),
    logger,
)
```

```go title="main.go"
// Metrics + readonly: le operazioni bloccate vengono registrate come errori
store := metrics.Wrap(
    readonly.Wrap(memory.NewStore()),
    rec,
)
```

!!! note
    Applicando `metrics.Wrap` sopra `readonly.Wrap`, ogni tentativo di scrittura rifiutato da `ErrReadOnly` viene conteggiato come errore in `RecordError`. Invertire l'ordine se si vuole misurare solo le operazioni che raggiungono il backend.

---

*[TTL]: Time To Live
