# Store

Interfaccia che ogni backend deve implementare. Rilevante per chi sviluppa un backend custom — non per chi usa la libreria tramite `Cache[T]`.

---

## Interfaccia

```go
type Store interface {
    Get(ctx context.Context, key string) (Entry, error)
    GetMany(ctx context.Context, keys []string) (map[string]Entry, error)
    Set(ctx context.Context, key string, value any, opts ...Option) error
    Delete(ctx context.Context, key string) error
    DeleteMany(ctx context.Context, keys []string) error
    DeleteByTag(ctx context.Context, tag string) error
    Clear(ctx context.Context) error
    Close() error
}
```

---

## Metodi

### Get

```go
Get(ctx context.Context, key string) (Entry, error)
```

Ritorna l'`Entry` associata alla chiave, completa di valore e metadati. Ritorna `ErrNotFound` se la chiave è assente o scaduta. Non ritornare mai `Entry{}, nil` per chiavi mancanti.

L'`Entry` deve avere `ExpiresAt` e `Tags` popolati correttamente: la chain cache li usa per propagare TTL e tag durante il backfill su L1.

### GetMany

```go
GetMany(ctx context.Context, keys []string) (map[string]Entry, error)
```

Ritorna solo le chiavi trovate. Le chiavi assenti o scadute vengono omesse dalla mappa. Nessun errore per le chiavi mancanti.

### Set

```go
Set(ctx context.Context, key string, value any, opts ...Option) error
```

Scrive un valore. Se `opts` include `WithTTL(d)` con `d > 0`, la chiave deve scadere dopo `d`. Su sovrascrittura, se il backend mantiene un indice tag, aggiornarlo: rimuovere la chiave dai tag vecchi prima di aggiungerla ai nuovi.

### Delete

```go
Delete(ctx context.Context, key string) error
```

Rimuove la chiave. No-op se la chiave non esiste, senza errore.

### DeleteMany

```go
DeleteMany(ctx context.Context, keys []string) error
```

Rimuove più chiavi. I backend possono ottimizzare internamente (es. pipeline Redis). Un'implementazione a loop su `Delete` è accettabile come fallback.

### DeleteByTag

```go
DeleteByTag(ctx context.Context, tag string) error
```

Rimuove tutte le chiavi associate al tag. No-op se il tag non esiste. I backend senza indice tag devono ritornare `ErrNotSupported`, non `nil`.

### Clear

```go
Clear(ctx context.Context) error
```

Rimuove tutte le chiavi e svuota l'indice tag.

### Close

```go
Close() error
```

Ferma goroutine background e chiude connessioni. Da chiamare sempre con `defer`. Può essere chiamato esattamente una volta.

---

## Entry

Struttura restituita dai metodi di lettura.

```go
type Entry struct {
    Value     any
    ExpiresAt time.Time // zero = nessuna scadenza
    Tags      []string
}
```

### RemainingTTL

```go
func (e Entry) RemainingTTL() time.Duration
```

Ritorna il TTL residuo calcolato da `ExpiresAt`. Ritorna `0` se l'entry non ha scadenza o è già scaduta. Usato dalla chain cache per propagare la scadenza corretta durante il backfill, evitando che L1 viva oltre L2.

---

## Contratti

| Contratto | Note |
|---|---|
| `Get` ritorna `ErrNotFound` per chiavi mancanti | Mai `Entry{}, nil` |
| `Get` popola `ExpiresAt` e `Tags` | Necessari per la propagazione nella chain cache |
| `GetMany` omette le chiavi mancanti | Nessun errore per le chiavi assenti |
| `Set` rispetta il TTL | La chiave deve scadere se `opts.TTL > 0` |
| `Set` aggiorna l'indice tag su sovrascrittura | Nessun riferimento stale |
| `DeleteByTag` su tag inesistente ritorna `nil` | No-op idempotente |
| `DeleteByTag` non supportato ritorna `ErrNotSupported` | Non `nil` |
| `Close` libera le risorse | Connessioni, goroutine background |

---

*[TTL]: Time To Live
*[L1]: Layer 1 — cache veloce in memoria
*[L2]: Layer 2 — cache distribuita
