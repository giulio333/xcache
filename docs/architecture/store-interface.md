# Interfaccia Store

Contratto che ogni backend deve implementare. Rilevante per chi sviluppa un backend custom — non per chi usa la libreria.

!!! note "Unico punto di integrazione"
    Implementare `Store` è sufficiente per far funzionare il backend con tutta la libreria: chain cache, singleflight, generics non richiedono modifiche.

---

## Tipo `Entry`

Struttura restituita dai metodi di lettura. Contiene il valore insieme ai metadati di storage.

```go
type Entry struct {
    Value     any
    ExpiresAt time.Time // zero = nessuna scadenza
    Tags      []string
}
```

`RemainingTTL()` calcola il TTL residuo a partire da `ExpiresAt`; ritorna `0` se l'entry non ha scadenza o se la scadenza è già passata. Usato internamente dalla chain cache per propagare la scadenza durante il write-back su L1.

`Tags` viene popolato dai backend che mantengono un indice tag (vedi `MemoryStore`). La chain cache usa anche `Tags` per propagare le label durante il write-back, così `DeleteByTag` continua a funzionare sui tier ripopolati.

---

## Interfaccia `Store`

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

### `Get`

Ritorna un `Entry` con valore e metadati. Se la chiave non esiste o è scaduta, ritorna `ErrNotFound`.

### `GetMany`

Ritorna una mappa con gli `Entry` trovati. Le chiavi mancanti o scadute vengono omesse — nessun errore.

### `Set`

Scrive un valore. Accetta `Option` per TTL e tag. Se la chiave esisteva già con tag diversi, l'indice tag viene aggiornato per non lasciare riferimenti stale.

### `Delete`

Rimuove una chiave. Non ritorna errore se la chiave non esiste.

### `DeleteMany`

Rimuove più chiavi in una sola chiamata. I backend possono ottimizzare (es. pipeline Redis); un'implementazione di default a loop su `Delete` è accettabile.

### `DeleteByTag`

Rimuove tutte le chiavi associate al tag. Backend che non mantengono un indice tag devono ritornare `ErrNotSupported`.

### `Clear`

Rimuove tutte le chiavi dallo store, indice tag incluso.

### `Close`

Obbligatorio: ferma goroutine background e chiude connessioni. Da chiamare sempre con `defer`.

---

## Errori sentinella

| Errore | Quando |
|---|---|
| `ErrNotFound` | `Get` su chiave assente o scaduta |
| `ErrNotSupported` | Operazione opzionale (es. `DeleteByTag`) non implementata dal backend |

Entrambi vanno controllati con `errors.Is`.

---

## Contratti da rispettare

| Contratto | Note |
|---|---|
| `Get` restituisce `ErrNotFound` | Mai `Entry{}, nil` per chiavi mancanti |
| `Get` popola `Entry.ExpiresAt` e `Entry.Tags` | Necessario per propagare TTL e tag nella chain cache |
| `GetMany` omette le chiavi mancanti | La mappa risultante ha solo le chiavi trovate |
| `Set` rispetta il TTL | Se `opts.TTL > 0`, la chiave deve scadere |
| `Set` aggiorna l'indice tag su overwrite | Un ulteriore `Set` con tag diversi deve detach la chiave dai tag vecchi |
| `DeleteByTag` non ritorna errore su tag inesistenti | È un no-op |
| `Close` libera le risorse | Connessioni, goroutine background |

---

Per un esempio completo di implementazione, vedere la [guida all'aggiunta di un backend](../guides/adding-a-sid.md).

Le opzioni di costruzione per `Cache[T]` (come `WithPrefix`) sono separate dalle `Option` per `Set` — vedere [Meccanismi interni § CacheOption e WithPrefix](core.md#cacheoption-e-withprefix).

---

*[TTL]: Time To Live
