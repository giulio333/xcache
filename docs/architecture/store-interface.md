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

`RemainingTTL()` calcola il TTL residuo a partire da `ExpiresAt`; ritorna `0` se l'entry non ha scadenza. Usato internamente dalla chain cache per propagare la scadenza durante il write-back su L1.

---

## Interfaccia `Store`

```go
type Store interface {
    Get(ctx context.Context, key string) (Entry, error)
    GetMany(ctx context.Context, keys []string) (map[string]Entry, error)
    Set(ctx context.Context, key string, value any, opts ...Option) error
    Delete(ctx context.Context, key string) error
    DeleteMany(ctx context.Context, keys []string) error
    Clear(ctx context.Context) error
    Close() error
}
```

### `Get`

Ritorna un `Entry` con valore e metadati. Se la chiave non esiste o è scaduta, ritorna `ErrNotFound`.

### `GetMany`

Ritorna una mappa con gli `Entry` trovati. Le chiavi mancanti o scadute vengono omesse — nessun errore.

### `Set`

Scrive un valore. Accetta `Option` per TTL e tag.

### `Delete`

Rimuove una chiave. Non ritorna errore se la chiave non esiste.

### `DeleteMany`

Rimuove più chiavi in una sola chiamata.

### `Clear`

Rimuove tutte le chiavi dallo store.

### `Close`

Obbligatorio: ferma goroutine background e chiude connessioni. Da chiamare sempre con `defer`.

---

## Contratti da rispettare

| Contratto | Note |
|---|---|
| `Get` restituisce `ErrNotFound` | Mai `Entry{}, nil` per chiavi mancanti |
| `Get` popola `Entry.ExpiresAt` | Necessario per propagare il TTL nella chain cache |
| `GetMany` omette le chiavi mancanti | La mappa risultante ha solo le chiavi trovate |
| `Set` rispetta il TTL | Se `opts.TTL > 0`, la chiave deve scadere |
| `Close` libera le risorse | Connessioni, goroutine background |

---

Per un esempio completo di implementazione, vedere la [guida all'aggiunta di un backend](../guides/adding-a-sid.md).

---

*[TTL]: Time To Live
